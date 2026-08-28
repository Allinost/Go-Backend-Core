// Package quote 东方财富行情数据源：A股/ETF/港股/美股实时行情
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
	eastmoneyQuoteURL = "https://push2.eastmoney.com/api/qt/stock/get?secid=%s&fields=f43,f57,f58,f60,f86&fltt=2&invt=2"

	// 详情端点：拉取完整行情字段（供标的解析等场景使用）
	eastmoneyDetailURL = "https://push2.eastmoney.com/api/qt/stock/get?secid=%s&fields=f43,f44,f45,f46,f47,f48,f50,f51,f52,f55,f57,f58,f60,f116,f117,f162,f167,f170,f171,f173&fltt=2&invt=2"
)

// EastmoneyProvider 东方财富行情提供方
type EastmoneyProvider struct {
	client *pkgNet.HTTPClient
}

// NewEastmoneyProvider 创建东方财富行情提供方
func NewEastmoneyProvider(client *pkgNet.HTTPClient) *EastmoneyProvider {
	return &EastmoneyProvider{client: client}
}

// Name 数据源名称
func (p *EastmoneyProvider) Name() string { return "eastmoney" }

// Support 支持股票与 ETF 的沪深港美市场
func (p *EastmoneyProvider) Support(asset AssetInfo) bool {
	if asset.AssetType != "stock" && asset.AssetType != "etf" {
		return false
	}
	switch asset.Market {
	case "CN-SH", "CN-SZ", "HK", "US":
		return true
	}
	return false
}

// emSecid 将市场映射为东财 secid 前缀；美股交易所未知时依次尝试 105(NASDAQ)/106(NYSE)/107(AMEX)
func emSecid(market, code string) []string {
	switch market {
	case "CN-SH":
		return []string{"1." + code}
	case "CN-SZ":
		return []string{"0." + code}
	case "HK":
		return []string{"116." + code}
	case "US":
		return []string{"105." + code, "106." + code, "107." + code}
	}
	return nil
}

// SecidCandidates 导出的 secid 候选映射（供包外 K 线等场景使用）
func SecidCandidates(market, code string) []string { return emSecid(market, code) }

// Fetch 拉取实时行情（美股交易所未知时逐个尝试）
func (p *EastmoneyProvider) Fetch(ctx context.Context, asset AssetInfo) (*Quote, error) {
	var lastErr error
	for _, secid := range emSecid(asset.Market, asset.Code) {
		q, err := p.fetchOne(ctx, secid)
		if err == nil && q.Price > 0 {
			q.Code = asset.Code
			return q, nil
		}
		if err != nil {
			lastErr = err
		}
		time.Sleep(100 * time.Millisecond)
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("quote: eastmoney 未查询到 %s.%s", asset.Market, asset.Code)
}

// fetchOne 拉取单个 secid 行情
func (p *EastmoneyProvider) fetchOne(ctx context.Context, secid string) (*Quote, error) {
	resp, err := p.client.Get(ctx, fmt.Sprintf(eastmoneyQuoteURL, secid))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("quote: eastmoney HTTP %d", resp.StatusCode)
	}
	return parseEastmoney(resp.Body)
}

// StockDetail 单股完整行情（供标的解析等场景）
type StockDetail struct {
	Code         string  `json:"code"`
	Name         string  `json:"name"`
	Price        float64 `json:"price"`         // 最新价
	PrevClose    float64 `json:"prev_close"`    // 昨收
	Open         float64 `json:"open"`          // 开盘价
	High         float64 `json:"high"`          // 最高价
	Low          float64 `json:"low"`           // 最低价
	Volume       float64 `json:"volume"`        // 成交量（手）
	Turnover     float64 `json:"turnover"`      // 成交额
	ChangePct    float64 `json:"change_pct"`    // 涨跌幅 %
	ChangeAmt    float64 `json:"change_amt"`    // 涨跌额
	TurnoverRate float64 `json:"turnover_rate"` // 换手率 %
	PEDynamic    float64 `json:"pe_dynamic"`    // 市盈率(动)
	PETTM        float64 `json:"pe_ttm"`        // 市盈率(TTM)
	TotalMV      float64 `json:"total_mv"`      // 总市值
	FloatMV      float64 `json:"float_mv"`      // 流通市值
	HighLimit    float64 `json:"high_limit"`    // 涨停价
	LowLimit     float64 `json:"low_limit"`     // 跌停价
	QuantityRFQ  float64 `json:"quantity_rfq"`  // 量比
	VolumeRatio  float64 `json:"volume_ratio"`  // 量比
}

// FetchStockDetail 拉取单只股票完整行情（push2 单股端点，通常不被反爬封锁）
func FetchStockDetail(ctx context.Context, client *pkgNet.HTTPClient, secid string) (*StockDetail, error) {
	resp, err := client.Get(ctx, fmt.Sprintf(eastmoneyDetailURL, secid))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("quote: detail HTTP %d", resp.StatusCode)
	}
	var raw struct {
		Data *struct {
			F43  flexFloat `json:"f43"`
			F44  flexFloat `json:"f44"`
			F45  flexFloat `json:"f45"`
			F46  flexFloat `json:"f46"`
			F47  flexFloat `json:"f47"`
			F48  flexFloat `json:"f48"`
			F50  flexFloat `json:"f50"`
			F51  flexFloat `json:"f51"`
			F52  flexFloat `json:"f52"`
			F55  flexFloat `json:"f55"`
			F57  string    `json:"f57"`
			F58  string    `json:"f58"`
			F60  flexFloat `json:"f60"`
			F116 flexFloat `json:"f116"`
			F117 flexFloat `json:"f117"`
			F162 flexFloat `json:"f162"`
			F167 flexFloat `json:"f167"`
			F170 flexFloat `json:"f170"`
			F171 flexFloat `json:"f171"`
			F173 flexFloat `json:"f173"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body, &raw); err != nil {
		return nil, fmt.Errorf("quote: 解析 detail 失败: %w", err)
	}
	if raw.Data == nil {
		return nil, fmt.Errorf("quote: detail 无数据")
	}
	d := raw.Data
	return &StockDetail{
		Code:         d.F57,
		Name:         d.F58,
		Price:        d.F43.Float(),
		PrevClose:    d.F60.Float(),
		Open:         d.F46.Float(),
		High:         d.F44.Float(),
		Low:          d.F45.Float(),
		Volume:       d.F47.Float(),
		Turnover:     d.F48.Float(),
		ChangePct:    d.F170.Float(),
		ChangeAmt:    d.F171.Float(),
		TurnoverRate: d.F173.Float(),
		PEDynamic:    d.F162.Float(),
		PETTM:        d.F167.Float(),
		TotalMV:      d.F116.Float(),
		FloatMV:      d.F117.Float(),
		HighLimit:    d.F51.Float(),
		LowLimit:     d.F52.Float(),
		QuantityRFQ:  d.F50.Float(),
		VolumeRatio:  d.F55.Float(),
	}, nil
}

// flexFloat 兼容数字、字符串与 "-" 占位的宽松数值解析
type flexFloat float64

// Float 取值
func (f flexFloat) Float() float64 { return float64(f) }

// UnmarshalJSON 宽松解析
func (f *flexFloat) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "null" || s == `"-"` || s == `""` {
		*f = 0
		return nil
	}
	if s[0] == '"' {
		var str string
		if err := json.Unmarshal(b, &str); err != nil {
			return err
		}
		str = strings.TrimSpace(strings.ReplaceAll(str, ",", ""))
		if str == "-" || str == "" {
			*f = 0
			return nil
		}
		v, err := strconv.ParseFloat(str, 64)
		if err != nil {
			return err
		}
		*f = flexFloat(v)
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return err
	}
	*f = flexFloat(v)
	return nil
}

// emResponse push2 stock/get 响应体
type emResponse struct {
	Data *emData `json:"data"`
}

// emData 行情字段（fltt=2 时价格为浮点数，停牌等场景可能为 null/"-"）
type emData struct {
	Code      string    `json:"f57"`
	Name      string    `json:"f58"`
	Latest    flexFloat `json:"f43"`
	PrevClose flexFloat `json:"f60"`
	Timestamp int64     `json:"f86"`
}

// parseEastmoney 解析东财行情响应
func parseEastmoney(body []byte) (*Quote, error) {
	var r emResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("quote: 解析 eastmoney 响应失败: %w", err)
	}
	if r.Data == nil || r.Data.Latest <= 0 {
		return nil, fmt.Errorf("quote: eastmoney 无有效行情数据")
	}
	q := &Quote{
		Name:      r.Data.Name,
		Price:     float64(r.Data.Latest),
		PrevClose: float64(r.Data.PrevClose),
		Source:    "official",
	}
	if r.Data.Timestamp > 0 {
		q.NavDate = time.Unix(r.Data.Timestamp, 0).Format(dateLayoutQuote)
	}
	return q, nil
}

// dateLayoutQuote 包内通用日期格式
const dateLayoutQuote = "2006-01-02"
