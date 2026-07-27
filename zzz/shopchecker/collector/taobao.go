package collector

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)
type TaobaoCollector struct {
	client *http.Client
}

// NewTaobaoCollector 创建淘宝价格采集器
func NewTaobaoCollector() *TaobaoCollector {
	return &TaobaoCollector{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

// Platform 返回平台标识
func (c *TaobaoCollector) Platform() string {
	return "taobao"
}

// FetchPrice 抓取淘宝商品价格
// 通过请求移动端页面 m.tb.cn 或 detail 页获取内嵌 JSON 数据
func (c *TaobaoCollector) FetchPrice(ctx context.Context, rawURL string) (*PriceResult, error) {
	html, err := c.fetchPage(ctx, rawURL)
	if err != nil {
		return nil, fmt.Errorf("taobao: 获取页面失败: %w", err)
	}

	result := &PriceResult{
		CurrentPrice:  0,
		OriginalPrice: 0,
		Title:         extractTaobaoTitle(html),
		FetchedAt:     time.Now(),
	}

	price, origPrice := extractTaobaoPrice(html)
	result.CurrentPrice = price
	result.OriginalPrice = origPrice

	return result, nil
}

func (c *TaobaoCollector) fetchPage(ctx context.Context, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	return string(body), nil
}

func extractTaobaoTitle(html string) string {
	re := regexp.MustCompile(`<title>([^<]+)</title>`)
	matches := re.FindStringSubmatch(html)
	if len(matches) >= 2 {
		title := strings.TrimSpace(matches[1])
		title = strings.TrimSuffix(title, "-淘宝网")
		title = strings.TrimSuffix(title, "-淘宝")
		title = strings.TrimSuffix(title, "-tmall")
		title = strings.TrimSuffix(title, "-天猫")
		return strings.TrimSpace(title)
	}
	return ""
}

func extractTaobaoPrice(html string) (float64, float64) {
	patterns := []struct {
		priceRe string
		origRe  string
	}{
		{`"priceInfo":{"price":"([\d.]+)"`, `"priceInfo":{"price":"[\d.]+","priceOrig":"([\d.]+)"`},
		{`"price":"([\d.]+)"`, `"originalPrice":"([\d.]+)"`},
		{`<em[^>]*class=["']tb-rmb["'][^>]*>([\d.]+)`, `<del[^>]*>([\d.]+)`},
	}

	for _, p := range patterns {
		priceRe := regexp.MustCompile(p.priceRe)
		priceMatches := priceRe.FindStringSubmatch(html)
		if len(priceMatches) >= 2 {
			var price float64
			fmt.Sscanf(priceMatches[1], "%f", &price)
			if price > 0 {
				var origPrice float64
				if p.origRe != "" {
					origRe := regexp.MustCompile(p.origRe)
					origMatches := origRe.FindStringSubmatch(html)
					if len(origMatches) >= 2 {
						fmt.Sscanf(origMatches[1], "%f", &origPrice)
					}
				}
				return price, origPrice
			}
		}
	}
	return 0, 0
}

var _ Collector = (*TaobaoCollector)(nil)