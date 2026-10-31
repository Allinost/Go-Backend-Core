package provider

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// PDDProvider 拼多多消费者端 Provider
type PDDProvider struct {
	client *http.Client
}

// NewPDDProvider 创建拼多多 Provider
func NewPDDProvider() *PDDProvider {
	return &PDDProvider{
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

func (p *PDDProvider) ID() string { return "pdd" }

func (p *PDDProvider) AutoLogin(ctx context.Context, username, password string) (*SessionData, error) {
	return nil, fmt.Errorf("拼多多自动登录需通过浏览器远程登录方式: %w", ErrNotSupported)
}

// Verify 验证拼多多消费者端 session。
func (p *PDDProvider) Verify(ctx context.Context, session *SessionData) (bool, error) {
	if session == nil || len(session.Cookies) == 0 {
		return false, nil
	}
	if !hasAnyCookie(session.Cookies, "pdd_user_id", "pdd_user_uin", "PDDAccessToken", "pdd_vds") {
		return false, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://mobile.yangkeduo.com/personal.html", nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 16_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.6 Mobile/15E148 Safari/604.1")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	addCookies(req, session.Cookies)

	resp, err := p.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 128*1024))
	if isLoginURL(resp.Request.URL.String()) {
		return false, nil
	}
	text := string(body)
	if strings.Contains(text, "login.html") && strings.Contains(text, "needLogin") {
		return false, nil
	}
	// 拼多多未登录时个人中心也可能 200 返回登录页。
	if strings.Contains(text, "手机登录") || strings.Contains(text, "发送验证码") ||
		strings.Contains(text, "打开拼多多APP") || strings.Contains(text, "登录后可查看") {
		return false, nil
	}
	return true, nil
}

func (p *PDDProvider) Refresh(ctx context.Context, session *SessionData) (*SessionData, error) {
	return nil, fmt.Errorf("拼多多不支持 token 刷新: %w", ErrNotSupported)
}
