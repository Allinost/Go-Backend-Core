package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Allinost/go-backend-core/zzz/login3rd/provider"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
)

const (
	tbCartURL          = "https://cart.taobao.com/cart.htm"
	tbCartCacheTTL     = 2 * time.Minute
	tbCartErrorTTL     = 30 * time.Second
	tbCartMaxScrolls   = 20
	tbCartScrollWait   = 1500 * time.Millisecond
	tbCartStaleLimit   = 3
	tbCartMaxFetchTime = 90 * time.Second
)

// tbCartRaw 购物车接口返回的单个商品行（v2 / v1 两种结构统一后的字段）。
type tbCartRaw struct {
	ItemID     string `json:"itemId"`
	SKUID      string `json:"skuId"`
	Quantity   int    `json:"quantity"`
	PayTotal   int64  `json:"payTotal"` // 分，商品行合计
	PayNow     int64  `json:"payNow"`   // 分，当前单价
	PayOrigin  int64  `json:"payOrigin"`
	Valid      bool   `json:"valid"`
	SKUInvalid bool   `json:"skuInvalid"`
	Title      string `json:"title"`
	CartID     string `json:"cartId"`
}

type taobaoCartSnapshot struct {
	ByKey     map[string]*PriceResult
	ByItem    map[string][]*PriceResult
	FetchedAt time.Time
	Err       error
}

var (
	tbCartCacheMu sync.Mutex
	tbCartCache   taobaoCartSnapshot
)

// getTaobaoCartSnapshot 完整拉取一次购物车并缓存。
// 淘宝与天猫共用同一登录态，因此共用同一份购物车快照。
func (c *TaobaoCollector) getTaobaoCartSnapshot(ctx context.Context, force bool) (map[string]*PriceResult, map[string][]*PriceResult, error) {
	tbCartCacheMu.Lock()
	defer tbCartCacheMu.Unlock()

	if !force && !tbCartCache.FetchedAt.IsZero() {
		ttl := tbCartCacheTTL
		if tbCartCache.Err != nil {
			ttl = tbCartErrorTTL
		}
		if time.Since(tbCartCache.FetchedAt) < ttl {
			return tbCartCache.ByKey, tbCartCache.ByItem, tbCartCache.Err
		}
	}

	raw, err := c.fetchTaobaoCart(ctx)
	var byKey map[string]*PriceResult
	var byItem map[string][]*PriceResult
	if err == nil {
		byKey, byItem = buildTaobaoCartIndex(raw)
	}
	tbCartCache = taobaoCartSnapshot{ByKey: byKey, ByItem: byItem, FetchedAt: time.Now(), Err: err}
	return byKey, byItem, err
}

// cacheTaobaoCartItem 将刚补进购物车的商品写入当前快照，避免同一周期重复加购。
func cacheTaobaoCartItem(itemID, skuID string, result *PriceResult) {
	tbCartCacheMu.Lock()
	defer tbCartCacheMu.Unlock()
	if tbCartCache.ByKey == nil || result == nil {
		return
	}
	key := itemID + "|" + skuID
	if _, exists := tbCartCache.ByKey[key]; exists {
		return
	}
	copied := *result
	copied.CartStatus = "hit"
	copied.CartNote = "已加入购物车"
	tbCartCache.ByKey[key] = &copied
	tbCartCache.ByItem[itemID] = append(tbCartCache.ByItem[itemID], &copied)
}

// fetchTaobaoCart 打开购物车页面，调用 mtop.trade.query.bag 拉取全部商品。
// 购物车前端采用分页接口且分页参数为压缩协议，这里通过拦截页面自身的分页响应，
// 配合滚动加载，收集所有 item/itemv2 节点。
func (c *TaobaoCollector) fetchTaobaoCart(ctx context.Context) ([]tbCartRaw, error) {
	return c.fetchTaobaoCartWithCookies(ctx, c.getSessionCookies(ctx))
}

func (c *TaobaoCollector) fetchTaobaoCartWithCookies(ctx context.Context, sessionCookies []*provider.Cookie) ([]tbCartRaw, error) {
	if len(sessionCookies) == 0 {
		return nil, fmt.Errorf("无登录 session")
	}

	cartCtx, cancel := context.WithTimeout(ctx, tbCartMaxFetchTime)
	defer cancel()

	tabCtx, cancelTab, err := c.createTempBrowser(cartCtx)
	if err != nil {
		return nil, err
	}
	defer cancelTab()

	if err := chromedp.Run(tabCtx,
		emulation.SetUserAgentOverride(tbDeskUA),
		emulation.SetDeviceMetricsOverride(1440, 900, 1, false),
	); err != nil {
		return nil, fmt.Errorf("设置浏览器身份失败: %w", err)
	}
	setBrowserCookies(tabCtx, sessionCookies)

	if err := chromedp.Run(tabCtx, chromedp.Navigate(tbCartURL)); err != nil {
		fmt.Fprintf(os.Stderr, "[taobao cart] Navigate 未在预期内完成（%v），继续轮询 body\n", err)
	}
	if err := waitTaobaoBody(tabCtx, 20*time.Second); err != nil {
		return nil, err
	}
	chromedp.Run(tabCtx, chromedp.Sleep(5*time.Second))

	var currentURL, pageTitle string
	chromedp.Run(tabCtx, chromedp.Location(&currentURL))
	chromedp.Run(tabCtx, chromedp.Title(&pageTitle))
	if isTaobaoLoginRedirect(currentURL, pageTitle) {
		return nil, fmt.Errorf("需要登录（session 已过期）")
	}
	if err := waitTaobaoMtop(tabCtx, 10*time.Second); err != nil {
		return nil, err
	}

	// 拦截 query.bag 响应，页面滚动触发的分页数据也会被收集。
	var patch string
	if err := chromedp.Run(tabCtx, chromedp.Evaluate(tbCartPatchJS, &patch)); err != nil {
		return nil, fmt.Errorf("挂载购物车接口监听失败: %w", err)
	}
	if patch != "patched" && patch != "already" {
		return nil, fmt.Errorf("购物车 mtop 不可用: %s", patch)
	}

	// 主动拉一次首屏（v2 结构，50 条）。
	var build string
	_ = chromedp.Run(tabCtx, chromedp.Evaluate(tbCartBuildJS, &build))
	chromedp.Run(tabCtx, chromedp.Sleep(3*time.Second))

	// 滚动触发分页，直到连续多次没有新增。
	lastCount := -1
	stale := 0
	for i := 0; i < tbCartMaxScrolls; i++ {
		var count int
		_ = chromedp.Run(tabCtx, chromedp.Evaluate(tbCartScrollJS, &count))
		time.Sleep(tbCartScrollWait)
		var newCount int
		_ = chromedp.Run(tabCtx, chromedp.Evaluate(tbCartUniqueCountJS, &newCount))
		if newCount <= lastCount {
			stale++
		} else {
			stale = 0
		}
		lastCount = newCount
		if stale >= tbCartStaleLimit && i >= tbCartStaleLimit {
			break
		}
	}

	var rawJSON string
	if err := chromedp.Run(tabCtx, chromedp.Evaluate(tbCartCollectJS, &rawJSON)); err != nil {
		return nil, fmt.Errorf("读取购物车数据失败: %w", err)
	}
	var raw []tbCartRaw
	if err := json.Unmarshal([]byte(rawJSON), &raw); err != nil {
		return nil, fmt.Errorf("解析购物车数据失败: %w", err)
	}
	fmt.Fprintf(os.Stderr, "[taobao cart] 完整拉取完成 items=%d\n", len(raw))
	return raw, nil
}

func waitTaobaoBody(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		var ok bool
		if err := chromedp.Run(ctx, chromedp.Evaluate(`!!document.body && document.readyState !== 'loading'`, &ok)); err == nil && ok {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("浏览器加载购物车页面失败: body 未就绪")
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func waitTaobaoMtop(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		var ok bool
		if err := chromedp.Run(ctx, chromedp.Evaluate(`!!(window.lib && window.lib.mtop && window.lib.mtop.request)`, &ok)); err == nil && ok {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("购物车页面未加载 mtop 接口")
		}
		time.Sleep(500 * time.Millisecond)
	}
}

const tbCartPatchJS = `(function(){
	var m = window.lib && window.lib.mtop;
	if(!m || !m.request) return 'no mtop';
	if(m.__tbCartPatched) return 'already';
	m.__tbCartPatched = true;
	window.__tbCartPages = [];
	function compact(d){
		var arr = [];
		for(var k in d){
			if(!/^item(v2)?_/.test(k)) continue;
			var node = d[k];
			if(!node || !node.fields) continue;
			var f = node.fields;
			var q = f.quantity;
			var qty = 1;
			if(q && typeof q === 'object') qty = parseInt(q.quantity, 10) || 1;
			else if(q !== undefined && q !== null && q !== '') qty = parseInt(q, 10) || 1;
			var valid = !(f.valid === false || f.valid === 0 || f.valid === '0' || f.valid === 'false');
			arr.push({
				itemId: String(f.itemId || ''),
				skuId: String((f.sku && f.sku.skuId) || ''),
				quantity: qty,
				payTotal: (f.pay && f.pay.total) || 0,
				payNow: (f.pay && f.pay.now) || 0,
				payOrigin: (f.pay && f.pay.origin) || 0,
				valid: valid,
				skuInvalid: !!(f.sku && f.sku.skuInvalid),
				title: f.title || '',
				cartId: String(f.cartId || '')
			});
		}
		return arr;
	}
	var orig = m.request;
	m.request = function(opts, cb){
		var api = opts && opts.api;
		var wrapped = function(res){
			try {
				if(api === 'mtop.trade.query.bag'){
					var d = res && res.data && res.data.data;
					if(d){
						window.__tbCartPages.push({tag: (opts && opts.__customTag__) || '', items: compact(d)});
					}
				}
			} catch(e) {}
			if(cb) return cb(res);
		};
		return orig.call(m, opts, wrapped);
	};
	return 'patched';
})()`

const tbCartBuildJS = `(function(){
	var m = window.lib && window.lib.mtop;
	if(!m) return 'no mtop';
	var ex = JSON.stringify({
		supportCalculateUpdate: 'true', mergeCombo: 'true', globalSell: '1',
		spiPage: 'pcPao', isPcMixCart: 'true', pcOrderFrom: 'pcWeb^cart^mainCart',
		pageVersion: '0.0.56'
	});
	m.request({
		api: 'mtop.trade.query.bag', v: '5.0', dataType: 'json', timeout: 10000,
		data: {
			mixed: false, netType: 0, extStatus: '0', cartFrom: 'main_site',
			isPage: false, exParams: ex
		}
	}, function(){});
	return 'requested';
})()`

const tbCartScrollJS = `(function(){
	window.scrollTo(0, document.documentElement.scrollHeight);
	return (window.__tbCartPages || []).length;
})()`

const tbCartUniqueCountJS = `(function(){
	var pages = window.__tbCartPages || [];
	var seen = {}, n = 0;
	for(var i = 0; i < pages.length; i++){
		var items = pages[i].items || [];
		for(var j = 0; j < items.length; j++){
			var it = items[j];
			var key = it.itemId + '|' + it.skuId;
			if(!seen[key]){ seen[key] = 1; n++; }
		}
	}
	return n;
})()`

const tbCartCollectJS = `(function(){
	var pages = window.__tbCartPages || [];
	var seen = {}, out = [];
	for(var i = 0; i < pages.length; i++){
		var items = pages[i].items || [];
		for(var j = 0; j < items.length; j++){
			var it = items[j];
			if(!it.itemId) continue;
			var key = it.itemId + '|' + it.skuId;
			if(seen[key]) continue;
			seen[key] = 1;
			out.push(it);
		}
	}
	return JSON.stringify(out);
})()`

// buildTaobaoCartIndex 把购物车原始行转换为按 SKU / 按商品索引的价格结果。
func buildTaobaoCartIndex(raw []tbCartRaw) (map[string]*PriceResult, map[string][]*PriceResult) {
	byKey := make(map[string]*PriceResult, len(raw))
	byItem := make(map[string][]*PriceResult, len(raw))
	for _, it := range raw {
		if it.ItemID == "" || it.PayTotal <= 0 {
			continue
		}
		qty := it.Quantity
		if qty <= 0 {
			qty = 1
		}
		unitCents := it.PayTotal
		if qty > 1 {
			unitCents = it.PayTotal / int64(qty)
		}
		price := round2(float64(unitCents) / 100)
		if price <= 0 {
			continue
		}
		orig := 0.0
		if it.PayOrigin > 0 {
			origCents := it.PayOrigin
			if qty > 1 {
				origCents = it.PayOrigin / int64(qty)
			}
			if o := round2(float64(origCents) / 100); o > price {
				orig = o
			}
		}
		productURL := fmt.Sprintf("https://item.taobao.com/item.htm?id=%s", it.ItemID)
		if it.SKUID != "" {
			productURL += "&skuId=" + url.QueryEscape(it.SKUID)
		}
		result := &PriceResult{
			CurrentPrice:  price,
			OriginalPrice: orig,
			BasePrice:     price,
			PriceSource:   "taobao_cart",
			Title:         cleanTaobaoTitle(strings.TrimSpace(it.Title)),
			StockStatus:   "in_stock",
			ProductURL:    productURL,
			CartStatus:    "hit",
			FetchedAt:     time.Now(),
		}
		if !it.Valid || it.SKUInvalid {
			result.StockStatus = "out_of_stock"
		}
		key := it.ItemID + "|" + it.SKUID
		byKey[key] = result
		byItem[it.ItemID] = append(byItem[it.ItemID], result)
	}
	return byKey, byItem
}

// matchTaobaoCartItem 优先按 itemId+skuId 精确匹配；商品 URL 未带 skuId 时按 itemId 匹配。
// 同一商品在购物车有多个规格时，优先有货、再取最低价规格（并记录备注说明）。
func matchTaobaoCartItem(byKey map[string]*PriceResult, byItem map[string][]*PriceResult, itemID, skuID string) *PriceResult {
	if itemID == "" {
		return nil
	}
	if skuID != "" {
		if r := byKey[itemID+"|"+skuID]; r != nil && r.CurrentPrice > 0 {
			cp := *r
			return &cp
		}
		return nil
	}
	list := byItem[itemID]
	if len(list) == 0 {
		return nil
	}
	best := list[0]
	for _, r := range list[1:] {
		if betterTaobaoCartCandidate(r, best) {
			best = r
		}
	}
	if best == nil || best.CurrentPrice <= 0 {
		return nil
	}
	cp := *best
	if len(list) > 1 {
		cp.CartNote = "同商品多规格，购物车按有货且最低价规格命中"
	}
	return &cp
}

func betterTaobaoCartCandidate(a, b *PriceResult) bool {
	if a == nil {
		return false
	}
	if b == nil {
		return true
	}
	aIn := a.StockStatus == "in_stock"
	bIn := b.StockStatus == "in_stock"
	if aIn != bIn {
		return aIn
	}
	return a.CurrentPrice > 0 && a.CurrentPrice < b.CurrentPrice
}

// ---------- 加购 ----------

type tbAddBagResult struct {
	Ret          []string `json:"ret"`
	ErrorMessage string   `json:"errorMessage"`
	ErrorCode    string   `json:"errorCode"`
	Error        string   `json:"error"`
}

// parseTaobaoAddBagResponse 判断 mtop.trade.addBag 是否成功。
func parseTaobaoAddBagResponse(raw string) error {
	var r tbAddBagResult
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		return fmt.Errorf("加购响应解析失败")
	}
	for _, ret := range r.Ret {
		if strings.Contains(strings.ToUpper(ret), "SUCCESS") {
			return nil
		}
	}
	if r.Error != "" {
		return fmt.Errorf("%s", r.Error)
	}
	if r.ErrorMessage != "" {
		return fmt.Errorf("%s", r.ErrorMessage)
	}
	if len(r.Ret) > 0 && r.Ret[0] != "" {
		return fmt.Errorf("%s", r.Ret[0])
	}
	return fmt.Errorf("加购结果未确认")
}

// addTaobaoCartItem 调用页面内 mtop.trade.addBag 把商品补进购物车（best-effort）。
func (c *TaobaoCollector) addTaobaoCartItem(ctx context.Context, itemID, skuID string) error {
	if itemID == "" || skuID == "" {
		return fmt.Errorf("缺少 itemId 或 skuId")
	}
	sessionCookies := c.getSessionCookies(ctx)
	if len(sessionCookies) == 0 {
		return fmt.Errorf("无登录 session")
	}

	addCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	tabCtx, cancelTab, err := c.createTempBrowser(addCtx)
	if err != nil {
		return err
	}
	defer cancelTab()

	if err := chromedp.Run(tabCtx,
		emulation.SetUserAgentOverride(tbDeskUA),
		emulation.SetDeviceMetricsOverride(1440, 900, 1, false),
	); err != nil {
		return fmt.Errorf("设置浏览器身份失败: %w", err)
	}
	setBrowserCookies(tabCtx, sessionCookies)

	if err := chromedp.Run(tabCtx, chromedp.Navigate(tbCartURL)); err != nil {
		fmt.Fprintf(os.Stderr, "[taobao cart] 加购页面 Navigate 未在预期内完成（%v），继续轮询 body\n", err)
	}
	if err := waitTaobaoBody(tabCtx, 20*time.Second); err != nil {
		return err
	}
	chromedp.Run(tabCtx, chromedp.Sleep(4*time.Second))

	var currentURL, pageTitle string
	chromedp.Run(tabCtx, chromedp.Location(&currentURL))
	chromedp.Run(tabCtx, chromedp.Title(&pageTitle))
	if isTaobaoLoginRedirect(currentURL, pageTitle) {
		return fmt.Errorf("需要登录（session 已过期）")
	}
	if err := waitTaobaoMtop(tabCtx, 10*time.Second); err != nil {
		return err
	}

	script := fmt.Sprintf(`(function(){
		window.__tbAddResult = null;
		var m = window.lib && window.lib.mtop;
		if(!m || !m.request){ window.__tbAddResult = {error:'no mtop'}; return 'no mtop'; }
		try {
			m.request({
				api: 'mtop.trade.addBag', v: '3.1', type: 'POST', dataType: 'json',
				timeout: 15000, needLogin: true,
				data: {
					itemId: %q, skuId: %q, quantity: 1,
					exParams: JSON.stringify({from: 'cart', cartFrom: 'main_site', pcOrderFrom: 'pcWeb^cart^mainCart'}),
					serviceId: ''
				}
			}, function(res){ window.__tbAddResult = res || {error:'empty'}; });
			return 'requested';
		} catch(e) {
			window.__tbAddResult = {error: String(e)};
			return 'error';
		}
	})()`, itemID, skuID)
	var call string
	if err := chromedp.Run(tabCtx, chromedp.Evaluate(script, &call)); err != nil {
		return fmt.Errorf("调用加购接口失败: %w", err)
	}
	if call != "requested" {
		return fmt.Errorf("调用加购接口失败: %s", call)
	}

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		var raw string
		if err := chromedp.Run(tabCtx, chromedp.Evaluate(`(function(){ try { return JSON.stringify(window.__tbAddResult); } catch(e) { return ''; } })()`, &raw)); err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if raw != "" && raw != "null" {
			return parseTaobaoAddBagResponse(raw)
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("加购结果超时")
}

// ---------- SKU 解析（sku_properties → skuId） ----------

var tbSkuPairRe = regexp.MustCompile(`"propPath":"([^"]+)","skuId":"(\d+)"`)

// extractTaobaoSkuProperties 解析 URL 中的 sku_properties（如 20122:118105;1627207:44168284707）。
func extractTaobaoSkuProperties(rawInput string) map[string]string {
	rawURL := extractTaobaoURL(rawInput)
	if rawURL == "" {
		rawURL = strings.TrimSpace(rawInput)
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil
	}
	// Go 1.17+ 的 url.Query() 会把分号视为非法分隔符并丢弃整个 sku_properties，
	// 这里手动按 & 拆分并解码。
	raw := ""
	for _, pair := range strings.Split(u.RawQuery, "&") {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key, _ := url.QueryUnescape(kv[0])
		if key != "sku_properties" && key != "skuProperties" {
			continue
		}
		v, _ := url.QueryUnescape(kv[1])
		raw = strings.TrimSpace(v)
		break
	}
	if raw == "" {
		return nil
	}
	out := map[string]string{}
	for _, part := range strings.Split(raw, ";") {
		kv := strings.SplitN(strings.TrimSpace(part), ":", 2)
		if len(kv) == 2 && kv[0] != "" && kv[1] != "" {
			out[kv[0]] = kv[1]
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// matchTaobaoSkuFromHTML 在商品页 skuBase 中查找同时命中所有 sku_properties 的 SKU。
// 仅当结果唯一时返回；多规格无法确定时返回空，避免加购错误规格。
func matchTaobaoSkuFromHTML(html string, props map[string]string) string {
	if len(props) == 0 {
		return ""
	}
	seen := map[string]bool{}
	var matched []string
	for _, m := range tbSkuPairRe.FindAllStringSubmatch(html, -1) {
		parts := strings.Split(m[1], ";")
		ok := true
		for pid, vid := range props {
			found := false
			for _, p := range parts {
				if p == pid+":"+vid {
					found = true
					break
				}
			}
			if !found {
				ok = false
				break
			}
		}
		if ok && !seen[m[2]] {
			seen[m[2]] = true
			matched = append(matched, m[2])
		}
	}
	if len(matched) == 1 {
		return matched[0]
	}
	return ""
}

// resolveTaobaoSkuID 在 URL 未显式带 skuId 时，尝试通过 sku_properties 从商品页解析。
func (c *TaobaoCollector) resolveTaobaoSkuID(ctx context.Context, itemID, skuID, rawURL, sessionCookie string) string {
	if skuID != "" {
		return skuID
	}
	props := extractTaobaoSkuProperties(rawURL)
	if len(props) == 0 {
		return ""
	}
	html := c.fetchHTMLOnly(ctx, tbItemURL(itemID, "", false), sessionCookie, true)
	if html == "" {
		return ""
	}
	return matchTaobaoSkuFromHTML(html, props)
}
