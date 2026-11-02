package browser

import (
	"context"
	"fmt"
	"os"
	"strings"
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
	// Verify 可选：拿到 Cookie 后做真实登录校验（例如访问订单页/个人中心）。
	// 用于避免“URL 已跳回首页但仍在等待验证码”被误判为登录成功。
	Verify func(ctx context.Context, session *provider.SessionData) (bool, error)
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

	// 无论是否配置了自动填表，都必须等待用户真正完成登录（远程登录场景尤其重要），
	// 不能在页面刚打开时就把现有 Cookie 当作登录成功。
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

var knownAuthCookies = []string{
	"pt_key", "pt_pin", "thor", // 京东系（新版登录可能只有 thor，没有 pt_key/pt_pin）
	"unb",                                                      // 淘宝/天猫用户数字 ID（匿名态不会出现）
	"pdd_user_id", "pdd_user_uin", "PDDAccessToken", "pdd_vds", // 拼多多
}

// waitLogin 轮询检测登录是否成功。
// 判定顺序：配置的成功 Cookie → 成功 URL 前缀 → 离开登录页且出现已知登录 Cookie。
func (f *LoginFlow) waitLogin(browserCtx context.Context) ([]*provider.Cookie, error) {
	successCookies := []string(nil)
	successURLPrefix := ""
	if f.FormCfg != nil {
		successCookies = f.FormCfg.SuccessCookies
		successURLPrefix = f.FormCfg.SuccessURLPrefix
	}
	fmt.Fprintf(os.Stderr, "[login3rd] waitLogin: 开始, SuccessCookies=%v SuccessURLPrefix=%q\n",
		successCookies, successURLPrefix)

	// URL 跳回首页并不代表登录完成（可能仍在等待验证码/二次验证）。
	// 检测到候选登录态后先等待 60 秒，再用 Cookie 调 Provider.Verify 做真实校验。
	const settleDuration = 60 * time.Second
	timeout := time.After(180 * time.Second)
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	startedAt := time.Now()
	var candidateSince time.Time
	var lastVerifyAt time.Time

	for {
		select {
		case <-browserCtx.Done():
			fmt.Fprintf(os.Stderr, "[login3rd] waitLogin: browserCtx 已取消: %v\n", browserCtx.Err())
			return nil, browserCtx.Err()
		case <-timeout:
			fmt.Fprintf(os.Stderr, "[login3rd] waitLogin: 超时（180s）\n")
			return nil, fmt.Errorf("等待登录超时（180s）")
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

			var currentURL string
			if err := chromedp.Run(opCtx, chromedp.Location(&currentURL)); err == nil {
				fmt.Fprintf(os.Stderr, "[login3rd] waitLogin: 当前 URL=%q\n", currentURL)
			} else {
				fmt.Fprintf(os.Stderr, "[login3rd] waitLogin: Location 失败: %v\n", err)
			}
			opCancel()

			cookieCandidate := currentURL != "" && !isLoginPageURL(currentURL) &&
				hasAnyNamedCookie(cookies, knownAuthCookies)
			urlCandidate := successURLPrefix != "" && currentURL != "" &&
				strings.HasPrefix(currentURL, successURLPrefix)
			successCookieCandidate := len(successCookies) > 0 &&
				hasAllCookies(cookies, successCookies) && currentURL != "" && !isLoginPageURL(currentURL)
			candidate := cookieCandidate || urlCandidate || successCookieCandidate

			// 给页面至少 8 秒完成跳转，避免把登录前的临时 Cookie/跳转误判为候选成功。
			if candidate && time.Since(startedAt) < 8*time.Second {
				candidate = false
			}
			if !candidate {
				// 回到登录/验证码页时重新计时，避免在验证码页上积累稳定时间。
				candidateSince = time.Time{}
				lastVerifyAt = time.Time{}
				continue
			}
			if candidateSince.IsZero() {
				candidateSince = time.Now()
				fmt.Fprintf(os.Stderr, "[login3rd] waitLogin: 检测到候选登录态，等待 %s 后校验 Cookie\n", settleDuration)
			}
			if time.Since(candidateSince) < settleDuration {
				continue
			}
			if f.Verify == nil {
				fmt.Fprintf(os.Stderr, "[login3rd] waitLogin: 未配置 Verify，候选登录态稳定，判定成功\n")
				return cookies, nil
			}
			// 校验本身会访问平台页面，最多每 10 秒一次，避免触发频控。
			if !lastVerifyAt.IsZero() && time.Since(lastVerifyAt) < 10*time.Second {
				continue
			}
			lastVerifyAt = time.Now()
			verifyCtx, verifyCancel := context.WithTimeout(browserCtx, 20*time.Second)
			ok, verifyErr := f.Verify(verifyCtx, cookiesToProviderSession(cookies))
			verifyCancel()
			if verifyErr != nil {
				fmt.Fprintf(os.Stderr, "[login3rd] waitLogin: Cookie 校验失败: %v\n", verifyErr)
				continue
			}
			if !ok {
				fmt.Fprintf(os.Stderr, "[login3rd] waitLogin: Cookie 尚未通过真实登录校验，继续等待\n")
				continue
			}
			fmt.Fprintf(os.Stderr, "[login3rd] waitLogin: 登录成功（Cookie 校验通过）\n")
			return cookies, nil
		}
	}
}

func cookiesToProviderSession(cookies []*provider.Cookie) *provider.SessionData {
	session := &provider.SessionData{Cookies: make([]provider.Cookie, 0, len(cookies))}
	for _, c := range cookies {
		if c != nil {
			session.Cookies = append(session.Cookies, *c)
		}
	}
	return session
}

func isLoginPageURL(raw string) bool {
	lower := strings.ToLower(raw)
	for _, kw := range []string{"login", "passport", "signin", "havanaone", "punish", "captcha", "verify"} {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func hasAnyNamedCookie(cookies []*provider.Cookie, names []string) bool {
	for _, c := range cookies {
		for _, n := range names {
			if c.Name == n && c.Value != "" {
				return true
			}
		}
	}
	return false
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
