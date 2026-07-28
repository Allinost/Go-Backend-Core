// Package provider 第三方登录 Provider 接口与各平台实现
package provider

import "context"

// SessionData 各平台通用的登录会话数据
type SessionData struct {
	Cookies   []Cookie          `json:"cookies,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
	Extra     map[string]any    `json:"extra,omitempty"`
	ExpiresAt int64             `json:"expires_at,omitempty"` // unix 时间戳
}

// Cookie cookie 数据
type Cookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain,omitempty"`
	Path     string `json:"path,omitempty"`
	Secure   bool   `json:"secure,omitempty"`
	HTTPOnly bool   `json:"http_only,omitempty"`
	Expires  int64  `json:"expires,omitempty"` // unix 时间戳
}

// Provider 每个平台需实现的登录接口
type Provider interface {
	// ID 返回平台标识，如 "jd"
	ID() string

	// AutoLogin 使用账号密码自动登录，返回 session
	// 不支持自动登录的平台返回 ErrNotSupported
	AutoLogin(ctx context.Context, username, password string) (*SessionData, error)

	// Verify 验证 session 是否仍然有效
	Verify(ctx context.Context, session *SessionData) (bool, error)

	// Refresh 尝试刷新 session（如用 refresh_token）
	// 不支持刷新的返回 ErrNotSupported
	Refresh(ctx context.Context, session *SessionData) (*SessionData, error)
}

// ErrNotSupported 该平台不支持此操作
var ErrNotSupported = &ProviderError{"该平台不支持此操作"}

// ProviderError provider 层错误
type ProviderError struct {
	Message string
}

func (e *ProviderError) Error() string {
	return e.Message
}
