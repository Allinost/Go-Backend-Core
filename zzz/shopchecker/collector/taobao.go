package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	tbCanonicalURL = "https://item.taobao.com/item.htm?id=%s"
	tbH5DetailURL  = "https://main.m.taobao.com/detail/index.html?id=%s"

	tbMobileUA = "Mozilla/5.0 (iPhone; CPU iPhone OS 16_6 like Mac OS X) AppleWebKit/537.36 (KHTML, like Gecko) Version/16.6 Mobile/15E148 Safari/604.1"
	tbDeskUA   = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36"
)

// TaobaoCollector 淘宝价格采集器
// 采集路径：短链解析(HTTP) → 会话注入(login3rd taobao 账号) → 浏览器渲染(chromedp) → 价格解析
type TaobaoCollector struct {
	client      *http.Client
	browserPath string
	remoteURL   string
	platform    string
	mu          sync.Mutex
}

// NewTaobaoCollector 创建淘宝价格采集器
func NewTaobaoCollector() *TaobaoCollector {
	return newTaobaoCollector("taobao")
}

// NewTmallCollector 创建天猫采集器（与淘宝共用采集逻辑与登录态）
func NewTmallCollector() *TaobaoCollector {
	return newTaobaoCollector("tmall")
}

func newTaobaoCollector(platform string) *TaobaoCollector {
	return &TaobaoCollector{
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
		platform:    platform,
	}
}

// Platform 返回平台标识
func (c *TaobaoCollector) Platform() string {
	return c.platform
}

// FetchPrice 抓取淘宝商品价格
// 1) 从分享口令短链(e.tb.cn/tb.cn/m.tb.cn)或商品页 URL 中解析商品 ID
// 2) HTTP 快速路径：带 session cookie 抓取商品页 HTML 并正则解析价格
// 3) 浏览器降级路径：chromedp 渲染 H5 详情页，从 DOM 文案标签提取到手价/国补价
func (c *TaobaoCollector) FetchPrice(ctx context.Context, rawURL string) (*PriceResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	itemID := c.resolveItemID(ctx, rawURL)
	if itemID == "" {
		return nil, fmt.Errorf("taobao: 无法从 URL 解析商品 ID")
	}
	skuID := extractTaobaoSKU(rawURL)
	canonicalURL := fmt.Sprintf(tbCanonicalURL, itemID)
	if skuID != "" {
		canonicalURL += "&skuId=" + url.QueryEscape(skuID)
	}

	sessionCookie := sessionCookieHeader(c.getSessionCookies(ctx))

	// HTTP 阶段：独立 15s 预算，不消耗浏览器阶段的预算
	httpCtx, httpCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer httpCancel()
	httpResult := c.fetchPageHTTP(httpCtx, itemID, skuID, sessionCookie)
	if httpResult != nil {
		fmt.Fprintf(os.Stderr, "[taobao] HTTP 阶段完成 id=%s price=%v title=%q\n", itemID, httpResult.CurrentPrice, httpResult.Title)
	}
	// 淘宝 HTTP 页面经常返回“宝贝描述”等占位页或错误价格（如把分当元），
	// 因此不再直接用 HTTP 结果，浏览器渲染结果优先；HTTP 仅作为浏览器失败后的兜底。

	// 浏览器阶段：H5 详情页优先，桌面页兜底
	result, err := c.fetchPriceBrowser(ctx, itemID, skuID)
	if err != nil {
		browserLoginErr := strings.Contains(err.Error(), "需要登录") || strings.Contains(err.Error(), "登录/风险")
		// 已登录会话被判定失效时不能回退到匿名价格，否则会掩盖登录过期并错误续期会话。
		if httpResult != nil && httpResult.CurrentPrice > 0 && !(sessionCookie != "" && browserLoginErr) &&
			isUsableTaobaoHTTPResult(httpResult) {
			httpResult.ProductURL = canonicalURL
			return httpResult, nil
		}
		return nil, fmt.Errorf("taobao: %v (id=%s)", err, itemID)
	}
	if result.ProductURL == "" {
		result.ProductURL = canonicalURL
	}
	return result, nil
}

// tbItemURL 生成淘宝/天猫商品页地址，带 skuId 时直接定位规格。
func tbItemURL(itemID, skuID string, mobile bool) string {
	base := tbCanonicalURL
	if mobile {
		base = tbH5DetailURL
	}
	pageURL := fmt.Sprintf(base, itemID)
	if skuID != "" {
		sep := "?"
		if strings.Contains(pageURL, "?") {
			sep = "&"
		}
		pageURL += sep + "skuId=" + url.QueryEscape(skuID)
	}
	return pageURL
}

// ---------- 商品 ID 解析 ----------

var tbItemRegexes = []*regexp.Regexp{
	regexp.MustCompile(`[?&]id=(\d+)`),
	regexp.MustCompile(`/i(\d{6,})\.html?`),
	regexp.MustCompile(`/item/(\d{6,})\.html?`),
	regexp.MustCompile(`goods\.html\?.*?id=(\d+)`),
}

var tbShortLinkHostRe = regexp.MustCompile(`(?i)(?:^|\.)((?:e|m)?\.?tb\.cn|[a-z]+\.tb\.cn)$`)

var tbURLInTextRe = regexp.MustCompile(`https?://[^\s「」『』"'<>()（）【】，。；]+`)

var tbShortTargetRe = regexp.MustCompile(`var\s+url\s*=\s*'([^']+)'`)

// resolveItemID 从原始输入（可能是完整 URL、短链或含链接的分享文本）中解析淘宝商品 ID
func (c *TaobaoCollector) resolveItemID(ctx context.Context, rawInput string) string {
	rawURL := extractTaobaoURL(rawInput)
	if rawURL == "" {
		return ""
	}

	if id := extractTaobaoItemID(rawURL); id != "" {
		return id
	}

	// 短链：抓取跳转页，从内嵌 JS 变量提取目标地址
	if isTaobaoShortLink(rawURL) {
		if target := c.resolveShortLink(ctx, rawURL); target != "" {
			return extractTaobaoItemID(target)
		}
	}
	return ""
}

// extractTaobaoURL 从混杂的分享文本中提取首个淘宝系 URL
func extractTaobaoURL(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}
	matches := tbURLInTextRe.FindAllString(input, -1)
	for _, m := range matches {
		if strings.Contains(m, "tb.cn") || strings.Contains(m, "taobao.com") || strings.Contains(m, "tmall.com") {
			return strings.TrimRight(m, ",.;）)")
		}
	}
	if !strings.Contains(input, " ") && strings.Contains(input, "tb.cn") {
		return input
	}
	return ""
}

// extractTaobaoItemID 从淘宝商品页 URL 提取数字商品 ID
func extractTaobaoItemID(pageURL string) string {
	cleaned := stripTrackingParams(pageURL)
	for _, re := range tbItemRegexes {
		if m := re.FindStringSubmatch(cleaned); len(m) >= 2 {
			return m[1]
		}
	}
	u, err := url.Parse(cleaned)
	if err != nil {
		return ""
	}
	if m := tbItemRegexes[0].FindStringSubmatch(u.RawQuery); len(m) >= 2 {
		return m[1]
	}
	return ""
}

// extractTaobaoSKU 从商品 URL 中提取 skuId（分享链接带 skuId 时可直接定位用户选中的规格）
func extractTaobaoSKU(rawInput string) string {
	rawURL := extractTaobaoURL(rawInput)
	if rawURL == "" {
		rawURL = strings.TrimSpace(rawInput)
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	q := u.Query()
	for _, key := range []string{"skuId", "sku_id", "sku"} {
		if v := strings.TrimSpace(q.Get(key)); v != "" {
			return v
		}
	}
	re := regexp.MustCompile(`(?i)[?&]sku(?:Id|_id)?=(\d+)`)
	if m := re.FindStringSubmatch(rawURL); len(m) >= 2 {
		return m[1]
	}
	return ""
}

// isTaobaoShortLink 判断是否为淘宝短链域名（e.tb.cn / m.tb.cn / tb.cn / t.cn 系）
func isTaobaoShortLink(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "tb.cn" || host == "m.tb.cn" || host == "e.tb.cn" ||
		strings.HasSuffix(host, ".tb.cn") || host == "t.cn"
}

// resolveShortLink 请求短链跳转页，从页面内嵌 JS 变量 var url='...' 中提取目标地址
func (c *TaobaoCollector) resolveShortLink(ctx context.Context, shortURL string) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, shortURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", tbMobileUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := c.client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	if m := tbShortTargetRe.FindStringSubmatch(html); len(m) >= 2 {
		target := strings.ReplaceAll(m[1], `\x3d`, "=")
		target = strings.ReplaceAll(target, `\x26`, "&")
		if t := extractTaobaoURL(target); t != "" {
			return t
		}
		return target
	}

	// 兜底：跟随最终重定向地址提取 ID
	if resp.Request != nil && resp.Request.URL != nil {
		final := resp.Request.URL.String()
		if id := extractTaobaoItemID(final); id != "" {
			return final
		}
	}
	return ""
}

// ---------- HTTP 快速路径 ----------

func (c *TaobaoCollector) fetchPageHTTP(ctx context.Context, itemID, skuID, sessionCookie string) *PriceResult {
	endpoints := []struct {
		url     string
		desktop bool
	}{
		{tbItemURL(itemID, skuID, false), true},
		{tbItemURL(itemID, skuID, true), false},
	}
	for _, ep := range endpoints {
		html := c.fetchHTMLOnly(ctx, ep.url, sessionCookie, ep.desktop)
		if html == "" {
			continue
		}
		price, origPrice := extractTaobaoPrice(html)
		if price <= 0 {
			continue
		}
		title := extractTaobaoTitle(html)
		if title == "" {
			title = extractTaobaoTitleFromHTMLString(html)
		}
		result := &PriceResult{
			CurrentPrice:  price,
			OriginalPrice: origPrice,
			BasePrice:     firstPositive(origPrice, price),
			PriceSource:   "http",
			Title:         title,
			StockStatus:   extractTaobaoStock(html),
			FetchedAt:     time.Now(),
		}
		if hint := ParseSubsidyHint(ExtractSubsidyContext(html)); hint.Amount > 0 || hint.Rate > 0 {
			base := firstPositive(origPrice, price)
			if final, amount := ComputeSubsidizedPrice(base, hint.Amount, hint.Rate, hint.Cap); final > 0 {
				result.CurrentPrice = final
				result.SubsidyAmount = amount
				result.SubsidyRate = hint.Rate
				result.PriceSource = "http_subsidy"
			}
		}
		return result
	}
	return nil
}

func (c *TaobaoCollector) fetchHTMLOnly(ctx context.Context, pageURL, sessionCookie string, desktop bool) string {
	ua := tbMobileUA
	if desktop {
		ua = tbDeskUA
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	if sessionCookie != "" {
		req.Header.Set("Cookie", sessionCookie)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	return string(body)
}

// isUsableTaobaoHTTPResult 过滤淘宝占位页/登录页解析出的明显错误价格。
func isUsableTaobaoHTTPResult(r *PriceResult) bool {
	if r == nil || r.CurrentPrice <= 0 {
		return false
	}
	title := strings.TrimSpace(r.Title)
	if title == "" {
		return false
	}
	for _, bad := range []string{"宝贝描述", "商品详情", "登录", "验证", "淘宝网"} {
		if strings.Contains(title, bad) {
			return false
		}
	}
	// 淘宝价格通常不会超过百万；出现整数万分位多半是把“分”当成了“元”。
	if r.CurrentPrice >= 1000000 {
		return false
	}
	return true
}

func extractTaobaoTitle(html string) string {
	re := regexp.MustCompile(`<title>([^<]+)</title>`)
	matches := re.FindStringSubmatch(html)
	if len(matches) >= 2 {
		return cleanTaobaoTitle(matches[1])
	}
	return ""
}

func extractTaobaoTitleFromHTMLString(html string) string {
	patterns := []string{
		`"title"\s*:\s*"([^"]{4,})"`,
		`"itemTitle"\s*:\s*"([^"]+)"`,
		`og:title"\s+content="([^"]+)"`,
	}
	for _, pat := range patterns {
		re := regexp.MustCompile(pat)
		if m := re.FindStringSubmatch(html); len(m) >= 2 {
			t := cleanTaobaoTitle(m[1])
			if t != "" {
				return t
			}
		}
	}
	return ""
}

// cleanTaobaoTitle 清理标题中的站点后缀与营销噪音
func cleanTaobaoTitle(t string) string {
	suffixes := []string{"-淘宝网", "-淘宝", "-tmall", "-天猫", "-天猫Tmall", " - 阿里巴巴"}
	for _, s := range suffixes {
		t = strings.TrimSuffix(strings.TrimSpace(t), s)
	}
	t = strings.TrimSpace(t)
	if t == "" || strings.Contains(t, "登录") || strings.Contains(t, "验证") || t == "商品详情" {
		return ""
	}
	return t
}

// extractTaobaoPrice 从 HTML 中正则提取价格（现价、原价）
func extractTaobaoPrice(html string) (float64, float64) {
	type pat struct {
		priceRe string
		origRe  string
	}
	patterns := []pat{
		{`"promotionPrice"\s*:\s*\{[^}]*?"price"\s*:\s*"?([\d.]+)"?`, `"priceInfo"\s*:\s*\{[^}]*?"price"\s*:\s*"?([\d.]+)"?`},
		{`"priceInfo"\s*:\s*\{[^}]*?"price"\s*:\s*"?([\d.]+)"?`, `"priceInfo"\s*:\s*\{[^}]*?"priceOrig(?:inal)?"\s*:\s*"?([\d.]+)"?`},
		{`"price"\s*:\s*"?([\d.]+)"?`, ""},
		{`class=["'][^"']*tb-rmb[^"']*["'][^>]*>\s*([\d.]+)`, `<del[^>]*>\s*[¥￥]?\s*([\d.]+)`},
	}
	for _, p := range patterns {
		priceRe := regexp.MustCompile(p.priceRe)
		pm := priceRe.FindStringSubmatch(html)
		if len(pm) >= 2 {
			price := parseFloatStr(pm[1])
			if price > 0 {
				var orig float64
				if p.origRe != "" {
					origRe := regexp.MustCompile(p.origRe)
					om := origRe.FindStringSubmatch(html)
					if len(om) >= 2 {
						orig = parseFloatStr(om[1])
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

// extractTaobaoStock 从 HTML 推断库存状态
func extractTaobaoStock(html string) string {
	lower := strings.ToLower(html)
	switch {
	case strings.Contains(lower, "已售罄"), strings.Contains(lower, "缺货"),
		strings.Contains(lower, "该宝贝已下架"), strings.Contains(lower, "已下架"):
		return "out_of_stock"
	case strings.Contains(lower, "有货"), strings.Contains(lower, "现货"):
		return "in_stock"
	default:
		return "unknown"
	}
}

// ---------- 浏览器降级路径 ----------

func (c *TaobaoCollector) fetchPriceBrowser(ctx context.Context, itemID, skuID string) (*PriceResult, error) {
	sessionCookies := c.getSessionCookies(ctx)

	tabCtx, cancelTab, err := c.createTempBrowser(ctx)
	if err != nil {
		return nil, err
	}
	defer cancelTab()

	if err := chromedp.Run(tabCtx,
		emulation.SetUserAgentOverride(tbMobileUA),
		emulation.SetDeviceMetricsOverride(390, 844, 1, false),
	); err != nil {
		return nil, fmt.Errorf("设置浏览器身份失败: %w", err)
	}

	// 会话 cookie 注入 .taobao.com 与 .tmall.com 双域，覆盖天猫店商品跨域跳转
	if len(sessionCookies) > 0 {
		for _, domain := range []string{".taobao.com", ".tmall.com"} {
			for _, sc := range sessionCookies {
				chromedp.Run(tabCtx,
					network.SetCookie(sc.Name, sc.Value).
						WithDomain(domain).
						WithPath("/"),
				)
			}
		}
	} else {
		fmt.Fprintf(os.Stderr, "[taobao] 未获取到 login3rd 淘宝会话，以游客身份尝试采集\n")
	}

	navDeadline := time.Now().Add(50 * time.Second)
	navCtx, navCancel := context.WithTimeout(tabCtx, 50*time.Second)

	pageURL := tbItemURL(itemID, skuID, true)
	if err := chromedp.Run(navCtx, chromedp.Navigate(pageURL)); err != nil {
		fmt.Fprintf(os.Stderr, "[taobao] Navigate 未在预期内完成（%v），继续轮询 body\n", err)
	}

	bodyOK := false
	for !bodyOK {
		if time.Now().After(navDeadline) {
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
	chromedp.Run(tabCtx, chromedp.Sleep(8*time.Second))
	navCancel()

	// 登录/风控重定向检测
	var currentURL, pageTitle string
	chromedp.Run(tabCtx, chromedp.Location(&currentURL))
	chromedp.Run(tabCtx, chromedp.Title(&pageTitle))
	fmt.Fprintf(os.Stderr, "[taobao] 浏览器导航完成 currentURL=%s title=%q\n", currentURL, pageTitle)
	if isTaobaoLoginRedirect(currentURL, pageTitle) {
		return nil, fmt.Errorf("需要登录（session 已过期或缺失），currentURL=%s", currentURL)
	}

	result, scrapeErr := c.scrapeProduct(tabCtx, currentURL)
	if scrapeErr != nil {
		// H5 失败时用桌面版详情页兜底一次
		fmt.Fprintf(os.Stderr, "[taobao] H5 采集失败（%v），尝试桌面版页面\n", scrapeErr)
		return c.fetchDesktopBrowser(tabCtx, itemID, skuID)
	}
	return result, nil
}

// fetchDesktopBrowser 桌面版商品页浏览器兜底采集
func (c *TaobaoCollector) fetchDesktopBrowser(tabCtx context.Context, itemID, skuID string) (*PriceResult, error) {
	if err := chromedp.Run(tabCtx,
		emulation.SetUserAgentOverride(tbDeskUA),
		emulation.SetDeviceMetricsOverride(1440, 900, 1, false),
	); err != nil {
		return nil, fmt.Errorf("切换桌面身份失败: %w", err)
	}

	pageURL := tbItemURL(itemID, skuID, false)
	navDeadline := time.Now().Add(40 * time.Second)
	navCtx, navCancel := context.WithTimeout(tabCtx, 40*time.Second)
	if err := chromedp.Run(navCtx, chromedp.Navigate(pageURL)); err != nil {
		fmt.Fprintf(os.Stderr, "[taobao] 桌面版 Navigate 未在预期内完成（%v），继续轮询 body\n", err)
	}
	bodyOK := false
	for !bodyOK {
		if time.Now().After(navDeadline) {
			navCancel()
			return nil, fmt.Errorf("桌面版页面加载失败: body 未就绪（%s）", pageURL)
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
	if isTaobaoLoginRedirect(currentURL, pageTitle) {
		return nil, fmt.Errorf("需要登录（session 已过期或缺失），currentURL=%s", currentURL)
	}
	return c.scrapeProduct(tabCtx, currentURL)
}

// isTaobaoLoginRedirect 判断当前页面是否被重定向到登录/风控页
func isTaobaoLoginRedirect(pageURL, pageTitle string) bool {
	lower := strings.ToLower(pageURL)
	for _, kw := range []string{"login.taobao.com", "login.m.taobao.com", "havanaone", "passport",
		"_____tmd_____", "punish", "captcha", "login_jump"} {
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

// scrapeProduct 从渲染完成的页面提取价格/标题/库存
// 优先级：国补/补贴后到手价 > 券后/到手价 > 页面首个 ¥ 价格；日常价/划线价作为原价
func (c *TaobaoCollector) scrapeProduct(browserCtx context.Context, currentURL string) (*PriceResult, error) {
	var priceJSON string
	var pageHTML string

	scrapeCtx, scrapeCancel := context.WithTimeout(browserCtx, 20*time.Second)
	defer scrapeCancel()

	err := chromedp.Run(scrapeCtx,
		chromedp.Evaluate(`(function() {
			var body = document.body ? document.body.innerText : '';
			var re = /[¥￥]\s*(\d+(?:,\d{3})*(?:\.\d{1,2})?)(?![\d?？])/;
			var reNum = /(?:^|[^\d.])(\d+(?:,\d{3})*(?:\.\d{1,2})?)(?![\d?？])/;
			function pick(label) {
				var i = body.indexOf(label);
				if (i === -1) return '';
				var seg = body.slice(Math.max(0, i - 80), i + 140);
				var m = seg.match(re) || seg.match(reNum);
				return m ? m[1].replace(/,/g, '') : '';
			}
			var labels = ['国补领后价','国补后到手价','国补到手价','国家补贴价','政府补贴价','补贴后到手价',
				'补贴后','国补价','补贴价','国家补贴后','政府补贴后','百亿补贴价',
				'到手价','券后','折后','实付','预售价','日常价','原价','划线价'];
			var found = {};
			for (var i = 0; i < labels.length; i++) found[labels[i]] = pick(labels[i]);
			var main = '';
			var m3 = body.match(re);
			if (m3) main = m3[1].replace(/,/g, '');
			var subsidyContext = '';
			var mi = body.search(/国补|国家补贴|政府补贴|补贴/);
			if (mi > -1) subsidyContext = body.slice(Math.max(0, mi - 100), mi + 240);
			var skuName = '';
			var sels = ['.tb-main-title', '[class*="mainTitle"]', '[class*="itemTitle"]', '.ItemHeader--mainTitle', 'h1'];
			for (var i = 0; i < sels.length; i++) {
				var el = document.querySelector(sels[i]);
				if (el && el.innerText && el.innerText.trim()) { skuName = el.innerText.trim(); break; }
			}
			var soldOut = /已售罄|缺货|已下架/.test(body);
			return JSON.stringify({found: found, main: main, skuName: skuName, soldOut: soldOut, subsidyContext: subsidyContext});
		})()`, &priceJSON),
		chromedp.OuterHTML("html", &pageHTML),
	)
	if err != nil {
		return nil, fmt.Errorf("页面采集失败: %w", err)
	}

	scrape := parseTaobaoScrape(priceJSON)
	found, main, skuName, soldOut := scrape.Found, scrape.Main, scrape.SkuName, scrape.SoldOut

	result := &PriceResult{
		StockStatus: "unknown",
		ProductURL:  currentURL,
		FetchedAt:   time.Now(),
	}
	if soldOut {
		result.StockStatus = "out_of_stock"
	}
	if skuName != "" {
		result.Title = cleanTaobaoTitle(skuName)
	}
	if result.Title == "" {
		result.Title = extractTaobaoTitleFromHTMLString(pageHTML)
	}

	subsidy, final, daily := parseTaobaoLabeledPrices(found)
	hint := ParseSubsidyHint(scrape.SubsidyContext)
	price := subsidy
	source := "gov_label"
	if price == 0 {
		price = final
		source = "final_label"
	}
	if price == 0 {
		price = main
		source = "page_main"
	}
	if price == 0 {
		price, _ = extractTaobaoPrice(pageHTML)
		source = "html"
	}
	origPrice := daily
	if origPrice == 0 {
		_, origPrice = extractTaobaoPrice(pageHTML)
	}
	if origPrice > 0 && price > 0 && origPrice < price*0.5 {
		origPrice = 0
	}
	basePrice := firstPositive(origPrice, price)
	subsidyAmount := 0.0
	if subsidy > 0 && basePrice > subsidy {
		subsidyAmount = round2(basePrice - subsidy)
	}

	// 页面只展示“国家补贴 15%”或“补贴减￥500”，未直接给出补贴后价格时，用基础价计算到手价。
	hasExplicitPrice := subsidy > 0 || final > 0
	if !hasExplicitPrice && price > 0 && subsidyAmount <= 0 && (hint.Amount > 0 || hint.Rate > 0) && basePrice > 0 {
		if computed, amount := ComputeSubsidizedPrice(basePrice, hint.Amount, hint.Rate, hint.Cap); computed > 0 {
			price = computed
			subsidyAmount = amount
			source = "gov_subsidy_text"
		}
	}
	if origPrice > 0 && origPrice <= price {
		origPrice = 0
	}

	if price <= 0 {
		return nil, fmt.Errorf("未在页面中解析到有效价格")
	}
	result.CurrentPrice = round2(price)
	result.OriginalPrice = round2(origPrice)
	result.BasePrice = round2(basePrice)
	result.SubsidyAmount = round2(subsidyAmount)
	result.SubsidyRate = hint.Rate
	result.PriceSource = source

	fmt.Fprintf(os.Stderr, "[taobao] 采集结果 title=%q price=%v orig=%v base=%v subsidy=%.2f rate=%.2f source=%s label_sub=%v final=%v daily=%v hint=%+v stock=%s url=%s\n",
		result.Title, result.CurrentPrice, result.OriginalPrice, result.BasePrice, result.SubsidyAmount, result.SubsidyRate,
		result.PriceSource, subsidy, final, daily, hint, result.StockStatus, currentURL)
	return result, nil
}

type taobaoScrape struct {
	Found          map[string]string
	Main           float64
	SkuName        string
	SoldOut        bool
	SubsidyContext string
}

func parseTaobaoScrape(raw string) taobaoScrape {
	var p struct {
		Found          map[string]string `json:"found"`
		Main           string            `json:"main"`
		SkuName        string            `json:"skuName"`
		SoldOut        bool              `json:"soldOut"`
		SubsidyContext string            `json:"subsidyContext"`
	}
	if json.Unmarshal([]byte(raw), &p) != nil {
		return taobaoScrape{}
	}
	return taobaoScrape{
		Found: p.Found, Main: parseFloatStr(p.Main),
		SkuName: strings.TrimSpace(p.SkuName), SoldOut: p.SoldOut,
		SubsidyContext: p.SubsidyContext,
	}
}

// parseTaobaoScrapeJSON 保留原有返回签名（供单测/兼容调用）。
func parseTaobaoScrapeJSON(raw string) (found map[string]string, main float64, skuName string, soldOut bool) {
	s := parseTaobaoScrape(raw)
	return s.Found, s.Main, s.SkuName, s.SoldOut
}

// parseTaobaoLabeledPrices 解析按文案标签提取的价格
// 返回值：subsidy 国补/补贴后到手价，final 券后/到手价，daily 日常价/原价
func parseTaobaoLabeledPrices(found map[string]string) (subsidy, final, daily float64) {
	firstOf := func(labels ...string) float64 {
		for _, l := range labels {
			if v := parseFloatStr(found[l]); v > 0 {
				return v
			}
		}
		return 0
	}
	subsidy = firstOf("国补领后价", "国补后到手价", "国补到手价", "国家补贴价", "政府补贴价",
		"补贴后到手价", "补贴后", "国补价", "补贴价", "国家补贴后", "政府补贴后", "百亿补贴价")
	final = firstOf("到手价", "券后", "折后", "实付", "预售价")
	daily = firstOf("日常价", "原价", "划线价")
	return
}

// ---------- login3rd 会话 ----------

// getLogin3rdSvc 懒加载 Login3rd 服务（避免模块初始化顺序依赖）
func (c *TaobaoCollector) getLogin3rdSvc() *login3rd.Service {
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

// getSessionCookies 从 Login3rd 获取淘宝/天猫共享登录 cookie（仅 verified=valid 时返回）。
// c.platform 为 tmall 时会先找天猫账号，再回退淘宝账号，反之亦然。
func (c *TaobaoCollector) getSessionCookies(ctx context.Context) []*provider.Cookie {
	svc := c.getLogin3rdSvc()
	if svc == nil {
		return nil
	}
	return svc.GetSessionCookies(ctx, c.Platform())
}

// createTempBrowser 创建临时浏览器上下文（无持久化状态）
func (c *TaobaoCollector) createTempBrowser(ctx context.Context) (context.Context, context.CancelFunc, error) {
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

// Close 释放资源
func (c *TaobaoCollector) Close() {}

var _ Collector = (*TaobaoCollector)(nil)
