package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/Allinost/go-backend-core/zzz/login3rd/provider"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

const jdCartURL = "https://cart.jd.com/cart_index"

var (
	jdCartPriceRe = regexp.MustCompile(`[¥￥]\s*(\d+(?:,\d{3})*(?:\.\d{1,2})?)`)
	errJDCartMiss = fmt.Errorf("jd cart: 购物车中未找到该 SKU")
)

type jdCartExtract struct {
	URL     string `json:"url"`
	Title   string `json:"title"`
	Body    string `json:"body"`
	Matches []struct {
		Text  string `json:"text"`
		Title string `json:"title"`
	} `json:"matches"`
}

// fetchJDCartPrice 从京东购物车读取指定 SKU 的价格。
//
// 商品详情页被频控时，购物车通常仍能渲染商品行（前提是用户已把商品加入购物车）。
// 这里按 SKU 找到对应商品行，优先取“到手价”，并把“京东价/划线价”作为原价。
func (c *JDCollector) fetchJDCartPrice(ctx context.Context, skuID string) (*PriceResult, error) {
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
	chromedp.Run(tabCtx, chromedp.Sleep(2*time.Second))

	var raw string
	if err := chromedp.Run(tabCtx, chromedp.Evaluate(fmt.Sprintf(`(function(){
		var sku=%q;
		var links=[];
		var seen={};
		function add(el){ if(el && !seen[el]){seen[el]=1; links.push(el);} }
		document.querySelectorAll('a[href*="item.jd.com/'+sku+'.html"]').forEach(add);
		document.querySelectorAll('[data-sku="'+sku+'"]').forEach(function(el){
			add(el.querySelector('a')||el);
		});
		var matches=[];
		links.forEach(function(a){
			var el=a;
			for(var d=0; d<10 && el; d++){
				var t=(el.innerText||'').replace(/\s+/g,' ').trim();
				if(t.indexOf('¥')>=0 || t.indexOf('￥')>=0){
					var img=a.querySelector('img');
					matches.push({text:t.slice(0,2000),title:(a.getAttribute('title')||(img&&img.alt)||'').trim()});
					break;
				}
				el=el.parentElement;
			}
		});
		var body=document.body?document.body.innerText:'';
		return JSON.stringify({url:location.href,title:document.title,body:body.slice(0,4000),matches:matches});
	})()`, skuID), &raw)); err != nil {
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
	if len(page.Matches) == 0 {
		if strings.Contains(page.Body, "购物车跑丢了") || strings.Contains(page.Body, "购物车(0)") {
			return nil, errJDCartMiss
		}
		return nil, errJDCartMiss
	}

	item := page.Matches[0]
	price, orig, subsidyAmount, subsidyRate := parseJDCartItemPrice(item.Text)
	if price <= 0 {
		return nil, fmt.Errorf("jd cart: 商品行未解析到价格")
	}
	title := cleanJdTitle(strings.TrimSpace(item.Title))
	stock := "in_stock"
	if strings.Contains(item.Text, "无货") || strings.Contains(item.Text, "失效") || strings.Contains(item.Text, "下架") {
		stock = "out_of_stock"
	}

	result := &PriceResult{
		CurrentPrice:  round2(price),
		OriginalPrice: round2(orig),
		BasePrice:     round2(firstPositive(orig, price)),
		SubsidyAmount: round2(subsidyAmount),
		SubsidyRate:   subsidyRate,
		PriceSource:   "jd_cart",
		Title:         title,
		StockStatus:   stock,
		ProductURL:    fmt.Sprintf(jdDesktopURL, skuID),
		FetchedAt:     time.Now(),
	}
	fmt.Fprintf(os.Stderr, "[jd cart] sku=%s price=%v orig=%v title=%q stock=%s\n",
		skuID, result.CurrentPrice, result.OriginalPrice, result.Title, result.StockStatus)
	return result, nil
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

	price = firstLabeledCartPrice(text, "到手价", "实付", "券后", "PLUS到手价")
	explicitPrice := price > 0
	orig = firstLabeledCartPrice(text, "京东价", "划线价", "原价")
	if price <= 0 {
		price = prices[0].Value
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

var jdCartLabelPriceRe = regexp.MustCompile(`([¥￥]\s*\d+(?:,\d{3})*(?:\.\d{1,2})?)\s*(到手价|实付|券后|PLUS到手价|京东价|划线价|原价)|(到手价|实付|券后|PLUS到手价|京东价|划线价|原价)\s*([¥￥]\s*\d+(?:,\d{3})*(?:\.\d{1,2})?)`)

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
