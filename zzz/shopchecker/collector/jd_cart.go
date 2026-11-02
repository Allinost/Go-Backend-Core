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

	"github.com/Allinost/go-backend-core/zzz/login3rd/provider"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

const (
	jdCartURL      = "https://cart.jd.com/cart_index"
	jdCartCacheTTL = 2 * time.Minute
	jdCartErrorTTL = 30 * time.Second
)

var jdCartPriceRe = regexp.MustCompile(`[¥￥]\s*(\d+(?:,\d{3})*(?:\.\d{1,2})?)`)

type jdCartItem struct {
	SKU   string `json:"sku"`
	Text  string `json:"text"`
	Title string `json:"title"`
}

type jdCartExtract struct {
	URL   string       `json:"url"`
	Title string       `json:"title"`
	Body  string       `json:"body"`
	Items []jdCartItem `json:"items"`
}

type jdCartSnapshot struct {
	Items     map[string]*PriceResult
	FetchedAt time.Time
	Err       error
}

var (
	jdCartCacheMu sync.Mutex
	jdCartCache   jdCartSnapshot
)

// getJDCartSnapshot 完整拉取一次购物车并缓存，供同一次检查周期内的多个商品复用。
// 京东/七鲜/京东移动版共用同一个京东账号，因此共用同一份购物车快照。
func (c *JDCollector) getJDCartSnapshot(ctx context.Context, force bool) (map[string]*PriceResult, error) {
	jdCartCacheMu.Lock()
	defer jdCartCacheMu.Unlock()

	if !force && !jdCartCache.FetchedAt.IsZero() {
		ttl := jdCartCacheTTL
		if jdCartCache.Err != nil {
			ttl = jdCartErrorTTL
		}
		if time.Since(jdCartCache.FetchedAt) < ttl {
			return jdCartCache.Items, jdCartCache.Err
		}
	}

	items, err := c.fetchJDCartSnapshot(ctx)
	jdCartCache = jdCartSnapshot{Items: items, FetchedAt: time.Now(), Err: err}
	return items, err
}

// addJDCartItem 把不在购物车的商品补进购物车（best-effort）。
func (c *JDCollector) addJDCartItem(ctx context.Context, skuID string) error {
	sessionCookies := c.getSessionCookies(ctx)
	if len(sessionCookies) == 0 {
		return fmt.Errorf("jd cart: 无登录 session")
	}

	addCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	return addJDCartItemHTTP(addCtx, c.client, "https://cart.jd.com", skuID, sessionCookieHeader(sessionCookies))
}

func addJDCartItemHTTP(ctx context.Context, client *http.Client, baseURL, skuID, cookieHeader string) error {
	addURL := fmt.Sprintf("%s/gate.action?pid=%s&pcount=1&ptype=1", strings.TrimRight(baseURL, "/"), skuID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, addURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Referer", fmt.Sprintf(jdDesktopURL, skuID))
	if cookieHeader != "" {
		req.Header.Set("Cookie", cookieHeader)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	text := string(body)
	finalURL := ""
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	if strings.Contains(text, "商品已成功加入购物车") {
		return nil
	}
	if strings.Contains(finalURL, "frequent") || isJDRateLimitText(text) {
		return fmt.Errorf("京东访问频率限制（加购）")
	}
	if strings.Contains(finalURL, "passport") || strings.Contains(text, "请登录") {
		return fmt.Errorf("需要登录（session 已过期）")
	}
	return fmt.Errorf("加购结果未确认")
}

// cacheJDCartItem 将刚补进购物车的商品写入当前快照，避免同一周期重复加购。
func cacheJDCartItem(skuID string, result *PriceResult) {
	jdCartCacheMu.Lock()
	defer jdCartCacheMu.Unlock()
	if jdCartCache.Items == nil {
		return
	}
	if _, exists := jdCartCache.Items[skuID]; exists {
		return
	}
	items := make(map[string]*PriceResult, len(jdCartCache.Items)+1)
	for k, v := range jdCartCache.Items {
		items[k] = v
	}
	copied := *result
	copied.CartStatus = "hit"
	copied.CartNote = "已加入购物车"
	items[skuID] = &copied
	jdCartCache.Items = items
}

// fetchJDCartSnapshot 打开一次购物车页面，提取全部购物车商品。
func (c *JDCollector) fetchJDCartSnapshot(ctx context.Context) (map[string]*PriceResult, error) {
	sessionCookies := c.getSessionCookies(ctx)
	if len(sessionCookies) == 0 {
		return nil, fmt.Errorf("jd cart: 无登录 session")
	}

	cartCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	tabCtx, cancelTab, err := c.createTempBrowser(cartCtx)
	if err != nil {
		return nil, err
	}
	defer cancelTab()

	if err := chromedp.Run(tabCtx,
		emulation.SetUserAgentOverride("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"),
		emulation.SetDeviceMetricsOverride(1440, 900, 1, false),
	); err != nil {
		return nil, fmt.Errorf("jd cart: 设置浏览器身份失败: %w", err)
	}
	setBrowserCookies(tabCtx, sessionCookies)

	if err := chromedp.Run(tabCtx, chromedp.Navigate(jdCartURL)); err != nil {
		fmt.Fprintf(os.Stderr, "[jd cart] Navigate 未在预期内完成（%v），继续轮询 body\n", err)
	}

	bodyOK := false
	deadline := time.Now().Add(20 * time.Second)
	for !bodyOK && time.Now().Before(deadline) {
		if err := chromedp.Run(tabCtx, chromedp.Evaluate(`!!document.body && document.readyState !== 'loading'`, &bodyOK)); err != nil {
			bodyOK = false
		}
		if !bodyOK {
			time.Sleep(500 * time.Millisecond)
		}
	}
	chromedp.Run(tabCtx, chromedp.Sleep(3*time.Second))

	var raw string
	if err := chromedp.Run(tabCtx, chromedp.Evaluate(`(function(){
		function skuFromHref(h){
			var m=(h||'').match(/item\.jd\.com\/(\d+)\.html/);
			return m?m[1]:'';
		}
		var out=[], seen={};
		document.querySelectorAll('a[href*="item.jd.com/"]').forEach(function(a){
			var sku=skuFromHref(a.getAttribute('href')||a.href);
			if(!sku || seen[sku]) return;
			var el=a, box=null, boxText='';
			// 找到最近的“商品行”容器：包含删除按钮，并且有价格或下架文案。
			for(var d=0; d<12 && el; d++){
				var t=(el.innerText||'').replace(/\s+/g,' ').trim();
				if(t.length>=20 && t.length<=2500 && t.indexOf('删除')>=0){
					var hasPrice=t.indexOf('¥')>=0 || t.indexOf('￥')>=0;
					var unavailable=t.indexOf('商品已下架')>=0 || t.indexOf('暂不支持购买')>=0 || t.indexOf('无货')>=0;
					if(hasPrice || unavailable){ box=el; boxText=t; break; }
				}
				el=el.parentElement;
			}
			if(box){
				seen[sku]=1;
				var img=a.querySelector('img');
				out.push({sku:sku,text:boxText,title:(a.getAttribute('title')||(img&&img.alt)||'').trim()});
			}
		});
		var body=document.body?document.body.innerText:'';
		return JSON.stringify({url:location.href,title:document.title,body:body.slice(0,4000),items:out});
	})()`, &raw)); err != nil {
		return nil, fmt.Errorf("jd cart: 读取页面失败: %w", err)
	}

	var page jdCartExtract
	if err := json.Unmarshal([]byte(raw), &page); err != nil {
		return nil, fmt.Errorf("jd cart: 解析页面数据失败: %w", err)
	}
	if strings.Contains(page.URL, "passport") || strings.Contains(page.URL, "plogin") {
		return nil, fmt.Errorf("需要登录（session 已过期）")
	}
	if strings.Contains(page.URL, "pc-frequent") || strings.Contains(page.URL, "reason=403") {
		return nil, fmt.Errorf("京东访问频率限制（购物车页），稍后重试")
	}
	if isJDRateLimitText(page.Body) {
		return nil, fmt.Errorf("京东访问频率限制（购物车页），稍后重试")
	}

	items := make(map[string]*PriceResult, len(page.Items))
	for _, it := range page.Items {
		if it.SKU == "" {
			continue
		}
		price, orig, subsidyAmount, subsidyRate := parseJDCartItemPrice(it.Text)
		if price <= 0 {
			continue
		}
		title := cleanJdTitle(strings.TrimSpace(it.Title))
		if title == "" {
			title = cartItemTitleFromText(it.Text)
		}
		items[it.SKU] = &PriceResult{
			CurrentPrice:  round2(price),
			OriginalPrice: round2(orig),
			BasePrice:     round2(firstPositive(orig, price)),
			SubsidyAmount: round2(subsidyAmount),
			SubsidyRate:   subsidyRate,
			PriceSource:   "jd_cart",
			Title:         title,
			StockStatus:   cartStockStatus(it.Text),
			ProductURL:    fmt.Sprintf(jdDesktopURL, it.SKU),
			CartStatus:    "hit",
			FetchedAt:     time.Now(),
		}
	}

	fmt.Fprintf(os.Stderr, "[jd cart] 完整拉取完成 items=%d\n", len(items))
	return items, nil
}

// cartItemTitleFromText 在缺少链接 title 时，从商品行文本中截取标题。
func cartItemTitleFromText(text string) string {
	text = strings.TrimSpace(text)
	for _, marker := range []string{"7天价保", "获取AI搭配", "商品", "服务", "¥", "￥"} {
		if i := strings.Index(text, marker); i > 0 {
			text = text[:i]
			break
		}
	}
	return cleanJdTitle(strings.TrimSpace(text))
}

func cartStockStatus(text string) string {
	for _, kw := range []string{"商品已下架", "暂不支持购买", "无货", "失效", "已下架"} {
		if strings.Contains(text, kw) {
			return "out_of_stock"
		}
	}
	return "in_stock"
}

func setBrowserCookies(ctx context.Context, cookies []*provider.Cookie) {
	actions := make([]chromedp.Action, 0, len(cookies))
	for _, sc := range cookies {
		if sc == nil || sc.Name == "" || sc.Value == "" {
			continue
		}
		ck := network.SetCookie(sc.Name, sc.Value).WithPath("/")
		if strings.HasPrefix(sc.Domain, ".") {
			ck = ck.WithDomain(sc.Domain)
		} else if sc.Domain != "" {
			ck = ck.WithURL("https://" + strings.TrimPrefix(sc.Domain, ".") + "/")
		} else {
			ck = ck.WithURL("https://cart.jd.com/")
		}
		if sc.Secure {
			ck = ck.WithSecure(true)
		}
		if sc.HTTPOnly {
			ck = ck.WithHTTPOnly(true)
		}
		actions = append(actions, ck)
	}
	if len(actions) > 0 {
		_ = chromedp.Run(ctx, actions...)
	}
}

func isJDRateLimitText(text string) bool {
	for _, kw := range []string{"访问频繁", "刷新太频繁", "系统繁忙", "pc-frequent", "稍后再试"} {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}

type jdPricePos struct {
	Value float64
	Index int
}

func parseJDCartItemPrice(text string) (price, orig, subsidyAmount, subsidyRate float64) {
	text = strings.ReplaceAll(text, ",", "")
	var prices []jdPricePos
	for _, m := range jdCartPriceRe.FindAllStringSubmatchIndex(text, -1) {
		if len(m) < 4 {
			continue
		}
		raw := text[m[2]:m[3]]
		var f float64
		fmt.Sscanf(raw, "%f", &f)
		if f > 0 {
			prices = append(prices, jdPricePos{Value: f, Index: m[0]})
		}
	}
	if len(prices) == 0 {
		return 0, 0, 0, 0
	}

	price = firstLabeledCartPrice(text,
		"套装到手价", "学生到手价", "新人专享到手价", "PLUS到手价",
		"政府补贴价", "补贴到手价", "补贴价", "到手价", "实付", "券后")
	explicitPrice := price > 0
	orig = firstLabeledCartPrice(text, "京东价", "划线价", "原价")
	if price <= 0 {
		price = prices[0].Value
	}
	if orig <= 0 {
		// 购物车常在到手价后面紧跟划线价，但没有“京东价”标签。
		for i, p := range prices {
			if p.Value != price {
				continue
			}
			if i+1 < len(prices) {
				next := prices[i+1].Value
				if next > price && next <= price*3 {
					orig = next
				}
			}
			break
		}
	}
	if orig > 0 && orig <= price {
		orig = 0
	}
	if strings.Contains(text, "国补") || strings.Contains(text, "国家补贴") || strings.Contains(text, "政府补贴") {
		hint := ParseSubsidyHint(ExtractSubsidyContext(text))
		if hint.Amount > 0 || hint.Rate > 0 {
			base := firstPositive(orig, price)
			if explicitPrice && base > price {
				// 购物车已经给出到手价时直接采用，只补算补贴金额/比例。
				subsidyAmount = base - price
				subsidyRate = hint.Rate
			} else if computed, amount := ComputeSubsidizedPrice(base, hint.Amount, hint.Rate, hint.Cap); computed > 0 && computed < price {
				price = computed
				subsidyAmount = amount
				subsidyRate = hint.Rate
			}
		}
	}
	return price, orig, subsidyAmount, subsidyRate
}

func firstLabeledCartPrice(text string, labels ...string) float64 {
	labeled := extractLabeledCartPrices(text)
	for _, label := range labels {
		if v := labeled[label]; v > 0 {
			return v
		}
	}
	return 0
}

var jdCartLabelPriceRe = regexp.MustCompile(`([¥￥]\s*\d+(?:,\d{3})*(?:\.\d{1,2})?)\s*(套装到手价|学生到手价|新人专享到手价|PLUS到手价|政府补贴价|补贴到手价|补贴价|到手价|实付|券后|京东价|划线价|原价)|(套装到手价|学生到手价|新人专享到手价|PLUS到手价|政府补贴价|补贴到手价|补贴价|到手价|实付|券后|京东价|划线价|原价)\s*([¥￥]\s*\d+(?:,\d{3})*(?:\.\d{1,2})?)`)

// extractLabeledCartPrices 解析“¥5.21 到手价”“到手价 ¥5.21”两种顺序，
// 避免把相邻的“京东价”误配到“到手价”前面的数字。
func extractLabeledCartPrices(text string) map[string]float64 {
	out := map[string]float64{}
	for _, m := range jdCartLabelPriceRe.FindAllStringSubmatch(text, -1) {
		if len(m) < 5 {
			continue
		}
		label := strings.TrimSpace(m[2])
		raw := strings.TrimSpace(m[1])
		if label == "" {
			label = strings.TrimSpace(m[3])
			raw = strings.TrimSpace(m[4])
		}
		if label == "" || raw == "" {
			continue
		}
		var f float64
		fmt.Sscanf(strings.NewReplacer("¥", "", "￥", "", ",", "").Replace(raw), "%f", &f)
		if f > 0 {
			if _, exists := out[label]; !exists {
				out[label] = f
			}
		}
	}
	return out
}
