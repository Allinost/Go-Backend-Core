package browser

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/Allinost/go-backend-core/zzz/login3rd/provider"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// FormConfig 浏览器自动填表单配置
type FormConfig struct {
	TabSelector       string
	UsernameSelector  string
	PasswordSelector  string
	SubmitSelector    string
	CaptchaSelector   string
	SuccessCookies    []string
	SuccessURLPrefix  string
	LoginWaitSelector string
	MaxAutoRetry      int
}

// LoginFlow 通用浏览器登录流
type LoginFlow struct {
	LoginURL string
	HomeURL  string
	FormCfg  *FormConfig
}

// Execute 执行浏览器登录流程
func (f *LoginFlow) Execute(ctx context.Context, browserCtx context.Context, username, password string) (*provider.SessionData, error) {
	if err := chromedp.Run(browserCtx,
		chromedp.Navigate(f.LoginURL),
		chromedp.WaitReady("body"),
		chromedp.Sleep(2*time.Second),
	); err != nil {
		return nil, fmt.Errorf("导航到登录页失败: %w", err)
	}

	if f.FormCfg != nil && username != "" && password != "" {
		tasks := []chromedp.Action{}
		if f.FormCfg.TabSelector != "" {
			tasks = append(tasks, chromedp.Click(f.FormCfg.TabSelector, chromedp.ByQuery))
			tasks = append(tasks, chromedp.Sleep(500*time.Millisecond))
		}
		tasks = append(tasks,
			chromedp.Clear(f.FormCfg.UsernameSelector),
			chromedp.SendKeys(f.FormCfg.UsernameSelector, username),
			chromedp.Clear(f.FormCfg.PasswordSelector),
			chromedp.SendKeys(f.FormCfg.PasswordSelector, password),
			chromedp.Sleep(500*time.Millisecond),
		)
		if err := chromedp.Run(browserCtx, tasks...); err != nil {
			return nil, fmt.Errorf("自动填充账号密码失败: %w", err)
		}

		if f.FormCfg.CaptchaSelector == "" && f.FormCfg.SubmitSelector != "" {
			if err := chromedp.Run(browserCtx,
				chromedp.Click(f.FormCfg.SubmitSelector),
				chromedp.Sleep(2*time.Second),
			); err != nil {
				return nil, fmt.Errorf("点击登录按钮失败: %w", err)
			}
		}
	}

	if f.FormCfg != nil {
		cookies, err := f.waitLogin(browserCtx)
		if err != nil {
			return nil, err
		}
		provCookies := make([]provider.Cookie, len(cookies))
		for i, c := range cookies {
			provCookies[i] = *c
		}
		return &provider.SessionData{Cookies: provCookies}, nil
	}

	cookies, err := f.extractCookies(browserCtx)
	if err != nil {
		return nil, err
	}
	provCookies := make([]provider.Cookie, len(cookies))
	for i, c := range cookies {
		provCookies[i] = *c
	}
	return &provider.SessionData{Cookies: provCookies}, nil
}

// ExecuteWithChrome 使用临时本地浏览器执行登录
func (f *LoginFlow) ExecuteWithChrome(ctx context.Context, chromePath string, username, password string) (*provider.SessionData, error) {
	profileDir, err := os.MkdirTemp("", "login3rd-*")
	if err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(profileDir)

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(chromePath),
		chromedp.NoSandbox,
		chromedp.UserDataDir(profileDir),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-dev-shm-usage", true),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, opts...)
	defer allocCancel()
	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	defer browserCancel()

	return f.Execute(ctx, browserCtx, username, password)
}

// ExecuteRemote 使用远程 browserless/chrome 执行登录
func (f *LoginFlow) ExecuteRemote(ctx context.Context, remoteURL string, username, password string) (*provider.SessionData, error) {
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(ctx, remoteURL)
	defer allocCancel()
	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	defer browserCancel()

	return f.Execute(ctx, browserCtx, username, password)
}

// waitLogin 轮询检测登录是否成功
func (f *LoginFlow) waitLogin(browserCtx context.Context) ([]*provider.Cookie, error) {
	timeout := time.After(120 * time.Second)
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-browserCtx.Done():
			return nil, browserCtx.Err()
		case <-timeout:
			return nil, fmt.Errorf("等待登录超时（120s）")
		case <-tick.C:
			cookies, err := f.extractCookies(browserCtx)
			if err != nil {
				continue
			}

			if len(f.FormCfg.SuccessCookies) > 0 {
				if hasAllCookies(cookies, f.FormCfg.SuccessCookies) {
					return cookies, nil
				}
			}

			if f.FormCfg.SuccessURLPrefix != "" {
				var currentURL string
				if err := chromedp.Run(browserCtx, chromedp.Location(&currentURL)); err == nil {
					if len(currentURL) >= len(f.FormCfg.SuccessURLPrefix) &&
						currentURL[:len(f.FormCfg.SuccessURLPrefix)] == f.FormCfg.SuccessURLPrefix {
						return cookies, nil
					}
				}
			}
		}
	}
}

// extractCookies 提取浏览器中的全部 cookie
func (f *LoginFlow) extractCookies(browserCtx context.Context) ([]*provider.Cookie, error) {
	var cdpCookies []*network.Cookie
	if err := chromedp.Run(browserCtx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			cdpCookies, err = network.GetCookies().Do(ctx)
			return err
		}),
	); err != nil {
		return nil, err
	}

	var cookies []*provider.Cookie
	for _, c := range cdpCookies {
		cookies = append(cookies, &provider.Cookie{
			Name: c.Name, Value: c.Value, Domain: c.Domain,
			Path: c.Path, Secure: c.Secure, HTTPOnly: c.HTTPOnly,
			Expires: int64(c.Expires),
		})
	}
	return cookies, nil
}

func hasAllCookies(cookies []*provider.Cookie, names []string) bool {
	have := make(map[string]bool)
	for _, c := range cookies {
		have[c.Name] = true
	}
	for _, n := range names {
		if !have[n] {
			return false
		}
	}
	return true
}
