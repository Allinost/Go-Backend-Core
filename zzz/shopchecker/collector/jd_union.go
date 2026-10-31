package collector

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// fetchJDUnionPrice 使用京东联盟开放接口获取价格（可选能力）。
// 配置环境变量 JD_UNION_APP_KEY / JD_UNION_APP_SECRET 后启用。
// 联盟价格通常是商品展示价/最低价，不一定包含国补；浏览器采集仍然是国补到手价的首选。
func fetchJDUnionPrice(ctx context.Context, skuID string) (*PriceResult, error) {
	appKey := strings.TrimSpace(os.Getenv("JD_UNION_APP_KEY"))
	appSecret := strings.TrimSpace(os.Getenv("JD_UNION_APP_SECRET"))
	if appKey == "" || appSecret == "" {
		return nil, fmt.Errorf("jd union 未配置")
	}

	paramJSON := fmt.Sprintf(`{"skuIds":"%s"}`, skuID)
	params := map[string]string{
		"method":            "jd.union.open.goods.promotiongoodsinfo.query",
		"app_key":           appKey,
		"timestamp":         time.Now().Format("2006-01-02 15:04:05"),
		"format":            "json",
		"v":                 "1.0",
		"sign_method":       "md5",
		"360buy_param_json": paramJSON,
	}
	params["sign"] = jdUnionSign(params, appSecret)

	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.jd.com/routerjson", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jd union HTTP %d", resp.StatusCode)
	}
	return parseJDUnionResponse(body)
}

func jdUnionSign(params map[string]string, secret string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		if k == "sign" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(secret)
	for _, k := range keys {
		b.WriteString(k)
		b.WriteString(params[k])
	}
	b.WriteString(secret)
	sum := md5.Sum([]byte(b.String()))
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}

func parseJDUnionResponse(body []byte) (*PriceResult, error) {
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, err
	}
	// 联盟接口会把业务响应放在 queryResult 字符串里。
	var data any = root
	if qr, ok := root["queryResult"].(string); ok && qr != "" {
		var parsed any
		if json.Unmarshal([]byte(qr), &parsed) == nil {
			data = parsed
		}
	} else {
		for _, v := range root {
			if m, ok := v.(map[string]any); ok {
				if qr, ok := m["queryResult"].(string); ok && qr != "" {
					var parsed any
					if json.Unmarshal([]byte(qr), &parsed) == nil {
						data = parsed
					}
					break
				}
			}
		}
	}

	priceFields := map[string]float64{}
	title := ""
	walkJDUnionJSON(data, func(key string, value any) {
		switch key {
		case "lowestPrice", "price", "purchasePrice", "marketPrice", "originalPrice", "jdPrice":
			if f := jdAnyFloat(value); f > 0 {
				priceFields[key] = f
			}
		case "skuName", "goodsName", "wareName", "title":
			if title == "" {
				if s, ok := value.(string); ok {
					title = cleanJdTitle(strings.TrimSpace(s))
				}
			}
		}
	})

	price := firstPositive(priceFields["lowestPrice"], priceFields["purchasePrice"], priceFields["price"], priceFields["jdPrice"])
	if price <= 0 {
		return nil, fmt.Errorf("jd union 未返回价格")
	}
	orig := firstPositive(priceFields["marketPrice"], priceFields["originalPrice"])
	if orig > 0 && orig < price {
		orig = 0
	}
	return &PriceResult{
		CurrentPrice:  price,
		OriginalPrice: orig,
		BasePrice:     firstPositive(orig, price),
		PriceSource:   "jd_union",
		Title:         title,
		StockStatus:   "unknown",
		FetchedAt:     time.Now(),
	}, nil
}

func walkJDUnionJSON(v any, fn func(key string, value any)) {
	switch t := v.(type) {
	case map[string]any:
		for k, child := range t {
			fn(k, child)
			walkJDUnionJSON(child, fn)
		}
	case []any:
		for _, child := range t {
			walkJDUnionJSON(child, fn)
		}
	}
}

func jdAnyFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case json.Number:
		f, _ := t.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(strings.Trim(t, "¥￥")), 64)
		return f
	default:
		return 0
	}
}
