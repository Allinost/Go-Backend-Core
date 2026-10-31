package shopchecker

import (
	"encoding/json"
	"time"
)

type Platform string

const (
	PlatformJD       Platform = "jd"
	PlatformTaobao   Platform = "taobao"
	PlatformTmall    Platform = "tmall"
	PlatformPDD      Platform = "pdd"
	PlatformQixian   Platform = "qixian"
	PlatformJDMobile Platform = "jdm"
)

type ResultSummary string

const (
	SummaryAllSuccess ResultSummary = "all_success"
	SummaryPartial    ResultSummary = "partial"
	SummaryAllFailed  ResultSummary = "all_failed"
)

// Product 商品监控配置
type Product struct {
	ID               string     `json:"_id" db:"id"`
	Name             string     `json:"name" db:"name"`
	Platform         string     `json:"platform" db:"platform"`
	URL              string     `json:"url" db:"url"`
	ImageURL         string     `json:"image_url,omitempty" db:"image_url"`
	TargetPrice      float64    `json:"target_price,omitempty" db:"target_price"`
	PriceDropRatio   float64    `json:"price_drop_ratio,omitempty" db:"price_drop_ratio"`
	LastPrice        float64    `json:"last_price" db:"last_price"`
	LastStockStatus  string     `json:"last_stock_status" db:"last_stock_status"`
	LastCheckedAt    *time.Time `json:"last_checked_at,omitempty" db:"last_checked_at"`
	NextCheckAt      *time.Time `json:"next_check_at,omitempty" db:"next_check_at"`
	CheckInterval    string     `json:"check_interval" db:"check_interval"`
	CronExpr         string     `json:"cron_expr,omitempty" db:"cron_expr"`
	CustomWebhookIDs []string   `json:"custom_webhook_ids,omitempty" db:"custom_webhook_ids"`
	Enabled          bool       `json:"enabled" db:"enabled"`
	Remark           *string    `json:"remark,omitempty" db:"remark"`
	CreatedAt        time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at" db:"updated_at"`
}

// PriceRecord 价格记录
type PriceRecord struct {
	ID            int64            `json:"id" db:"id"`
	ProductID     string           `json:"product_id" db:"product_id"`
	Price         float64          `json:"price" db:"price"`
	OriginalPrice float64          `json:"original_price,omitempty" db:"original_price"`
	Title         string           `json:"title,omitempty" db:"title"`
	StockStatus   string           `json:"stock_status" db:"stock_status"`
	IsNotified    bool             `json:"is_notified" db:"is_notified"`
	RawData       *json.RawMessage `json:"raw_data,omitempty" db:"raw_data"`
	ErrorMsg      string           `json:"error_msg,omitempty" db:"error_msg"`
	CreatedAt     time.Time        `json:"created_at" db:"created_at"`
}

// NotificationLog 通知日志
type NotificationLog struct {
	ID             int64     `json:"id" db:"id"`
	ProductID      string    `json:"product_id" db:"product_id"`
	PriceRecordID  int64     `json:"price_record_id" db:"price_record_id"`
	WebhookIDs     []string  `json:"webhook_ids" db:"webhook_ids"`
	OldPrice       float64   `json:"old_price" db:"old_price"`
	NewPrice       float64   `json:"new_price" db:"new_price"`
	OldStockStatus string    `json:"old_stock_status" db:"old_stock_status"`
	NewStockStatus string    `json:"new_stock_status" db:"new_stock_status"`
	NotifyType     string    `json:"notify_type" db:"notify_type"`
	ResultSummary  string    `json:"result_summary" db:"result_summary"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
}

type CreateProductReq struct {
	Name             string   `json:"name" binding:"required"`
	Platform         string   `json:"platform" binding:"required"`
	URL              string   `json:"url" binding:"required"`
	ImageURL         string   `json:"image_url"`
	TargetPrice      *float64 `json:"target_price"`
	PriceDropRatio   *float64 `json:"price_drop_ratio"`
	CheckInterval    string   `json:"check_interval"`
	CronExpr         string   `json:"cron_expr"`
	CustomWebhookIDs []string `json:"custom_webhook_ids"`
	Remark           *string  `json:"remark"`
}

type UpdateProductReq struct {
	Name             *string  `json:"name"`
	Platform         *string  `json:"platform"`
	URL              *string  `json:"url"`
	ImageURL         *string  `json:"image_url"`
	TargetPrice      *float64 `json:"target_price"`
	PriceDropRatio   *float64 `json:"price_drop_ratio"`
	CheckInterval    *string  `json:"check_interval"`
	CronExpr         *string  `json:"cron_expr"`
	CustomWebhookIDs []string `json:"custom_webhook_ids"`
	Enabled          *bool    `json:"enabled"`
	Remark           *string  `json:"remark"`
}

type ListProductsReq struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Platform string `form:"platform"`
	Keyword  string `form:"keyword"`
	Enabled  *bool  `form:"enabled"`
}

func (r *ListProductsReq) Normalize() {
	if r.Page < 1 {
		r.Page = 1
	}
	if r.PageSize < 1 || r.PageSize > 100 {
		r.PageSize = 20
	}
}

func (r *ListProductsReq) Offset() int {
	return (r.Page - 1) * r.PageSize
}

type ListPriceRecordsReq struct {
	Page     int `form:"page"`
	PageSize int `form:"page_size"`
}

func (r *ListPriceRecordsReq) Normalize() {
	if r.Page < 1 {
		r.Page = 1
	}
	if r.PageSize < 1 || r.PageSize > 100 {
		r.PageSize = 20
	}
}

func (r *ListPriceRecordsReq) Offset() int {
	return (r.Page - 1) * r.PageSize
}

type ListNotificationLogsReq struct {
	Page     int `form:"page"`
	PageSize int `form:"page_size"`
}

func (r *ListNotificationLogsReq) Normalize() {
	if r.Page < 1 {
		r.Page = 1
	}
	if r.PageSize < 1 || r.PageSize > 100 {
		r.PageSize = 20
	}
}

func (r *ListNotificationLogsReq) Offset() int {
	return (r.Page - 1) * r.PageSize
}

type DashboardResp struct {
	TotalProducts   int        `json:"total_products"`
	EnabledProducts int        `json:"enabled_products"`
	TodayChecks     int        `json:"today_checks"`
	TodayNotified   int        `json:"today_notified"`
	RecentDrops     []DropItem `json:"recent_drops,omitempty"`
}

type DropItem struct {
	ProductID   string  `json:"product_id"`
	Name        string  `json:"name"`
	Platform    string  `json:"platform"`
	OldPrice    float64 `json:"old_price"`
	NewPrice    float64 `json:"new_price"`
	DropPercent float64 `json:"drop_percent"`
	DroppedAt   string  `json:"dropped_at"`
}

type PaginatedResp[T any] struct {
	Items   []T  `json:"items"`
	HasMore bool `json:"has_more"`
	Total   int  `json:"total"`
}
