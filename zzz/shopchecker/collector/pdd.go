package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
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
	pddCanonicalURL = "https://mobile.yangkeduo.com/goods1.html?goods_id=%s"
	pddMobileUA     = "Mozilla/5.0 (iPhone; CPU iPhone OS 16_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.6 Mobile/15E148 Safari/604.1"
)

// PDDCollector 拼多多价格采集器。
// 拼多多网页强依赖登录与风控，优先 HTTP 快路径，失败后使用浏览器 + login3rd pdd 会话。
type PDDCollector struct {
	client      *http.Client
	browserPath string
	remoteURL   string
	mu          sync.Mutex
}

// NewPDDCollector 创建拼多多价格采集器
func NewPDDCollector() *PDDCollector {
	return &PDDCollector{
		client: &http.Client{
			Timeout: 15 * time.Second,
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

// Platform 返回平台标识
func (c *PDDCollector) Platform() string {
	return "pdd"
}

// FetchPrice 抓取拼多多商品价格
func (c *PDDCollector) FetchPrice(ctx context.Context, rawURL string) (*PriceResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	goodsID := c.resolveGoodsID(ctx, rawURL)
	if goodsID == "" {
		return nil, fmt.Errorf("pdd: 无法从 URL 解析商品 goods_id")
	}
	canonicalURL := fmt.Sprintf(pddCanonicalURL, goodsID)

	sessionCookie := sessionCookieHeader(c.getSessionCookies(ctx))
	httpCtx, httpCancel := context.WithTimeout(context.Background(), 15*time.Second)
	httpResult := c.fetchPageHTTP(httpCtx, canonicalURL, sessionCookie)
	httpCancel()
	if httpResult != nil && httpResult.CurrentPrice > 0 && httpResult.Title != "" {
		httpResult.ProductURL = canonicalURL
		return httpResult, nil
	}

	result, err := c.fetchPriceBrowser(ctx, canonicalURL)
	if err != nil {
		browserLoginErr := strings.Contains(err.Error(), "需要登录") || strings.Contains(err.Error(), "登录/风险")
		if httpResult != nil && httpResult.CurrentPrice > 0 && !(sessionCookie != "" && browserLoginErr) {
			httpResult.ProductURL = canonicalURL
			return httpResult, nil
		}
		return nil, fmt.Errorf("pdd: %v (goods_id=%s)", err, goodsID)
	}
	if result.ProductURL == "" {
		result.ProductURL = canonicalURL
	}
	return result, nil
}

// ---------- 商品 ID 解析 ----------

var pddGoodsIDRe = regexp.MustCompile(`(?i)(?:goods_id|goodsId|gid)=(\d+)`)

func (c *PDDCollector) resolveGoodsID(ctx context.Context, rawInput string) string {
	rawURL := extractPDDURL(rawInput)
	if rawURL == "" {
		return ""
	}
	if m := pddGoodsIDRe.FindStringSubmatch(rawURL); len(m) >= 2 {
		return m[1]
	}
	// 分享短链只有 ps 参数时，跟随一次跳转获取 goods_id。
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", pddMobileUA)
	resp, err := c.client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.Request != nil && resp.Request.URL != nil {
		final := resp.Request.URL.String()
		if m := pddGoodsIDRe.FindStringSubmatch(final); len(m) >= 2 {
			return m[1]
		}
	}
	return ""
}

func extractPDDURL(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}
	re := regexp.MustCompile(`https?://[^\s"'<>]+`)
	for _, candidate := range re.FindAllString(input, -1) {
		if strings.Contains(candidate, "yangkeduo.com") || strings.Contains(candidate, "pinduoduo.com") {
			return strings.TrimRight(candidate, ",.;）)")
		}
	}
	if strings.Contains(input, "yangkeduo.com") && !strings.Contains(input, " ") {
		return input
	}
	return ""
}

// ---------- HTTP 快路径 ----------

func (c *PDDCollector) fetchPageHTTP(ctx context.Context, pageURL, sessionCookie string) *PriceResult {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", pddMobileUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	if sessionCookie != "" {
		req.Header.Set("Cookie", sessionCookie)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	html := string(body)
	price, origPrice := extractPDDPrice(html)
	if price <= 0 {
		return nil
	}
	return &PriceResult{
		CurrentPrice:  price,
		OriginalPrice: origPrice,
		BasePrice:     firstPositive(origPrice, price),
		PriceSource:   "http",
		Title:         extractPDDTitle(html),
		StockStatus:   extractPDDStock(html),
		FetchedAt:     time.Now(),
	}
}

func extractPDDTitle(html string) string {
	patterns := []string{
		`<title>([^<]+)</title>`,
		`"goodsName"\s*:\s*"([^"]+)"`,
		`"goods_name"\s*:\s*"([^"]+)"`,
	}
	for _, pat := range patterns {
		re := regexp.MustCompile(pat)
		if m := re.FindStringSubmatch(html); len(m) >= 2 {
			title := strings.TrimSpace(m[1])
			title = strings.TrimSuffix(title, "-拼多多")
			title = strings.TrimSpace(title)
			if title != "" && !strings.Contains(title, "拼多多") {
				return title
			}
		}
	}
	return ""
}

func extractPDDPrice(html string) (float64, float64) {
	patterns := []struct {
		priceRe string
		origRe  string
		cents   bool
	}{
		{`"minOnSaleGroupPrice"\s*:\s*"?(\d+(?:\.\d+)?)"?`, `"linePrice"\s*:\s*"?(\d+(?:\.\d+)?)"?`, true},
		{`"minGroupPrice"\s*:\s*"?(\d+(?:\.\d+)?)"?`, `"marketPrice"\s*:\s*"?(\d+(?:\.\d+)?)"?`, true},
		{`"price"\s*:\s*"?(\d+(?:\.\d+)?)"?`, `"marketPrice"\s*:\s*"?(\d+(?:\.\d+)?)"?`, false},
		{`[¥￥]\s*(\d+(?:\.\d+)?)`, `[¥￥]\s*(\d+(?:\.\d+)?)`, false},
	}
	for _, p := range patterns {
		re := regexp.MustCompile(p.priceRe)
		m := re.FindStringSubmatch(html)
		if len(m) < 2 {
			continue
		}
		price := normalizePDDNumber(m[1], p.cents)
		if price <= 0 {
			continue
		}
		var orig float64
		if p.origRe != "" {
			ore := regexp.MustCompile(p.origRe)
			if om := ore.FindStringSubmatch(html); len(om) >= 2 {
				orig = normalizePDDNumber(om[1], p.cents)
			}
		}
		if orig > 0 && orig < price {
			orig = 0
		}
		return price, orig
	}
	return 0, 0
}

// normalizePDDNumber 拼多多接口价格常以“分”为单位；带小数点的按元处理。
func normalizePDDNumber(raw string, cents bool) float64 {
	v := parseFloatStr(raw)
	if v <= 0 {
		return 0
	}
	if cents && !strings.Contains(raw, ".") && v >= 100 {
		v = v / 100
	}
	return round2(v)
}

func extractPDDStock(html string) string {
	lower := strings.ToLower(html)
	switch {
	case strings.Contains(lower, "已售罄"), strings.Contains(lower, "已下架"),
		strings.Contains(lower, "暂不支持购买"), strings.Contains(lower, "该商品已下架"):
		return "out_of_stock"
	case strings.Contains(lower, "立即购买"), strings.Contains(lower, "单独购买"),
		strings.Contains(lower, "发起拼单"):
		return "in_stock"
	default:
		return "unknown"
	}
}

// ---------- 浏览器降级 ----------

func (c *PDDCollector) fetchPriceBrowser(ctx context.Context, pageURL string) (*PriceResult, error) {
	sessionCookies := c.getSessionCookies(ctx)
	tabCtx, cancelTab, err := c.createTempBrowser(ctx)
	if err != nil {
		return nil, err
	}
	defer cancelTab()

	if err := chromedp.Run(tabCtx,
		emulation.SetUserAgentOverride(pddMobileUA),
		emulation.SetDeviceMetricsOverride(390, 844, 1, false),
	); err != nil {
		return nil, fmt.Errorf("设置浏览器身份失败: %w", err)
	}
	for _, sc := range sessionCookies {
		// 按 Cookie 原始 domain 注入。拼多多部分登录 Cookie 是 host-only（mobile.yangkeduo.com），
		// 强制改成 .yangkeduo.com 会导致商品页识别不到登录态。
		domain := sc.Domain
		if domain == "" {
			domain = ".yangkeduo.com"
		}
		chromedp.Run(tabCtx,
			network.SetCookie(sc.Name, sc.Value).WithDomain(domain).WithPath("/"),
		)
	}

	navCtx, navCancel := context.WithTimeout(tabCtx, 45*time.Second)
	if err := chromedp.Run(navCtx, chromedp.Navigate(pageURL)); err != nil {
		fmt.Fprintf(os.Stderr, "[pdd] Navigate 未在预期内完成（%v），继续轮询 body\n", err)
	}
	bodyOK := false
	deadline := time.Now().Add(45 * time.Second)
	for !bodyOK {
		if time.Now().After(deadline) {
			navCancel()
			return nil, fmt.Errorf("浏览器加载页面失败: body 未就绪（%s）", pageURL)
		}
		if err := chromedp.Run(tabCtx, chromedp.Evaluate(`!!document.body && document.readyState !== 'loading'`, &bodyOK)); err != nil {
			bodyOK = false
		}
		if !bodyOK {
			time.Sleep(500 * time.Millisecond)
		}
	}
	chromedp.Run(tabCtx, chromedp.Sleep(6*time.Second))
	navCancel()

	var currentURL, pageTitle string
	chromedp.Run(tabCtx, chromedp.Location(&currentURL))
	chromedp.Run(tabCtx, chromedp.Title(&pageTitle))
	fmt.Fprintf(os.Stderr, "[pdd] 浏览器导航完成 currentURL=%s title=%q\n", currentURL, pageTitle)
	if isPDDLoginRedirect(currentURL, pageTitle) {
		return nil, fmt.Errorf("需要登录（session 已过期或缺失），currentURL=%s", currentURL)
	}

	var scrapeJSON, pageHTML string
	scrapeCtx, scrapeCancel := context.WithTimeout(tabCtx, 20*time.Second)
	defer scrapeCancel()
	err = chromedp.Run(scrapeCtx,
		chromedp.Evaluate(`(function() {
			var body = document.body ? document.body.innerText : '';
			var re = /[¥￥]\s*(\d+(?:\.\d+)?)/;
			function pick(label) {
				var i = body.indexOf(label);
				if (i === -1) return '';
				var seg = body.slice(Math.max(0, i - 80), i + 160);
				var m = seg.match(re);
				return m ? m[1] : '';
			}
			var labels = ['百亿补贴价','补贴价','券后价','到手价','拼单价','单独购买','原价','划线价'];
			var found = {};
			for (var i = 0; i < labels.length; i++) found[labels[i]] = pick(labels[i]);
			var title = '';
			var sels = ['[class*="goodsName"]','[class*="goods-name"]','[class*="title"]','h1'];
			for (var i = 0; i < sels.length; i++) {
				var el = document.querySelector(sels[i]);
				if (el && el.innerText && el.innerText.trim()) { title = el.innerText.trim(); break; }
			}
			var soldOut = /已售罄|已下架|暂不支持购买/.test(body);
			return JSON.stringify({found: found, title: title, soldOut: soldOut,
				raw: (function(){ try { return JSON.stringify(window.rawData || null).slice(0, 200000); } catch(e) { return ''; } })()});
		})()`, &scrapeJSON),
		chromedp.OuterHTML("html", &pageHTML),
	)
	if err != nil {
		return nil, fmt.Errorf("页面采集失败: %w", err)
	}

	var scrape struct {
		Found   map[string]string `json:"found"`
		Title   string            `json:"title"`
		SoldOut bool              `json:"soldOut"`
		Raw     string            `json:"raw"`
	}
	_ = json.Unmarshal([]byte(scrapeJSON), &scrape)

	price, origPrice := extractPDDPrice(pageHTML)
	if price <= 0 && scrape.Raw != "" {
		price, origPrice = extractPDDPrice(scrape.Raw)
	}
	if price <= 0 {
		for _, label := range []string{"百亿补贴价", "补贴价", "券后价", "到手价", "拼单价", "单独购买", "原价", "划线价"} {
			if v := parseFloatStr(scrape.Found[label]); v > 0 {
				if price <= 0 || strings.Contains(label, "补贴") || strings.Contains(label, "券后") || strings.Contains(label, "到手") {
					price = v
				}
				if strings.Contains(label, "原价") || strings.Contains(label, "划线价") {
					origPrice = v
				}
			}
		}
	}
	if price <= 0 {
		return nil, fmt.Errorf("未在页面中解析到有效价格")
	}

	result := &PriceResult{
		CurrentPrice:  round2(price),
		OriginalPrice: round2(origPrice),
		BasePrice:     round2(firstPositive(origPrice, price)),
		PriceSource:   "browser",
		Title:         strings.TrimSpace(scrape.Title),
		StockStatus:   "unknown",
		ProductURL:    currentURL,
		FetchedAt:     time.Now(),
	}
	if result.Title == "" {
		result.Title = extractPDDTitle(pageHTML)
	}
	if scrape.SoldOut {
		result.StockStatus = "out_of_stock"
	} else if s := extractPDDStock(pageHTML); s != "unknown" {
		result.StockStatus = s
	}
	return result, nil
}

func isPDDLoginRedirect(pageURL, pageTitle string) bool {
	lower := strings.ToLower(pageURL)
	for _, kw := range []string{"login.html", "passport", "verify", "risk"} {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	for _, kw := range []string{"登录", "验证", "安全验证"} {
		if strings.Contains(pageTitle, kw) {
			return true
		}
	}
	return false
}

// ---------- login3rd 会话 ----------

func (c *PDDCollector) getLogin3rdSvc() *login3rd.Service {
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

// getSessionCookies 获取拼多多登录会话（login3rd 账号）
func (c *PDDCollector) getSessionCookies(ctx context.Context) []*provider.Cookie {
	svc := c.getLogin3rdSvc()
	if svc == nil {
		return nil
	}
	return svc.GetSessionCookies(ctx, "pdd")
}

func (c *PDDCollector) createTempBrowser(ctx context.Context) (context.Context, context.CancelFunc, error) {
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
		return newBrowserWithProfile(ctx, c.browserPath, c.Platform(),
			chromedp.Flag("disable-blink-features", "AutomationControlled"),
			chromedp.Flag("disable-dev-shm-usage", true),
		)
	}
	return nil, nil, fmt.Errorf("未找到浏览器（Chrome 或远程连接）")
}

func (c *PDDCollector) Close() {}

var _ Collector = (*PDDCollector)(nil)
