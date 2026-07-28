package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// JDCollector 京东 Provider 实现
type JDCollector struct {
	client *http.Client
}

// NewJDCollector 创建京东 Provider
func NewJDCollector() *JDCollector {
	return &JDCollector{
		client: &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return http.ErrUseLastResponse
				}
				return nil
			},
		},
	}
}

func (c *JDCollector) ID() string { return "jd" }

// AutoLogin 京东自动登录（通过浏览器表单填充，无 API）
// 京东无公开的密码登录 API，需要通过浏览器模拟
func (c *JDCollector) AutoLogin(ctx context.Context, username, password string) (*SessionData, error) {
	return nil, fmt.Errorf("京东自动登录需通过浏览器远程登录方式: %w", ErrNotSupported)
}

// Verify 验证京东 session 是否有效
// 用 session 中的 cookie 访问用户中心，检查是否重定向到登录页
func (c *JDCollector) Verify(ctx context.Context, session *SessionData) (bool, error) {
	if session == nil || len(session.Cookies) == 0 {
		return false, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://home.m.jd.com", nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	for _, cookie := range session.Cookies {
		req.AddCookie(&http.Cookie{
			Name:  cookie.Name,
			Value: cookie.Value,
			Domain: func() string {
				if cookie.Domain != "" {
					return cookie.Domain
				}
				return ".jd.com"
			}(),
		})
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	// 检查是否被重定向到登录页
	redirected := resp.Request.URL.String()
	if strings.Contains(redirected, "plogin") || strings.Contains(redirected, "passport") {
		return false, nil
	}
	return true, nil
}

// Refresh 京东暂不支持 refresh token
func (c *JDCollector) Refresh(ctx context.Context, session *SessionData) (*SessionData, error) {
	return nil, fmt.Errorf("京东不支持 token 刷新: %w", ErrNotSupported)
}

// ---------- 京东 cookie 注入辅助 ----------

// InjectCookiesToHTTP 将 session cookie 注入到 http.Request 中
func InjectCookiesToHTTP(session *SessionData, targetURL string, req *http.Request) {
	parsed, _ := url.Parse(targetURL)
	for _, c := range session.Cookies {
		if c.Domain != "" && !strings.HasSuffix(parsed.Host, strings.TrimPrefix(c.Domain, ".")) {
			continue
		}
		req.AddCookie(&http.Cookie{
			Name:  c.Name,
			Value: c.Value,
		})
	}
}
