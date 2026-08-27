// Package ia 告警与导入层 — 价格止盈止损告警、风险巡检推送、券商对账单 CSV 导入
package ia

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Allinost/go-backend-core/internal/modules/webhook"
)

// ============================================================================
// 价格告警
// ============================================================================

// CreateAlert 创建价格告警规则
func (s *Service) CreateAlert(ctx context.Context, req *CreateAlertReq) (*PriceAlert, error) {
	if !validAlertTypes[req.AlertType] {
		return nil, NewIAError(ErrInvalidAlert, "无效的告警类型（take_profit/stop_loss/target_price）")
	}
	if _, err := s.repo.GetAccount(ctx, req.AccountID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, NewIAError(ErrAccountNotFound)
		}
		return nil, err
	}
	if _, err := s.repo.GetAsset(ctx, req.AssetID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, NewIAError(ErrAssetNotFound)
		}
		return nil, err
	}
	return s.repo.CreateAlert(ctx, &PriceAlert{
		AccountID: req.AccountID, AssetID: req.AssetID,
		AlertType: req.AlertType, TargetPrice: req.TargetPrice, Note: req.Note,
	})
}

// UpdateAlert 更新告警规则
func (s *Service) UpdateAlert(ctx context.Context, id string, req *UpdateAlertReq) (*PriceAlert, error) {
	if _, err := s.repo.GetAlert(ctx, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, NewIAError(ErrAlertNotFound)
		}
		return nil, err
	}
	if req.Status != nil {
		st := AlertStatus(*req.Status)
		if st != AlertActive && st != AlertDisabled && st != AlertTriggered {
			return nil, NewIAError(ErrInvalidAlert)
		}
	}
	if req.TargetPrice != nil && *req.TargetPrice <= 0 {
		return nil, NewIAError(ErrInvalidRatio)
	}
	return s.repo.UpdateAlert(ctx, id, req)
}

// GetAlert 获取告警规则
func (s *Service) GetAlert(ctx context.Context, id string) (*PriceAlert, error) {
	a, err := s.repo.GetAlert(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, NewIAError(ErrAlertNotFound)
	}
	return a, err
}

// DeleteAlert 删除告警规则
func (s *Service) DeleteAlert(ctx context.Context, id string) error {
	return s.repo.DeleteAlert(ctx, id)
}

// ListAlerts 分页查询告警规则
func (s *Service) ListAlerts(ctx context.Context, req *ListAlertsReq) ([]PriceAlert, bool, int, error) {
	return s.repo.ListAlerts(ctx, req)
}

// alertHit 判断告警条件是否满足
func alertHit(alertType string, target, lastPrice float64) bool {
	if lastPrice <= 0 || target <= 0 {
		return false
	}
	switch AlertType(alertType) {
	case AlertTakeProfit:
		return lastPrice >= target
	case AlertStopLoss:
		return lastPrice <= target
	case AlertTargetHit:
		dev := abs(lastPrice-target) / target
		return dev <= 0.01 // 目标价 ±1% 视为到价
	}
	return false
}

// CheckPriceAlerts 检查全部活跃价格告警，命中则标记触发并推送 webhook。
// 幂等：active→triggered 原子转移保证同一规则仅通知一次。
func (s *Service) CheckPriceAlerts(ctx context.Context) (*AlertCheckResult, error) {
	rows, err := s.repo.ListActiveAlerts(ctx)
	if err != nil {
		return nil, err
	}
	res := &AlertCheckResult{Checked: len(rows), Items: []AlertTriggeredView{}}

	for i := range rows {
		row := &rows[i]
		if !alertHit(row.Alert.AlertType, row.Alert.TargetPrice, row.LastPrice) {
			continue
		}
		ok, merr := s.repo.MarkAlertTriggered(ctx, row.Alert.ID, row.LastPrice)
		if merr != nil || !ok {
			continue // 并发竞争或失败：跳过避免重复通知
		}
		res.Triggered++
		res.Items = append(res.Items, AlertTriggeredView{
			Code: row.Code, Name: row.Name, AlertType: row.Alert.AlertType,
			TargetPrice: row.Alert.TargetPrice, LastPrice: row.LastPrice, AccountID: row.Alert.AccountID,
		})
		s.notifyRoom("alert", map[string]any{
			"event": "price_alert_triggered", "code": row.Code, "name": row.Name,
			"alert_type": row.Alert.AlertType, "target": row.Alert.TargetPrice, "last_price": row.LastPrice,
		})
	}

	if res.Triggered > 0 && s.webhook != nil {
		if _, err = s.webhook.Send(ctx, &webhook.SendReq{
			Source: "zzz-ia", Title: "价格告警触发", Content: buildAlertMessage(res.Items), MsgType: "markdown",
		}); err == nil {
			res.Notified = true
		}
	}
	return res, nil
}

// buildAlertMessage 构造告警消息正文
func buildAlertMessage(items []AlertTriggeredView) string {
	var sb strings.Builder
	sb.WriteString("## 价格告警触发\n\n")
	sb.WriteString("| 名称 | 类型 | 目标价 | 最新价 |\n|---|---|---|---|\n")
	typeName := map[string]string{"take_profit": "止盈", "stop_loss": "止损", "target_price": "到价"}
	for _, it := range items {
		n := typeName[it.AlertType]
		if n == "" {
			n = it.AlertType
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %.4f | %.4f |\n", it.Name, n, it.TargetPrice, it.LastPrice))
	}
	return sb.String()
}

// ============================================================================
// 风险巡检（集中度 + 配置偏差）
// ============================================================================

// CheckRiskAlerts 每日风险巡检：
//   - 单标的权重超阈值
//   - 配置目标偏差超容忍度
//
// 存在异常时聚合为一条 webhook 消息推送。
func (s *Service) CheckRiskAlerts(ctx context.Context) (*RiskMetricsResp, error) {
	risk, err := s.RiskMetrics(ctx, &AnalyticsReq{}, 0.3)
	if err != nil {
		return nil, err
	}
	review, aerr := s.AllocationReview(ctx, "")

	var lines []string
	breachCount := 0
	for _, ci := range risk.Concentration {
		if ci.Breach {
			breachCount++
			lines = append(lines, fmt.Sprintf("- **集中度**：%s(%s) 权重 %s 超过阈值 30%%", ci.Name, ci.Code, fmtPct(ci.Weight)))
		}
	}
	if aerr == nil {
		for _, it := range review.Items {
			if it.Breach {
				breachCount++
				lines = append(lines, fmt.Sprintf("- **配置偏差**：%s 实际 %s / 目标 %s（偏差 %s）",
					categoryName(it.Category), fmtPct(it.ActualRatio), fmtPct(it.TargetRatio), fmtPct(it.Deviation)))
			}
		}
	}

	if breachCount > 0 {
		content := fmt.Sprintf("## 投资风险巡检（%d 项异常）\n\n%s\n", breachCount, strings.Join(lines, "\n"))
		if s.webhook != nil {
			_, _ = s.webhook.Send(ctx, &webhook.SendReq{
				Source: "zzz-ia", Title: "投资风险巡检提醒", Content: content, MsgType: "markdown",
			})
		}
		s.notifyRoom("risk", map[string]any{"event": "risk_check", "breaches": breachCount, "lines": lines})
	}
	return risk, nil
}

// categoryName 配置类别中文名
func categoryName(c string) string {
	m := map[string]string{
		string(AllocStock): "股票", string(AllocETF): "ETF", string(AllocFund): "基金",
		string(AllocDeposit): "存款", string(AllocWealth): "理财", string(AllocCash): "现金",
	}
	if v, ok := m[c]; ok {
		return v
	}
	return c
}

// ============================================================================
// 券商对账单 CSV 导入
// ============================================================================

// csvHeaderAlias 表头别名 → 标准字段名
var csvHeaderAlias = map[string]string{
	"trade_date": "date", "日期": "date", "成交日期": "date",
	"trade_type": "type", "类型": "type", "业务类型": "type", "操作": "type",
	"code": "code", "代码": "code", "标的代码": "code",
	"market": "market", "市场": "market",
	"name": "name", "名称": "name", "标的名称": "name",
	"asset_type": "asset_type", "资产类型": "asset_type",
	"shares": "shares", "份额": "shares", "数量": "shares", "成交数量": "shares",
	"price": "price", "价格": "price", "成交价": "price", "成交价格": "price",
	"fee": "fee", "费用": "fee", "手续费": "fee", "佣金": "fee",
	"tax": "tax", "税": "tax", "印花税": "tax", "红利税": "tax",
	"amount": "amount", "金额": "amount", "成交金额": "amount",
	"settle_date": "settle_date", "确认日期": "settle_date",
	"remark": "remark", "备注": "remark",
}

// tradeTypeAlias 交易类型别名 → 标准 TradeType
var tradeTypeAlias = map[string]string{
	"buy": string(TradeBuy), "b": string(TradeBuy), "买入": string(TradeBuy), "申购": string(TradeBuy),
	"sell": string(TradeSell), "s": string(TradeSell), "卖出": string(TradeSell), "赎回": string(TradeSell),
	"dividend_cash": string(TradeDividendCash), "分红": string(TradeDividendCash), "现金分红": string(TradeDividendCash), "派息": string(TradeDividendCash),
	"dividend_reinvest": string(TradeDividendReinv), "红利再投": string(TradeDividendReinv), "再投": string(TradeDividendReinv),
	"interest": string(TradeInterest), "利息": string(TradeInterest), "结息": string(TradeInterest),
	"bonus_shares": string(TradeBonusShares), "送股": string(TradeBonusShares), "转增": string(TradeBonusShares), "送转": string(TradeBonusShares),
	"rights_issue": string(TradeRightsIssue), "配股": string(TradeRightsIssue),
}

var validAlertTypes = map[string]bool{
	string(AlertTakeProfit): true, string(AlertStopLoss): true, string(AlertTargetHit): true,
}

// inferMarket 推断 A股/ETF 市场（无 market 列时兜底）
func inferMarket(code, assetType string) string {
	switch assetType {
	case string(AssetTypeStock):
		if strings.HasPrefix(code, "6") {
			return string(MarketCNSH)
		}
		if strings.HasPrefix(code, "0") || strings.HasPrefix(code, "3") {
			return string(MarketCNSZ)
		}
	case string(AssetTypeETF):
		if strings.HasPrefix(code, "5") || strings.HasPrefix(code, "6") {
			return string(MarketCNSH)
		}
		if strings.HasPrefix(code, "1") {
			return string(MarketCNSZ)
		}
	}
	return ""
}

// parseCSVDate 解析多种常见日期格式
func parseCSVDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{dateLayout, "2006/01/02", "20060102", "2006-1-2"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, &IAError{Code: ErrImportFormat, Message: fmt.Sprintf("无法解析日期: %q", s)}
}

// csvRow 标准化后的导入行
type csvRow struct {
	Date       time.Time
	Type       string
	Code       string
	Market     string
	Name       string
	AssetType  string
	Shares     float64
	Price      float64
	Fee        float64
	Tax        float64
	Amount     float64
	SettleDate string
	Remark     string
	Line       int
}

// ParseStatementCSV 解析对账单 CSV 文本为标准化行（Line 为含表头的文件行号）
func ParseStatementCSV(text string) ([]csvRow, error) {
	reader := csv.NewReader(strings.NewReader(text))
	reader.FieldsPerRecord = -1 // 允许行内列数不同，逐行校验
	reader.TrimLeadingSpace = true
	records, err := reader.ReadAll()
	if err != nil {
		return nil, NewIAError(ErrImportFormat, "%v", err)
	}
	if len(records) < 2 {
		return nil, NewIAError(ErrImportFormat, "缺少表头或数据行为空")
	}

	// 表头映射
	header := records[0]
	colIdx := make(map[string]int)
	for i, h := range header {
		if std, ok := csvHeaderAlias[strings.ToLower(strings.TrimSpace(h))]; ok {
			colIdx[std] = i
		}
	}
	if _, ok := colIdx["date"]; !ok {
		return nil, NewIAError(ErrImportFormat, "表头缺少日期列（trade_date/日期）")
	}
	if _, ok := colIdx["type"]; !ok {
		return nil, NewIAError(ErrImportFormat, "表头缺少类型列（trade_type/类型）")
	}
	if _, ok := colIdx["code"]; !ok {
		return nil, NewIAError(ErrImportFormat, "表头缺少代码列（code/代码）")
	}

	get := func(record []string, field string) string {
		i, ok := colIdx[field]
		if !ok || i >= len(record) {
			return ""
		}
		return strings.TrimSpace(record[i])
	}
	num := func(record []string, field string) float64 {
		v, _ := strconv.ParseFloat(strings.ReplaceAll(get(record, field), ",", ""), 64)
		return v
	}

	rows := make([]csvRow, 0, len(records)-1)
	for lineNo, rec := range records[1:] {
		fileLine := lineNo + 2 // 含表头的实际行号
		if strings.TrimSpace(strings.Join(rec, "")) == "" {
			continue
		}
		date, derr := parseCSVDate(get(rec, "date"))
		if derr != nil {
			return nil, wrapRowErr(fileLine, derr)
		}
		rawType := strings.ToLower(get(rec, "type"))
		tt, ok := tradeTypeAlias[rawType]
		if !ok {
			tt, ok = tradeTypeAlias[get(rec, "type")]
		}
		if !ok {
			return nil, wrapRowErr(fileLine, NewIAError(ErrImportFormat, "未知交易类型: %q", get(rec, "type")))
		}
		code := get(rec, "code")
		if code == "" {
			return nil, wrapRowErr(fileLine, NewIAError(ErrImportFormat, "代码为空"))
		}
		assetType := get(rec, "asset_type")
		if assetType == "" {
			assetType = inferDefaultAssetType(tt)
		}
		market := get(rec, "market")
		if market == "" {
			market = inferMarket(code, assetType)
		}
		settle := get(rec, "settle_date")
		if settle != "" {
			if t, e := parseCSVDate(settle); e == nil {
				settle = t.Format(dateLayout)
			} else {
				settle = ""
			}
		}
		rows = append(rows, csvRow{
			Date: date, Type: tt, Code: code, Market: market,
			Name: get(rec, "name"), AssetType: assetType,
			Shares: num(rec, "shares"), Price: num(rec, "price"),
			Fee: num(rec, "fee"), Tax: num(rec, "tax"), Amount: num(rec, "amount"),
			SettleDate: settle, Remark: get(rec, "remark"),
			Line: fileLine,
		})
	}
	if len(rows) == 0 {
		return nil, NewIAError(ErrImportFormat, "无有效数据行")
	}
	return rows, nil
}

func wrapRowErr(line int, err error) error {
	if ie, ok := err.(*IAError); ok {
		return &IAError{Code: ie.Code, Message: fmt.Sprintf("第 %d 行: %s", line, ie.Message)}
	}
	return fmt.Errorf("第 %d 行: %w", line, err)
}

// inferDefaultAssetType 由交易类型推断默认资产类别
func inferDefaultAssetType(tradeType string) string {
	switch TradeType(tradeType) {
	case TradeInterest:
		return string(AssetTypeDeposit)
	default:
		return string(AssetTypeStock)
	}
}

// ImportTransactions 导入对账单 CSV：
//   - 逐行校验、查重（账户+标的+类型+日期+份额+价格 元组）、缺失标的可自动创建
//   - 全部成功插入后按 (账户, 标的) 分组批量重放一次
func (s *Service) ImportTransactions(ctx context.Context, req *ImportTxReq) (*ImportResult, error) {
	if _, err := s.repo.GetAccount(ctx, req.AccountID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, NewIAError(ErrImportNoAccount)
		}
		return nil, err
	}
	rows, perr := ParseStatementCSV(req.CSV)
	if perr != nil {
		return nil, perr
	}

	res := &ImportResult{TotalRows: len(rows), Errors: []string{}, AssetsAdded: []string{}}
	type pair struct{ acct, asset string }
	replaySet := make(map[pair]bool)
	assetCache := make(map[string]*Asset) // key: code|market

	for _, r := range rows {
		key := r.Code + "|" + r.Market
		asset := assetCache[key]
		if asset == nil {
			a, created, gerr := s.findOrCreateAsset(ctx, req, r)
			if gerr != nil {
				res.Failed++
				res.Errors = append(res.Errors, fmt.Sprintf("第 %d 行: %v", r.Line, gerr))
				continue
			}
			if created {
				res.AssetsAdded = append(res.AssetsAdded, a.Code+"."+a.Market)
			}
			asset = a
			assetCache[key] = asset
		}

		tradeAt := r.Date.Add(15 * time.Hour) // 统一记为当日 15:00
		exists, derr := s.repo.TxTupleExists(ctx, req.AccountID, asset.ID, r.Type, tradeAt, round6(r.Shares), round6(r.Price))
		if derr != nil {
			res.Failed++
			res.Errors = append(res.Errors, fmt.Sprintf("第 %d 行: 查重失败 %v", r.Line, derr))
			continue
		}
		if exists {
			res.Duplicates++
			continue
		}

		amount := r.Amount
		if amount == 0 && r.Price > 0 {
			amount = round4(r.Shares * r.Price)
		}
		if _, cerr := s.repo.CreateTransaction(ctx, &Transaction{
			AccountID: req.AccountID, AssetID: asset.ID, TradeType: r.Type,
			Shares: round6(r.Shares), Price: round6(r.Price), Amount: amount,
			Fee: round4(r.Fee), Tax: round4(r.Tax),
			TradeAt: tradeAt, SettleDate: r.SettleDate,
			Status: string(TxStatusConfirmed), Remark: orDefault(r.Remark, "import"),
		}); cerr != nil {
			res.Failed++
			res.Errors = append(res.Errors, fmt.Sprintf("第 %d 行: 写入失败 %v", r.Line, cerr))
			continue
		}
		res.Created++
		replaySet[pair{req.AccountID, asset.ID}] = true
	}

	// 批量重放受影响持仓
	for p := range replaySet {
		if err := s.replay(ctx, p.acct, p.asset); err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("重放失败 %s: %v", p.asset, err))
		}
	}
	return res, nil
}

// findOrCreateAsset 按 code+market 查找标的；auto_create 开启且不存在时创建。返回是否新建。
func (s *Service) findOrCreateAsset(ctx context.Context, req *ImportTxReq, r csvRow) (*Asset, bool, error) {
	dup, err := s.repo.AssetExistsByCode(ctx, r.Code, r.Market)
	if err != nil {
		return nil, false, err
	}
	if dup {
		var id string
		if err := s.repo.db.QueryRowContext(ctx,
			`SELECT id FROM zzz_ia_assets WHERE code = ? AND market = ?`, r.Code, r.Market).Scan(&id); err != nil {
			return nil, false, err
		}
		a, gerr := s.repo.GetAsset(ctx, id)
		return a, false, gerr
	}
	if !req.AutoCreate {
		return nil, false, NewIAError(ErrAssetNotFound, "标的不存在且未开启 auto_create: %s.%s", r.Market, r.Code)
	}
	defType := req.DefaultType
	if defType == "" {
		defType = r.AssetType
	}
	if defType == "" {
		defType = string(AssetTypeStock)
	}
	name := orDefault(r.Name, r.Code)
	created, err := s.CreateAsset(ctx, &CreateAssetReq{
		Code: r.Code, Name: name, AssetType: defType,
		Market: r.Market, Currency: "CNY",
	})
	if err != nil {
		return nil, false, err
	}
	return created, true, nil
}

// notifyRoom 向 WS 房间广播事件（Notifier 未注入时静默跳过）
func (s *Service) notifyRoom(msgType string, payload map[string]any) {
	if s.notifier == nil {
		return
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return
	}
	s.notifier.BroadcastToRoom(IARoom, msgType, b)
}

// ============================================================================
// 内部工具
// ============================================================================

// orDefault 空值兜底
func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
