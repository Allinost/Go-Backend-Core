// Package quote 天天基金数据源：场外基金/QDII 官方净值与实时估值
package quote

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	pkgNet "github.com/Allinost/go-backend-core/pkg/net"
)

const (
	fundGzURL    = "https://fundgz.1234567.com.cn/js/%s.js?rt=%d"
	fundReferer  = "http://fund.eastmoney.com/"
	fundNavGrace = 7 // 官方净值超过 N 天未更新时视为估值不可用
)

// FundProvider 天天基金行情提供方（场外基金/QDII）。
//
// fundgz 接口单次返回：dwjz/jzrq（最新官方单位净值及日期）、gsz/gztime（实时估算净值）。
// QDII 的估算值基于海外指数盘中走势，T+1 净值公布前作为市值参考。
type FundProvider struct {
	client *pkgNet.HTTPClient
	now    func() time.Time // 可注入时钟，便于测试
}

// NewFundProvider 创建天天基金行情提供方
func NewFundProvider(client *pkgNet.HTTPClient) *FundProvider {
	return &FundProvider{client: client, now: time.Now}
}

// Name 数据源名称
func (p *FundProvider) Name() string { return "fund" }

// Support 支持场外基金
func (p *FundProvider) Support(asset AssetInfo) bool {
	return asset.AssetType == "fund" && asset.Market == "OTC"
}

// Fetch 拉取净值与估值
func (p *FundProvider) Fetch(ctx context.Context, asset AssetInfo) (*Quote, error) {
	url := fmt.Sprintf(fundGzURL, asset.Code, p.now().Unix())
	resp, err := p.client.Do(ctx, "GET", url, nil, map[string]string{"Referer": fundReferer})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("quote: fund HTTP %d", resp.StatusCode)
	}
	q, err := ParseFundGzAt(string(resp.Body), p.now())
	if err != nil {
		return nil, err
	}
	q.Code = asset.Code
	return q, nil
}

// ParseFundGz 以当前时间解析 fundgz 响应（测试便利封装）
func ParseFundGz(body string) (*Quote, error) {
	return ParseFundGzAt(body, time.Now())
}

// ParseFundGzAt 解析 fundgz JSONP 响应（可注入当前时间）：
//
//	jsonpgz({"fundcode":"270042","name":"...","jzrq":"2026-08-25","dwjz":"1.2340","gsz":"1.2405","gszzl":"0.53","gztime":"2026-08-26 15:00"});
func ParseFundGzAt(body string, now time.Time) (*Quote, error) {
	start := strings.IndexByte(body, '(')
	end := strings.LastIndexByte(body, ')')
	if start < 0 || end <= start {
		return nil, fmt.Errorf("quote: fundgz 响应格式非法")
	}
	payload := body[start+1 : end]

	var raw struct {
		Code   string `json:"fundcode"`
		Name   string `json:"name"`
		JZRQ   string `json:"jzrq"`  // 官方净值日期
		DWJZ   string `json:"dwjz"`  // 官方单位净值
		GSZ    string `json:"gsz"`   // 估算净值
		GSZZL  string `json:"gszzl"` // 估算涨跌幅 %
		GZTime string `json:"gztime"`
	}
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		return nil, fmt.Errorf("quote: 解析 fundgz 数据失败: %w", err)
	}
	if raw.Code == "" || raw.DWJZ == "" {
		return nil, fmt.Errorf("quote: fundgz 无有效净值数据")
	}

	nav, err := strconv.ParseFloat(strings.TrimSpace(raw.DWJZ), 64)
	if err != nil || nav <= 0 {
		return nil, fmt.Errorf("quote: fundgz 官方净值非法: %q", raw.DWJZ)
	}

	q := &Quote{
		Name:    raw.Name,
		Nav:     nav,
		NavDate: strings.TrimSpace(raw.JZRQ),
		Source:  "official",
	}
	price := nav
	if est, err := strconv.ParseFloat(strings.TrimSpace(raw.GSZ), 64); err == nil && est > 0 && q.NavDate != "" {
		// 官方净值在宽限期内才认为估值有效；长期未更新则退化为官方净值
		navDate, derr := time.ParseInLocation(dateLayoutQuote, q.NavDate, time.Local)
		fresh := derr == nil && now.Sub(navDate) < time.Duration(fundNavGrace)*24*time.Hour
		if fresh {
			q.Estimate = est
			price = est
			q.Source = "estimated"
		}
	}
	q.Price = price
	return q, nil
}
