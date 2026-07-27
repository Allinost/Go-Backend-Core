// Package webhook 业务逻辑层 — 配置 CRUD + 消息发送核心流程

package webhook

import (
	"context"
	"database/sql"
	"strings"
	"sync"
	"time"

	"github.com/Allinost/go-backend-core/internal/modules/webhook/dedup"
	"github.com/Allinost/go-backend-core/internal/modules/webhook/limiter"
	"github.com/Allinost/go-backend-core/internal/modules/webhook/sender"
)

// Service Webhook 业务逻辑
type Service struct {
	repo          *Repository
	senderFactory *sender.Factory
	rateLimiter   limiter.Limiter
	deduper       *dedup.Deduper
}

// NewService 创建 Service
func NewService(repo *Repository, senderFactory *sender.Factory, rateLimiter limiter.Limiter, deduper *dedup.Deduper) *Service {
	return &Service{
		repo:          repo,
		senderFactory: senderFactory,
		rateLimiter:   rateLimiter,
		deduper:       deduper,
	}
}

// CreateConfig 创建 Webhook 配置（校验类型）
func (s *Service) CreateConfig(ctx context.Context, req *CreateConfigReq) (*WebhookConfig, error) {
	if !isValidType(req.Type) {
		return nil, NewWebhookError(ErrInvalidType)
	}
	return s.repo.CreateConfig(ctx, req)
}

// UpdateConfig 更新 Webhook 配置
func (s *Service) UpdateConfig(ctx context.Context, id string, req *UpdateConfigReq) (*WebhookConfig, error) {
	if req.Type != nil && !isValidType(*req.Type) {
		return nil, NewWebhookError(ErrInvalidType)
	}
	c, err := s.repo.UpdateConfig(ctx, id, req)
	if err == sql.ErrNoRows {
		return nil, NewWebhookError(ErrWebhookNotFound)
	}
	return c, err
}

// DeleteConfig 删除 Webhook 配置
func (s *Service) DeleteConfig(ctx context.Context, id string) error {
	return s.repo.DeleteConfig(ctx, id)
}

// UpdateStatus 更新状态（仅允许 active / paused / disabled）
func (s *Service) UpdateStatus(ctx context.Context, id, status string) error {
	if status != "active" && status != "paused" && status != "disabled" {
		return NewWebhookError(ErrInvalidStatus)
	}
	_, err := s.repo.GetConfig(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return NewWebhookError(ErrWebhookNotFound)
		}
		return err
	}
	return s.repo.UpdateStatus(ctx, id, status)
}

// GetConfig 获取配置详情
func (s *Service) GetConfig(ctx context.Context, id string) (*WebhookConfig, error) {
	c, err := s.repo.GetConfig(ctx, id)
	if err == sql.ErrNoRows {
		return nil, NewWebhookError(ErrWebhookNotFound)
	}
	return c, err
}

// ListConfigs 分页查询配置列表
func (s *Service) ListConfigs(ctx context.Context, req *ListConfigsReq) ([]WebhookConfig, bool, int, error) {
	return s.repo.ListConfigs(ctx, req)
}

// Send 消息发送核心流程：
// resolveWebhooks → 白名单匹配 → 限流检查 → 去重检查 → sender.Send → 写投递日志 → 更新统计
func (s *Service) Send(ctx context.Context, req *SendReq) (*SendResult, error) {
	webhooks, err := s.resolveWebhooks(ctx, req.WebhookIDs, req.Tags)
	if err != nil {
		return nil, err
	}
	if len(webhooks) == 0 {
		return nil, NewWebhookError(ErrNoWebhookTarget)
	}

	result := &SendResult{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 5)

	for _, wh := range webhooks {
		if wh.Status != "active" {
			continue
		}
		if !s.matchWhitelist(&wh, req.Content) {
			continue
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(wh WebhookConfig) {
			defer wg.Done()
			defer func() { <-sem }()
			whResult := s.sendToWebhook(ctx, &wh, req)
			mu.Lock()
			result.Results = append(result.Results, whResult)
			if whResult.Status == "success" {
				result.Success++
			} else {
				result.Failed++
			}
			mu.Unlock()
		}(wh)
	}
	wg.Wait()

	if result.Success == 0 && result.Failed > 0 {
		return result, NewWebhookError(ErrAllWebhooksFailed)
	}
	return result, nil
}

// TestSend 测试发送，向指定 Webhook 发送一条测试消息
func (s *Service) TestSend(ctx context.Context, webhookID, testMsg string) (*WebhookResult, error) {
	wh, err := s.repo.GetConfig(ctx, webhookID)
	if err != nil {
		return nil, NewWebhookError(ErrWebhookNotFound)
	}
	if testMsg == "" {
		testMsg = "这是一条测试消息，来自 Webhook 管理模块"
	}
	req := &SendReq{
		Source:  "webhook-test",
		Title:   "测试消息",
		Content: testMsg,
		MsgType: "text",
	}
	whResult := s.sendToWebhook(ctx, wh, req)
	return &whResult, nil
}

// sendToWebhook 执行单次消息发送：获取发送器 → 限流检查 → 去重检查 → 实际发送 → 写日志 → 更新统计
func (s *Service) sendToWebhook(ctx context.Context, wh *WebhookConfig, req *SendReq) WebhookResult {
	snd := s.senderFactory.Get(sender.WebhookType(wh.Type))
	if snd == nil {
		return WebhookResult{WebhookID: wh.ID, WebhookName: wh.Name, Status: "failed", Error: "不支持的 Webhook 类型: " + wh.Type}
	}

	if s.rateLimiter != nil {
		allowed, err := s.rateLimiter.Allow(ctx, wh.ID, wh.MaxPerMinute, wh.MaxPerHour)
		if err != nil || !allowed {
			reason := "rate_limited"
			if err != nil {
				reason = "rate_limit_error"
			}
			s.repo.SaveDeliveryLog(ctx, &DeliveryLog{
				WebhookID: wh.ID, Source: req.Source, Title: req.Title,
				Status: string(DeliveryRateLimited), ErrorMsg: reason,
			})
			return WebhookResult{WebhookID: wh.ID, WebhookName: wh.Name, Status: "rate_limited"}
		}
	}

	contentHash := sha256Hex(req.Title + req.Content)
	if s.deduper != nil && wh.DedupWindow > 0 {
		isDup, _ := s.deduper.IsDuplicate(ctx, wh.ID, contentHash, wh.DedupWindow)
		if isDup {
			s.repo.SaveDeliveryLog(ctx, &DeliveryLog{
				WebhookID: wh.ID, Source: req.Source, Title: req.Title,
				ContentHash: contentHash, Status: string(DeliveryDeduped),
			})
			return WebhookResult{WebhookID: wh.ID, WebhookName: wh.Name, Status: "deduped"}
		}
	}

	start := time.Now()
	scfg := &sender.Config{
		URL:           wh.URL,
		SigningKey:    wh.SigningKey,
		TimeoutMS:     wh.TimeoutMS,
		AuthType:      wh.AuthType,
		AuthValue:     wh.AuthValue,
		CustomHeaders: wh.CustomHeaders,
	}
	smsg := &sender.Message{
		Title:   req.Title,
		Content: req.Content,
		Source:  req.Source,
		MsgType: req.MsgType,
		Data:    req.Data,
	}
	respCode, respBody, err := snd.Send(ctx, scfg, smsg)
	duration := time.Since(start)

	status := "success"
	errMsg := ""
	if err != nil {
		status = "failed"
		errMsg = err.Error()
	}

	bodyStr := ""
	if respBody != "" {
		if len(respBody) > 1000 {
			bodyStr = respBody[:1000]
		} else {
			bodyStr = respBody
		}
	}

	reqBodyStr := ""
	if req.Content != "" {
		if len(req.Content) > 1000 {
			reqBodyStr = req.Content[:1000]
		} else {
			reqBodyStr = req.Content
		}
	}

	s.repo.SaveDeliveryLog(ctx, &DeliveryLog{
		WebhookID:    wh.ID,
		Source:       req.Source,
		Title:        req.Title,
		ContentHash:  contentHash,
		Status:       status,
		RequestBody:  &reqBodyStr,
		ResponseCode: respCode,
		ResponseBody: &bodyStr,
		ErrorMsg:     errMsg,
		DurationMS:   int(duration.Milliseconds()),
	})

	if status == "success" {
		s.repo.UpdateStats(ctx, wh.ID, true)
	} else {
		s.repo.UpdateStats(ctx, wh.ID, false)
		s.repo.UpdateLastError(ctx, wh.ID, errMsg)
	}

	return WebhookResult{
		WebhookID:   wh.ID,
		WebhookName: wh.Name,
		Status:      status,
		Error:       errMsg,
		DurationMS:  int(duration.Milliseconds()),
	}
}

// resolveWebhooks 解析发送目标：优先使用 IDs，其次 Tags，最后全部活跃配置
func (s *Service) resolveWebhooks(ctx context.Context, ids, tags []string) ([]WebhookConfig, error) {
	if len(ids) > 0 {
		return s.repo.FindByIDs(ctx, ids)
	}
	if len(tags) > 0 {
		return s.repo.FindByTags(ctx, tags)
	}
	return s.repo.FindActive(ctx)
}

// matchWhitelist 根据白名单关键字和匹配模式判断是否允许发送
// any 模式：只要命中任一关键字即可；all 模式：需全部关键字均命中
func (s *Service) matchWhitelist(wh *WebhookConfig, content string) bool {
	if len(wh.WhitelistKeywords) == 0 {
		return true
	}
	if wh.WhitelistMode == "" {
		wh.WhitelistMode = "any"
	}
	for _, kw := range wh.WhitelistKeywords {
		matched := strings.Contains(content, kw)
		if wh.WhitelistMode == "any" && matched {
			return true
		}
		if wh.WhitelistMode == "all" && !matched {
			return false
		}
	}
	return wh.WhitelistMode == "all"
}

// isValidType 校验 Webhook 类型是否合法
func isValidType(t string) bool {
	switch t {
	case "feishu", "dingtalk", "wecom", "github", "generic":
		return true
	}
	return false
}

// DeleteDeliveryLog 删除单条投递日志
func (s *Service) DeleteDeliveryLog(ctx context.Context, id int64) error {
	return s.repo.DeleteDeliveryLog(ctx, id)
}

// BatchDeleteDeliveryLogs 批量删除投递日志
func (s *Service) BatchDeleteDeliveryLogs(ctx context.Context, ids []int64) error {
	return s.repo.BatchDeleteDeliveryLogs(ctx, ids)
}