// Package webhook 数据模型定义

package webhook

import (
	"time"
)

// WebhookType Webhook 类型
type WebhookType string

const (
	WebhookTypeFeishu   WebhookType = "feishu"
	WebhookTypeDingtalk WebhookType = "dingtalk"
	WebhookTypeWecom    WebhookType = "wecom"
	WebhookTypeGitHub   WebhookType = "github"
	WebhookTypeGeneric  WebhookType = "generic"
)

// ConfigStatus Webhook 配置状态
type ConfigStatus string

const (
	ConfigStatusActive   ConfigStatus = "active"
	ConfigStatusPaused   ConfigStatus = "paused"
	ConfigStatusDisabled ConfigStatus = "disabled"
)

// DeliveryStatus 投递状态枚举
type DeliveryStatus string

const (
	DeliveryPending     DeliveryStatus = "pending"
	DeliverySuccess     DeliveryStatus = "success"
	DeliveryFailed      DeliveryStatus = "failed"
	DeliveryRateLimited DeliveryStatus = "rate_limited"
	DeliveryDeduped     DeliveryStatus = "deduped"
)

// WhitelistMode 白名单匹配模式
type WhitelistMode string

const (
	WhitelistModeAny WhitelistMode = "any"
	WhitelistModeAll WhitelistMode = "all"
)

// WebhookConfig Webhook 配置，对应 zzz_webhook_configs 表
type WebhookConfig struct {
	ID                string            `json:"_id" db:"id"`
	Name              string            `json:"name" db:"name"`
	Type              string            `json:"type" db:"type"`
	URL               string            `json:"url" db:"url"`
	Status            string            `json:"status" db:"status"`
	Tags              []string          `json:"tags,omitempty" db:"tags"`
	Description       string            `json:"description,omitempty" db:"description"`
	Source            string            `json:"source" db:"source"`
	App               string            `json:"app,omitempty" db:"app"`
	SigningKey        string            `json:"signing_key,omitempty" db:"signing_key"`
	AccessToken       string            `json:"access_token,omitempty" db:"access_token"`
	AuthType          string            `json:"auth_type,omitempty" db:"auth_type"`
	AuthValue         string            `json:"auth_value,omitempty" db:"auth_value"`
	CustomHeaders     map[string]string `json:"custom_headers,omitempty" db:"custom_headers"`
	WhitelistKeywords []string          `json:"whitelist_keywords,omitempty" db:"whitelist_keywords"`
	WhitelistMode     string            `json:"whitelist_mode,omitempty" db:"whitelist_mode"`
	MaxPerMinute      int               `json:"max_per_minute" db:"max_per_minute"`
	MaxPerHour        int               `json:"max_per_hour" db:"max_per_hour"`
	DedupWindow       int               `json:"dedup_window" db:"dedup_window"`
	MaxRetries        int               `json:"max_retries" db:"max_retries"`
	RetryInterval     int               `json:"retry_interval" db:"retry_interval"`
	TimeoutMS         int               `json:"timeout_ms" db:"timeout_ms"`
	LastSentAt        *time.Time        `json:"last_sent_at,omitempty" db:"last_sent_at"`
	LastSuccessAt     *time.Time        `json:"last_success_at,omitempty" db:"last_success_at"`
	LastError         string            `json:"last_error,omitempty" db:"last_error"`
	TotalSent         int               `json:"total_sent" db:"total_sent"`
	TotalFailed       int               `json:"total_failed" db:"total_failed"`
	CreatedAt         time.Time         `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at" db:"updated_at"`
}

// WebhookConfigMasked 响应时脱敏的配置视图（签名密钥、令牌、认证值脱敏）
type WebhookConfigMasked struct {
	WebhookConfig
	SigningKey  string `json:"signing_key"`
	AccessToken string `json:"access_token"`
	AuthValue   string `json:"auth_value"`
}

// Mask 对敏感字段进行脱敏处理，返回 Masked 视图
func (c *WebhookConfig) Mask() *WebhookConfigMasked {
	m := &WebhookConfigMasked{WebhookConfig: *c}
	if len(m.SigningKey) > 6 {
		m.SigningKey = m.SigningKey[:3] + "***" + m.SigningKey[len(m.SigningKey)-3:]
	} else if m.SigningKey != "" {
		m.SigningKey = "***"
	}
	if len(m.AccessToken) > 6 {
		m.AccessToken = m.AccessToken[:3] + "***" + m.AccessToken[len(m.AccessToken)-3:]
	} else if m.AccessToken != "" {
		m.AccessToken = "***"
	}
	if m.AuthValue != "" {
		m.AuthValue = "***"
	}
	return m
}

// DeliveryLog 投递日志，对应 zzz_webhook_delivery_logs 表
type DeliveryLog struct {
	ID           int64     `json:"id" db:"id"`
	WebhookID    string    `json:"webhook_id" db:"webhook_id"`
	Source       string    `json:"source" db:"source"`
	Title        string    `json:"title,omitempty" db:"title"`
	ContentHash  string    `json:"content_hash" db:"content_hash"`
	Status       string    `json:"status" db:"status"`
	RequestBody  *string   `json:"request_body,omitempty" db:"request_body"`
	ResponseCode int       `json:"response_code" db:"response_code"`
	ResponseBody *string   `json:"response_body,omitempty" db:"response_body"`
	ErrorMsg     string    `json:"error_msg,omitempty" db:"error_msg"`
	RetryCount   int       `json:"retry_count" db:"retry_count"`
	DurationMS   int       `json:"duration_ms" db:"duration_ms"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

// SendReq 发送消息请求体
type SendReq struct {
	WebhookIDs []string       `json:"webhook_ids"`    // 精确指定 Webhook ID，为空时按 tags 或全部活跃配置发送
	Tags       []string       `json:"tags"`           // 按标签匹配（OR 关系）
	Source     string         `json:"source"`         // 来源标识，用于统计
	Title      string         `json:"title"`          // 消息标题
	Content    string         `json:"content"`        // 消息正文
	Data       map[string]any `json:"data,omitempty"` // 扩展数据（仅通用类型会透传）
	MsgType    string         `json:"msg_type"`       // 消息类型：text / markdown / card / interactive
}

// SendResult 批量发送结果
type SendResult struct {
	Success int             `json:"success"`
	Failed  int             `json:"failed"`
	Results []WebhookResult `json:"results"`
}

// WebhookResult 单次发送结果
type WebhookResult struct {
	WebhookID   string `json:"webhook_id"`
	WebhookName string `json:"webhook_name"`
	Status      string `json:"status"` // success / failed / rate_limited / deduped
	Error       string `json:"error,omitempty"`
	DurationMS  int    `json:"duration_ms"`
}

// TestReq 测试发送请求
type TestReq struct {
	WebhookID string `json:"webhook_id"`
	TestMsg   string `json:"test_msg,omitempty"`
}

// CreateConfigReq 创建配置请求
type CreateConfigReq struct {
	Name              string            `json:"name" binding:"required"`
	Type              string            `json:"type" binding:"required"`
	URL               string            `json:"url" binding:"required"`
	Source            string            `json:"source"`
	App               string            `json:"app"`
	Description       string            `json:"description"`
	Tags              []string          `json:"tags"`
	SigningKey        string            `json:"signing_key"`
	AccessToken       string            `json:"access_token"`
	AuthType          string            `json:"auth_type"`
	AuthValue         string            `json:"auth_value"`
	CustomHeaders     map[string]string `json:"custom_headers"`
	WhitelistKeywords []string          `json:"whitelist_keywords"`
	WhitelistMode     string            `json:"whitelist_mode"`
	MaxPerMinute      *int              `json:"max_per_minute"`
	MaxPerHour        *int              `json:"max_per_hour"`
	DedupWindow       *int              `json:"dedup_window"`
	MaxRetries        *int              `json:"max_retries"`
	RetryInterval     *int              `json:"retry_interval"`
	TimeoutMS         *int              `json:"timeout_ms"`
}

type UpdateConfigReq struct {
	Name              *string           `json:"name"`
	Type              *string           `json:"type"`
	URL               *string           `json:"url"`
	Source            *string           `json:"source"`
	App               *string           `json:"app"`
	Description       *string           `json:"description"`
	Tags              []string          `json:"tags"`
	SigningKey        *string           `json:"signing_key"`
	AccessToken       *string           `json:"access_token"`
	AuthType          *string           `json:"auth_type"`
	AuthValue         *string           `json:"auth_value"`
	CustomHeaders     map[string]string `json:"custom_headers"`
	WhitelistKeywords []string          `json:"whitelist_keywords"`
	WhitelistMode     *string           `json:"whitelist_mode"`
	MaxPerMinute      *int              `json:"max_per_minute"`
	MaxPerHour        *int              `json:"max_per_hour"`
	DedupWindow       *int              `json:"dedup_window"`
	MaxRetries        *int              `json:"max_retries"`
	RetryInterval     *int              `json:"retry_interval"`
	TimeoutMS         *int              `json:"timeout_ms"`
}

type UpdateStatusReq struct {
	Status string `json:"status" binding:"required"`
}

type ListConfigsReq struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Type     string `form:"type"`
	Status   string `form:"status"`
	Source   string `form:"source"`
	Tag      string `form:"tag"`
	Keyword  string `form:"keyword"`
}

func (r *ListConfigsReq) Normalize() {
	if r.Page < 1 {
		r.Page = 1
	}
	if r.PageSize < 1 || r.PageSize > 100 {
		r.PageSize = 20
	}
}

type ListDeliveryLogsReq struct {
	Page      int    `form:"page"`
	PageSize  int    `form:"page_size"`
	WebhookID string `form:"webhook_id"`
	Source    string `form:"source"`
	Status    string `form:"status"`
	StartTime string `form:"start"`
	EndTime   string `form:"end"`
}

func (r *ListDeliveryLogsReq) Normalize() {
	if r.Page < 1 {
		r.Page = 1
	}
	if r.PageSize < 1 || r.PageSize > 100 {
		r.PageSize = 20
	}
}

// BatchDeleteDeliveryLogsReq 批量删除投递日志请求
type BatchDeleteDeliveryLogsReq struct {
	IDs []int64 `json:"ids" binding:"required"`
}

type PaginatedResp[T any] struct {
	Items   []T  `json:"items"`
	HasMore bool `json:"has_more"`
	Total   int  `json:"total"`
}

type StatsResp struct {
	TotalConfigs     int            `json:"total_configs"`
	ActiveConfigs    int            `json:"active_configs"`
	TotalSentToday   int            `json:"total_sent_today"`
	SuccessToday     int            `json:"success_today"`
	FailedToday      int            `json:"failed_today"`
	RateLimitedToday int            `json:"rate_limited_today"`
	ByType           map[string]int `json:"by_type"`
	BySource         map[string]int `json:"by_source"`
}

type ConfigStatsResp struct {
	TotalSent     int        `json:"total_sent"`
	TotalFailed   int        `json:"total_failed"`
	LastSentAt    *time.Time `json:"last_sent_at"`
	LastSuccessAt *time.Time `json:"last_success_at"`
	LastError     string     `json:"last_error"`
}

func (r *ListConfigsReq) Offset() int {
	return (r.Page - 1) * r.PageSize
}

func (r *ListDeliveryLogsReq) Offset() int {
	return (r.Page - 1) * r.PageSize
}

func intPtr(i int) *int       { return &i }
func strPtr(s string) *string { return &s }
