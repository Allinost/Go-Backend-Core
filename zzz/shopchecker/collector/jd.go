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
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

const (
	jdProductURL = "https://item.m.jd.com/product/%s.html"
	jdLoginURL   = "https://plogin.m.jd.com/login/login?appid=2193"
)

type JDCollector struct {
	client      *http.Client
	browserPath string
	remoteURL   string
	mu          sync.Mutex
}

func NewJDCollector() *JDCollector {
	return &JDCollector{
		client: &http.Client{
			Timeout: 5 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return http.ErrUseLastResponse
				}
				return nil
			},
		},
		browserPath: findBrowser(),
		remoteURL:   os.Getenv("CHROME_REMOTE_URL"),
	}
}

func (c *JDCollector) Platform() string { return "jd" }

func (c *JDCollector) FetchPrice(ctx context.Context, rawURL string) (*PriceResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	skuID := resolveSkuID(ctx, c.client, rawURL)
	if skuID == "" {
		return nil, fmt.Errorf("jd: 无法从 URL 解析商品 SKU")
	}

	price, origPrice := c.fetchPriceHTTP(ctx, skuID)
	title := c.fetchTitleHTTP(ctx, skuID)
	stock := "unknown"

	if price > 0 && title != "" {
		return &PriceResult{
			CurrentPrice:  price,
			OriginalPrice: origPrice,
			Title:         title,
			StockStatus:   stock,
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
				FetchedAt:     time.Now(),
			}, nil
		}
		return nil, fmt.Errorf("jd: %v (sku=%s)", err, skuID)
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

func (c *JDCollector) fetchPriceHTTP(ctx context.Context, skuID string) (float64, float64) {
	endpoints := []string{
		fmt.Sprintf("https://p.3.cn/prices/mgets?skuIds=J_%s&type=1", skuID),
		fmt.Sprintf("http://p.3.cn/prices/mgets?skuIds=J_%s&type=1", skuID),
		fmt.Sprintf("https://p.3.cn/prices/mgets?skuIds=%s&type=1", skuID),
	}
	for _, apiURL := range endpoints {
		p, op, err := c.callPriceAPI(ctx, apiURL)
		if err == nil && p > 0 {
			return p, op
		}
	}
	return 0, 0
}

func (c *JDCollector) callPriceAPI(ctx context.Context, apiURL string) (float64, float64, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://item.jd.com/")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Cookie", "__jdu=1713072000000000000; ipLoc-djd=1-72-2799-0")

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

func (c *JDCollector) fetchTitleHTTP(ctx context.Context, skuID string) string {
	apiURL := fmt.Sprintf("https://cd.jd.com/description/%s.html", skuID)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://item.jd.com/")

	resp, err := c.client.Do(req)
	if err != nil {
		return ""
	}
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
	return ""
}

// ---------- 浏览器采集（通过 Login3rd 获取 session） ----------

func (c *JDCollector) fetchPriceBrowser(ctx context.Context, skuID string) (*PriceResult, error) {
	pageURL := fmt.Sprintf(jdProductURL, skuID)

	// 从 Login3rd 获取京东 session cookie
	var sessionCookies []*provider.Cookie
	if svc := c.getLogin3rdSvc(); svc != nil {
		accounts, _, _, err := svc.ListAccounts(ctx, &login3rd.ListAccountsReq{
			PlatformID: "jd",
			Page:       1,
			PageSize:   1,
		})
		if err == nil && len(accounts) > 0 {
			acct := accounts[0]
			if acct.SessionData != nil && acct.VerifiedStatus == "valid" {
				for _, sc := range acct.SessionData.Cookies {
					sessionCookies = append(sessionCookies, &provider.Cookie{
						Name: sc.Name, Value: sc.Value, Domain: sc.Domain,
						Path: sc.Path, Secure: sc.Secure, HTTPOnly: sc.HTTPOnly,
					})
				}
			}
		}
	}

	// 创建临时浏览器上下文
	tabCtx, cancelTab, err := c.createTempBrowser(ctx)
	if err != nil {
		return nil, err
	}
	defer cancelTab()

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

	// 导航到商品页
	if err := chromedp.Run(tabCtx,
		chromedp.Navigate(pageURL),
		chromedp.WaitReady("body"),
		chromedp.Sleep(3*time.Second),
	); err != nil {
		return nil, fmt.Errorf("浏览器加载页面失败: %w", err)
	}

	// 检查是否被重定向到登录页
	var currentURL string
	chromedp.Run(tabCtx, chromedp.Location(&currentURL))
	if strings.Contains(currentURL, "plogin") || strings.Contains(currentURL, "passport") || strings.Contains(currentURL, "risk_handler") {
		if svc := c.getLogin3rdSvc(); svc != nil && len(sessionCookies) > 0 {
			svc.SendWebhookNotification(ctx, 0, "[ShopChecker] 京东登录已过期",
				"京东 session 已失效，请通过 Login3rd 重新登录")
		}
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

// createTempBrowser 创建临时浏览器上下文（无持久化状态）
func (c *JDCollector) createTempBrowser(ctx context.Context) (context.Context, context.CancelFunc, error) {
	if c.remoteURL != "" {
		allocCtx, _ := chromedp.NewRemoteAllocator(ctx, c.remoteURL)
		tabCtx, cancel := chromedp.NewContext(allocCtx)
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
		allocCtx, _ := chromedp.NewExecAllocator(ctx, opts...)
		tabCtx, cancel := chromedp.NewContext(allocCtx)
		return tabCtx, cancel, nil
	}
	return nil, nil, fmt.Errorf("未找到浏览器（Chrome 或远程连接）")
}

func (c *JDCollector) Close() {}

func (c *JDCollector) scrapeProduct(browserCtx context.Context) (*PriceResult, error) {
	var pageHTML string
	var pageTitle string
	var priceStr string
	var stateJSON string
	var currentURL string

	err := chromedp.Run(browserCtx,
		chromedp.Sleep(1*time.Second),

		chromedp.ActionFunc(func(ctx context.Context) error {
			_ = chromedp.WaitVisible(`[class*="price"], [class*="Price"]`, chromedp.ByQueryAll).Do(ctx)
			return nil
		}),
		chromedp.Sleep(2*time.Second),

		chromedp.Evaluate(`document.querySelector('[class*="price"]')?.innerText || ''`, &priceStr),
		chromedp.Title(&pageTitle),
		chromedp.Location(&currentURL),

		chromedp.Evaluate(`(function() {
			var m = document.documentElement.outerHTML.match(/window\.__INITIAL_STATE__\s*=\s*({.*?});/);
			return m ? m[1] : '';
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
		Title:       extractBrowserTitle(pageHTML, pageTitle),
		StockStatus: extractStockStatus(pageHTML),
		FetchedAt:   time.Now(),
	}

	price, origPrice := parsePriceFromStr(priceStr)
	if price == 0 {
		price, origPrice = extractBrowserPrice(pageHTML)
	}
	if price == 0 && stateJSON != "" {
		price, origPrice = parsePriceFromState(stateJSON)
	}
	result.CurrentPrice = price
	result.OriginalPrice = origPrice

	return result, nil
}

func parsePriceFromStr(s string) (float64, float64) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "¥")
	s = strings.TrimPrefix(s, "￥")
	var price float64
	fmt.Sscanf(s, "%f", &price)
	if price > 0 {
		return price, 0
	}
	return 0, 0
}

func parsePriceFromState(stateJSON string) (float64, float64) {
	var state struct {
		Item struct {
			Price struct {
				P string `json:"p"`
				M string `json:"m"`
			} `json:"price"`
		} `json:"item"`
	}
	if json.Unmarshal([]byte(stateJSON), &state) == nil && state.Item.Price.P != "" {
		var price, orig float64
		fmt.Sscanf(state.Item.Price.P, "%f", &price)
		fmt.Sscanf(state.Item.Price.M, "%f", &orig)
		if price > 0 {
			return price, orig
		}
	}

	var simple struct {
		WarePrice string `json:"warePrice"`
	}
	if json.Unmarshal([]byte(stateJSON), &simple) == nil && simple.WarePrice != "" {
		var price float64
		fmt.Sscanf(simple.WarePrice, "%f", &price)
		return price, 0
	}
	return 0, 0
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
