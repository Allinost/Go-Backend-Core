// Package shopchecker 商品价格监控业务逻辑层
package shopchecker

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"

	"github.com/Allinost/go-backend-core/internal/modules/webhook"
	"github.com/Allinost/go-backend-core/zzz/shopchecker/collector"
)

// Service 商品价格监控业务逻辑
type Service struct {
	repo     *Repository
	whSvc    *webhook.Service
	registry *collector.Registry
}

// NewService 创建 Service
func NewService(repo *Repository, whSvc *webhook.Service, registry *collector.Registry) *Service {
	return &Service{
		repo:     repo,
		whSvc:    whSvc,
		registry: registry,
	}
}

// CreateProduct 创建商品监控（校验平台/URL/间隔，计算下次检查时间）
func (s *Service) CreateProduct(ctx context.Context, req *CreateProductReq) (*Product, error) {
	if req.Platform != string(PlatformJD) &&
		req.Platform != string(PlatformTaobao) &&
		req.Platform != string(PlatformPDD) {
		return nil, NewShopCheckerError(ErrInvalidPlatform)
	}
	if req.URL == "" {
		return nil, NewShopCheckerError(ErrInvalidURL)
	}
	if req.CheckInterval == "" {
		req.CheckInterval = "1h"
	}
	if _, err := time.ParseDuration(req.CheckInterval); err != nil {
		return nil, NewShopCheckerError(ErrInvalidInterval)
	}
	if req.CronExpr != "" {
		if _, err := cron.ParseStandard(req.CronExpr); err != nil {
			return nil, NewShopCheckerError(ErrInvalidCronExpr)
		}
	}

	now := time.Now()
	nextCheck := s.computeNextCheck(now, req.CheckInterval, req.CronExpr)

	p := &Product{
		ID:            uuid.New().String(),
		Name:          req.Name,
		Platform:      req.Platform,
		URL:           req.URL,
		ImageURL:      req.ImageURL,
		CheckInterval: req.CheckInterval,
		CronExpr:      req.CronExpr,
		Enabled:       true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if req.TargetPrice != nil {
		p.TargetPrice = *req.TargetPrice
	}
	if req.PriceDropRatio != nil {
		p.PriceDropRatio = *req.PriceDropRatio
	}
	if req.CustomWebhookIDs != nil {
		p.CustomWebhookIDs = req.CustomWebhookIDs
	}
	if req.Remark != nil {
		p.Remark = req.Remark
	}
	p.NextCheckAt = &nextCheck

	return s.repo.CreateProduct(ctx, p)
}

// GetProduct 获取商品详情
func (s *Service) GetProduct(ctx context.Context, id string) (*Product, error) {
	p, err := s.repo.GetProduct(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, NewShopCheckerError(ErrProductNotFound)
	}
	return p, err
}

// UpdateProduct 更新商品监控配置
func (s *Service) UpdateProduct(ctx context.Context, id string, req *UpdateProductReq) (*Product, error) {
	if req.CheckInterval != nil && *req.CheckInterval != "" {
		if _, err := time.ParseDuration(*req.CheckInterval); err != nil {
			return nil, NewShopCheckerError(ErrInvalidInterval)
		}
	}
	if req.CronExpr != nil && *req.CronExpr != "" {
		if _, err := cron.ParseStandard(*req.CronExpr); err != nil {
			return nil, NewShopCheckerError(ErrInvalidCronExpr)
		}
	}
	p, err := s.repo.UpdateProduct(ctx, id, req)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, NewShopCheckerError(ErrProductNotFound)
	}
	return p, err
}

// DeleteProduct 删除商品监控
func (s *Service) DeleteProduct(ctx context.Context, id string) error {
	return s.repo.DeleteProduct(ctx, id)
}

// ListProducts 分页查询商品列表
func (s *Service) ListProducts(ctx context.Context, req *ListProductsReq) ([]Product, bool, int, error) {
	return s.repo.ListProducts(ctx, req)
}

// ExecuteCheck 执行单次价格检查核心流程：
// 抓取价格 → 保存记录 → 判断是否通知（价格/库存变化） → 发送 Webhook → 保存通知日志 → 更新检查结果
func (s *Service) ExecuteCheck(ctx context.Context, p *Product, col collector.Collector) error {
	result, err := col.FetchPrice(ctx, p.URL)
	if err != nil {
		errMsg := err.Error()
		s.repo.SavePriceRecord(ctx, p.ID, 0, "", "unknown", errMsg)
		nextCheck := s.computeNextCheck(time.Now(), p.CheckInterval, p.CronExpr)
		s.repo.UpdateCheckResult(ctx, p.ID, 0, "unknown", time.Now(), nextCheck)
		return err
	}

	stockStatus := result.StockStatus
	if stockStatus == "" {
		stockStatus = "unknown"
	}

	pr, err := s.repo.SavePriceRecord(ctx, p.ID, result.CurrentPrice, result.Title, stockStatus, "")
	if err != nil {
		return err
	}

	oldPrice := p.LastPrice
	newPrice := result.CurrentPrice
	oldStock := p.LastStockStatus
	newStock := stockStatus

	stockChanged := oldStock != "" && newStock != "" && oldStock != newStock

	shouldNotifyPrice := s.shouldNotify(oldPrice, newPrice, p.TargetPrice, p.PriceDropRatio)
	shouldNotifyStock := stockChanged && newStock == "in_stock"

	if shouldNotifyPrice || shouldNotifyStock {
		var title, content, notifyType string

		if shouldNotifyPrice {
			dropPercent := (oldPrice - newPrice) / oldPrice * 100
			notifyType = "price_drop"
			title = fmt.Sprintf("💰 商品降价通知 - %s", p.Name)
			content = fmt.Sprintf(
				"| 字段 | 值 |\n|---|---|\n| 商品名称 | %s |\n| 平台 | %s |\n| 原价 | %.2f |\n| 现价 | %.2f |\n| 降幅 | %.2f%% |\n| 查看链接 | [点击前往](%s) |",
				p.Name, p.Platform, oldPrice, newPrice, dropPercent, p.URL,
			)
		} else {
			notifyType = "stock_restore"
			title = fmt.Sprintf("📦 商品补货通知 - %s", p.Name)
			content = fmt.Sprintf(
				"| 字段 | 值 |\n|---|---|\n| 商品名称 | %s |\n| 平台 | %s |\n| 当前价格 | %.2f |\n| 库存状态 | 有货 |\n| 查看链接 | [点击前往](%s) |",
				p.Name, p.Platform, newPrice, p.URL,
			)
		}

		sendReq := &webhook.SendReq{
			Source:  "shopchecker",
			Title:   title,
			Content: content,
			MsgType: "markdown",
		}
		if len(p.CustomWebhookIDs) > 0 {
			sendReq.WebhookIDs = p.CustomWebhookIDs
		} else {
			sendReq.Tags = []string{"shopchecker", notifyType}
		}

		sendResult, sendErr := s.whSvc.Send(ctx, sendReq)

		summary := SummaryAllSuccess
		if sendErr != nil {
			summary = SummaryAllFailed
		} else if sendResult.Failed > 0 {
			if sendResult.Success > 0 {
				summary = SummaryPartial
			} else {
				summary = SummaryAllFailed
			}
		}

		s.repo.SaveNotificationLog(ctx, &NotificationLog{
			ProductID:     p.ID,
			PriceRecordID: pr.ID,
			WebhookIDs:    p.CustomWebhookIDs,
			OldPrice:      oldPrice,
			NewPrice:      newPrice,
			OldStockStatus: oldStock,
			NewStockStatus: newStock,
			NotifyType:    notifyType,
			ResultSummary: string(summary),
		})

		s.repo.MarkPriceRecordNotified(ctx, pr.ID)
	}

	nextCheck := s.computeNextCheck(time.Now(), p.CheckInterval, p.CronExpr)
	s.repo.UpdateCheckResult(ctx, p.ID, newPrice, stockStatus, time.Now(), nextCheck)

	return nil
}

// FindDueProductIDs 返回需要检查的商品 ID 列表，用于调度器
func (s *Service) FindDueProductIDs(ctx context.Context, limit int) ([]string, error) {
	products, err := s.repo.FindDueProducts(ctx, limit)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(products))
	for i, p := range products {
		ids[i] = p.ID
	}
	return ids, nil
}

// RunCheck 根据商品 ID 执行一次价格检查，供调度器调用
func (s *Service) RunCheck(ctx context.Context, productID string) error {
	p, err := s.GetProduct(ctx, productID)
	if err != nil {
		return err
	}
	col, ok := s.registry.Get(p.Platform)
	if !ok {
		return NewShopCheckerError(ErrInvalidPlatform)
	}
	return s.ExecuteCheck(ctx, p, col)
}

// ManualCheck 手动触发价格检查：获取商品 → 解析采集器 → 执行检查 → 返回最新价格记录
func (s *Service) ManualCheck(ctx context.Context, productID string) (*PriceRecord, error) {
	p, err := s.GetProduct(ctx, productID)
	if err != nil {
		return nil, err
	}

	col, ok := s.registry.Get(p.Platform)
	if !ok {
		return nil, NewShopCheckerError(ErrInvalidPlatform)
	}

	if err := s.ExecuteCheck(ctx, p, col); err != nil {
		return nil, err
	}

	records, _, _, err := s.repo.ListPriceRecords(ctx, productID, &ListPriceRecordsReq{Page: 1, PageSize: 1})
	if err != nil || len(records) == 0 {
		return nil, err
	}
	return &records[0], nil
}

// ListPriceRecords 分页查询价格记录
func (s *Service) ListPriceRecords(ctx context.Context, productID string, req *ListPriceRecordsReq) ([]PriceRecord, bool, int, error) {
	return s.repo.ListPriceRecords(ctx, productID, req)
}

// ListNotificationLogs 分页查询通知记录
func (s *Service) ListNotificationLogs(ctx context.Context, productID string, req *ListNotificationLogsReq) ([]NotificationLog, bool, int, error) {
	return s.repo.ListNotificationLogs(ctx, productID, req)
}

// Dashboard 获取仪表盘统计数据
func (s *Service) Dashboard(ctx context.Context) (*DashboardResp, error) {
	return s.repo.GetDashboardStats(ctx)
}

// shouldNotify 判断是否需要发送降价通知
func (s *Service) shouldNotify(oldPrice, newPrice, targetPrice, dropRatio float64) bool {
	if oldPrice <= 0 || newPrice <= 0 || newPrice >= oldPrice {
		return false
	}
	if targetPrice > 0 && newPrice > targetPrice {
		return false
	}
	if dropRatio > 0 {
		dropPercent := (oldPrice - newPrice) / oldPrice * 100
		if dropPercent < dropRatio {
			return false
		}
	}
	return true
}

const sleepStartHour = 1  // 凌晨 1 点
const sleepEndHour   = 9  // 上午 9 点

// computeNextCheck 计算下次检查时间，加入随机抖动并避开休眠时段
func (s *Service) computeNextCheck(from time.Time, interval, cronExpr string) time.Time {
	var next time.Time

	if cronExpr != "" {
		schedule, err := cron.ParseStandard(cronExpr)
		if err == nil {
			next = schedule.Next(from)
		}
	}

	if next.IsZero() {
		d, err := time.ParseDuration(interval)
		if err != nil || d <= 0 {
			d = time.Hour
		}
		// 随机抖动：±15%
		jitter := time.Duration(float64(d) * (0.85 + rand.Float64()*0.30))
		next = from.Add(jitter)
	}

	// 如果落在休眠时段，推到 9:00（当天或次日）
	if inSleepHours(next) {
		next = time.Date(next.Year(), next.Month(), next.Day(), sleepEndHour, 0, 0, 0, next.Location())
		if next.Before(from) {
			next = next.AddDate(0, 0, 1)
		}
	}

	return next
}

// inSleepHours 判断给定时间是否在 1:00 - 9:00 之间
func inSleepHours(t time.Time) bool {
	h := t.Hour()
	return h >= sleepStartHour && h < sleepEndHour
}
