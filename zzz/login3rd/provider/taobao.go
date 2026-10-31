package provider

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// TaobaoProvider 淘宝/天猫 Provider（两端共用登录态）
type TaobaoProvider struct {
	client *http.Client
}

// NewTaobaoProvider 创建淘宝/天猫 Provider
func NewTaobaoProvider() *TaobaoProvider {
	return &TaobaoProvider{
		client: &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 8 {
					return http.ErrUseLastResponse
				}
				return nil
			},
		},
	}
}

func (p *TaobaoProvider) ID() string { return "taobao" }

func (p *TaobaoProvider) AutoLogin(ctx context.Context, username, password string) (*SessionData, error) {
	return nil, fmt.Errorf("淘宝自动登录需通过浏览器远程登录方式: %w", ErrNotSupported)
}

// Verify 验证淘宝/天猫 session：必须包含用户标识 cookie，且访问“我的淘宝”不被重定向到登录页。
func (p *TaobaoProvider) Verify(ctx context.Context, session *SessionData) (bool, error) {
	if session == nil || len(session.Cookies) == 0 {
		return false, nil
	}
	if !hasAnyCookie(session.Cookies, "unb", "_tb_token_") {
		return false, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://i.taobao.com/my_taobao.htm", nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	addCookies(req, session.Cookies)

	resp, err := p.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	finalURL := resp.Request.URL.String()
	if isLoginURL(finalURL) {
		return false, nil
	}
	text := string(body)
	if strings.Contains(text, "havanaone") || strings.Contains(text, "login.taobao.com") ||
		strings.Contains(text, "亲，请登录") || strings.Contains(text, "_____tmd_____") {
		return false, nil
	}
	return true, nil
}

func (p *TaobaoProvider) Refresh(ctx context.Context, session *SessionData) (*SessionData, error) {
	return nil, fmt.Errorf("淘宝不支持 token 刷新: %w", ErrNotSupported)
}

func hasAnyCookie(cookies []Cookie, names ...string) bool {
	for _, c := range cookies {
		for _, name := range names {
			if c.Name == name && c.Value != "" {
				return true
			}
		}
	}
	return false
}

func addCookies(req *http.Request, cookies []Cookie) {
	for _, c := range cookies {
		if c.Name == "" || c.Value == "" {
			continue
		}
		req.AddCookie(&http.Cookie{Name: c.Name, Value: c.Value})
	}
}

func isLoginURL(raw string) bool {
	lower := strings.ToLower(raw)
	for _, kw := range []string{"login.taobao.com", "login.m.taobao.com", "login.tmall.com",
		"login.html", "passport", "havanaone", "punish", "captcha"} {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}
