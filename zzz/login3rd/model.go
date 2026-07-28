// Package login3rd 第三方应用登录管理模块
// 管理京东、淘宝、拼多多、1688、TJUPT 等平台登录信息与会话
package login3rd

import (
	"time"
)

// Platform 第三方平台注册信息，对应 login3rd_platforms 表
type Platform struct {
	ID        string       `json:"id" db:"id"`                    // 平台标识 jd/taobao/pdd/_1688/tjupt
	Name      string       `json:"name" db:"name"`                // 显示名称
	Category  string       `json:"category" db:"category"`        // shopping/pt/sign
	LoginType string       `json:"login_type" db:"login_type"`    // browser/oauth/qrcode
	HasAuto   bool         `json:"has_auto" db:"has_auto"`        // 是否支持账号密码自动登录
	LoginURL  string       `json:"login_url" db:"login_url"`      // 登录页 URL
	HomeURL   string       `json:"home_url" db:"home_url"`        // 登录后首页 URL（提取 cookie）
	FormCfg   *FormConfig  `json:"form_config,omitempty" db:"form_config"` // 自动填表单配置
	IconURL   string       `json:"icon_url,omitempty" db:"icon_url"`
	CreatedAt time.Time    `json:"created_at" db:"created_at"`
}

// FormConfig 浏览器自动填表单的配置，存放在 platform.form_config 字段
type FormConfig struct {
	TabSelector       string   `json:"tab_selector,omitempty"`        // 切换到账号密码 tab
	UsernameSelector  string   `json:"username_selector,omitempty"`   // 账号输入框
	PasswordSelector  string   `json:"password_selector,omitempty"`   // 密码输入框
	SubmitSelector    string   `json:"submit_selector,omitempty"`     // 登录按钮
	CaptchaSelector   string   `json:"captcha_selector,omitempty"`    // 验证码元素
	SuccessCookies    []string `json:"success_cookies,omitempty"`     // 出现这些 cookie 判定登录成功
	SuccessURLPrefix  string   `json:"success_url_prefix,omitempty"`  // URL 前缀判断登录成功
	LoginWaitSelector string   `json:"login_wait_selector,omitempty"` // 等待出现的元素（二维码等）
	MaxAutoRetry      int      `json:"max_auto_retry,omitempty"`      // 自动填最大重试
}

// Account 第三方账号与当前登录态，对应 login3rd_accounts 表
type Account struct {
	ID               int64         `json:"id" db:"id"`
	PlatformID       string        `json:"platform_id" db:"platform_id"`

	// 用户标识
	Label            string        `json:"label" db:"label"`

	// 凭证（存储时加密，响应时脱敏）
	Username         string        `json:"username" db:"username"`
	EncryptedPwd     string        `json:"-" db:"encrypted_password"`
	EncryptedToken   string        `json:"-" db:"encrypted_token"`

	// Webhook 通知
	WebhookIDs       []string      `json:"webhook_ids,omitempty" db:"webhook_ids"`

	// Session（当前登录态，替代文件系统 profile）
	SessionData      *SessionData  `json:"session_data,omitempty" db:"session_data"`
	LoginMethod      string        `json:"login_method,omitempty" db:"login_method"`          // auto / remote / manual
	SessionExpiresAt *time.Time    `json:"session_expires_at,omitempty" db:"session_expires_at"`
	LastVerifiedAt   *time.Time    `json:"last_verified_at,omitempty" db:"last_verified_at"`
	VerifiedStatus   string        `json:"verified_status" db:"verified_status"`              // valid / expired / unknown

	// 状态
	Status           string        `json:"status" db:"status"`                     // active / expired / disabled
	LastLoginAt      *time.Time    `json:"last_login_at,omitempty" db:"last_login_at"`
	LastError        string        `json:"last_error,omitempty" db:"last_error"`
	CreatedAt        time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at" db:"updated_at"`
}

// SessionData 登录会话数据，存放在 accounts.session_data 字段
type SessionData struct {
	Cookies   []SessionCookie   `json:"cookies,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
	Extra     map[string]any    `json:"extra,omitempty"`
	ExpiresAt *time.Time        `json:"expires_at,omitempty"`
}

// SessionCookie cookie 数据结构
type SessionCookie struct {
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Domain   string    `json:"domain,omitempty"`
	Path     string    `json:"path,omitempty"`
	Secure   bool      `json:"secure,omitempty"`
	HTTPOnly bool      `json:"http_only,omitempty"`
	Expires  time.Time `json:"expires,omitempty"`
}

// ---------- 请求/响应结构 ----------

type CreateAccountReq struct {
	PlatformID string   `json:"platform_id" binding:"required"`   // 平台标识
	Label      string   `json:"label" binding:"required"`         // 自定义名称
	Username   string   `json:"username"`                         // 用户名
	Password   string   `json:"password"`                         // 明文密码（仅用于创建，不存储明文）
	Token      string   `json:"token"`                            // 备用 token
	WebhookIDs []string `json:"webhook_ids"`                      // webhook 配置 ID 列表
}

type UpdateAccountReq struct {
	Label      *string   `json:"label"`
	Username   *string   `json:"username"`
	Password   *string   `json:"password"`
	Token      *string   `json:"token"`
	WebhookIDs []string  `json:"webhook_ids"`
	Status     *string   `json:"status"`
}

type ListAccountsReq struct {
	Page       int    `form:"page"`
	PageSize   int    `form:"page_size"`
	PlatformID string `form:"platform_id"`
	Status     string `form:"status"`
	Keyword    string `form:"keyword"`
}

func (r *ListAccountsReq) Normalize() {
	if r.Page < 1 {
		r.Page = 1
	}
	if r.PageSize < 1 || r.PageSize > 100 {
		r.PageSize = 20
	}
}

func (r *ListAccountsReq) Offset() int {
	return (r.Page - 1) * r.PageSize
}

// AccountStatusResp 账号状态详情响应
type AccountStatusResp struct {
	Account          Account    `json:"account"`
	Platform         Platform   `json:"platform"`
	SessionValid     bool       `json:"session_valid"`
	SessionExpiresIn string     `json:"session_expires_in,omitempty"`
	CanAutoLogin     bool       `json:"can_auto_login"`
}

// PaginatedResp 通用分页响应
type PaginatedResp[T any] struct {
	Items   []T   `json:"items"`
	HasMore bool  `json:"has_more"`
	Total   int   `json:"total"`
}
