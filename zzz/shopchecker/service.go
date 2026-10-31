// Package shopchecker 商品价格监控业务逻辑层
package shopchecker

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"

	"github.com/Allinost/go-backend-core/internal/modules"
	"github.com/Allinost/go-backend-core/internal/modules/webhook"
	"github.com/Allinost/go-backend-core/zzz/login3rd"
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

// getLogin3rdSvc 懒加载 Login3rd 服务（避免模块初始化顺序依赖）
func (s *Service) getLogin3rdSvc() *login3rd.Service {
	mod := modules.Get("login3rd")
	if mod == nil {
		return nil
	}
	svc, ok := mod.(interface{ GetService() *login3rd.Service })
	if !ok {
		return nil
	}
	return svc.GetService()
}

// isPlatformLoginExpired 判断平台下是否有账号已标记登录失效（登录失效时跳过价格检查）
func (s *Service) isPlatformLoginExpired(ctx context.Context, platformID string) bool {
	svc := s.getLogin3rdSvc()
	if svc == nil {
		return false
	}
	expired, err := svc.IsPlatformLoginExpired(ctx, platformID)
	if err != nil {
		return false
	}
	return expired
}

// CreateProduct 创建商品监控（校验平台/URL/间隔，计算下次检查时间）
func (s *Service) CreateProduct(ctx context.Context, req *CreateProductReq) (*Product, error) {
	if !isSupportedPlatform(req.Platform) {
		return nil, NewShopCheckerError(ErrInvalidPlatform)
	}
	if req.URL == "" {
		return nil, NewShopCheckerError(ErrInvalidURL)
	}
	interval, err := normalizeCheckInterval(req.CheckInterval)
	if err != nil {
		return nil, err
	}
	req.CheckInterval = interval
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
	if req.Platform != nil && !isSupportedPlatform(*req.Platform) {
		return nil, NewShopCheckerError(ErrInvalidPlatform)
	}
	if req.CheckInterval != nil {
		interval, err := normalizeCheckInterval(*req.CheckInterval)
		if err != nil {
			return nil, err
		}
		*req.CheckInterval = interval
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
	if err != nil {
		return nil, err
	}
	// 修改检查周期/cron 后立即重算下次检查时间，避免沿用旧周期。
	if req.CheckInterval != nil || req.CronExpr != nil {
		next := s.computeNextCheck(time.Now(), p.CheckInterval, p.CronExpr)
		if err := s.repo.UpdateNextCheck(ctx, p.ID, next); err != nil {
			return nil, err
		}
		p.NextCheckAt = &next
	}
	return p, nil
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
	// 渠道登录已失效：在重新登录前跳过价格检查（由 login3rd 同步账号状态）
	if s.isPlatformLoginExpired(ctx, p.Platform) {
		return fmt.Errorf("平台 %s 登录已失效，已跳过价格检查，请先登录", p.Platform)
	}

	// 地址回退：地址为空时自动读取备注中的原始地址
	fetchURL := p.URL
	if fetchURL == "" && p.Remark != nil && *p.Remark != "" {
		fetchURL = *p.Remark
	}

	result, err := col.FetchPrice(ctx, fetchURL)
	if err != nil {
		errMsg := err.Error()
		pr, _ := s.repo.SavePriceRecord(ctx, p.ID, 0, 0, "", "unknown", errMsg, nil)
		now := time.Now()
		nextCheck := s.computeNextCheck(now, p.CheckInterval, p.CronExpr)
		// 平台频率限制时强制退避至少 6 小时，避免继续触发风控。
		if collector.IsRateLimitError(err) {
			if backoff := now.Add(6 * time.Hour); nextCheck.Before(backoff) {
				nextCheck = backoff
			}
		}
		// 失败时保留上次有效价格/库存，避免历史价格被清零导致下一轮误判降价。
		s.repo.UpdateCheckFailure(ctx, p.ID, now, nextCheck)

		// 登录失效/反爬拦截 → 通过商品绑定的 webhook 发送告警（与价格通知同一链路）
		if isLoginFailure(errMsg) {
			s.notifyLoginExpired(ctx, p, pr, errMsg)
		}
		return err
	}

	// 地址规范化：首次采集成功后，用采集到的规范商品页地址更新商品 URL，
	// 并把原始地址存入备注，后续检查直接使用规范地址，不再走跳转/解析分支
	if result.ProductURL != "" && result.ProductURL != p.URL {
		origURL := p.URL
		if p.Remark == nil || *p.Remark == "" {
			if origURL != "" {
				p.Remark = &origURL
			} else {
				empty := ""
				p.Remark = &empty
			}
		}
		p.URL = result.ProductURL
		upd := &UpdateProductReq{URL: &p.URL, Remark: p.Remark}
		if _, err := s.repo.UpdateProduct(ctx, p.ID, upd); err != nil {
			return fmt.Errorf("更新商品地址失败: %w", err)
		}
	}

	stockStatus := result.StockStatus
	if stockStatus == "" {
		stockStatus = "unknown"
	}

	pr, err := s.repo.SavePriceRecord(ctx, p.ID, result.CurrentPrice, result.OriginalPrice, result.Title, stockStatus, "", result)
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
			if oldPrice > 0 && newPrice < oldPrice {
				dropPercent := (oldPrice - newPrice) / oldPrice * 100
				notifyType = "price_drop"
				title = fmt.Sprintf("💰 商品降价通知 - %s", p.Name)
				content = fmt.Sprintf(
					"| 字段 | 值 |\n|---|---|\n| 商品名称 | %s |\n| 平台 | %s |\n| 原价 | %.2f |\n| 现价 | %.2f |\n| 降幅 | %.2f%% |\n| 查看链接 | [点击前往](%s) |",
					p.Name, p.Platform, oldPrice, newPrice, dropPercent, p.URL,
				)
			} else {
				notifyType = "target_reached"
				title = fmt.Sprintf("🎯 商品到达目标价 - %s", p.Name)
				content = fmt.Sprintf(
					"| 字段 | 值 |\n|---|---|\n| 商品名称 | %s |\n| 平台 | %s |\n| 当前价格 | %.2f |\n| 目标价格 | %.2f |\n| 查看链接 | [点击前往](%s) |",
					p.Name, p.Platform, newPrice, p.TargetPrice, p.URL,
				)
			}
		} else {
			notifyType = "stock_restore"
			title = fmt.Sprintf("📦 商品补货通知 - %s", p.Name)
			content = fmt.Sprintf(
				"| 字段 | 值 |\n|---|---|\n| 商品名称 | %s |\n| 平台 | %s |\n| 当前价格 | %.2f |\n| 库存状态 | 有货 |\n| 查看链接 | [点击前往](%s) |",
				p.Name, p.Platform, newPrice, p.URL,
			)
		}

		if result.SubsidyAmount > 0 {
			content += fmt.Sprintf("\n| 国补抵扣 | -%.2f（%.2f%%） |", result.SubsidyAmount, result.SubsidyRate)
		}
		if result.BasePrice > 0 && result.BasePrice > newPrice {
			content += fmt.Sprintf("\n| 补贴前价格 | %.2f |", result.BasePrice)
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
			ProductID:      p.ID,
			PriceRecordID:  pr.ID,
			WebhookIDs:     p.CustomWebhookIDs,
			OldPrice:       oldPrice,
			NewPrice:       newPrice,
			OldStockStatus: oldStock,
			NewStockStatus: newStock,
			NotifyType:     notifyType,
			ResultSummary:  string(summary),
		})

		s.repo.MarkPriceRecordNotified(ctx, pr.ID)
	}

	// 价格采集成功说明渠道登录态可用，顺延 login3rd 会话有效期，避免频繁检查期间会话失效
	if svc := s.getLogin3rdSvc(); svc != nil {
		svc.TouchSession(ctx, p.Platform, 24*time.Hour)
	}

	nextCheck := s.computeNextCheck(time.Now(), p.CheckInterval, p.CronExpr)
	s.repo.UpdateCheckResult(ctx, p.ID, newPrice, stockStatus, time.Now(), nextCheck)

	return nil
}

// isLoginFailure 判断采集错误是否为登录失效/反爬拦截（需要人工重新登录）
func isLoginFailure(errMsg string) bool {
	lower := strings.ToLower(errMsg)
	for _, kw := range []string{"需要登录", "登录已过期", "登录/风险页", "反爬拦截", "session 已失效", "plogin", "passport"} {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// notifyLoginExpired 登录失效/反爬拦截时：
// 1) 同步标记 login3rd 账号为失效（后续跳过价格检查，直到重新登录）
// 2) 通过渠道（账号）绑定的 webhook 发送登录失效告警，内容含登录失效的平台与远程登录地址
// 3) 渠道 webhook 已发送时不再重复发送第二条商品 webhook；仅当平台没有 login3rd 账号时才回退到商品 webhook
func (s *Service) notifyLoginExpired(ctx context.Context, p *Product, pr *PriceRecord, errMsg string) {
	svc := s.getLogin3rdSvc()
	if svc == nil {
		// 没有 login3rd 模块时回退到商品 webhook
		s.doNotifyLoginExpired(ctx, p, pr, errMsg)
		return
	}

	notified, acctWebhookIDs := svc.NotifyLoginExpired(ctx, p.Platform, errMsg)

	// 渠道 webhook 已发送：仅记录通知日志，不再发送第二条商品 webhook
	if notified {
		s.saveLoginExpiredLog(ctx, p, pr, acctWebhookIDs, SummaryAllSuccess)
		return
	}

	// 平台没有 login3rd 账号（或无账号 webhook）时，回退到商品 webhook
	s.doNotifyLoginExpired(ctx, p, pr, errMsg)
}

// doNotifyLoginExpired 使用商品 webhook（回退路径）发送登录失效告警，并记录通知日志
func (s *Service) doNotifyLoginExpired(ctx context.Context, p *Product, pr *PriceRecord, errMsg string) {
	title := fmt.Sprintf("⚠️ %s 登录已失效 - %s", p.Platform, p.Name)
	content := fmt.Sprintf(
		"| 字段 | 值 |\n|---|---|\n| 商品名称 | %s |\n| 平台 | %s |\n| 错误信息 | %s |\n| 查看链接 | [点击前往](%s) |",
		p.Name, p.Platform, errMsg, p.URL,
	)

	sendReq := &webhook.SendReq{
		Source:     "shopchecker",
		Title:      title,
		Content:    content,
		MsgType:    "markdown",
		WebhookIDs: p.CustomWebhookIDs,
	}
	if len(p.CustomWebhookIDs) == 0 {
		sendReq.Tags = []string{"shopchecker", "login_expired"}
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

	s.saveLoginExpiredLog(ctx, p, pr, p.CustomWebhookIDs, summary)
}

// saveLoginExpiredLog 记录登录失效通知日志
func (s *Service) saveLoginExpiredLog(ctx context.Context, p *Product, pr *PriceRecord, webhookIDs []string, summary ResultSummary) {
	priceRecordID := int64(0)
	if pr != nil {
		priceRecordID = pr.ID
	}
	stockStatus := ""
	if p.LastStockStatus != "" {
		stockStatus = p.LastStockStatus
	}
	s.repo.SaveNotificationLog(ctx, &NotificationLog{
		ProductID:      p.ID,
		PriceRecordID:  priceRecordID,
		WebhookIDs:     webhookIDs,
		OldStockStatus: stockStatus,
		NewStockStatus: stockStatus,
		NotifyType:     "login_expired",
		ResultSummary:  string(summary),
		CreatedAt:      time.Now(),
	})
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

// isSupportedPlatform 判断平台是否受支持
func isSupportedPlatform(platform string) bool {
	switch Platform(platform) {
	case PlatformJD, PlatformTaobao, PlatformTmall, PlatformPDD, PlatformQixian, PlatformJDMobile:
		return true
	default:
		return false
	}
}

// normalizeCheckInterval 校验检查间隔：空值默认 1h，必须为正时长
func normalizeCheckInterval(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "1h", nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return "", NewShopCheckerError(ErrInvalidInterval)
	}
	return raw, nil
}

// shouldNotify 判断是否需要发送降价/达标通知
func (s *Service) shouldNotify(oldPrice, newPrice, targetPrice, dropRatio float64) bool {
	if newPrice <= 0 {
		return false
	}
	// 目标价只在“从未达标/刚跌破目标价”时触发一次；已低于目标价后不再每轮重复通知。
	if targetPrice > 0 && newPrice <= targetPrice {
		if oldPrice <= 0 || oldPrice > targetPrice {
			return true
		}
	}
	// 降价通知：需有历史价格且价格确实下降
	if oldPrice <= 0 || newPrice >= oldPrice {
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

const sleepStartHour = 1 // 凌晨 1 点
const sleepEndHour = 9   // 上午 9 点

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
