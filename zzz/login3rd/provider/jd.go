package provider

import (
	"context"
	"fmt"
	"io"
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
	// pt_key/pt_pin 是京东系（含七鲜、京东移动版）的登录凭证；缺失说明会话已被清除。
	if !hasAnyCookie(session.Cookies, "pt_key", "pt_pin") {
		return false, nil
	}

	// 订单中心未登录会 302 到 passport.jd.com；比首页更适合判断真实登录态。
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://order.jd.com/center/list.action", nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	addCookies(req, session.Cookies)

	resp, err := c.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))

	// 检查是否被重定向到登录页，或返回登录页 HTML。
	finalURL := resp.Request.URL.String()
	lowerURL := strings.ToLower(finalURL)
	if strings.Contains(lowerURL, "plogin") || strings.Contains(lowerURL, "passport") ||
		strings.Contains(lowerURL, "login") || strings.Contains(lowerURL, "risk_handler") {
		return false, nil
	}
	text := string(body)
	// 京东未登录时订单中心可能以 200 返回登录页，必须结合页面标题/登录表单文案判断。
	if strings.Contains(text, "京东验证") || strings.Contains(text, "请登录") ||
		strings.Contains(text, "passport.jd.com") || strings.Contains(text, "登录注册") ||
		strings.Contains(text, "欢迎登录") || strings.Contains(text, "账号登录") ||
		strings.Contains(text, "扫码登录") || strings.Contains(text, "立即登录") {
		return false, nil
	}
	if title := extractHTMLTitle(text); title != "" && strings.Contains(title, "登录") {
		return false, nil
	}
	return true, nil
}

func extractHTMLTitle(html string) string {
	start := strings.Index(strings.ToLower(html), "<title>")
	if start < 0 {
		return ""
	}
	start += len("<title>")
	end := strings.Index(strings.ToLower(html[start:]), "</title>")
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(html[start : start+end])
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
