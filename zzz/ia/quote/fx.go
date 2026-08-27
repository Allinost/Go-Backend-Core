// Package quote 免费公开汇率数据源（open.er-api.com），用于多币种资产自动折算
package quote

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	pkgNet "github.com/Allinost/go-backend-core/pkg/net"
)

const fxURL = "https://open.er-api.com/v6/latest/%s"

// SupportedQuoteCcys 模块支持的报价币种（资产表 currency 取值范围）
var SupportedQuoteCcys = []string{"USD", "HKD"}

// FetchRatesFromCNY 以 CNY 为基准拉取最新汇率。
// 返回 map[目标币种]rate，rate 为存储口径 CNY->X：1 单位 X 兑换的 CNY 数量。
func FetchRatesFromCNY(ctx context.Context, client *pkgNet.HTTPClient, targets []string) (map[string]float64, error) {
	resp, err := client.Get(ctx, fmt.Sprintf(fxURL, "CNY"))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("quote: fx HTTP %d", resp.StatusCode)
	}
	return parseFxResponse(resp.Body, targets)
}

// parseFxResponse 解析 er-api 响应并换算为存储口径
func parseFxResponse(body []byte, targets []string) (map[string]float64, error) {
	var raw struct {
		Result string             `json:"result"`
		Rates  map[string]float64 `json:"rates"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("quote: 解析 fx 响应失败: %w", err)
	}
	if !strings.EqualFold(raw.Result, "success") || len(raw.Rates) == 0 {
		return nil, fmt.Errorf("quote: fx 响应无有效汇率")
	}

	out := make(map[string]float64)
	for _, ccy := range targets {
		q := raw.Rates[strings.ToUpper(ccy)] // 1 CNY = q X
		if q <= 0 {
			continue
		}
		rate := 1.0 / q // 1 X = rate CNY
		if rate > 0 {
			out[strings.ToUpper(ccy)] = rate
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("quote: 目标币种均无汇率数据")
	}
	return out, nil
}

// FxSnapshot 汇率快照（带时间戳）
type FxSnapshot struct {
	Rates      map[string]float64 `json:"rates"`
	FetchedAt  time.Time          `json:"fetched_at"`
	SourceName string             `json:"source"`
}

// FetchFxSnapshot 拉取并附带元信息的汇率快照
func FetchFxSnapshot(ctx context.Context, client *pkgNet.HTTPClient, targets []string) (*FxSnapshot, error) {
	rates, err := FetchRatesFromCNY(ctx, client, targets)
	if err != nil {
		return nil, err
	}
	return &FxSnapshot{Rates: rates, FetchedAt: time.Now(), SourceName: "er-api"}, nil
}
