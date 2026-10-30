// Package webhook 数据访问层 — MySQL + Redis

package webhook

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	tableConfigs      = "zzz_webhook_configs"
	tableDeliveryLogs = "zzz_webhook_delivery_logs"
)

// Repository 数据访问层，封装 MySQL 和 Redis 操作
type Repository struct {
	db  *sql.DB
	rdb *redis.Client
}

// NewRepository 创建 Repository
func NewRepository(db *sql.DB, rdb *redis.Client) *Repository {
	return &Repository{db: db, rdb: rdb}
}

// ListConfigs 分页查询配置列表，支持按 type/status/source/tag/keyword 筛选
func (r *Repository) ListConfigs(ctx context.Context, req *ListConfigsReq) ([]WebhookConfig, bool, int, error) {
	var conditions []string
	var args []any

	conditions = append(conditions, "1=1")

	if req.Type != "" {
		conditions = append(conditions, "type = ?")
		args = append(args, req.Type)
	}
	if req.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, req.Status)
	}
	if req.Source != "" {
		conditions = append(conditions, "source = ?")
		args = append(args, req.Source)
	}
	if req.Tag != "" {
		conditions = append(conditions, "JSON_CONTAINS(tags, ?)")
		tagJSON, _ := json.Marshal(req.Tag)
		args = append(args, string(tagJSON))
	}
	if req.Keyword != "" {
		conditions = append(conditions, "(name LIKE ? OR description LIKE ?)")
		kw := "%" + req.Keyword + "%"
		args = append(args, kw, kw)
	}

	where := strings.Join(conditions, " AND ")

	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", tableConfigs, where)
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, false, 0, err
	}

	limit := req.PageSize
	offset := req.Offset()

	dataQuery := fmt.Sprintf(`SELECT id, name, type, url, status, tags, description, source, app,
		signing_key, access_token, auth_type, auth_value, custom_headers,
		whitelist_keywords, whitelist_mode,
		max_per_minute, max_per_hour, dedup_window, max_retries, retry_interval, timeout_ms,
		last_sent_at, last_success_at, last_error, total_sent, total_failed,
		created_at, updated_at
		FROM %s WHERE %s ORDER BY created_at DESC LIMIT ? OFFSET ?`, tableConfigs, where)

	dataArgs := append(args, limit+1, offset)
	rows, err := r.db.QueryContext(ctx, dataQuery, dataArgs...)
	if err != nil {
		return nil, false, 0, err
	}
	defer rows.Close()

	items, err := scanConfigs(rows)
	if err != nil {
		return nil, false, 0, err
	}
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	return items, hasMore, total, nil
}

// GetConfig 根据 ID 获取单个配置
func (r *Repository) GetConfig(ctx context.Context, id string) (*WebhookConfig, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, name, type, url, status, tags, description, source, app,
		 signing_key, access_token, auth_type, auth_value, custom_headers,
		 whitelist_keywords, whitelist_mode,
		 max_per_minute, max_per_hour, dedup_window, max_retries, retry_interval, timeout_ms,
		 last_sent_at, last_success_at, last_error, total_sent, total_failed,
		 created_at, updated_at
		 FROM `+tableConfigs+` WHERE id = ?`, id)
	return scanConfig(row)
}

// FindByIDs 按 ID 列表查询活跃配置（仅返回 status='active'）
func (r *Repository) FindByIDs(ctx context.Context, ids []string) ([]WebhookConfig, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	query := fmt.Sprintf(`SELECT id, name, type, url, status, tags, description, source, app,
		signing_key, access_token, auth_type, auth_value, custom_headers,
		whitelist_keywords, whitelist_mode,
		max_per_minute, max_per_hour, dedup_window, max_retries, retry_interval, timeout_ms,
		last_sent_at, last_success_at, last_error, total_sent, total_failed,
		created_at, updated_at
		FROM %s WHERE id IN (%s) AND status = 'active'`, tableConfigs, strings.Join(placeholders, ","))
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanConfigs(rows)
}

// FindByTags 按标签查询活跃配置（标签间 OR 关系，且 status='active'）
func (r *Repository) FindByTags(ctx context.Context, tags []string) ([]WebhookConfig, error) {
	if len(tags) == 0 {
		return nil, nil
	}
	var conditions []string
	var args []any
	for _, tag := range tags {
		tagJSON, _ := json.Marshal(tag)
		conditions = append(conditions, "JSON_CONTAINS(tags, ?)")
		args = append(args, string(tagJSON))
	}
	query := fmt.Sprintf(`SELECT id, name, type, url, status, tags, description, source, app,
		signing_key, access_token, auth_type, auth_value, custom_headers,
		whitelist_keywords, whitelist_mode,
		max_per_minute, max_per_hour, dedup_window, max_retries, retry_interval, timeout_ms,
		last_sent_at, last_success_at, last_error, total_sent, total_failed,
		created_at, updated_at
		FROM %s WHERE status = 'active' AND (%s)`, tableConfigs, strings.Join(conditions, " OR "))
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanConfigs(rows)
}

// FindActive 查询全部活跃配置
func (r *Repository) FindActive(ctx context.Context) ([]WebhookConfig, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, type, url, status, tags, description, source, app,
		 signing_key, access_token, auth_type, auth_value, custom_headers,
		 whitelist_keywords, whitelist_mode,
		 max_per_minute, max_per_hour, dedup_window, max_retries, retry_interval, timeout_ms,
		 last_sent_at, last_success_at, last_error, total_sent, total_failed,
		 created_at, updated_at
		 FROM `+tableConfigs+` WHERE status = 'active'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanConfigs(rows)
}

// CreateConfig 创建 Webhook 配置并返回完整对象
func (r *Repository) CreateConfig(ctx context.Context, req *CreateConfigReq) (*WebhookConfig, error) {
	now := time.Now()
	c := &WebhookConfig{
		ID:                uuid.New().String(),
		Name:              req.Name,
		Type:              req.Type,
		URL:               req.URL,
		Status:            "active",
		Source:            req.Source,
		App:               req.App,
		Description:       req.Description,
		Tags:              req.Tags,
		SigningKey:        req.SigningKey,
		AccessToken:       req.AccessToken,
		AuthType:          req.AuthType,
		AuthValue:         req.AuthValue,
		CustomHeaders:     req.CustomHeaders,
		WhitelistKeywords: req.WhitelistKeywords,
		WhitelistMode:     "any",
		MaxPerMinute:      60,
		MaxPerHour:        1000,
		MaxRetries:        3,
		RetryInterval:     30,
		TimeoutMS:         10000,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if req.WhitelistMode != "" {
		c.WhitelistMode = req.WhitelistMode
	}
	if req.MaxPerMinute != nil {
		c.MaxPerMinute = *req.MaxPerMinute
	}
	if req.MaxPerHour != nil {
		c.MaxPerHour = *req.MaxPerHour
	}
	if req.DedupWindow != nil {
		c.DedupWindow = *req.DedupWindow
	}
	if req.MaxRetries != nil {
		c.MaxRetries = *req.MaxRetries
	}
	if req.RetryInterval != nil {
		c.RetryInterval = *req.RetryInterval
	}
	if req.TimeoutMS != nil {
		c.TimeoutMS = *req.TimeoutMS
	}

	tagsJSON, _ := json.Marshal(req.Tags)
	headersJSON, _ := json.Marshal(req.CustomHeaders)
	kwJSON, _ := json.Marshal(req.WhitelistKeywords)

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO `+tableConfigs+` (id, name, type, url, status, tags, description, source, app,
		 signing_key, access_token, auth_type, auth_value, custom_headers,
		 whitelist_keywords, whitelist_mode,
		 max_per_minute, max_per_hour, dedup_window, max_retries, retry_interval, timeout_ms,
		 created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.Name, c.Type, c.URL, c.Status, tagsJSON, c.Description, c.Source, c.App,
		c.SigningKey, c.AccessToken, c.AuthType, c.AuthValue, headersJSON,
		kwJSON, c.WhitelistMode,
		c.MaxPerMinute, c.MaxPerHour, c.DedupWindow, c.MaxRetries, c.RetryInterval, c.TimeoutMS,
		c.CreatedAt, c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// UpdateConfig 更新配置（仅更新非 nil 字段），返回更新后的配置
func (r *Repository) UpdateConfig(ctx context.Context, id string, req *UpdateConfigReq) (*WebhookConfig, error) {
	c, err := r.GetConfig(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		c.Name = *req.Name
	}
	if req.Type != nil {
		c.Type = *req.Type
	}
	if req.URL != nil {
		c.URL = *req.URL
	}
	if req.Source != nil {
		c.Source = *req.Source
	}
	if req.App != nil {
		c.App = *req.App
	}
	if req.Description != nil {
		c.Description = *req.Description
	}
	if req.Tags != nil {
		c.Tags = req.Tags
	}
	if req.SigningKey != nil {
		c.SigningKey = *req.SigningKey
	}
	if req.AccessToken != nil {
		c.AccessToken = *req.AccessToken
	}
	if req.AuthType != nil {
		c.AuthType = *req.AuthType
	}
	if req.AuthValue != nil {
		c.AuthValue = *req.AuthValue
	}
	if req.CustomHeaders != nil {
		c.CustomHeaders = req.CustomHeaders
	}
	if req.WhitelistKeywords != nil {
		c.WhitelistKeywords = req.WhitelistKeywords
	}
	if req.WhitelistMode != nil {
		c.WhitelistMode = *req.WhitelistMode
	}
	if req.MaxPerMinute != nil {
		c.MaxPerMinute = *req.MaxPerMinute
	}
	if req.MaxPerHour != nil {
		c.MaxPerHour = *req.MaxPerHour
	}
	if req.DedupWindow != nil {
		c.DedupWindow = *req.DedupWindow
	}
	if req.MaxRetries != nil {
		c.MaxRetries = *req.MaxRetries
	}
	if req.RetryInterval != nil {
		c.RetryInterval = *req.RetryInterval
	}
	if req.TimeoutMS != nil {
		c.TimeoutMS = *req.TimeoutMS
	}

	c.UpdatedAt = time.Now()

	tagsJSON, _ := json.Marshal(c.Tags)
	headersJSON, _ := json.Marshal(c.CustomHeaders)
	kwJSON, _ := json.Marshal(c.WhitelistKeywords)

	_, err = r.db.ExecContext(ctx,
		`UPDATE `+tableConfigs+` SET name=?, type=?, url=?, tags=?, description=?, source=?, app=?,
		 signing_key=?, access_token=?, auth_type=?, auth_value=?, custom_headers=?,
		 whitelist_keywords=?, whitelist_mode=?,
		 max_per_minute=?, max_per_hour=?, dedup_window=?, max_retries=?, retry_interval=?, timeout_ms=?,
		 updated_at=? WHERE id=?`,
		c.Name, c.Type, c.URL, tagsJSON, c.Description, c.Source, c.App,
		c.SigningKey, c.AccessToken, c.AuthType, c.AuthValue, headersJSON,
		kwJSON, c.WhitelistMode,
		c.MaxPerMinute, c.MaxPerHour, c.DedupWindow, c.MaxRetries, c.RetryInterval, c.TimeoutMS,
		c.UpdatedAt, c.ID)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// DeleteConfig 删除配置
func (r *Repository) DeleteConfig(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM `+tableConfigs+` WHERE id = ?`, id)
	return err
}

// UpdateStatus 更新配置状态
func (r *Repository) UpdateStatus(ctx context.Context, id, status string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE `+tableConfigs+` SET status = ?, updated_at = ? WHERE id = ?`,
		status, time.Now(), id)
	return err
}

// UpdateStats 更新发送统计（total_sent、total_failed、last_sent_at、last_success_at）
func (r *Repository) UpdateStats(ctx context.Context, id string, success bool) {
	now := time.Now()
	if success {
		r.db.ExecContext(ctx,
			`UPDATE `+tableConfigs+` SET total_sent = total_sent + 1, last_sent_at = ?, last_success_at = ? WHERE id = ?`,
			now, now, id)
	} else {
		r.db.ExecContext(ctx,
			`UPDATE `+tableConfigs+` SET total_sent = total_sent + 1, total_failed = total_failed + 1, last_sent_at = ? WHERE id = ?`,
			now, id)
	}
}

// UpdateLastError 更新 last_error 字段
func (r *Repository) UpdateLastError(ctx context.Context, id, errMsg string) {
	r.db.ExecContext(ctx,
		`UPDATE `+tableConfigs+` SET last_error = ? WHERE id = ?`, errMsg, id)
}

// SaveDeliveryLog 保存投递日志
func (r *Repository) SaveDeliveryLog(ctx context.Context, log *DeliveryLog) error {
	bodyForHash := log.Title
	if log.RequestBody != nil {
		bodyForHash += *log.RequestBody
	}
	log.ContentHash = sha256Hex(bodyForHash)
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO `+tableDeliveryLogs+` (webhook_id, source, title, content_hash, status,
		 request_body, response_code, response_body, error_msg, retry_count, duration_ms)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		log.WebhookID, log.Source, log.Title, log.ContentHash, log.Status,
		log.RequestBody, log.ResponseCode, log.ResponseBody, log.ErrorMsg, log.RetryCount, log.DurationMS)
	return err
}

// ListDeliveryLogs 分页查询投递日志，支持按 webhook_id / source / status 筛选
func (r *Repository) ListDeliveryLogs(ctx context.Context, req *ListDeliveryLogsReq) ([]DeliveryLog, bool, int, error) {
	var conditions []string
	var args []any

	conditions = append(conditions, "1=1")

	if req.WebhookID != "" {
		conditions = append(conditions, "webhook_id = ?")
		args = append(args, req.WebhookID)
	}
	if req.Source != "" {
		conditions = append(conditions, "source = ?")
		args = append(args, req.Source)
	}
	if req.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, req.Status)
	}

	where := strings.Join(conditions, " AND ")

	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", tableDeliveryLogs, where)
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, false, 0, err
	}

	limit := req.PageSize
	offset := req.Offset()

	dataArgs := append(args, limit+1, offset)
	rows, err := r.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT id, webhook_id, source, title, content_hash, status,
		 request_body, response_code, response_body, error_msg, retry_count, duration_ms, created_at
		 FROM %s WHERE %s ORDER BY created_at DESC LIMIT ? OFFSET ?`, tableDeliveryLogs, where),
		dataArgs...)
	if err != nil {
		return nil, false, 0, err
	}
	defer rows.Close()

	var items []DeliveryLog
	for rows.Next() {
		var l DeliveryLog
		if err := rows.Scan(&l.ID, &l.WebhookID, &l.Source, &l.Title, &l.ContentHash,
			&l.Status, &l.RequestBody, &l.ResponseCode, &l.ResponseBody,
			&l.ErrorMsg, &l.RetryCount, &l.DurationMS, &l.CreatedAt); err != nil {
			return nil, false, 0, err
		}
		items = append(items, l)
	}
	if err := rows.Err(); err != nil {
		return nil, false, 0, err
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	return items, hasMore, total, nil
}

// DeleteDeliveryLog 删除单条投递日志
func (r *Repository) DeleteDeliveryLog(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM `+tableDeliveryLogs+` WHERE id = ?`, id)
	return err
}

// BatchDeleteDeliveryLogs 批量删除投递日志
func (r *Repository) BatchDeleteDeliveryLogs(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM `+tableDeliveryLogs+` WHERE id IN (`+strings.Join(placeholders, ",")+`)`, args...)
	return err
}

// GetStats 获取全局统计概览（配置总数/活跃数、今日投递量、按 type/source 分布）
func (r *Repository) GetStats(ctx context.Context) (*StatsResp, error) {
	stats := &StatsResp{
		ByType:   make(map[string]int),
		BySource: make(map[string]int),
	}

	r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM `+tableConfigs).Scan(&stats.TotalConfigs)
	r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM `+tableConfigs+` WHERE status = 'active'`).Scan(&stats.ActiveConfigs)

	todayStart := time.Now().Truncate(24 * time.Hour)
	r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM `+tableDeliveryLogs+` WHERE created_at >= ?`, todayStart).Scan(&stats.TotalSentToday)
	r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM `+tableDeliveryLogs+` WHERE created_at >= ? AND status = 'success'`, todayStart).Scan(&stats.SuccessToday)
	r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM `+tableDeliveryLogs+` WHERE created_at >= ? AND status = 'failed'`, todayStart).Scan(&stats.FailedToday)
	r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM `+tableDeliveryLogs+` WHERE created_at >= ? AND status = 'rate_limited'`, todayStart).Scan(&stats.RateLimitedToday)

	rows, _ := r.db.QueryContext(ctx,
		`SELECT type, COUNT(*) FROM `+tableConfigs+` GROUP BY type`)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var t string
			var c int
			rows.Scan(&t, &c)
			stats.ByType[t] = c
		}
	}

	srows, _ := r.db.QueryContext(ctx,
		`SELECT source, COUNT(*) as cnt FROM `+tableDeliveryLogs+` WHERE created_at >= ? GROUP BY source`, todayStart)
	if srows != nil {
		defer srows.Close()
		for srows.Next() {
			var src string
			var cnt int
			srows.Scan(&src, &cnt)
			stats.BySource[src] = cnt
		}
	}

	return stats, nil
}

// GetConfigStats 获取单个配置的统计
func (r *Repository) GetConfigStats(ctx context.Context, id string) (*ConfigStatsResp, error) {
	stats := &ConfigStatsResp{}
	err := r.db.QueryRowContext(ctx,
		`SELECT total_sent, total_failed, last_sent_at, last_success_at, last_error
		 FROM `+tableConfigs+` WHERE id = ?`, id).Scan(
		&stats.TotalSent, &stats.TotalFailed, &stats.LastSentAt, &stats.LastSuccessAt, &stats.LastError)
	if err != nil {
		return nil, err
	}
	return stats, nil
}

// scanConfigs 遍历 rows 解析为 WebhookConfig 列表
func scanConfigs(rows *sql.Rows) ([]WebhookConfig, error) {
	var items []WebhookConfig
	for rows.Next() {
		var c WebhookConfig
		if err := scanConfigRow(rows, &c); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

// scanConfig 解析单行配置
func scanConfig(row *sql.Row) (*WebhookConfig, error) {
	var c WebhookConfig
	if err := scanConfigRow(row, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// scanConfigRow 通用行解析，支持 sql.Row 和 sql.Rows，处理 JSON 字段反序列化
func scanConfigRow(scanner interface {
	Scan(dest ...any) error
}, c *WebhookConfig) error {
	var tags, headers, kw sql.NullString
	var lastSent, lastSuccess sql.NullTime

	err := scanner.Scan(
		&c.ID, &c.Name, &c.Type, &c.URL, &c.Status, &tags, &c.Description,
		&c.Source, &c.App,
		&c.SigningKey, &c.AccessToken, &c.AuthType, &c.AuthValue, &headers,
		&kw, &c.WhitelistMode,
		&c.MaxPerMinute, &c.MaxPerHour, &c.DedupWindow,
		&c.MaxRetries, &c.RetryInterval, &c.TimeoutMS,
		&lastSent, &lastSuccess,
		&c.LastError, &c.TotalSent, &c.TotalFailed,
		&c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return err
	}

	if tags.Valid {
		json.Unmarshal([]byte(tags.String), &c.Tags)
	}
	if headers.Valid {
		json.Unmarshal([]byte(headers.String), &c.CustomHeaders)
	}
	if kw.Valid {
		json.Unmarshal([]byte(kw.String), &c.WhitelistKeywords)
	}
	if lastSent.Valid {
		c.LastSentAt = &lastSent.Time
	}
	if lastSuccess.Valid {
		c.LastSuccessAt = &lastSuccess.Time
	}
	return nil
}

// sha256Hex 计算字符串的 SHA256 十六进制摘要
func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h)
}
