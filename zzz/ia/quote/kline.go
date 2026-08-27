// Package quote 东方财富日K线数据源：基准指数对比、定投模拟等场景使用
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
	klineURL = "https://push2his.eastmoney.com/api/qt/stock/kline/get?secid=%s&fields1=f1,f2,f3&fields2=f51,f53&klt=101&fqt=1&beg=%s&end=20500101"
)

// 常用基准指数 secid 映射
var BenchmarkSecids = map[string]string{
	"CSI300":  "1.000300", // 沪深300
	"CSI500":  "1.000905", // 中证500
	"CSI1000": "1.000852", // 中证1000
	"SSE":     "1.000001", // 上证指数
	"SZSE":    "0.399001", // 深证成指
	"GEM":     "0.399006", // 创业板指
}

// ResolveBenchmarkSecid 解析基准标识：支持内置别名或直接传 secid（如 1.000300）
func ResolveBenchmarkSecid(name string) (string, bool) {
	if v, ok := BenchmarkSecids[strings.ToUpper(strings.TrimSpace(name))]; ok {
		return v, true
	}
	parts := strings.SplitN(name, ".", 2)
	if len(parts) == 2 {
		if _, err := strconv.Atoi(parts[0]); err == nil && parts[1] != "" {
			return name, true
		}
	}
	return "", false
}

// KLinePoint 日K线收盘点
type KLinePoint struct {
	Date  string  `json:"date"` // YYYY-MM-DD
	Close float64 `json:"close"`
}

// FetchKline 拉取日K线收盘序列（beg 格式 YYYYMMDD，返回按日期升序）
func FetchKline(ctx context.Context, client *pkgNet.HTTPClient, secid, beg string) ([]KLinePoint, error) {
	url := fmt.Sprintf(klineURL, secid, strings.ReplaceAll(beg, "-", ""))
	resp, err := client.Get(ctx, url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("quote: kline HTTP %d", resp.StatusCode)
	}
	return ParseKline(resp.Body, secid)
}

// Kline 通过注册表的 HTTP 客户端拉取日K线
func (r *Registry) Kline(ctx context.Context, secid, beg string) ([]KLinePoint, error) {
	return FetchKline(ctx, r.httpClient, secid, beg)
}

// ParseKline 解析 kline 接口响应体
func ParseKline(body []byte, secid string) ([]KLinePoint, error) {
	var raw struct {
		Data *struct {
			Code   string   `json:"code"`
			Name   string   `json:"name"`
			Klines []string `json:"klines"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("quote: 解析 kline 响应失败: %w", err)
	}
	if raw.Data == nil || len(raw.Data.Klines) == 0 {
		return nil, fmt.Errorf("quote: kline 无数据 (%s)", secid)
	}
	out := make([]KLinePoint, 0, len(raw.Data.Klines))
	for _, line := range raw.Data.Klines {
		parts := strings.Split(line, ",")
		if len(parts) < 2 {
			continue
		}
		closeVal, err := strconv.ParseFloat(parts[1], 64)
		if err != nil || closeVal <= 0 {
			continue
		}
		out = append(out, KLinePoint{Date: parts[0], Close: closeVal})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("quote: kline 解析结果为空 (%s)", secid)
	}
	return out, nil
}

// NormalizeSeries 将序列归一化为指定基期=100 的指数序列；基期取与 base 对齐的首个日期
// 返回 map[日期]归一值，便于与组合曲线按日期对齐
func NormalizeSeries(points []KLinePoint, baseDate string) map[string]float64 {
	var baseVal float64
	baseEffective := ""
	for _, p := range points {
		if p.Date >= baseDate {
			baseVal, baseEffective = p.Close, p.Date
			break
		}
	}
	if baseVal <= 0 {
		return nil
	}
	out := make(map[string]float64)
	for _, p := range points {
		if p.Date >= baseEffective {
			out[p.Date] = p.Close / baseVal * 100
		}
	}
	return out
}

// FilterKlineByRange 按闭区间过滤 K 线序列
func FilterKlineByRange(points []KLinePoint, start, end string) []KLinePoint {
	now := time.Now().Format(dateLayoutQuote)
	if end == "" || end > now {
		end = now
	}
	out := make([]KLinePoint, 0, len(points))
	for _, p := range points {
		if p.Date >= start && p.Date <= end {
			out = append(out, p)
		}
	}
	return out
}
