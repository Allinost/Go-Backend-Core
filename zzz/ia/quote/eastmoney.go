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
