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

// PDDCollector 拼多多价格采集器
type PDDCollector struct {
	client *http.Client
}

// NewPDDCollector 创建拼多多价格采集器
func NewPDDCollector() *PDDCollector {
	return &PDDCollector{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

// Platform 返回平台标识
func (c *PDDCollector) Platform() string {
	return "pdd"
}

// FetchPrice 抓取拼多多商品价格
// PDD 反爬较严，尝试从页面 HTML 中提取内嵌价格数据
func (c *PDDCollector) FetchPrice(ctx context.Context, rawURL string) (*PriceResult, error) {
	html, err := c.fetchPage(ctx, rawURL)
	if err != nil {
		return nil, fmt.Errorf("pdd: 获取页面失败: %w", err)
	}

	result := &PriceResult{
		CurrentPrice:  0,
		OriginalPrice: 0,
		Title:         extractPDDTitle(html),
		FetchedAt:     time.Now(),
	}

	price, origPrice := extractPDDPrice(html)
	result.CurrentPrice = price
	result.OriginalPrice = origPrice

	return result, nil
}

func (c *PDDCollector) fetchPage(ctx context.Context, rawURL string) (string, error) {
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

func extractPDDTitle(html string) string {
	re := regexp.MustCompile(`<title>([^<]+)</title>`)
	matches := re.FindStringSubmatch(html)
	if len(matches) >= 2 {
		title := strings.TrimSpace(matches[1])
		title = strings.TrimSuffix(title, "-拼多多")
		return strings.TrimSpace(title)
	}
	return ""
}

func extractPDDPrice(html string) (float64, float64) {
	patterns := []struct {
		priceRe string
		origRe  string
	}{
		{`"minOnSaleGroupPrice":(\d+)`, `"minOnSalePrice":(\d+)`},
		{`"price":"?(\d+\.?\d*)"?`, `"marketPrice":"?(\d+\.?\d*)"?`},
		{`¥([\d.]+)`, `¥([\d.]+)`},
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
				if origPrice < price {
					origPrice = 0
				}
				return price, origPrice
			}
		}
	}
	return 0, 0
}

var _ Collector = (*PDDCollector)(nil)