// Package quote 东方财富 A股快照列表数据源：筛选与评分场景使用
package quote

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	pkgNet "github.com/Allinost/go-backend-core/pkg/net"
)

const (
	// clistURL A股全市场快照列表（按成交额降序）
	// fs 覆盖：沪主板A(m:1 t:2)、科创板(m:1 t:23)、深主板A(m:0 t:6)、创业板(m:0 t:80)
	clistURL = "https://push2.eastmoney.com/api/qt/clist/get?pn=%d&pz=%d&po=1&np=1&fltt=2&invt=2&fid=f6" +
		"&fs=m:0+t:6,m:0+t:80,m:1+t:2,m:1+t:23" +
		"&fields=f12,f14,f2,f3,f8,f9,f20,f21,f23,f37,f45,f46,f62,f115"

	screenPageSize = 100

	// fallbackQuoteURL 单股行情 fallback（push2 单股端点通常不被封锁）
	fallbackQuoteURL = "https://push2.eastmoney.com/api/qt/stock/get?secid=%s&fields=f43,f57,f58,f60,f116,f117,f162,f167,f170,f173&fltt=2&invt=2"
)

// clist 字段语义（东财公开接口约定，缺失字段以 flexFloat 归零容错）：
//
//	f12 代码  f14 名称  f2 最新价  f3 涨跌幅%  f8 换手率%
//	f9 市盈率(动)  f115 市盈率(TTM)  f23 市净率
//	f20 总市值  f21 流通市值  f37 ROE(加权)%
//	f45 净利润(元)  f46 净利润同比%  f62 主力净流入(元)
type ScreenRow struct {
	Code            string  `json:"code"`
	Name            string  `json:"name"`
	Price           float64 `json:"price"`
	ChangePct       float64 `json:"change_pct"`        // 当日涨跌幅 %
	TurnoverRate    float64 `json:"turnover_rate"`     // 换手率 %
	PEDynamic       float64 `json:"pe_dynamic"`        // 市盈率(动)
	PETTM           float64 `json:"pe_ttm"`            // 市盈率(TTM)
	PB              float64 `json:"pb"`                // 市净率
	ROE             float64 `json:"roe"`               // ROE %
	ProfitGrowth    float64 `json:"profit_growth"`     // 净利润同比 %
	TotalMVYi       float64 `json:"total_mv_yi"`       // 总市值（亿元）
	FloatMVYi       float64 `json:"float_mv_yi"`       // 流通市值（亿元）
	MainInflowYi    float64 `json:"main_inflow_yi"`    // 主力净流入（亿元）
	MainInflowRatio float64 `json:"main_inflow_ratio"` // 主力净流入/流通市值
}

// FetchStockSnapshot 分页拉取 A股快照（pages 页 × 100 只，成交额降序，保证流动性池）
// clist 失败时自动 fallback 到单股行情端点（字段较少但仍可用于筛选）
func FetchStockSnapshot(ctx context.Context, client *pkgNet.HTTPClient, pages int) ([]ScreenRow, error) {
	if pages <= 0 {
		pages = 1
	}
	var out []ScreenRow
	for pn := 1; pn <= pages; pn++ {
		rows, err := fetchSnapshotPage(ctx, client, pn)
		if err != nil {
			// 首页失败时尝试 fallback（单股端点兜底）
			if pn == 1 {
				fallbackRows, fErr := fetchSnapshotFallback(ctx, client, pages)
				if fErr == nil && len(fallbackRows) > 0 {
					return fallbackRows, nil
				}
			}
			return nil, err
		}
		out = append(out, rows...)
		if len(rows) < screenPageSize {
			break
		}
		time.Sleep(120 * time.Millisecond) // 接口礼貌间隔
	}
	return out, nil
}

// fetchSnapshotFallback clist 被封时的兜底方案：
// 用 push2 单股端点逐只拉取 A 股主要指数成分（沪深300 + 中证500 成分股）。
// 字段有限（无 ROE/利润增速/主力流入），但仍可做基础筛选评分。
func fetchSnapshotFallback(ctx context.Context, client *pkgNet.HTTPClient, pages int) ([]ScreenRow, error) {
	// 常见 A 股大盘+热门标的代码（覆盖主要行业龙头，作为候选池兜底）
	// 实际使用时可从配置或缓存加载
	fallbackCodes := []string{
		// 沪市主板
		"600519", "601318", "600036", "600900", "601166", "600276", "600030",
		"601888", "600809", "603259", "601012", "600309", "601899", "600585",
		"600887", "601398", "600048", "601668", "600016", "601601", "600000",
		"601857", "600104", "601688", "600031", "601211", "601088", "600196",
		"601919", "600188", "601229", "601633", "600690", "601669", "601985",
		"600436", "601138", "600406", "601658", "600763", "603288", "600438",
		"601111", "600050", "601816", "600132", "601766", "603501", "600570",
		"601628", "600588", "601236", "600426", "603993", "601186", "600941",
		// 科创板
		"688981", "688041", "688111", "688599", "688012", "688036", "688005",
		"688185", "688561", "688008", "688256", "688396", "688065", "688303",
		// 深市主板
		"000858", "000333", "002714", "000651", "002475", "000568", "002304",
		"000725", "002415", "000002", "000001", "002142", "000876", "002594",
		"000538", "000661", "002352", "002607", "000423", "002241", "000063",
		"000100", "002460", "000895", "002311", "002027", "000963", "002049",
		// 创业板
		"300750", "300059", "300015", "300760", "300274", "300413", "300124",
		"300033", "300529", "300498", "300142", "300347", "300999", "300782",
		"300433", "300308", "300223", "300896", "300454", "300122", "300316",
		"300747", "300285", "300618", "300628", "300769", "300014", "300207",
	}
	// 取前 N 只（pages 控制数量）
	limit := pages * 50
	if limit > len(fallbackCodes) {
		limit = len(fallbackCodes)
	}
	codes := fallbackCodes[:limit]

	var out []ScreenRow
	for _, code := range codes {
		select {
		case <-ctx.Done():
			return out, ctx.Err()
		default:
		}
		secid := secidByCode(code)
		row, err := fetchSingleFallback(ctx, client, secid, code)
		if err != nil || row == nil {
			continue
		}
		out = append(out, *row)
		time.Sleep(80 * time.Millisecond) // 避免触发限流
	}
	return out, nil
}

// fetchSingleFallback 用单股端点拉取单只股票快照
func fetchSingleFallback(ctx context.Context, client *pkgNet.HTTPClient, secid, code string) (*ScreenRow, error) {
	url := fmt.Sprintf(fallbackQuoteURL, secid)
	resp, err := client.Get(ctx, url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("quote: fallback HTTP %d", resp.StatusCode)
	}
	var raw struct {
		Data *struct {
			F43  flexFloat `json:"f43"` // 最新价
			F57  string    `json:"f57"` // 代码
			F58  string    `json:"f58"` // 名称
			F60  flexFloat `json:"f60"` // 昨收
			F116 flexFloat `json:"f116"` // 总市值
			F117 flexFloat `json:"f117"` // 流通市值
			F162 flexFloat `json:"f162"` // PE 动态
			F167 flexFloat `json:"f167"` // PE TTM
			F170 flexFloat `json:"f170"` // 涨跌幅 %
			F173 flexFloat `json:"f173"` // 换手率 %
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body, &raw); err != nil {
		return nil, err
	}
	if raw.Data == nil {
		return nil, fmt.Errorf("quote: fallback 无数据 %s", code)
	}
	d := raw.Data
	price := d.F43.Float()
	if price <= 0 {
		return nil, fmt.Errorf("quote: fallback 价格为0 %s", code)
	}
	totalMV := d.F116.Float()
	floatMV := d.F117.Float()
	return &ScreenRow{
		Code:         d.F57,
		Name:         d.F58,
		Price:        price,
		ChangePct:    d.F170.Float(),
		TurnoverRate: d.F173.Float(),
		PEDynamic:    d.F162.Float(),
		PETTM:        d.F167.Float(),
		TotalMVYi:    totalMV / 1e8,
		FloatMVYi:    floatMV / 1e8,
	}, nil
}

// secidByCode 根据纯数字代码推断东财 secid
func secidByCode(code string) string {
	if len(code) == 6 {
		switch code[0] {
		case '6', '9':
			return "1." + code // 沪市
		default:
			return "0." + code // 深市
		}
	}
	return "0." + code
}

func fetchSnapshotPage(ctx context.Context, client *pkgNet.HTTPClient, page int) ([]ScreenRow, error) {
	url := fmt.Sprintf(clistURL, page, screenPageSize)
	resp, err := client.Get(ctx, url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("quote: snapshot HTTP %d", resp.StatusCode)
	}
	return ParseSnapshot(resp.Body)
}

// ParseSnapshot 解析 clist 快照响应
func ParseSnapshot(body []byte) ([]ScreenRow, error) {
	var raw struct {
		Data *struct {
			Total int `json:"total"`
			Diff  []struct {
				Code      string    `json:"f12"`
				Name      string    `json:"f14"`
				Price     flexFloat `json:"f2"`
				ChangePct flexFloat `json:"f3"`
				Turnover  flexFloat `json:"f8"`
				PEDyn     flexFloat `json:"f9"`
				TotalMV   flexFloat `json:"f20"`
				FloatMV   flexFloat `json:"f21"`
				PB        flexFloat `json:"f23"`
				ROE       flexFloat `json:"f37"`
				Profit    flexFloat `json:"f45"`
				Growth    flexFloat `json:"f46"`
				Inflow    flexFloat `json:"f62"`
				PETTM     flexFloat `json:"f115"`
			} `json:"diff"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("quote: 解析 snapshot 失败: %w", err)
	}
	if raw.Data == nil || len(raw.Data.Diff) == 0 {
		return nil, fmt.Errorf("quote: snapshot 无数据")
	}
	out := make([]ScreenRow, 0, len(raw.Data.Diff))
	for _, d := range raw.Data.Diff {
		code := strings.TrimSpace(d.Code)
		if code == "" || strings.Contains(strings.ToUpper(d.Name), "ST") || d.Price.Float() <= 0 {
			continue // 过滤 ST 与无效价格
		}
		floatMV := d.FloatMV.Float()
		inflow := d.Inflow.Float()
		row := ScreenRow{
			Code: code, Name: d.Name,
			Price: d.Price.Float(), ChangePct: d.ChangePct.Float(), TurnoverRate: d.Turnover.Float(),
			PEDynamic: d.PEDyn.Float(), PETTM: d.PETTM.Float(), PB: d.PB.Float(),
			ROE: d.ROE.Float(), ProfitGrowth: d.Growth.Float(),
			TotalMVYi:    d.TotalMV.Float() / 1e8,
			FloatMVYi:    floatMV / 1e8,
			MainInflowYi: inflow / 1e8,
		}
		if floatMV > 0 {
			row.MainInflowRatio = inflow / floatMV
		}
		out = append(out, row)
	}
	return out, nil
}
