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
	fmt.Fprintf(os.Stderr, "[login3rd] Execute: 开始导航到 %s\n", f.LoginURL)

	navCtx, navCancel := context.WithTimeout(browserCtx, 30*time.Second)
	defer navCancel()
	if err := chromedp.Run(navCtx,
		chromedp.Navigate(f.LoginURL),
		chromedp.WaitReady("body"),
		chromedp.Sleep(2*time.Second),
	); err != nil {
		fmt.Fprintf(os.Stderr, "[login3rd] Execute: 导航失败 %v\n", err)
		return nil, fmt.Errorf("导航到登录页失败: %w", err)
	}
	fmt.Fprintf(os.Stderr, "[login3rd] Execute: 导航成功，FormCfg=%v username=%q password=%q\n", f.FormCfg != nil, username, password)

	if f.FormCfg != nil && username != "" && password != "" {
		fmt.Fprintf(os.Stderr, "[login3rd] Execute: 尝试自动填充表单\n")
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
		autoCtx, autoCancel := context.WithTimeout(browserCtx, 10*time.Second)
		if err := chromedp.Run(autoCtx, tasks...); err != nil {
			fmt.Fprintf(os.Stderr, "[login3rd] Execute: 自动填充失败 %v，降级到手动登录\n", err)
			autoCancel()
		} else {
			autoCancel()
			if f.FormCfg.CaptchaSelector == "" && f.FormCfg.SubmitSelector != "" {
				fmt.Fprintf(os.Stderr, "[login3rd] Execute: 尝试自动点击登录按钮\n")
				clickCtx, clickCancel := context.WithTimeout(browserCtx, 10*time.Second)
				if err := chromedp.Run(clickCtx,
					chromedp.Click(f.FormCfg.SubmitSelector),
					chromedp.Sleep(2*time.Second),
				); err != nil {
					fmt.Fprintf(os.Stderr, "[login3rd] Execute: 点击登录失败 %v，降级到手动登录\n", err)
				}
				clickCancel()
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
	fmt.Fprintf(os.Stderr, "[login3rd] waitLogin: 开始, SuccessCookies=%v SuccessURLPrefix=%q\n",
		f.FormCfg.SuccessCookies, f.FormCfg.SuccessURLPrefix)
	timeout := time.After(120 * time.Second)
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-browserCtx.Done():
			fmt.Fprintf(os.Stderr, "[login3rd] waitLogin: browserCtx 已取消: %v\n", browserCtx.Err())
			return nil, browserCtx.Err()
		case <-timeout:
			fmt.Fprintf(os.Stderr, "[login3rd] waitLogin: 超时（120s）\n")
			return nil, fmt.Errorf("等待登录超时（120s）")
		case ts := <-tick.C:
			fmt.Fprintf(os.Stderr, "[login3rd] waitLogin: tick at %s\n", ts.Format("15:04:05"))
			opCtx, opCancel := context.WithTimeout(browserCtx, 10*time.Second)
			cookies, err := f.extractCookies(opCtx)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[login3rd] waitLogin: extractCookies 失败: %v\n", err)
				opCancel()
				continue
			}
			fmt.Fprintf(os.Stderr, "[login3rd] waitLogin: 获取 %d 个 cookie\n", len(cookies))
			for _, c := range cookies {
				fmt.Fprintf(os.Stderr, "[login3rd] waitLogin:   cookie %s=%s...\n", c.Name, safePrefix(c.Value, 20))
			}

			matched := false
			if len(f.FormCfg.SuccessCookies) > 0 {
				if hasAllCookies(cookies, f.FormCfg.SuccessCookies) {
					fmt.Fprintf(os.Stderr, "[login3rd] waitLogin: cookies 匹配成功\n")
					matched = true
				} else {
					fmt.Fprintf(os.Stderr, "[login3rd] waitLogin: cookies 不匹配（需要 %v）\n", f.FormCfg.SuccessCookies)
				}
			}
			if !matched && f.FormCfg.SuccessURLPrefix != "" {
				var currentURL string
				if err := chromedp.Run(opCtx, chromedp.Location(&currentURL)); err == nil {
					fmt.Fprintf(os.Stderr, "[login3rd] waitLogin: 当前 URL=%q\n", currentURL)
					ok := len(currentURL) >= len(f.FormCfg.SuccessURLPrefix) &&
						currentURL[:len(f.FormCfg.SuccessURLPrefix)] == f.FormCfg.SuccessURLPrefix
					fmt.Fprintf(os.Stderr, "[login3rd] waitLogin: URL 前缀匹配 %s = %v\n", f.FormCfg.SuccessURLPrefix, ok)
					if ok {
						matched = true
					}
				} else {
					fmt.Fprintf(os.Stderr, "[login3rd] waitLogin: Location 失败: %v\n", err)
				}
			}
			opCancel()
			if matched {
				fmt.Fprintf(os.Stderr, "[login3rd] waitLogin: 登录成功\n")
				return cookies, nil
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

func safePrefix(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
