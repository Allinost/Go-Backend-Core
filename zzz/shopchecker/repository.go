// Package shopchecker 数据访问层
package shopchecker

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	tableProducts     = "zzz_shopchecker_products"
	tablePriceRecords = "zzz_shopchecker_price_records"
	tableNotifLogs    = "zzz_shopchecker_notification_logs"
)

// Repository 数据访问层
type Repository struct {
	db *sql.DB
}

// NewRepository 创建 Repository
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// scanProductRow 通用行解析，处理 JSON 字段反序列化
func scanProductRow(scanner interface {
	Scan(dest ...any) error
}, p *Product) error {
	var wh sql.NullString
	var lastChecked, nextCheck sql.NullTime
	var remark sql.NullString

	err := scanner.Scan(
		&p.ID, &p.Name, &p.Platform, &p.URL, &p.ImageURL,
		&p.TargetPrice, &p.PriceDropRatio, &p.LastPrice,
		&p.LastStockStatus,
		&lastChecked, &nextCheck,
		&p.CheckInterval, &p.CronExpr,
		&wh,
		&p.Enabled, &remark,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return err
	}

	if wh.Valid {
		json.Unmarshal([]byte(wh.String), &p.CustomWebhookIDs)
	}
	if lastChecked.Valid {
		p.LastCheckedAt = &lastChecked.Time
	}
	if nextCheck.Valid {
		p.NextCheckAt = &nextCheck.Time
	}
	if remark.Valid {
		p.Remark = &remark.String
	}
	return nil
}

func scanProduct(row *sql.Row) (*Product, error) {
	var p Product
	if err := scanProductRow(row, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// scanPriceRecordRow 通用行解析
func scanPriceRecordRow(scanner interface {
	Scan(dest ...any) error
}, pr *PriceRecord) error {
	var raw sql.NullString

	err := scanner.Scan(
		&pr.ID, &pr.ProductID, &pr.Price, &pr.OriginalPrice,
		&pr.Title, &pr.StockStatus, &pr.IsNotified, &raw, &pr.ErrorMsg,
		&pr.CreatedAt,
	)
	if err != nil {
		return err
	}

	if raw.Valid {
		rm := json.RawMessage(raw.String)
		pr.RawData = &rm
	}
	return nil
}

func scanPriceRecord(row *sql.Row) (*PriceRecord, error) {
	var pr PriceRecord
	if err := scanPriceRecordRow(row, &pr); err != nil {
		return nil, err
	}
	return &pr, nil
}

// scanNotifLogRow 通用行解析
func scanNotifLogRow(scanner interface {
	Scan(dest ...any) error
}, nl *NotificationLog) error {
	var wh sql.NullString

	err := scanner.Scan(
		&nl.ID, &nl.ProductID, &nl.PriceRecordID,
		&wh, &nl.OldPrice, &nl.NewPrice,
		&nl.OldStockStatus, &nl.NewStockStatus, &nl.NotifyType,
		&nl.ResultSummary, &nl.CreatedAt,
	)
	if err != nil {
		return err
	}

	if wh.Valid {
		json.Unmarshal([]byte(wh.String), &nl.WebhookIDs)
	}
	return nil
}

func scanNotifLog(row *sql.Row) (*NotificationLog, error) {
	var nl NotificationLog
	if err := scanNotifLogRow(row, &nl); err != nil {
		return nil, err
	}
	return &nl, nil
}

const productColumns = "id, name, platform, url, image_url, target_price, price_drop_ratio, last_price, " +
	"last_stock_status, last_checked_at, next_check_at, check_interval, cron_expr, " +
	"custom_webhook_ids, enabled, remark, created_at, updated_at"

const priceRecordColumns = "id, product_id, price, original_price, title, stock_status, is_notified, raw_data, error_msg, created_at"

const notifLogColumns = "id, product_id, price_record_id, webhook_ids, old_price, new_price, old_stock_status, new_stock_status, notify_type, result_summary, created_at"

// ListProducts 分页查询商品列表
func (r *Repository) ListProducts(ctx context.Context, req *ListProductsReq) ([]Product, bool, int, error) {
	req.Normalize()

	var conditions []string
	var args []any

	conditions = append(conditions, "1=1")

	if req.Platform != "" {
		conditions = append(conditions, "platform = ?")
		args = append(args, req.Platform)
	}
	if req.Keyword != "" {
		conditions = append(conditions, "name LIKE ?")
		args = append(args, "%"+req.Keyword+"%")
	}
	if req.Enabled != nil {
		conditions = append(conditions, "enabled = ?")
		args = append(args, *req.Enabled)
	}

	where := strings.Join(conditions, " AND ")

	var total int
	countQuery := "SELECT COUNT(*) FROM " + tableProducts + " WHERE " + where
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, false, 0, err
	}

	limit := req.PageSize + 1
	offset := req.Offset()

	selectQuery := "SELECT " + productColumns + " FROM " + tableProducts +
		" WHERE " + where + " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	queryArgs := append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, selectQuery, queryArgs...)
	if err != nil {
		return nil, false, 0, err
	}
	defer rows.Close()

	var products []Product
	for rows.Next() {
		var p Product
		if err := scanProductRow(rows, &p); err != nil {
			return nil, false, 0, err
		}
		products = append(products, p)
	}
	if err := rows.Err(); err != nil {
		return nil, false, 0, err
	}

	hasMore := len(products) > req.PageSize
	if hasMore {
		products = products[:req.PageSize]
	}

	return products, hasMore, total, nil
}

// GetProduct 查询单个商品
func (r *Repository) GetProduct(ctx context.Context, id string) (*Product, error) {
	query := "SELECT " + productColumns + " FROM " + tableProducts + " WHERE id = ?"
	p, err := scanProduct(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, NewShopCheckerError(ErrProductNotFound)
		}
		return nil, err
	}
	return p, nil
}

// CreateProduct 创建商品
func (r *Repository) CreateProduct(ctx context.Context, p *Product) (*Product, error) {
	p.ID = uuid.New().String()
	now := time.Now()
	p.CreatedAt = now
	p.UpdatedAt = now

	whJSON, _ := json.Marshal(p.CustomWebhookIDs)

	var lastChecked, nextCheck sql.NullTime
	if p.LastCheckedAt != nil {
		lastChecked = sql.NullTime{Time: *p.LastCheckedAt, Valid: true}
	}
	if p.NextCheckAt != nil {
		nextCheck = sql.NullTime{Time: *p.NextCheckAt, Valid: true}
	}

	var remark any
	if p.Remark != nil {
		remark = *p.Remark
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO `+tableProducts+` (id, name, platform, url, image_url,
		 target_price, price_drop_ratio, last_price, last_stock_status,
		 last_checked_at, next_check_at, check_interval, cron_expr,
		 custom_webhook_ids, enabled, remark,
		 created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.Platform, p.URL, p.ImageURL,
		p.TargetPrice, p.PriceDropRatio, p.LastPrice, p.LastStockStatus,
		lastChecked, nextCheck,
		p.CheckInterval, p.CronExpr,
		string(whJSON), p.Enabled, remark,
		p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return p, nil
}

// UpdateProduct 更新商品（仅更新非 nil 字段），返回更新后的商品
func (r *Repository) UpdateProduct(ctx context.Context, id string, req *UpdateProductReq) (*Product, error) {
	p, err := r.GetProduct(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		p.Name = *req.Name
	}
	if req.Platform != nil {
		p.Platform = *req.Platform
	}
	if req.URL != nil {
		p.URL = *req.URL
	}
	if req.ImageURL != nil {
		p.ImageURL = *req.ImageURL
	}
	if req.TargetPrice != nil {
		p.TargetPrice = *req.TargetPrice
	}
	if req.PriceDropRatio != nil {
		p.PriceDropRatio = *req.PriceDropRatio
	}
	if req.CheckInterval != nil {
		p.CheckInterval = *req.CheckInterval
	}
	if req.CronExpr != nil {
		p.CronExpr = *req.CronExpr
	}
	if req.CustomWebhookIDs != nil {
		p.CustomWebhookIDs = req.CustomWebhookIDs
	}
	if req.Enabled != nil {
		p.Enabled = *req.Enabled
	}
	if req.Remark != nil {
		p.Remark = req.Remark
	}

	p.UpdatedAt = time.Now()

	whJSON, _ := json.Marshal(p.CustomWebhookIDs)

	var lastChecked, nextCheck sql.NullTime
	if p.LastCheckedAt != nil {
		lastChecked = sql.NullTime{Time: *p.LastCheckedAt, Valid: true}
	}
	if p.NextCheckAt != nil {
		nextCheck = sql.NullTime{Time: *p.NextCheckAt, Valid: true}
	}

	var remark any
	if p.Remark != nil {
		remark = *p.Remark
	}

	_, err = r.db.ExecContext(ctx,
		`UPDATE `+tableProducts+` SET name=?, platform=?, url=?, image_url=?,
		 target_price=?, price_drop_ratio=?, last_price=?, last_stock_status=?,
		 last_checked_at=?, next_check_at=?, check_interval=?, cron_expr=?,
		 custom_webhook_ids=?, enabled=?, remark=?,
		 updated_at=? WHERE id=?`,
		p.Name, p.Platform, p.URL, p.ImageURL,
		p.TargetPrice, p.PriceDropRatio, p.LastPrice, p.LastStockStatus,
		lastChecked, nextCheck,
		p.CheckInterval, p.CronExpr,
		string(whJSON), p.Enabled, remark,
		p.UpdatedAt, p.ID,
	)
	if err != nil {
		return nil, err
	}

	return p, nil
}

// DeleteProduct 删除商品
func (r *Repository) DeleteProduct(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM `+tableProducts+` WHERE id = ?`, id)
	return err
}

// FindDueProducts 查询需要检查的商品
func (r *Repository) FindDueProducts(ctx context.Context, limit int) ([]Product, error) {
	query := "SELECT " + productColumns + " FROM " + tableProducts +
		" WHERE enabled = 1 AND (next_check_at IS NULL OR next_check_at <= NOW())" +
		" ORDER BY next_check_at ASC LIMIT ?"

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []Product
	for rows.Next() {
		var p Product
		if err := scanProductRow(rows, &p); err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return products, nil
}

// UpdateCheckResult 更新检查结果
func (r *Repository) UpdateCheckResult(ctx context.Context, productID string, price float64, stockStatus string, checkedAt, nextCheckAt time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE `+tableProducts+` SET last_price = ?, last_stock_status = ?, last_checked_at = ?, next_check_at = ? WHERE id = ?`,
		price, stockStatus, checkedAt, nextCheckAt, productID,
	)
	return err
}

// SavePriceRecord 保存价格记录（含原价与采集原始数据）
func (r *Repository) SavePriceRecord(ctx context.Context, productID string, price, originalPrice float64, title, stockStatus, errMsg string, rawData any) (*PriceRecord, error) {
	now := time.Now()
	var raw any
	if rawData != nil {
		if b, err := json.Marshal(rawData); err == nil && string(b) != "null" {
			raw = string(b)
		}
	}
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO `+tablePriceRecords+` (product_id, price, original_price, title, stock_status, is_notified, raw_data, error_msg, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		productID, price, originalPrice, title, stockStatus, false, raw, errMsg, now,
	)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &PriceRecord{
		ID:            id,
		ProductID:     productID,
		Price:         price,
		OriginalPrice: originalPrice,
		Title:         title,
		StockStatus:   stockStatus,
		ErrorMsg:      errMsg,
		CreatedAt:     now,
	}, nil
}

// UpdateCheckFailure 记录失败检查时间，保留上一次有效价格/库存，避免一次网络失败把历史价格清零。
func (r *Repository) UpdateCheckFailure(ctx context.Context, productID string, checkedAt, nextCheckAt time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE `+tableProducts+` SET last_checked_at = ?, next_check_at = ? WHERE id = ?`,
		checkedAt, nextCheckAt, productID,
	)
	return err
}

// UpdateNextCheck 仅更新下次检查时间（修改检查周期后立即生效）。
func (r *Repository) UpdateNextCheck(ctx context.Context, productID string, nextCheckAt time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE `+tableProducts+` SET next_check_at = ? WHERE id = ?`,
		nextCheckAt, productID,
	)
	return err
}

// MarkPriceRecordNotified 标记价格记录已通知
func (r *Repository) MarkPriceRecordNotified(ctx context.Context, recordID int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE `+tablePriceRecords+` SET is_notified = 1 WHERE id = ?`, recordID,
	)
	return err
}

// ListPriceRecords 分页查询价格记录
func (r *Repository) ListPriceRecords(ctx context.Context, productID string, req *ListPriceRecordsReq) ([]PriceRecord, bool, int, error) {
	req.Normalize()

	var total int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM `+tablePriceRecords+` WHERE product_id = ?`, productID,
	).Scan(&total)
	if err != nil {
		return nil, false, 0, err
	}

	limit := req.PageSize + 1
	offset := req.Offset()

	query := "SELECT " + priceRecordColumns + " FROM " + tablePriceRecords +
		" WHERE product_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?"

	rows, err := r.db.QueryContext(ctx, query, productID, limit, offset)
	if err != nil {
		return nil, false, 0, err
	}
	defer rows.Close()

	var records []PriceRecord
	for rows.Next() {
		var pr PriceRecord
		if err := scanPriceRecordRow(rows, &pr); err != nil {
			return nil, false, 0, err
		}
		records = append(records, pr)
	}
	if err := rows.Err(); err != nil {
		return nil, false, 0, err
	}

	hasMore := len(records) > req.PageSize
	if hasMore {
		records = records[:req.PageSize]
	}

	return records, hasMore, total, nil
}

// SaveNotificationLog 保存通知记录
func (r *Repository) SaveNotificationLog(ctx context.Context, nl *NotificationLog) error {
	whJSON, _ := json.Marshal(nl.WebhookIDs)
	if nl.CreatedAt.IsZero() {
		nl.CreatedAt = time.Now()
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO `+tableNotifLogs+` (product_id, price_record_id, webhook_ids, old_price, new_price, old_stock_status, new_stock_status, notify_type, result_summary, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		nl.ProductID, nl.PriceRecordID, string(whJSON),
		nl.OldPrice, nl.NewPrice,
		nl.OldStockStatus, nl.NewStockStatus, nl.NotifyType,
		nl.ResultSummary, nl.CreatedAt,
	)
	return err
}

// ListNotificationLogs 分页查询通知记录
func (r *Repository) ListNotificationLogs(ctx context.Context, productID string, req *ListNotificationLogsReq) ([]NotificationLog, bool, int, error) {
	req.Normalize()

	var total int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM `+tableNotifLogs+` WHERE product_id = ?`, productID,
	).Scan(&total)
	if err != nil {
		return nil, false, 0, err
	}

	limit := req.PageSize + 1
	offset := req.Offset()

	query := "SELECT " + notifLogColumns + " FROM " + tableNotifLogs +
		" WHERE product_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?"

	rows, err := r.db.QueryContext(ctx, query, productID, limit, offset)
	if err != nil {
		return nil, false, 0, err
	}
	defer rows.Close()

	var logs []NotificationLog
	for rows.Next() {
		var nl NotificationLog
		if err := scanNotifLogRow(rows, &nl); err != nil {
			return nil, false, 0, err
		}
		logs = append(logs, nl)
	}
	if err := rows.Err(); err != nil {
		return nil, false, 0, err
	}

	hasMore := len(logs) > req.PageSize
	if hasMore {
		logs = logs[:req.PageSize]
	}

	return logs, hasMore, total, nil
}

// GetDashboardStats 获取仪表盘统计
func (r *Repository) GetDashboardStats(ctx context.Context) (*DashboardResp, error) {
	var stats DashboardResp

	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM `+tableProducts,
	).Scan(&stats.TotalProducts)
	if err != nil {
		return nil, err
	}

	err = r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM `+tableProducts+` WHERE enabled = 1`,
	).Scan(&stats.EnabledProducts)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	err = r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM `+tablePriceRecords+` WHERE created_at >= ?`, todayStart,
	).Scan(&stats.TodayChecks)
	if err != nil {
		return nil, err
	}

	err = r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM `+tablePriceRecords+` WHERE created_at >= ? AND is_notified = 1`, todayStart,
	).Scan(&stats.TodayNotified)
	if err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT n.product_id, p.name, p.platform, n.old_price, n.new_price, n.created_at
		 FROM `+tableNotifLogs+` n
		 JOIN `+tableProducts+` p ON n.product_id = p.id
		 ORDER BY n.created_at DESC LIMIT 5`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var item DropItem
		var droppedAt time.Time
		if err := rows.Scan(&item.ProductID, &item.Name, &item.Platform, &item.OldPrice, &item.NewPrice, &droppedAt); err != nil {
			return nil, err
		}
		if item.OldPrice > 0 {
			item.DropPercent = ((item.OldPrice - item.NewPrice) / item.OldPrice) * 100
		}
		item.DroppedAt = droppedAt.Format(time.DateTime)
		stats.RecentDrops = append(stats.RecentDrops, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &stats, nil
}

// GetProductStats 获取商品统计
func (r *Repository) GetProductStats(ctx context.Context, productID string) (totalChecks, totalDrops int, err error) {
	err = r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM `+tablePriceRecords+` WHERE product_id = ?`, productID,
	).Scan(&totalChecks)
	if err != nil {
		return 0, 0, err
	}

	err = r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM `+tablePriceRecords+` WHERE product_id = ? AND is_notified = 1`, productID,
	).Scan(&totalDrops)
	if err != nil {
		return 0, 0, err
	}

	return totalChecks, totalDrops, nil
}
