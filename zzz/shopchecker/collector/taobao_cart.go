package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
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
	ItemID        string  `json:"itemId"`
	SKUID         string  `json:"skuId"`
	Quantity      int     `json:"quantity"`
	PayTotal      int64   `json:"payTotal"` // 分，商品行合计
	PayNow        int64   `json:"payNow"`   // 分，当前单价
	PayOrigin     int64   `json:"payOrigin"`
	Valid         bool    `json:"valid"`
	SKUInvalid    bool    `json:"skuInvalid"`
	Title         string  `json:"title"`
	CartID        string  `json:"cartId"`
	SubsidyPrice  float64 `json:"subsidyPrice"`  // 元，购物车“领政府补贴后约”价格
	PlatformPrice float64 `json:"platformPrice"` // 元，购物车“平台加补后”价格
}

type tbCartDomInfo struct {
	SubsidyPrice  float64 `json:"subsidyPrice"`
	PlatformPrice float64 `json:"platformPrice"`
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
	// 购物车 DOM 会展示“平台加补后 / 领政府补贴后约”价格，先采集首屏。
	_ = chromedp.Run(tabCtx, chromedp.Evaluate(tbCartDomCollectJS, nil))

	// 滚动触发分页，直到连续多次没有新增。
	lastCount := -1
	stale := 0
	for i := 0; i < tbCartMaxScrolls; i++ {
		var count int
		_ = chromedp.Run(tabCtx, chromedp.Evaluate(tbCartScrollJS, &count))
		time.Sleep(tbCartScrollWait)
		_ = chromedp.Run(tabCtx, chromedp.Evaluate(tbCartDomCollectJS, nil))
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
	_ = chromedp.Run(tabCtx, chromedp.Evaluate(tbCartDomCollectJS, nil))

	var rawJSON string
	if err := chromedp.Run(tabCtx, chromedp.Evaluate(tbCartCollectJS, &rawJSON)); err != nil {
		return nil, fmt.Errorf("读取购物车数据失败: %w", err)
	}
	var raw []tbCartRaw
	if err := json.Unmarshal([]byte(rawJSON), &raw); err != nil {
		return nil, fmt.Errorf("解析购物车数据失败: %w", err)
	}

	// 合并 DOM 中的国补/平台加补价格。
	var domJSON string
	if err := chromedp.Run(tabCtx, chromedp.Evaluate(`JSON.stringify(window.__tbCartDomMap || {})`, &domJSON)); err == nil && domJSON != "" && domJSON != "{}" {
		var domMap map[string]tbCartDomInfo
		if json.Unmarshal([]byte(domJSON), &domMap) == nil {
			for i := range raw {
				key := raw[i].ItemID + "|" + raw[i].SKUID
				if info, ok := domMap[key]; ok {
					raw[i].SubsidyPrice = info.SubsidyPrice
					raw[i].PlatformPrice = info.PlatformPrice
				}
			}
		}
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

const tbCartDomCollectJS = `(function(){
	function numAfter(text, labels){
		for(var i = 0; i < labels.length; i++){
			var idx = text.indexOf(labels[i]);
			if(idx < 0) continue;
			var seg = text.slice(idx + labels[i].length, idx + labels[i].length + 80);
			var m = seg.match(/[¥￥]?\s*(\d+(?:\s*\.\s*\d{1,2})?)/);
			if(!m) continue;
			var v = parseFloat(m[1].replace(/\s+/g, ''));
			if(!isNaN(v) && v > 0) return v;
		}
		return 0;
	}
	var map = window.__tbCartDomMap || {};
	var boxes = document.querySelectorAll('[class*="trade-cart-item-info"]');
	for(var i = 0; i < boxes.length; i++){
		var box = boxes[i];
		var a = box.querySelector('a[href*="id="]');
		if(!a) continue;
		var href = a.getAttribute('href') || a.href || '';
		var m = href.match(/[?&]id=(\d+)/);
		if(!m) continue;
		var sk = href.match(/[?&]skuId=(\d+)/);
		var skuId = sk ? sk[1] : '';
		var text = (box.innerText || '').replace(/\s+/g, ' ');
		var subsidy = numAfter(text, ['领政府补贴后约','政府补贴后约','国补后约','补贴后约','领政府补贴后','政府补贴后','国补后']);
		var platform = numAfter(text, ['平台加补后','平台补贴后','平台补后']);
		if(subsidy > 0 || platform > 0){
			var key = m[1] + '|' + skuId;
			var prev = map[key] || {};
			map[key] = {
				subsidyPrice: subsidy > 0 ? subsidy : (prev.subsidyPrice || 0),
				platformPrice: platform > 0 ? platform : (prev.platformPrice || 0)
			};
		}
	}
	window.__tbCartDomMap = map;
	return Object.keys(map).length;
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
		current := round2(float64(unitCents) / 100)
		if current <= 0 {
			continue
		}

		orig := 0.0
		if it.PayOrigin > 0 {
			origCents := it.PayOrigin
			if qty > 1 {
				origCents = it.PayOrigin / int64(qty)
			}
			if o := round2(float64(origCents) / 100); o > current {
				orig = o
			}
		}

		price := current
		basePrice := current
		subsidyAmount := 0.0
		subsidyRate := 0.0
		source := "taobao_cart"
		note := ""

		// 购物车行展示“平台加补后 / 领政府补贴后约”时，以后者作为到手价。
		if it.SubsidyPrice > 0 && round2(it.SubsidyPrice) < current {
			base := it.PlatformPrice
			if base <= 0 || base >= current {
				base = current
			}
			base = round2(base)
			price = round2(it.SubsidyPrice)
			basePrice = base
			orig = current // 国补前的购物车当前价（如 ￥2300）
			if base > price {
				subsidyAmount = round2(base - price)
				if it.PlatformPrice > 0 {
					subsidyRate = round2((base - price) / base * 100)
				}
				source = "taobao_cart_subsidy"
				note = "购物车国补价"
			} else {
				price = current
				basePrice = current
			}
		}

		productURL := fmt.Sprintf("https://item.taobao.com/item.htm?id=%s", it.ItemID)
		if it.SKUID != "" {
			productURL += "&skuId=" + url.QueryEscape(it.SKUID)
		}
		result := &PriceResult{
			CurrentPrice:  price,
			OriginalPrice: orig,
			BasePrice:     basePrice,
			SubsidyAmount: subsidyAmount,
			SubsidyRate:   subsidyRate,
			PriceSource:   source,
			Title:         cleanTaobaoTitle(strings.TrimSpace(it.Title)),
			StockStatus:   "in_stock",
			ProductURL:    productURL,
			CartStatus:    "hit",
			CartNote:      note,
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
// 分享链接里的 price 参数（如淘宝 1690、天猫 2300）可用于在同商品多规格中定位正确规格。
func matchTaobaoCartItem(byKey map[string]*PriceResult, byItem map[string][]*PriceResult, itemID, skuID string, priceHint float64) *PriceResult {
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

	if priceHint > 0 {
		var matched []*PriceResult
		for _, r := range list {
			if taobaoCartPriceMatches(r, priceHint) {
				matched = append(matched, r)
			}
		}
		if len(matched) > 0 {
			best := matched[0]
			for _, r := range matched[1:] {
				if betterTaobaoCartCandidate(r, best) {
					best = r
				}
			}
			cp := *best
			if len(list) > 1 {
				cp.CartNote = appendCartNote(cp.CartNote, "同商品多规格，按分享价匹配规格")
			}
			return &cp
		}
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
		cp.CartNote = appendCartNote(cp.CartNote, "同商品多规格，按有货且最低价规格命中")
	}
	return &cp
}

// taobaoCartPriceMatches 用分享链接里的价格定位规格；国补后价格会变化，
// 因此同时比较到手价、国补前基础价和原价。
func taobaoCartPriceMatches(r *PriceResult, hint float64) bool {
	if r == nil || hint <= 0 {
		return false
	}
	for _, v := range []float64{r.CurrentPrice, r.BasePrice, r.OriginalPrice} {
		if v > 0 && math.Abs(v-hint) < 0.01 {
			return true
		}
	}
	return false
}

func appendCartNote(note, add string) string {
	if note == "" {
		return add
	}
	return note + "；" + add
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

// extractTaobaoSharePrice 解析分享链接/商品 URL 中的 price 参数（元）。
// 淘宝同一链接可能对应多个 SKU，分享价是定位正确规格的关键线索。
func extractTaobaoSharePrice(rawInput string) float64 {
	rawURL := extractTaobaoURL(rawInput)
	if rawURL == "" {
		rawURL = strings.TrimSpace(rawInput)
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return 0
	}
	for _, pair := range strings.Split(u.RawQuery, "&") {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key, _ := url.QueryUnescape(kv[0])
		if key != "price" {
			continue
		}
		v, _ := url.QueryUnescape(kv[1])
		return parseFloatStr(strings.TrimSpace(v))
	}
	return 0
}

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
