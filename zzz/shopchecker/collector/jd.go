package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Allinost/go-backend-core/internal/modules"
	"github.com/Allinost/go-backend-core/zzz/login3rd"
	"github.com/Allinost/go-backend-core/zzz/login3rd/provider"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

const (
	jdProductURL = "https://item.m.jd.com/product/%s.html"
	jdDesktopURL = "https://item.jd.com/%s.html"
	jdLoginURL   = "https://passport.jd.com/new/login.aspx?ReturnUrl=https%3A%2F%2Fwww.jd.com"
)

type JDCollector struct {
	client      *http.Client
	browserPath string
	remoteURL   string
	platformID  string
	mobile      bool
	mu          sync.Mutex
}

// NewJDCollector 创建京东采集器（桌面版页面 + 京东平台 session）
func NewJDCollector() *JDCollector {
	return newJDCollector("jd", false)
}

// NewQixianCollector 创建七鲜采集器（移动版页面 + 七鲜平台 session，登录走移动版京东）
func NewQixianCollector() *JDCollector {
	return newJDCollector("qixian", true)
}

// NewJDMobileCollector 创建京东移动版采集器（移动版页面 + jdm 平台 session，登录走移动版京东）
func NewJDMobileCollector() *JDCollector {
	return newJDCollector("jdm", true)
}

func newJDCollector(platformID string, mobile bool) *JDCollector {
	return &JDCollector{
		client: &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return http.ErrUseLastResponse
				}
				return nil
			},
		},
		browserPath: findBrowser(),
		remoteURL:   os.Getenv("CHROME_REMOTE_URL"),
		platformID:  platformID,
		mobile:      mobile,
	}
}

func (c *JDCollector) Platform() string { return c.platformID }

func (c *JDCollector) FetchPrice(ctx context.Context, rawURL string) (*PriceResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	skuID := resolveSkuID(ctx, c.client, rawURL)
	if skuID == "" {
		return nil, fmt.Errorf("jd: 无法从 URL 解析商品 SKU")
	}

	// HTTP 阶段：独立 15s 预算，不消耗浏览器阶段的 90s
	httpCtx, httpCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer httpCancel()

	// 注入登录 session cookie，使价格/标题接口走已登录快路径
	sessionCookie := sessionCookieHeader(c.getSessionCookies(ctx))
	fmt.Fprintf(os.Stderr, "[%s] FetchPrice sku=%s session_cookie=%s\n", c.platformID, skuID, fmt.Sprintf("%t(len=%d)", sessionCookie != "", len(sessionCookie)))

	price, origPrice := c.fetchPriceHTTP(httpCtx, skuID, sessionCookie)
	title := c.fetchTitleHTTP(httpCtx, skuID, sessionCookie)
	fmt.Fprintf(os.Stderr, "[jd] HTTP 阶段完成 sku=%s price=%v orig=%v title=%q\n", skuID, price, origPrice, title)
	stock := "unknown"
	canonicalURL := fmt.Sprintf(jdProductURL, skuID)

	if price > 0 && title != "" {
		return &PriceResult{
			CurrentPrice:  price,
			OriginalPrice: origPrice,
			Title:         title,
			StockStatus:   stock,
			ProductURL:    canonicalURL,
			FetchedAt:     time.Now(),
		}, nil
	}

	result, err := c.fetchPriceBrowser(ctx, skuID)
	if err != nil {
		if price > 0 {
			return &PriceResult{
				CurrentPrice:  price,
				OriginalPrice: origPrice,
				Title:         title,
				StockStatus:   stock,
				ProductURL:    canonicalURL,
				FetchedAt:     time.Now(),
			}, nil
		}
		return nil, fmt.Errorf("jd: %v (sku=%s)", err, skuID)
	}
	if result.ProductURL == "" {
		result.ProductURL = canonicalURL
	}
	return result, nil
}

// ---------- SKU 解析 ----------

var skuRegexes = []*regexp.Regexp{
	regexp.MustCompile(`/(\d+)\.html`),
	regexp.MustCompile(`product[/](\d{6,})`),
	regexp.MustCompile(`sku[=_/](\d{6,})`),
	regexp.MustCompile(`wareId[=_/](\d{6,})`),
}

var shortCodeRe = regexp.MustCompile(`3\.cn/([\w-]+)`)

var digitFallbackRe = regexp.MustCompile(`(\d{6,15})`)

func resolveSkuID(ctx context.Context, client *http.Client, rawURL string) string {
	cleanURL := stripTrackingParams(rawURL)

	if id := extractSkuFromPath(cleanURL); id != "" {
		return id
	}

	if id := digFallback(cleanURL); id != "" {
		return id
	}

	if matched := shortCodeRe.FindStringSubmatch(rawURL); len(matched) >= 2 {
		if id := followAndExtract(ctx, client, rawURL); id != "" {
			return id
		}
	}

	if id := decodeShortCode(rawURL); id != "" {
		return id
	}

	return ""
}

func extractSkuFromPath(pageURL string) string {
	u, err := url.Parse(pageURL)
	if err != nil {
		return ""
	}
	for _, re := range skuRegexes {
		m := re.FindStringSubmatch(u.Path)
		if len(m) >= 2 {
			return m[1]
		}
	}
	return ""
}

func digFallback(rawURL string) string {
	m := digitFallbackRe.FindStringSubmatch(rawURL)
	if len(m) >= 2 {
		return m[1]
	}
	return ""
}

func decodeShortCode(rawURL string) string {
	m := shortCodeRe.FindStringSubmatch(rawURL)
	if len(m) < 2 {
		return ""
	}
	code := m[1]
	var num int64
	for _, c := range code {
		n := base62Val(c)
		if n < 0 {
			return ""
		}
		num = num*62 + int64(n)
	}
	if num > 10000 && num < 100000000000000 {
		return strconv.FormatInt(num, 10)
	}
	return ""
}

func base62Val(c rune) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'z':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'Z':
		return int(c-'A') + 36
	default:
		return -1
	}
}

func followAndExtract(ctx context.Context, client *http.Client, urlStr string) string {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "text/html,*/*")

	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.Request != nil && resp.Request.URL != nil {
		final := resp.Request.URL.String()
		if id := extractSkuFromPath(final); id != "" {
			return id
		}
		if id := digFallback(final); id != "" {
			return id
		}
	}
	return ""
}

func stripTrackingParams(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	q := u.Query()
	trackingKeys := []string{
		"utm_source", "utm_medium", "utm_campaign", "utm_term", "utm_content",
		"utm_user", "gx", "gxd", "ad_od", "jkl",
		"shareid", "share", "from", "from_type",
	}
	for _, k := range trackingKeys {
		q.Del(k)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// ---------- HTTP 价格获取 ----------

func (c *JDCollector) fetchPriceHTTP(ctx context.Context, skuID, sessionCookie string) (float64, float64) {
	endpoints := []string{
		fmt.Sprintf("https://p.3.cn/prices/mgets?skuIds=J_%s&type=1", skuID),
		fmt.Sprintf("http://p.3.cn/prices/mgets?skuIds=J_%s&type=1", skuID),
		fmt.Sprintf("https://p.3.cn/prices/mgets?skuIds=%s&type=1", skuID),
		fmt.Sprintf("https://pe.3.cn/prices/mgets?skuIds=J_%s", skuID),
		fmt.Sprintf("https://item.jd.com/%s.html", skuID),
		fmt.Sprintf("https://item.m.jd.com/product/%s.html", skuID),
	}
	// 前 4 个是 JSON API，后 2 个是 HTML 页面需解析
	for i, apiURL := range endpoints {
		if i < 4 {
			p, op, err := c.callPriceAPI(ctx, apiURL, sessionCookie)
			fmt.Fprintf(os.Stderr, "[jd] priceAPI[%d] %s -> price=%v err=%v\n", i, apiURL, p, err)
			if err == nil && p > 0 {
				return p, op
			}
		} else {
			p, op := c.parsePriceFromPage(ctx, apiURL, sessionCookie)
			fmt.Fprintf(os.Stderr, "[jd] pricePage[%d] %s -> price=%v\n", i, apiURL, p)
			if p > 0 {
				return p, op
			}
		}
	}
	return 0, 0
}

// parsePriceFromPage 从 HTML 页面中解析价格
func (c *JDCollector) parsePriceFromPage(ctx context.Context, pageURL, sessionCookie string) (float64, float64) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "text/html,*/*")
	if sessionCookie != "" {
		req.Header.Set("Cookie", sessionCookie)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return 0, 0
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	html := string(body)
	re := regexp.MustCompile(`"jdPrice"\s*:\s*\{[^}]*"p"\s*:\s*"([\d.]+)"`)
	if m := re.FindStringSubmatch(html); len(m) >= 2 {
		var price float64
		fmt.Sscanf(m[1], "%f", &price)
		if price > 0 {
			re2 := regexp.MustCompile(`"jdPrice"\s*:\s*\{[^}]*"m"\s*:\s*"([\d.]+)"`)
			var orig float64
			if m2 := re2.FindStringSubmatch(html); len(m2) >= 2 {
				fmt.Sscanf(m2[1], "%f", &orig)
			}
			return price, orig
		}
	}
	re2 := regexp.MustCompile(`<span[^>]*class="p-price"[^>]*>\s*¥?([\d.]+)`)
	if m := re2.FindStringSubmatch(html); len(m) >= 2 {
		var price float64
		fmt.Sscanf(m[1], "%f", &price)
		return price, 0
	}
	return 0, 0
}

func (c *JDCollector) callPriceAPI(ctx context.Context, apiURL, sessionCookie string) (float64, float64, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://item.jd.com/")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Cookie", "__jdu=1713072000000000000; ipLoc-djd=1-72-2799-0")
	if sessionCookie != "" {
		req.Header.Set("Cookie", req.Header.Get("Cookie")+"; "+sessionCookie)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	raw := strings.TrimSpace(string(body))
	if raw == "" || raw == "null" || raw == "[]" {
		return 0, 0, fmt.Errorf("empty response")
	}

	var arr []struct {
		P string `json:"p"`
		M string `json:"m"`
	}
	if json.Unmarshal(body, &arr) == nil && len(arr) > 0 {
		var price, orig float64
		fmt.Sscanf(arr[0].P, "%f", &price)
		fmt.Sscanf(arr[0].M, "%f", &orig)
		if price > 0 {
			return price, orig, nil
		}
	}

	return 0, 0, fmt.Errorf("no price in response")
}

// ---------- HTTP 标题获取 ----------

func (c *JDCollector) fetchTitleHTTP(ctx context.Context, skuID, sessionCookie string) string {
	// 尝试 JSON API
	apiURL := fmt.Sprintf("https://cd.jd.com/description/%s.html", skuID)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://item.jd.com/")
	if sessionCookie != "" {
		req.Header.Set("Cookie", sessionCookie)
	}

	resp, err := c.client.Do(req)
	if err == nil {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		var data struct {
			WareShowDesc string `json:"wareShowDesc"`
		}
		if json.Unmarshal(body, &data) == nil && data.WareShowDesc != "" {
			return data.WareShowDesc
		}
		re := regexp.MustCompile(`"Name":"([^"]+)"`)
		m := re.FindStringSubmatch(string(body))
		if len(m) >= 2 {
			return m[1]
		}
	}

	// 降级：从产品页 HTML 解析
	pageURL := fmt.Sprintf("https://item.m.jd.com/product/%s.html", skuID)
	req2, _ := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	req2.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	if sessionCookie != "" {
		req2.Header.Set("Cookie", sessionCookie)
	}
	resp2, err := c.client.Do(req2)
	if err != nil {
		return ""
	}
	defer resp2.Body.Close()
	body2, _ := io.ReadAll(resp2.Body)
	html := string(body2)
	re := regexp.MustCompile(`"name"\s*:\s*"([^"]+)"`)
	if m := re.FindStringSubmatch(html); len(m) >= 2 {
		t := strings.TrimSpace(m[1])
		if t != "" {
			return t
		}
	}
	re2 := regexp.MustCompile(`<title>([^<]+)</title>`)
	if m := re2.FindStringSubmatch(html); len(m) >= 2 {
		t := strings.TrimSpace(m[1])
		t = strings.TrimSuffix(t, "-京东")
		t = strings.TrimSpace(t)
		if t != "" {
			return t
		}
	}
	return ""
}

// ---------- 浏览器采集（通过 Login3rd 获取 session） ----------

func (c *JDCollector) fetchPriceBrowser(ctx context.Context, skuID string) (*PriceResult, error) {
	// 移动版渠道走 item.m.jd.com，否则用桌面版页面（结构规整、含 pageConfig/价格）
	pageURL := fmt.Sprintf(jdDesktopURL, skuID)
	if c.mobile {
		pageURL = fmt.Sprintf(jdProductURL, skuID)
	}

	// 从 Login3rd 获取京东 session cookie
	sessionCookies := c.getSessionCookies(ctx)

	// 创建临时浏览器上下文
	tabCtx, cancelTab, err := c.createTempBrowser(ctx)
	if err != nil {
		return nil, err
	}
	defer cancelTab()

	// 移动版渠道使用移动端浏览器身份，其余用 Windows 桌面 Chrome 身份，避免被识别成机器身份触发验证
	jdUA := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
	width, height := 1440, 900
	if c.mobile {
		jdUA = "Mozilla/5.0 (iPhone; CPU iPhone OS 16_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.6 Mobile/15E148 Safari/604.1"
		width, height = 390, 844
	}
	if err := chromedp.Run(tabCtx,
		emulation.SetUserAgentOverride(jdUA),
		emulation.SetDeviceMetricsOverride(int64(width), int64(height), 1, false),
	); err != nil {
		return nil, fmt.Errorf("设置浏览器身份失败: %w", err)
	}

	// 注入 cookie
	if len(sessionCookies) > 0 {
		for _, sc := range sessionCookies {
			chromedp.Run(tabCtx,
				network.SetCookie(sc.Name, sc.Value).
					WithDomain(".jd.com").
					WithPath("/"),
			)
		}
	}

	// 导航到商品页（Navigate 可能因桌面页 load 事件迟迟不触发而超时，但 body 未必不可用，
	// 因此超时后仍继续轮询 body，预算 45s）
	navDeadline := time.Now().Add(45 * time.Second)
	navCtx, navCancel := context.WithTimeout(tabCtx, 45*time.Second)
	defer navCancel()
	if err := chromedp.Run(navCtx, chromedp.Navigate(pageURL)); err != nil {
		fmt.Fprintf(os.Stderr, "[jd] Navigate 未在预期内完成（%v），继续轮询 body\n", err)
	}
	bodyOK := false
	for !bodyOK {
		if time.Now().After(navDeadline) {
			return nil, fmt.Errorf("浏览器加载页面失败: body 未就绪（%s）", pageURL)
		}
		if err := chromedp.Run(tabCtx, chromedp.Evaluate(`!!document.body && document.readyState !== 'loading'`, &bodyOK)); err != nil {
			bodyOK = false
		}
		if !bodyOK {
			time.Sleep(500 * time.Millisecond)
		}
	}
	chromedp.Run(tabCtx, chromedp.Sleep(3*time.Second))

	// 检查是否被重定向到登录页
	var currentURL string
	chromedp.Run(tabCtx, chromedp.Location(&currentURL))
	fmt.Fprintf(os.Stderr, "[jd] 浏览器导航完成 currentURL=%s\n", currentURL)
	if strings.Contains(currentURL, "plogin") || strings.Contains(currentURL, "passport") || strings.Contains(currentURL, "risk_handler") {
		// 登录失效告警由 shopchecker service 统一发送（复用商品绑定的 webhook），此处仅返回错误
		return nil, fmt.Errorf("需要登录（session 已过期）")
	}

	return c.scrapeProduct(tabCtx)
}

// getLogin3rdSvc 懒加载 Login3rd 服务（避免模块初始化顺序依赖）
func (c *JDCollector) getLogin3rdSvc() *login3rd.Service {
	mod := modules.Get("login3rd")
	if mod == nil {
		return nil
	}
	m, ok := mod.(interface{ GetService() *login3rd.Service })
	if !ok {
		return nil
	}
	return m.GetService()
}

// getSessionCookies 从 Login3rd 获取京东账号的登录 cookie（仅 verified=valid 时返回）
func (c *JDCollector) getSessionCookies(ctx context.Context) []*provider.Cookie {
	svc := c.getLogin3rdSvc()
	if svc == nil {
		return nil
	}
	accounts, _, _, err := svc.ListAccounts(ctx, &login3rd.ListAccountsReq{
		PlatformID: c.platformID,
		Page:       1,
		PageSize:   1,
	})
	if err != nil || len(accounts) == 0 {
		return nil
	}
	acct := accounts[0]
	if acct.SessionData == nil || acct.VerifiedStatus != "valid" {
		return nil
	}
	var cookies []*provider.Cookie
	for _, sc := range acct.SessionData.Cookies {
		cookies = append(cookies, &provider.Cookie{
			Name: sc.Name, Value: sc.Value, Domain: sc.Domain,
			Path: sc.Path, Secure: sc.Secure, HTTPOnly: sc.HTTPOnly,
		})
	}
	return cookies
}

// sessionCookieHeader 将 session cookie 拼成 HTTP Cookie 头
func sessionCookieHeader(cookies []*provider.Cookie) string {
	parts := make([]string, 0, len(cookies))
	for _, sc := range cookies {
		if sc != nil && sc.Name != "" && sc.Value != "" {
			parts = append(parts, sc.Name+"="+sc.Value)
		}
	}
	return strings.Join(parts, "; ")
}

// createTempBrowser 创建临时浏览器上下文（无持久化状态）
func (c *JDCollector) createTempBrowser(ctx context.Context) (context.Context, context.CancelFunc, error) {
	if c.remoteURL != "" {
		allocCtx, aCancel := chromedp.NewRemoteAllocator(ctx, c.remoteURL)
		tabCtx, cancel := chromedp.NewContext(allocCtx)
		_ = aCancel
		select {
		case <-tabCtx.Done():
			return nil, nil, fmt.Errorf("远程浏览器连接失败（%s）: %w", c.remoteURL, tabCtx.Err())
		default:
		}
		return tabCtx, cancel, nil
	}
	if c.browserPath != "" {
		profileDir, err := os.MkdirTemp("", "jd-scrape-*")
		if err != nil {
			return nil, nil, err
		}
		opts := append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.ExecPath(c.browserPath),
			chromedp.NoSandbox,
			chromedp.UserDataDir(profileDir),
			chromedp.Flag("disable-blink-features", "AutomationControlled"),
			chromedp.Flag("disable-dev-shm-usage", true),
			chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"),
		)
		allocCtx, aCancel := chromedp.NewExecAllocator(ctx, opts...)
		tabCtx, cancel := chromedp.NewContext(allocCtx)
		_ = aCancel
		select {
		case <-tabCtx.Done():
			return nil, nil, fmt.Errorf("本地浏览器启动失败（%s）: %w", c.browserPath, tabCtx.Err())
		default:
		}
		return tabCtx, cancel, nil
	}
	return nil, nil, fmt.Errorf("未找到浏览器（Chrome 或远程连接）")
}

func (c *JDCollector) Close() {}

func (c *JDCollector) scrapeProduct(browserCtx context.Context) (*PriceResult, error) {
	var pageHTML string
	var pageTitle string
	var priceJSON string
	var stateJSON string
	var currentURL string
	var mobileInfoJSON string

	// 加 20s 超时隔离，不耗尽外层 90s
	scrapeCtx, scrapeCancel := context.WithTimeout(browserCtx, 20*time.Second)
	defer scrapeCancel()

	err := chromedp.Run(scrapeCtx,
		chromedp.Sleep(1*time.Second),
		chromedp.Evaluate(`(function() {
			// 提取按文案标签标注的价格 + pageConfig 价格相关字段（供定位国补/到手价）
			var body = document.body ? document.body.innerText : '';
			var re = /[¥￥]\s*(\d+(?:\.\d{1,2})?)/;
			var reNum = /(\d+(?:\.\d{1,2})?)/;
			function pick(label) {
				var i = body.indexOf(label);
				if (i === -1) return '';
				var before = body.slice(Math.max(0, i - 80), i);
				var after = body.slice(i, i + 140);
				var seg = before + ' ' + after;
				var m = seg.match(re) || seg.match(reNum);
				return m ? m[1] : '';
			}
			var labels = ['国补领后价','国补后到手价','国补到手价','补贴后到手价','国补后',
				'到手价','实付','券后','领券价','PLUS到手价','日常价','京东价','划线价','原价'];
			var found = {};
			for (var i = 0; i < labels.length; i++) found[labels[i]] = pick(labels[i]);
			var main = '';
			var m3 = body.match(re);
			if (m3) main = m3[1];
			var skuName = '';
			var el = document.querySelector('.sku-name');
			if (el) skuName = el.innerText.trim();
			var priceKeys = {};
			(function walk(obj, prefix) {
				if (!obj || typeof obj !== 'object' || prefix.length > 120) return;
				for (var k in obj) {
					if (/price|promo|subsidy|coupon|discount|到手|diff|plus/i.test(k)) {
						var v = obj[k];
						var sv = (typeof v === 'object') ? JSON.stringify(v) : String(v);
						if (sv && sv.length < 200) priceKeys[prefix + k] = sv;
					}
					if (typeof obj[k] === 'object' && obj[k] !== null) walk(obj[k], prefix + k + '.');
				}
			})(window.pageConfig || null, '');
			return JSON.stringify({found: found, main: main, skuName: skuName, priceKeys: priceKeys});
		})()`, &priceJSON),
		chromedp.Evaluate(`(function() {
			// 移动版页面：window.itemPrice / window._itemInfo 持有解密后的明文价格（DOM 价格为蝌蚪字体加密）
			var out = {};
			try {
				if (window.itemPrice && window.itemPrice.jdPrice) out.jdPrice = window.itemPrice.jdPrice;
			} catch (e) {}
			try {
				if (window._itemInfo && window._itemInfo.priceFloor && window._itemInfo.priceFloor.ext
					&& window._itemInfo.priceFloor.ext.jdPrice) out.extJdPrice = window._itemInfo.priceFloor.ext.jdPrice;
			} catch (e) {}
			try {
				if (window._itemInfo && window._itemInfo.product && window._itemInfo.product.skuName)
					out.skuName = window._itemInfo.product.skuName;
			} catch (e) {}
			try {
				if (window.stockState !== undefined) out.stockState = window.stockState;
			} catch (e) {}
			// 国补信息：newBanner 腰带含补贴文案（"领国家补贴减￥500"），priceFloor 含补贴类型/PLUS 优惠
			try {
				var nb = window._itemInfo && window._itemInfo.newBanner;
				if (nb && nb.fData) {
					if (nb.fData.rightDownText) out.subsidyText = nb.fData.rightDownText;
					if (nb.fData.bannerName) out.bannerName = nb.fData.bannerName;
				}
			} catch (e) {}
			try {
				var pf = window._itemInfo && window._itemInfo.priceFloor;
				if (pf) {
					if (pf.purchaseSubType) out.purchaseSubType = pf.purchaseSubType;
					var ext = pf.ext || {};
					if (ext.realPriceExt && ext.realPriceExt.JD && ext.realPriceExt.JD.plusDiscountAmount)
						out.plusDiscountAmount = ext.realPriceExt.JD.plusDiscountAmount;
				}
			} catch (e) {}
			return JSON.stringify(out);
		})()`, &mobileInfoJSON),
		chromedp.Title(&pageTitle),
		chromedp.Location(&currentURL),
		chromedp.Evaluate(`(function() {
			try { return JSON.stringify(window.__INITIAL_STATE__ || null); } catch (e) { return ''; }
		})()`, &stateJSON),
		chromedp.OuterHTML("html", &pageHTML),
	)
	if err != nil {
		return nil, fmt.Errorf("页面采集失败: %w", err)
	}

	if strings.Contains(currentURL, "plogin") || strings.Contains(currentURL, "risk_handler") || strings.Contains(currentURL, "passport") {
		return nil, fmt.Errorf("采集时被重定向到登录/风险页: %s", currentURL)
	}
	if strings.Contains(pageTitle, "京东验证") || strings.Contains(pageTitle, "登录注册") {
		return nil, fmt.Errorf("采集时被反爬拦截: title=%s url=%s", pageTitle, currentURL)
	}

	result := &PriceResult{
		Title:       parseTitleFromState(stateJSON),
		StockStatus: extractStockStatus(pageHTML),
		ProductURL:  currentURL,
		FetchedAt:   time.Now(),
	}
	if s := parseMobileStock(mobileInfoJSON); s != "unknown" {
		result.StockStatus = s
	}
	if result.Title == "" {
		result.Title = parseLabeledSkuName(priceJSON)
	}
	if result.Title == "" {
		if t := parseMobileSkuName(mobileInfoJSON); t != "" {
			result.Title = t
		}
	}
	if result.Title == "" {
		result.Title = extractBrowserTitle(pageHTML, pageTitle)
	}

	// 优先级：国补领后价（由移动版 JS 全局计算）> 国补类到手价 > 到手/实付/券后 > 页面首个 ¥；日常价作为原价
	subsidy, final, daily, main, priceKeys := parseLabeledPrices(priceJSON)
	govPrice, govOrig := parseMobileEffectivePrice(mobileInfoJSON)
	price, origPrice := govPrice, govOrig
	if price == 0 {
		if subsidy > 0 {
			price = subsidy
		} else if final > 0 {
			price = final
		} else if main > 0 {
			price = main
		}
	}
	// 移动版明文价格（window.itemPrice.jdPrice）优先兜底，DOM 蝌蚪字体无法直接解析
	if price == 0 {
		if jp := parseMobileJdPrice(mobileInfoJSON); jp > 0 {
			price = jp
		}
	}
	if price > 0 && daily > 0 && daily != price && daily > price*0.5 {
		origPrice = daily
	}
	if price == 0 {
		price, origPrice = extractBrowserPrice(pageHTML)
	}
	if price == 0 && stateJSON != "" {
		price, origPrice = parsePriceFromState(stateJSON)
	}
	result.CurrentPrice = price
	result.OriginalPrice = origPrice

	fmt.Fprintf(os.Stderr, "[jd] 采集结果 title=%q price=%v orig=%v subsidy=%v final=%v daily=%v stock=%s state_len=%d url=%s\n",
		result.Title, result.CurrentPrice, result.OriginalPrice, subsidy, final, daily, result.StockStatus, len(stateJSON), currentURL)
	for k, v := range priceKeys {
		fmt.Fprintf(os.Stderr, "[jd] pageConfig %s = %s\n", k, v)
	}

	return result, nil
}

// parseMobileEffectivePrice 从移动版页面 JS 全局计算到手价（PLUS 会员价）
// 规则：
//  1. 国补场景：当前价 = 日常价 - 国补补贴 - PLUS 优惠，原价 = 日常价
//  2. 无国补但存在 PLUS 优惠：当前价 = 日常价 - PLUS 优惠，原价 = 日常价
//  3. 均无：返回 0（走其他兜底路径）
//
// 数据来源：newBanner.fData.rightDownText（如 "领国家补贴减￥500"）、priceFloor.ext.jdPrice、
//
//	priceFloor.ext.realPriceExt.JD.plusDiscountAmount
func parseMobileEffectivePrice(mobileInfoJSON string) (float64, float64) {
	var p struct {
		JdPrice         string `json:"jdPrice"`
		ExtJdPrice      string `json:"extJdPrice"`
		SubsidyText     string `json:"subsidyText"`
		BannerName      string `json:"bannerName"`
		PurchaseSubType string `json:"purchaseSubType"`
		PlusDiscountAmt string `json:"plusDiscountAmount"`
	}
	if json.Unmarshal([]byte(mobileInfoJSON), &p) != nil {
		return 0, 0
	}

	basePrice := parseFloatStr(p.ExtJdPrice)
	if basePrice <= 0 {
		basePrice = parseFloatStr(p.JdPrice)
	}
	if basePrice <= 0 {
		return 0, 0
	}

	subsidyAmt := parseSubsidyText(p.SubsidyText)
	plusAmt := parseFloatStr(p.PlusDiscountAmt)

	// 国补领后价 = 日常价 - 补贴金额，PLUS 会员再叠加优惠
	if subsidyAmt > 0 {
		govPrice := basePrice - subsidyAmt
		if govPrice <= 0 {
			return 0, 0
		}
		if plusAmt > 0 {
			govPrice -= plusAmt
		}
		if govPrice <= 0 {
			return 0, 0
		}
		return govPrice, basePrice
	}

	// 无国补但 PLUS 会员有优惠
	if plusAmt > 0 {
		plusPrice := basePrice - plusAmt
		if plusPrice <= 0 {
			return 0, 0
		}
		return plusPrice, basePrice
	}

	return 0, 0
}

// parseSubsidyText 从国补腰带文案解析补贴金额，如 "领国家补贴减￥500" -> 500
func parseSubsidyText(text string) float64 {
	re := regexp.MustCompile(`[减-]\s*[¥￥]\s*([\d.]+)`)
	if m := re.FindStringSubmatch(text); len(m) >= 2 {
		return parseFloatStr(m[1])
	}
	return 0
}

// parseFloatStr 解析字符串为 float64，失败或非法返回 0
func parseFloatStr(s string) float64 {
	s = strings.TrimSpace(strings.Trim(s, "¥￥ "))
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f <= 0 {
		return 0
	}
	return f
}

// parseMobileJdPrice 从移动版页面 JS 全局（window.itemPrice / _itemInfo.priceFloor.ext）解析明文京东价
func parseMobileJdPrice(mobileInfoJSON string) float64 {
	var p struct {
		JdPrice    string `json:"jdPrice"`
		ExtJdPrice string `json:"extJdPrice"`
	}
	if json.Unmarshal([]byte(mobileInfoJSON), &p) != nil {
		return 0
	}
	for _, s := range []string{p.JdPrice, p.ExtJdPrice} {
		var f float64
		if s != "" && strings.Trim(s, "¥￥ ") != "" {
			fmt.Sscanf(strings.Trim(s, "¥￥ "), "%f", &f)
			if f > 0 {
				return f
			}
		}
	}
	return 0
}

// parseMobileSkuName 从移动版页面 JS 全局 _itemInfo.product.skuName 解析商品名
func parseMobileSkuName(mobileInfoJSON string) string {
	var p struct {
		SkuName string `json:"skuName"`
	}
	if json.Unmarshal([]byte(mobileInfoJSON), &p) != nil {
		return ""
	}
	return cleanJdTitle(strings.TrimSpace(p.SkuName))
}

// parseMobileStock 从移动版页面 JS 全局 window.stockState 解析库存状态
// 状态码语义：33=现货, 39/40=有货, 36=预定, 其余=无货
func parseMobileStock(mobileInfoJSON string) string {
	var p struct {
		StockState int `json:"stockState"`
	}
	if json.Unmarshal([]byte(mobileInfoJSON), &p) != nil || p.StockState == 0 {
		return "unknown"
	}
	switch p.StockState {
	case 33, 39, 40:
		return "in_stock"
	case 36:
		return "on_order"
	default:
		return "out_of_stock"
	}
}

// parseLabeledPrices 解析页面上按文案标签提取的价格
func parseLabeledPrices(priceJSON string) (subsidy, final, daily, main float64, priceKeys map[string]string) {
	var p struct {
		Found     map[string]string `json:"found"`
		Main      string            `json:"main"`
		SkuName   string            `json:"skuName"`
		PriceKeys map[string]string `json:"priceKeys"`
	}
	if json.Unmarshal([]byte(priceJSON), &p) != nil {
		return 0, 0, 0, 0, nil
	}
	toF := func(s string) float64 {
		var f float64
		fmt.Sscanf(s, "%f", &f)
		if f > 0 {
			return f
		}
		return 0
	}
	firstOf := func(labels ...string) float64 {
		for _, l := range labels {
			if v := toF(p.Found[l]); v > 0 {
				return v
			}
		}
		return 0
	}
	subsidy = firstOf("国补领后价", "国补后到手价", "国补到手价", "补贴后到手价", "国补后")
	final = firstOf("到手价", "实付", "券后", "领券价", "PLUS到手价")
	daily = firstOf("日常价", "京东价", "划线价", "原价")
	main = toF(p.Main)
	return subsidy, final, daily, main, p.PriceKeys
}

// parseLabeledSkuName 从采集结果中取 .sku-name 文本
func parseLabeledSkuName(priceJSON string) string {
	var p struct {
		SkuName string `json:"skuName"`
	}
	if json.Unmarshal([]byte(priceJSON), &p) != nil {
		return ""
	}
	return cleanJdTitle(strings.TrimSpace(p.SkuName))
}

func parsePriceFromState(stateJSON string) (float64, float64) {
	var root map[string]any
	if json.Unmarshal([]byte(stateJSON), &root) != nil {
		return 0, 0
	}

	// 移动版商品页常见的价格状态路径
	for _, path := range []string{
		"item.item.price", "item.price", "wareInfo.price", "goods.price",
	} {
		v := digJSON(root, strings.Split(path, ".")...)
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		pStr := anyToString(m["p"])
		origStr := anyToString(m["m"])
		if pStr == "" {
			continue
		}
		price, err := strconv.ParseFloat(pStr, 64)
		if err != nil || price <= 0 {
			continue
		}
		var orig float64
		if origStr != "" {
			orig, _ = strconv.ParseFloat(origStr, 64)
		}
		return price, orig
	}

	// 兜底：warePrice 纯字符串
	if wp := anyToString(digJSON(root, "item", "item", "warePrice")); wp != "" {
		if price, err := strconv.ParseFloat(wp, 64); err == nil && price > 0 {
			return price, 0
		}
	}
	return 0, 0
}

// parseTitleFromState 从 __INITIAL_STATE__ 提取商品名
func parseTitleFromState(stateJSON string) string {
	var root map[string]any
	if json.Unmarshal([]byte(stateJSON), &root) != nil {
		return ""
	}
	for _, path := range []string{
		"item.item.skuName", "item.item.name", "item.skuName", "sku.name",
	} {
		t := cleanJdTitle(strings.TrimSpace(anyToString(digJSON(root, strings.Split(path, ".")...))))
		if t != "" {
			return t
		}
	}
	return ""
}

// digJSON 沿 key 路径逐层取 map[string]any 中的值
func digJSON(m map[string]any, keys ...string) any {
	var cur any = m
	for _, k := range keys {
		mm, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = mm[k]
	}
	return cur
}

// anyToString 将 JSON 中的字符串/数字统一转成 string
func anyToString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", t)
	}
}

func extractBrowserPrice(html string) (float64, float64) {
	patterns := []struct {
		priceRe string
		origRe  string
	}{
		{`"jdPrice"\s*:\s*\{[^}]*"p"\s*:\s*"([\d.]+)"`, `"jdPrice"\s*:\s*\{[^}]*"m"\s*:\s*"([\d.]+)"`},
		{`"warePrice"\s*:\s*"([\d.]+)"`, ``},
		{`"priceValue"\s*:\s*"([\d.]+)"`, ``},
		{`class="price"[^>]*>\s*¥?([\d.]+)`, `<del[^>]*>¥?([\d.]+)`},
		{`<span[^>]*class="p-price"[^>]*>\s*¥?([\d.]+)`, `<del[^>]*>¥?([\d.]+)`},
	}

	for _, p := range patterns {
		re := regexp.MustCompile(p.priceRe)
		m := re.FindStringSubmatch(html)
		if len(m) >= 2 {
			var price float64
			fmt.Sscanf(m[1], "%f", &price)
			if price > 0 {
				var orig float64
				if p.origRe != "" {
					re2 := regexp.MustCompile(p.origRe)
					m2 := re2.FindStringSubmatch(html)
					if len(m2) >= 2 {
						fmt.Sscanf(m2[1], "%f", &orig)
					}
				}
				if orig > 0 && orig < price {
					orig = 0
				}
				return price, orig
			}
		}
	}

	return 0, 0
}

func extractBrowserTitle(html, domTitle string) string {
	if domTitle != "" && !strings.Contains(domTitle, "京东") {
		t := cleanJdTitle(domTitle)
		if t != "" {
			return t
		}
	}

	patterns := []string{
		`"name"\s*:\s*"([^"]+)"`,
		`"title"\s*:\s*"([^"]+)"`,
		`<title>([^<]+)</title>`,
	}
	for _, pat := range patterns {
		re := regexp.MustCompile(pat)
		m := re.FindStringSubmatch(html)
		if len(m) >= 2 {
			t := strings.TrimSpace(m[1])
			t = cleanJdTitle(t)
			if t != "" {
				return t
			}
		}
	}
	return ""
}

func extractStockStatus(html string) string {
	lower := strings.ToLower(html)
	switch {
	case strings.Contains(lower, "无货") || strings.Contains(lower, "暂时缺货"):
		return "out_of_stock"
	case strings.Contains(lower, "有货") || strings.Contains(lower, "现货"):
		return "in_stock"
	case strings.Contains(lower, `"stock":1`) || strings.Contains(lower, `"stockState":1`):
		return "in_stock"
	case strings.Contains(lower, `"stock":0`) || strings.Contains(lower, `"stockState":0`):
		return "out_of_stock"
	default:
		return "unknown"
	}
}

func cleanJdTitle(t string) string {
	suffixes := []string{
		"【价格_图片_品牌_报价】-京东",
		"【行情_价格_评价】-京东",
		"-京东",
		"【正品】",
		"【品牌】",
		"【官方】",
	}
	for _, s := range suffixes {
		t = strings.TrimSuffix(t, s)
	}
	t = strings.TrimSpace(t)
	if t == "" || strings.Contains(t, "验证") || strings.Contains(t, "404") || strings.Contains(t, "京东JD.COM") {
		return ""
	}
	// 过滤服务/营销类文案，避免误当商品标题（如 PLUS 180天只换不修、价保、补贴领取等）
	for _, kw := range []string{
		"只换不修", "价保", "价格保护", "以旧换新", "国补", "领券", "PLUS", "PLUS会员",
		"补贴", "售后", "全国联保", "正品保障", "七天无理由", "退货无忧", "京准达",
	} {
		if strings.Contains(t, kw) {
			return ""
		}
	}
	return t
}

// ---------- 浏览器查找 ----------

func findBrowser() string {
	candidates := []string{
		`C:\Program Files\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		filepath.Join(os.Getenv("LOCALAPPDATA"), `Google\Chrome\Application\chrome.exe`),
		`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
		filepath.Join(os.Getenv("LOCALAPPDATA"), `Microsoft\Edge\Application\msedge.exe`),
		"/usr/bin/chromium-browser",
		"/usr/bin/google-chrome",
		"/usr/bin/chromium",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
