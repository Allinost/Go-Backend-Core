// Package ia 筛选与多币种层 — 动态潜力个股、绩优个股筛选、指数分析与汇率自动刷新
package ia

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Allinost/go-backend-core/internal/pkg/logger"
	pkgNet "github.com/Allinost/go-backend-core/pkg/net"
	"github.com/Allinost/go-backend-core/zzz/ia/quote"
)

// ============================================================================
// 通用数学辅助（筛选评分用）
// ============================================================================

// screenSnapshotCache 全市场快照缓存（TTL 10 分钟），避免频繁拉取东财接口，
// 同时在断路器熔断时可返回最近一次有效数据。
type screenSnapshotCache struct {
	mu      sync.RWMutex
	data    []quote.ScreenRow
	at      time.Time
	ttl     time.Duration
	warning string // 熔断时附加的提示信息
}

var _snapshotCache = &screenSnapshotCache{ttl: 10 * time.Minute}

// get 获取缓存数据。fresh=true 表示新鲜数据（无熔断）；fresh=false 表示使用了 stale 数据。
func (c *screenSnapshotCache) get() (data []quote.ScreenRow, fresh bool, warning string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.data == nil || time.Since(c.at) > c.ttl {
		return nil, false, ""
	}
	return c.data, c.warning == "", c.warning
}

// set 写入缓存（成功时 warning 为空）
func (c *screenSnapshotCache) set(data []quote.ScreenRow, warning string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = data
	c.at = time.Now()
	c.warning = warning
}

// fetchScreenSnapshot 带缓存的快照拉取：
//   - 成功时更新缓存
//   - 失败但有缓存时返回 stale 数据（不报错，调用方静默降级）
//   - 失败且无缓存时返回错误
func fetchScreenSnapshot(ctx context.Context, client *pkgNet.HTTPClient, pages int) ([]quote.ScreenRow, error) {
	data, err := quote.FetchStockSnapshot(ctx, client, pages)
	if err == nil {
		_snapshotCache.set(data, "")
		return data, nil
	}
	// 拉取失败，尝试返回缓存的 stale 数据
	if stale, _, _ := _snapshotCache.get(); stale != nil {
		logger.Warn().Err(err).Int("stale_count", len(stale)).Msg("快照拉取失败，使用缓存数据")
		return stale, nil
	}
	return nil, err
}

// clamp 归一化到 [0,1]
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// normalizeScores 将一组值线性归一到 [0,1]（min-max）
func normalizeScores(values []float64) []float64 {
	mn, mx := math.Inf(1), math.Inf(-1)
	for _, v := range values {
		if v < mn {
			mn = v
		}
		if v > mx {
			mx = v
		}
	}
	out := make([]float64, len(values))
	if !math.IsInf(mn, 1) && mx > mn {
		for i, v := range values {
			out[i] = (v - mn) / (mx - mn)
		}
	}
	return out
}

// percentileInRange 当前值在 [low,high] 区间内的分位
func percentileInRange(low, high, v float64) float64 {
	if high <= low {
		return 0.5
	}
	return clamp01((v - low) / (high - low))
}

// movingAverage 最近 n 个收盘价均值（不足 n 取全量）
func movingAverage(closes []float64, n int) float64 {
	if len(closes) == 0 || n <= 0 {
		return 0
	}
	start := len(closes) - n
	if start < 0 {
		start = 0
	}
	sum := 0.0
	cnt := 0
	for i := start; i < len(closes); i++ {
		sum += closes[i]
		cnt++
	}
	if cnt == 0 {
		return 0
	}
	return sum / float64(cnt)
}

// klineCloses 从 K 线序列提取收盘价切片
func klineCloses(points []quote.KLinePoint) []float64 {
	out := make([]float64, 0, len(points))
	for _, p := range points {
		out = append(out, p.Close)
	}
	return out
}

// returnPct 区间收益率百分比：末值相对 n 天前
func returnPct(closes []float64, n int) float64 {
	if len(closes) < n+1 || n <= 0 {
		return 0
	}
	base := closes[len(closes)-1-n]
	if base <= 0 {
		return 0
	}
	return (closes[len(closes)-1]/base - 1) * 100
}

// ============================================================================
// 动态潜力个股筛选
// ============================================================================

// DynamicPotential 两阶段动态潜力筛选：
//  1. 全市场快照按成交额取流动性池，剔除 ST/低价股，按主力净流入率预选候选
//  2. 对候选逐只拉取 120 日 K 线，计算动量(R5/R20/R60)、均线位置、波动率，
//     加权合成为潜力分（动量 55% / 趋势 25% / 资金 15% / 低波 5%）
func (s *Service) DynamicPotential(ctx context.Context, req *DynamicPotentialReq) (*DynamicPotentialResp, error) {
	if s.quotes == nil {
		return nil, NewIAError(ErrQuoteUnavailable)
	}
	top, pool, pages := req.Top, req.PoolSize, req.Pages
	if top <= 0 || top > 50 {
		top = 20
	}
	if pool <= 0 || pool > 100 {
		pool = 50
	}
	if pages <= 0 || pages > 5 {
		pages = 2
	}

	universe, err := fetchScreenSnapshot(ctx, s.quotes.ScreenHTTP(), pages)
	if err != nil {
		if errors.Is(err, pkgNet.ErrCircuitOpen) {
			return nil, NewIAError(ErrCircuitOpen)
		}
		return nil, fmt.Errorf("quote: 快照拉取失败: %w", err)
	}
	resp := &DynamicPotentialResp{
		GeneratedAt: time.Now().Format(time.RFC3339),
		Universe:    len(universe),
		Items:       []PotentialStock{},
	}

	// 预选：主力净流入率降序
	sort.Slice(universe, func(i, j int) bool { return universe[i].MainInflowRatio > universe[j].MainInflowRatio })
	candidates := universe
	if len(candidates) > pool {
		candidates = candidates[:pool]
	}

	type cand struct {
		row   quote.ScreenRow
		item  PotentialStock
		score float64
	}
	results := make([]cand, 0, len(candidates))

	for i := range candidates {
		row := candidates[i]
		points, err := s.quotes.Kline(ctx, secidOfRow(row), begForDays(140))
		if err != nil || len(points) < 21 {
			continue // 历史不足或获取失败跳过
		}
		closes := klineCloses(points)
		it := PotentialStock{
			Code: row.Code, Name: row.Name, Price: row.Price,
			ChangePct: row.ChangePct, TurnoverRate: row.TurnoverRate,
			MainInflowRatio: round6(row.MainInflowRatio),
			R5:              round2(returnPct(closes, 5)),
			R20:             round2(returnPct(closes, 20)),
			R60:             round2(returnPct(closes, minInt(60, len(closes)-1))),
			AboveMA20:       closes[len(closes)-1] >= movingAverage(closes, 20),
			AboveMA60:       closes[len(closes)-1] >= movingAverage(closes, minInt(60, len(closes))),
			VolAnnual:       round4(AnnualVolatility(closes[len(closes)-61:])),
		}

		// 分项打分
		momentum := 0.5*clamp01(it.R20/15+0.5) + 0.3*clamp01(it.R60/30+0.5) + 0.2*clamp01(it.R5/8+0.5)
		trendScore := 0.0
		if it.AboveMA20 {
			trendScore += 0.6
		}
		if it.AboveMA60 {
			trendScore += 0.4
		}
		fundScore := clamp01(it.MainInflowRatio * 100) // 净流入率 ≥1% 得满分
		volPenalty := clamp01(1 - it.VolAnnual/0.8)    // 年化波动 80% 以上零分
		it.ScoreParts = map[string]float64{
			"momentum": round4(momentum), "trend": round4(trendScore),
			"funds": round4(fundScore), "low_vol": round4(volPenalty),
		}
		score := 55*momentum + 25*trendScore + 15*fundScore + 5*volPenalty
		results = append(results, cand{row: row, item: it, score: score})
	}

	sort.Slice(results, func(i, j int) bool { return results[i].score > results[j].score })
	if len(results) > top {
		results = results[:top]
	}
	for i := range results {
		results[i].item.Score = round2(results[i].score)
		resp.Items = append(resp.Items, results[i].item)
	}
	resp.Analyzed = len(results)
	return resp, nil
}

// ============================================================================
// 绩优个股筛选
// ============================================================================

// QualityStockScreen 基本面质量筛选：
// 硬过滤（ROE≥下限、0<PE-TTM≤上限、PB≤上限、市值≥下限、净利润正增长）后，
// 按 ROE 40% / 利润增速 25% / 低估值(PE+PB) 25% / 规模 10% 加权排名。
func (s *Service) QualityStockScreen(ctx context.Context, req *QualityStockReq) (*QualityStockResp, error) {
	if s.quotes == nil {
		return nil, NewIAError(ErrQuoteUnavailable)
	}
	top, pages := req.Top, req.Pages
	if top <= 0 || top > 100 {
		top = 30
	}
	if pages <= 0 || pages > 5 {
		pages = 3
	}
	minROE, maxPE, maxPB, minMV := req.MinROE, req.MaxPE, req.MaxPB, req.MinMVYi
	if minROE <= 0 {
		minROE = 10
	}
	if maxPE <= 0 {
		maxPE = 35
	}
	if maxPB <= 0 {
		maxPB = 8
	}
	if minMV <= 0 {
		minMV = 50
	}

	universe, err := fetchScreenSnapshot(ctx, s.quotes.ScreenHTTP(), pages)
	if err != nil {
		if errors.Is(err, pkgNet.ErrCircuitOpen) {
			return nil, NewIAError(ErrCircuitOpen)
		}
		return nil, fmt.Errorf("quote: 快照拉取失败: %w", err)
	}
	resp := &QualityStockResp{GeneratedAt: time.Now().Format(time.RFC3339), Universe: len(universe), Items: []QualityStock{}}

	var passed []QualityStock
	for _, r := range universe {
		if r.PETTM <= 0 || r.PETTM > maxPE || r.PB <= 0 || r.PB > maxPB ||
			r.ROE < minROE || r.TotalMVYi < minMV || r.ProfitGrowth <= 0 {
			continue
		}
		passed = append(passed, QualityStock{
			Code: r.Code, Name: r.Name, Price: r.Price, ChangePct: r.ChangePct,
			PETTM: round2(r.PETTM), PEDynamic: round2(r.PEDynamic), PB: round2(r.PB),
			ROE: round2(r.ROE), ProfitGrowth: round2(r.ProfitGrowth), TotalMVYi: round2(r.TotalMVYi),
		})
	}
	resp.Filtered = len(passed)

	// 排名归一化加权（低 PE/PB 为优 → 反向归一）
	n := len(passed)
	if n == 0 {
		return resp, nil
	}
	roes := make([]float64, n)
	growths := make([]float64, n)
	pes := make([]float64, n)
	pbs := make([]float64, n)
	mvs := make([]float64, n)
	for i, p := range passed {
		roes[i], growths[i], pes[i], pbs[i], mvs[i] = p.ROE, p.ProfitGrowth, p.PETTM, p.PB, p.TotalMVYi
	}
	roeN := normalizeScores(roes)
	growthN := normalizeScores(growths)
	mvN := normalizeScores(mvs)
	peN := reverseNorm(normalizeScores(pes))
	pbN := reverseNorm(normalizeScores(pbs))

	for i := range passed {
		sParts := map[string]float64{"roe": roeN[i], "growth": growthN[i], "cheap_pe": peN[i], "cheap_pb": pbN[i], "size": mvN[i]}
		score := 40*roeN[i] + 25*growthN[i] + 15*peN[i] + 10*pbN[i] + 10*mvN[i]
		passed[i].ScoreParts = sParts
		passed[i].Score = round2(score)
	}
	sort.Slice(passed, func(i, j int) bool { return passed[i].Score > passed[j].Score })
	if len(passed) > top {
		passed = passed[:top]
	}
	resp.Items = passed
	return resp, nil
}

// reverseNorm 反向归一化（值越小得分越高）
func reverseNorm(norm []float64) []float64 {
	out := make([]float64, len(norm))
	for i, v := range norm {
		out[i] = 1 - v
	}
	return out
}

// ============================================================================
// 指数分析
// ============================================================================

var defaultIndexes = []string{"CSI300", "CSI500", "SSE"}

// IndexAnalysis 指数技术面与区间风险收益分析（支持多指数并列）
func (s *Service) IndexAnalysis(ctx context.Context, req *IndexAnalysisReq) (*IndexAnalysisResp, error) {
	if s.quotes == nil {
		return nil, NewIAError(ErrQuoteUnavailable)
	}
	names := req.Indexes
	if names == "" {
		names = strings.Join(defaultIndexes, ",")
	}
	years := req.Years
	if years <= 0 || years > 10 {
		years = 3
	}
	resp := &IndexAnalysisResp{Years: years, GeneratedAt: time.Now().Format(time.RFC3339), Items: []IndexMetrics{}}

	for _, name := range strings.Split(names, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		secid, ok := quote.ResolveBenchmarkSecid(name)
		if !ok {
			resp.Items = append(resp.Items, IndexMetrics{Secid: name, Error: "无法识别的指数标识"})
			continue
		}
		beg := addDays(todayStr(), -years*365)
		points, err := s.quotes.Kline(ctx, secid, beg)
		if err != nil {
			logger.Warn().Err(err).Str("index", name).Msg("zzz-ia: 指数K线获取失败")
			resp.Items = append(resp.Items, IndexMetrics{Secid: secid, Error: truncateStr(err.Error(), 120)})
			continue
		}
		resp.Items = append(resp.Items, analyzeIndex(secid, points))
	}
	return resp, nil
}

// analyzeIndex 单指数指标计算（纯逻辑便于测试）
func analyzeIndex(secid string, points []quote.KLinePoint) IndexMetrics {
	closes := klineCloses(points)
	last := closes[len(closes)-1]

	ma := func(n int) float64 { return round4(movingAverage(closes, n)) }
	ma20, ma60, ma120 := ma(20), ma(60), ma(120)
	ma250 := round4(movingAverage(closes, minInt(250, len(closes))))

	hi, lo := closes[0], closes[0]
	for _, c := range closes {
		if c > hi {
			hi = c
		}
		if c < lo {
			lo = c
		}
	}

	// 年初至今
	ytdBase := closes[0]
	ytdDate := fmt.Sprintf("%d-01-01", time.Now().Year())
	for i, p := range points {
		if p.Date >= ytdDate {
			ytdBase = closes[i]
			break
		}
	}

	above := func(v float64) bool { return last >= v }
	trend := "震荡整理"
	a20, a60 := above(ma20), above(ma60)
	switch {
	case a20 && a60 && above(ma120):
		trend = "多头排列"
	case a20 && a60:
		trend = "中短期强势"
	case !a20 && !a60:
		trend = "空头排列"
	case a20:
		trend = "反弹修复"
	default:
		trend = "弱势整理"
	}

	return IndexMetrics{
		Secid: secid, Name: indexDisplayName(secid),
		Latest:    last,
		ChangeYTD: round2(safeDiv(last-ytdBase, ytdBase) * 100),
		MA20:      ma20, MA60: ma60, MA120: ma120, MA250: ma250,
		Trend: trend, AboveMA20: a20, AboveMA60: a60, AboveMA120: above(ma120), AboveMA250: above(ma250),
		WindowHigh: hi, WindowLow: lo,
		Percentile:  round4(percentileInRange(lo, hi, last)),
		AnnReturn:   annualizedReturn(points),
		AnnVol:      round4(AnnualVolatility(closes)),
		MaxDrawdown: round6(MaxDrawdown(closes).MaxDrawdown),
		Samples:     len(points),
	}
}

// annualizedReturn 窗口年化复合收益
func annualizedReturn(points []quote.KLinePoint) float64 {
	if len(points) < 2 {
		return 0
	}
	first, last := points[0].Close, points[len(points)-1].Close
	d0, e1 := time.ParseInLocation(dateLayout, points[0].Date, time.Local)
	d1, e2 := time.ParseInLocation(dateLayout, points[len(points)-1].Date, time.Local)
	if e1 != nil || e2 != nil || first <= 0 {
		return 0
	}
	years := d1.Sub(d0).Hours() / 24 / daysPerYear
	if years <= 0.25 {
		return 0
	}
	return round4(math.Pow(last/first, 1/years) - 1)
}

// indexDisplayName 指数显示名
func indexDisplayName(secid string) string {
	m := map[string]string{
		"1.000300": "沪深300", "1.000905": "中证500", "1.000852": "中证1000",
		"1.000001": "上证指数", "0.399001": "深证成指", "0.399006": "创业板指",
	}
	if v, ok := m[secid]; ok {
		return v
	}
	return secid
}

// ============================================================================
// 多币种：汇率自动刷新
// ============================================================================

// RefreshFxRates 自动刷新 CNY→USD/HKD 最新汇率写入 fx_rates（source=auto）
func (s *Service) RefreshFxRates(ctx context.Context) (*FxRefreshResult, error) {
	if s.quotes == nil {
		return nil, NewIAError(ErrQuoteUnavailable)
	}
	snap, err := quote.FetchFxSnapshot(ctx, s.quotes.HTTP(), quote.SupportedQuoteCcys)
	if err != nil {
		return nil, err
	}
	res := &FxRefreshResult{Updated: []string{}, Rates: map[string]float64{}, Errors: []string{}}
	date := todayStr()
	for ccy, rate := range snap.Rates {
		rate = round6(rate)
		if uerr := s.repo.UpsertFxRate(ctx, &FxRate{
			BaseCcy: "CNY", QuoteCcy: ccy, Rate: rate, RateDate: date, Source: snap.SourceName,
		}); uerr != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", ccy, uerr))
			continue
		}
		res.Updated = append(res.Updated, "CNY->"+ccy)
		res.Rates["CNY->"+ccy] = rate
	}
	s.notifyRoom("fx", map[string]any{"event": "fx_refreshed", "rates": res.Rates})
	return res, nil
}

// ============================================================================
// 内部工具
// ============================================================================

// secidOfRow 快照行转东财 secid：6/9 开头为沪市(1.)，其余为深市(0.)
func secidOfRow(r quote.ScreenRow) string {
	if strings.HasPrefix(r.Code, "6") || strings.HasPrefix(r.Code, "9") {
		return "1." + r.Code
	}
	return "0." + r.Code
}

// begForDays N 天前的 YYYYMMDD 起始日期
func begForDays(days int) string {
	return time.Now().AddDate(0, 0, -days).Format("20060102")
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// truncateStr 截断字符串
func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
