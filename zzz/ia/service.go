// Package ia 业务逻辑层 — 校验、持仓重放引擎、汇总分析
package ia

import (
	"context"
	"database/sql"
	"math"
	"strings"
	"time"

	"github.com/Allinost/go-backend-core/internal/modules/webhook"
	"github.com/Allinost/go-backend-core/internal/pkg/logger"
	"github.com/Allinost/go-backend-core/zzz/ia/quote"
)

// IARoom WebSocket 推送房间名
const IARoom = "zzz-ia"

// RoomNotifier 房间广播接口（解耦 ws.Hub，便于注入与测试）
type RoomNotifier interface {
	BroadcastToRoom(room, msgType string, payload []byte)
}

// Service 投资分析业务逻辑
type Service struct {
	repo     *Repository
	quotes   *quote.Registry  // 行情注册表（P3 注入）
	webhook  *webhook.Service // webhook 通知（可选注入）
	notifier RoomNotifier     // WS 房间广播（可选注入）
}

// NewService 创建 Service
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// SetNotifier 注入 WS 广播器（模块装配时调用，可为 nil）
func (s *Service) SetNotifier(n RoomNotifier) { s.notifier = n }

// ============================================================================
// 账户
// ============================================================================

var validAccountTypes = map[string]bool{string(AccountTypeBroker): true, string(AccountTypeBank): true, string(AccountTypeFund): true, string(AccountTypeOther): true}
var validCostMethods = map[string]bool{string(CostMethodWA): true, string(CostMethodFIFO): true}

// CreateAccount 创建账户
func (s *Service) CreateAccount(ctx context.Context, req *CreateAccountReq) (*Account, error) {
	if !validAccountTypes[req.Type] {
		return nil, NewIAError(ErrInvalidAccountType)
	}
	if req.Currency != "" && !validCurrency(req.Currency) {
		return nil, NewIAError(ErrInvalidCurrencyCode)
	}
	if req.CostMethod != "" && !validCostMethods[req.CostMethod] {
		return nil, NewIAError(ErrInvalidCostMethod)
	}
	return s.repo.CreateAccount(ctx, req)
}

// UpdateAccount 更新账户；成本法变更后需重算该账户全部持仓
func (s *Service) UpdateAccount(ctx context.Context, id string, req *UpdateAccountReq) (*Account, error) {
	if req.Type != nil && !validAccountTypes[*req.Type] {
		return nil, NewIAError(ErrInvalidAccountType)
	}
	if req.Currency != nil && *req.Currency != "" && !validCurrency(*req.Currency) {
		return nil, NewIAError(ErrInvalidCurrencyCode)
	}
	if req.CostMethod != nil && !validCostMethods[*req.CostMethod] {
		return nil, NewIAError(ErrInvalidCostMethod)
	}
	acct, err := s.repo.UpdateAccount(ctx, id, req)
	if err == sql.ErrNoRows {
		return nil, NewIAError(ErrAccountNotFound)
	}
	if err != nil {
		return nil, err
	}
	// 成本法变化时全量重放该账户持仓，保证已实现收益口径一致
	if req.CostMethod != nil {
		if err = s.ReplayAccountAll(ctx, id); err != nil {
			return nil, err
		}
	}
	return acct, nil
}

// GetAccount 获取账户详情
func (s *Service) GetAccount(ctx context.Context, id string) (*Account, error) {
	a, err := s.repo.GetAccount(ctx, id)
	if err == sql.ErrNoRows {
		return nil, NewIAError(ErrAccountNotFound)
	}
	return a, err
}

// DeleteAccount 删除账户（存在持仓或交易时拒绝）
func (s *Service) DeleteAccount(ctx context.Context, id string) error {
	if _, err := s.repo.GetAccount(ctx, id); err == sql.ErrNoRows {
		return NewIAError(ErrAccountNotFound)
	} else if err != nil {
		return err
	}
	has, err := s.repo.PositionExists(ctx, id)
	if err != nil {
		return err
	}
	if has {
		return NewIAError(ErrAccountHasPositions)
	}
	return s.repo.DeleteAccount(ctx, id)
}

// ListAccounts 分页查询账户
func (s *Service) ListAccounts(ctx context.Context, req *ListAccountsReq) ([]Account, bool, int, error) {
	return s.repo.ListAccounts(ctx, req)
}

// ============================================================================
// 标的
// ============================================================================

var validAssetTypes = map[string]bool{string(AssetTypeDeposit): true, string(AssetTypeWealth): true, string(AssetTypeFund): true, string(AssetTypeStock): true, string(AssetTypeETF): true}
var validMarkets = map[string]bool{string(MarketCNSH): true, string(MarketCNSZ): true, string(MarketHK): true, string(MarketUS): true, string(MarketOTC): true, string(MarketBank): true}
var validCurrencies = map[string]bool{"CNY": true, "USD": true, "HKD": true}

func validCurrency(c string) bool { return validCurrencies[c] }

// ResolveAssetCode 根据证券代码自动识别市场并拉取行情信息，用于新建标的时自动补全
func (s *Service) ResolveAssetCode(ctx context.Context, code string, marketHint string) (*AssetResolveResp, error) {
	if s.quotes == nil {
		return nil, NewIAError(ErrQuoteUnavailable)
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, NewIAError(ErrInvalidParams)
	}

	// 确定市场
	market := marketHint
	if market == "" {
		market = detectMarket(code)
	}
	if market == "" {
		return nil, NewIAError(ErrInvalidMarket)
	}

	// 确定币种
	currency := "CNY"
	switch market {
	case "HK":
		currency = "HKD"
	case "US":
		currency = "USD"
	}

	// 推断资产类型
	assetType := inferAssetType(code, market)

	// 用单股端点直接拉取完整行情（push2 单股端点通常不被反爬封锁）
	secid := secidForMarket(code, market)
	detail, err := quote.FetchStockDetail(ctx, s.quotes.HTTP(), secid)
	if err != nil {
		// 行情拉取失败也返回基础信息，让用户手动补全
		return &AssetResolveResp{
			Code: code, Market: market, Currency: currency, AssetType: assetType,
		}, nil
	}

	resp := &AssetResolveResp{
		Code:       code,
		Name:       detail.Name,
		Market:     market,
		Currency:   currency,
		AssetType:  assetType,
		Price:      detail.Price,
		ChangePct:  detail.ChangePct,
		TurnoverRate: detail.TurnoverRate,
		PE:         detail.PETTM,
		PEDynamic:  detail.PEDynamic,
		TotalMVYi:  detail.TotalMV / 1e8,
		FloatMVYi:  detail.FloatMV / 1e8,
		High:       detail.High,
		Low:        detail.Low,
		Open:       detail.Open,
		PrevClose:  detail.PrevClose,
		Volume:     detail.Volume,
		Turnover:   detail.Turnover,
	}
	return resp, nil
}

// secidForMarket 根据市场推断东财 secid
func secidForMarket(code, market string) string {
	switch market {
	case "CN-SH":
		return "1." + code
	case "CN-SZ":
		return "0." + code
	case "HK":
		return "116." + code
	case "US":
		return "105." + code
	}
	return "0." + code
}

// inferAssetType 根据代码/名称推断资产类型
func inferAssetType(code, market string) string {
	// 科创板
	if market == "CN-SH" && len(code) == 6 && code[:3] == "688" {
		return "stock"
	}
	// 创业板
	if market == "CN-SZ" && len(code) == 6 && code[:3] == "300" {
		return "stock"
	}
	// 默认股票
	return "stock"
}

// detectMarket 根据代码格式推断市场
func detectMarket(code string) string {
	// 纯数字
	if len(code) == 6 && isDigitStr(code) {
		switch {
		case code[0] == '6' || code[0] == '9':
			return "CN-SH" // 沪市主板/科创板B股
		case code[0] == '0' || code[0] == '3':
			return "CN-SZ" // 深市主板/创业板
		}
	}
	// 港股：5位数字
	if len(code) == 5 && isDigitStr(code) {
		return "HK"
	}
	// 美股：字母开头
	if len(code) > 0 && code[0] >= 'A' && code[0] <= 'Z' || len(code) > 0 && code[0] >= 'a' && code[0] <= 'z' {
		return "US"
	}
	return ""
}

func isDigitStr(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// CreateAsset 创建标的
func (s *Service) CreateAsset(ctx context.Context, req *CreateAssetReq) (*Asset, error) {
	if !validAssetTypes[req.AssetType] {
		return nil, NewIAError(ErrInvalidAssetType)
	}
	if !validMarkets[req.Market] {
		return nil, NewIAError(ErrInvalidMarket)
	}
	if req.Currency != "" && !validCurrency(req.Currency) {
		return nil, NewIAError(ErrInvalidCurrencyCode)
	}
	if req.StartDate != "" && !isDateStr(req.StartDate) {
		return nil, NewIAError(ErrInvalidDate)
	}
	if req.MaturityDate != "" && !isDateStr(req.MaturityDate) {
		return nil, NewIAError(ErrInvalidDate)
	}
	dup, err := s.repo.AssetExistsByCode(ctx, req.Code, req.Market)
	if err != nil {
		return nil, err
	}
	if dup {
		return nil, NewIAError(ErrDuplicateAsset)
	}
	return s.repo.CreateAsset(ctx, req)
}

// UpdateAsset 更新标的
func (s *Service) UpdateAsset(ctx context.Context, id string, req *UpdateAssetReq) (*Asset, error) {
	if req.AssetType != nil && !validAssetTypes[*req.AssetType] {
		return nil, NewIAError(ErrInvalidAssetType)
	}
	if req.Market != nil && !validMarkets[*req.Market] {
		return nil, NewIAError(ErrInvalidMarket)
	}
	if req.Currency != nil && *req.Currency != "" && !validCurrency(*req.Currency) {
		return nil, NewIAError(ErrInvalidCurrencyCode)
	}
	if req.StartDate != nil && *req.StartDate != "" && !isDateStr(*req.StartDate) {
		return nil, NewIAError(ErrInvalidDate)
	}
	if req.MaturityDate != nil && *req.MaturityDate != "" && !isDateStr(*req.MaturityDate) {
		return nil, NewIAError(ErrInvalidDate)
	}
	a, err := s.repo.UpdateAsset(ctx, id, req)
	if err == sql.ErrNoRows {
		return nil, NewIAError(ErrAssetNotFound)
	}
	return a, err
}

// GetAsset 获取标的详情
func (s *Service) GetAsset(ctx context.Context, id string) (*Asset, error) {
	a, err := s.repo.GetAsset(ctx, id)
	if err == sql.ErrNoRows {
		return nil, NewIAError(ErrAssetNotFound)
	}
	return a, err
}

// DeleteAsset 删除标的（仅当无任何持仓记录时允许）
func (s *Service) DeleteAsset(ctx context.Context, id string) error {
	var n int
	if err := s.repo.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM zzz_ia_positions WHERE asset_id = ?`, id).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return NewIAError(ErrAccountHasPositions)
	}
	_, err := s.repo.db.ExecContext(ctx, `DELETE FROM zzz_ia_assets WHERE id = ?`, id)
	return err
}

// ListAssets 分页查询标的
func (s *Service) ListAssets(ctx context.Context, req *ListAssetsReq) ([]Asset, bool, int, error) {
	return s.repo.ListAssets(ctx, req)
}

// UpdateLastPrice 手工更新最新价/净值（存款理财等无行情资产估值入口）
func (s *Service) UpdateLastPrice(ctx context.Context, accountID, assetID string, price float64) error {
	if price <= 0 {
		return NewIAError(ErrInvalidRatio)
	}
	_, err := s.repo.db.ExecContext(ctx,
		`UPDATE zzz_ia_positions SET last_price = ?, last_price_at = NOW(), last_price_source = 'manual'
		 WHERE account_id = ? AND asset_id = ?`, price, accountID, assetID)
	return err
}

// ============================================================================
// 资金流水
// ============================================================================

var validFlowTypes = map[string]bool{
	string(CashFlowDepositIn): true, string(CashFlowWithdrawOut): true,
	string(CashFlowTransferIn): true, string(CashFlowTransferOut): true,
}

// CreateCashFlow 创建资金流水
func (s *Service) CreateCashFlow(ctx context.Context, req *CreateCashFlowReq) (*CashFlow, error) {
	if !validFlowTypes[req.FlowType] {
		return nil, NewIAError(ErrInvalidFlowType)
	}
	if _, err := s.repo.GetAccount(ctx, req.AccountID); err != nil {
		if err == sql.ErrNoRows {
			return nil, NewIAError(ErrAccountNotFound)
		}
		return nil, err
	}
	if req.CounterAcctID != "" {
		if req.CounterAcctID == req.AccountID {
			return nil, NewIAError(ErrTransferSameAccount)
		}
		if _, err := s.repo.GetAccount(ctx, req.CounterAcctID); err != nil {
			if err == sql.ErrNoRows {
				return nil, NewIAError(ErrAccountNotFound)
			}
			return nil, err
		}
	}
	currency := req.Currency
	if currency == "" {
		acct, err := s.repo.GetAccount(ctx, req.AccountID)
		if err != nil {
			return nil, err
		}
		currency = acct.Currency
	}
	date := req.FlowDate
	if date == "" {
		date = time.Now().Format(dateLayout)
	} else if !isDateStr(date) {
		return nil, NewIAError(ErrInvalidDate)
	}
	return s.repo.CreateCashFlow(ctx, &CashFlow{
		AccountID: req.AccountID, FlowType: req.FlowType, Amount: req.Amount,
		Currency: currency, FlowDate: date, CounterAcctID: req.CounterAcctID, Remark: req.Remark,
	})
}

// ListCashFlows 分页查询资金流水
func (s *Service) ListCashFlows(ctx context.Context, req *ListCashFlowsReq) ([]CashFlow, bool, int, error) {
	return s.repo.ListCashFlows(ctx, req)
}

// DeleteCashFlow 删除资金流水
func (s *Service) DeleteCashFlow(ctx context.Context, id int64) error {
	res, err := s.repo.db.ExecContext(ctx, `DELETE FROM zzz_ia_cash_flows WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return NewIAError(ErrCashFlowNotFound)
	}
	return nil
}

// ============================================================================
// 交易流水 + 持仓重放引擎
// ============================================================================

var validTradeTypes = map[string]bool{
	string(TradeBuy): true, string(TradeSell): true,
	string(TradeDividendCash): true, string(TradeDividendReinv): true,
	string(TradeInterest): true, string(TradeBonusShares): true, string(TradeRightsIssue): true,
}

// CreateTransaction 创建交易并重放持仓。
// 核算规则：买入费用资本化计入成本；卖出费用与税款冲减已实现收益；
// 现金分红计税后收益不改成本；红利再投视同零费买入参与加权；送转股增份额不改成本。
func (s *Service) CreateTransaction(ctx context.Context, req *CreateTransactionReq) (*Transaction, error) {
	t, err := s.buildTx(ctx, req)
	if err != nil {
		return nil, err
	}

	// 卖出前置校验：当前可用份额充足（重放引擎二次兜底）
	if t.TradeType == string(TradeSell) {
		pos, err := s.getPosRow(ctx, t.AccountID, t.AssetID)
		if err != nil {
			return nil, err
		}
		if pos != nil && pos.AvailableShares+1e-6 < t.Shares {
			return nil, NewIAError(ErrInsufficientShares)
		}
	}

	created, err := s.repo.CreateTransaction(ctx, t)
	if err != nil {
		return nil, err
	}
	if t.Status == string(TxStatusConfirmed) {
		if err = s.replay(ctx, t.AccountID, t.AssetID); err != nil {
			// 重放异常则回滚本次写入，保证账实一致
			_ = s.repo.DeleteTransaction(ctx, created.ID)
			return nil, err
		}
		// 买入时设置追踪目标（可选）：target_price/stop_loss_price 随买入库
		if req.TradeType == string(TradeBuy) && (req.TargetPrice > 0 || req.StopLossPrice > 0) {
			if terr := s.repo.SetPositionTarget(ctx, t.AccountID, t.AssetID, round6(req.TargetPrice), round6(req.StopLossPrice)); terr != nil {
				// 目标设置失败不影响交易成立，仅记录
				logger.Warn().Err(terr).Msgf("zzz-ia: 追踪目标设置失败 acct=%s asset=%s", t.AccountID, t.AssetID)
			}
		}
	}
	return created, nil
}

// buildTx 构造并校验交易实体
func (s *Service) buildTx(ctx context.Context, req *CreateTransactionReq) (*Transaction, error) {
	if !validTradeTypes[req.TradeType] {
		return nil, NewIAError(ErrInvalidTradeType)
	}
	if _, err := s.repo.GetAccount(ctx, req.AccountID); err != nil {
		if err == sql.ErrNoRows {
			return nil, NewIAError(ErrAccountNotFound)
		}
		return nil, err
	}
	asset, err := s.repo.GetAsset(ctx, req.AssetID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, NewIAError(ErrAssetNotFound)
		}
		return nil, err
	}

	status := req.Status
	if status == "" {
		status = string(TxStatusConfirmed)
	}
	if status != string(TxStatusPending) && status != string(TxStatusConfirmed) {
		return nil, NewIAError(ErrInvalidTxStatus)
	}

	tradeAt := time.Now()
	if req.TradeAt != "" {
		t, err := parseDateTime(req.TradeAt)
		if err != nil {
			return nil, NewIAError(ErrInvalidDate)
		}
		tradeAt = t
	}

	fee := round4(req.Fee)
	tax := round4(req.Tax)
	var shares, price, amount float64

	switch req.TradeType {
	case string(TradeBuy), string(TradeRightsIssue):
		if req.Shares <= 0 || req.Price <= 0 {
			return nil, NewIAError(ErrInvalidTradeType, "买入需提供份额与价格")
		}
		shares, price = round6(req.Shares), round6(req.Price)
		amount = round4(shares * price)
	case string(TradeSell):
		if req.Shares <= 0 || req.Price <= 0 {
			return nil, NewIAError(ErrInvalidTradeType, "卖出需提供份额与价格")
		}
		shares, price = round6(req.Shares), round6(req.Price)
		amount = round4(shares * price)
	case string(TradeDividendReinv):
		if req.Shares <= 0 || req.Price <= 0 {
			return nil, NewIAError(ErrInvalidTradeType, "红利再投需提供份额与除息净值")
		}
		shares, price = round6(req.Shares), round6(req.Price)
		amount = round4(shares * price)
	case string(TradeBonusShares):
		if req.Shares <= 0 {
			return nil, NewIAError(ErrInvalidTradeType, "送转股需提供新增份额")
		}
		shares = round6(req.Shares)
	case string(TradeDividendCash):
		if req.Amount <= 0 {
			return nil, NewIAError(ErrInvalidTradeType, "现金分红需提供税前金额")
		}
		amount = round4(req.Amount)
		if tax < 0 || tax > amount {
			return nil, NewIAError(ErrInvalidRatio, "红利税应在 0 与税前金额之间")
		}
	case string(TradeInterest):
		if req.Amount <= 0 {
			return nil, NewIAError(ErrInvalidTradeType, "利息需提供金额")
		}
		amount = round4(req.Amount)
	}

	// 类型与标的兼容性校验
	if err = checkTradeCompatibility(req.TradeType, asset.AssetType); err != nil {
		return nil, err
	}

	settle := req.SettleDate
	if settle != "" && !isDateStr(settle) {
		return nil, NewIAError(ErrInvalidDate)
	}

	return &Transaction{
		AccountID: req.AccountID, AssetID: req.AssetID, TradeType: req.TradeType,
		Shares: shares, Price: price, Amount: amount, Fee: fee, Tax: tax,
		FeeDetail: marshalFeeDetail(req.FeeDetail),
		TradeAt:   tradeAt, SettleDate: settle, Status: status, Remark: req.Remark,
	}, nil
}

// checkTradeCompatibility 交易类型与资产类型兼容性
func checkTradeCompatibility(tradeType, assetType string) error {
	isSecurity := assetType == string(AssetTypeStock) || assetType == string(AssetTypeETF) || assetType == string(AssetTypeFund)
	isDepositLike := assetType == string(AssetTypeDeposit) || assetType == string(AssetTypeWealth)
	switch tradeType {
	case string(TradeBuy), string(TradeSell), string(TradeRightsIssue):
		return nil // 各类资产均可买卖
	case string(TradeDividendCash), string(TradeDividendReinv), string(TradeBonusShares):
		if !isSecurity {
			return NewIAError(ErrInvalidTradeType, "分红/送转仅适用于股票、ETF、基金")
		}
	case string(TradeInterest):
		if !isDepositLike {
			return NewIAError(ErrInvalidTradeType, "利息仅适用于存款、理财")
		}
	}
	return nil
}

// ConfirmTransaction 确认在途交易（场外申购到账）
func (s *Service) ConfirmTransaction(ctx context.Context, id int64) error {
	t, err := s.repo.GetTransaction(ctx, id)
	if err == sql.ErrNoRows {
		return NewIAError(ErrTransactionNotFound)
	}
	if err != nil {
		return err
	}
	if t.Status != string(TxStatusPending) {
		return NewIAError(ErrInvalidTxStatus, "仅待确认交易可确认")
	}
	if err = s.repo.UpdateTxStatus(ctx, id, string(TxStatusConfirmed)); err != nil {
		return err
	}
	return s.replay(ctx, t.AccountID, t.AssetID)
}

// DeleteTransaction 删除交易并重放；重放失败则恢复被删流水
func (s *Service) DeleteTransaction(ctx context.Context, id int64) error {
	t, err := s.repo.GetTransaction(ctx, id)
	if err == sql.ErrNoRows {
		return NewIAError(ErrTransactionNotFound)
	}
	if err != nil {
		return err
	}
	if err = s.repo.DeleteTransaction(ctx, id); err != nil {
		return err
	}
	if err = s.replay(ctx, t.AccountID, t.AssetID); err != nil {
		// 补偿：恢复删除的流水
		restored := t
		_, _ = s.repo.CreateTransaction(ctx, &Transaction{
			AccountID: restored.AccountID, AssetID: restored.AssetID, TradeType: restored.TradeType,
			Shares: restored.Shares, Price: restored.Price, Amount: restored.Amount, Fee: restored.Fee,
			FeeDetail: restored.FeeDetail, Tax: restored.Tax, TradeAt: restored.TradeAt,
			SettleDate: restored.SettleDate, Status: restored.Status, Remark: restored.Remark,
		})
		_ = s.replay(ctx, t.AccountID, t.AssetID)
		return err
	}
	return nil
}

// ListTransactions 分页查询交易
func (s *Service) ListTransactions(ctx context.Context, req *ListTransactionsReq) ([]Transaction, bool, int, error) {
	return s.repo.ListTransactions(ctx, req)
}

// GetTransaction 交易详情
func (s *Service) GetTransaction(ctx context.Context, id int64) (*Transaction, error) {
	t, err := s.repo.GetTransaction(ctx, id)
	if err == sql.ErrNoRows {
		return nil, NewIAError(ErrTransactionNotFound)
	}
	return t, err
}

// ----------------------------------------------------------------------------
// 重放引擎
// ----------------------------------------------------------------------------

// fifoLot FIFO 成本批次
type fifoLot struct {
	txID     int64
	shares   float64
	unitCost float64 // 含分摊买入费用的每股成本
}

// ReplayAccountAll 重放某账户下全部标的持仓（成本法切换时调用）
func (s *Service) ReplayAccountAll(ctx context.Context, accountID string) error {
	const q = `SELECT DISTINCT asset_id FROM zzz_ia_transactions WHERE account_id = ? AND status = 'confirmed'`
	rows, err := s.repo.db.QueryContext(ctx, q, accountID)
	if err != nil {
		return err
	}
	defer rows.Close()
	var assetIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		assetIDs = append(assetIDs, id)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	for _, aid := range assetIDs {
		if err = s.replay(ctx, accountID, aid); err != nil {
			return err
		}
	}
	return nil
}

// replay 从已确认流水重放某账户某标的的持仓与已实现收益台账。
// 幂等：任意流水增删改后调用均能重建正确状态。
func (s *Service) replay(ctx context.Context, accountID, assetID string) error {
	acct, err := s.repo.GetAccount(ctx, accountID)
	if err != nil {
		return err
	}
	asset, err := s.repo.GetAsset(ctx, assetID)
	if err != nil {
		return err
	}
	txs, err := s.repo.ListConfirmedByAcctAsset(ctx, accountID, assetID)
	if err != nil {
		return err
	}

	method := CostMethod(acct.CostMethod)
	var shares, costAmount float64        // 加权平均状态
	var lots []fifoLot                    // FIFO 批次队列
	var realizedLots []RealizedPnL        // 已实现收益台账重建结果
	sellResults := make(map[int64][2]any) // txID → [method, net_pnl]
	now := time.Now()

	for i := range txs {
		t := &txs[i]
		realizedAt := t.TradeAt.Format(dateLayout)
		switch TradeType(t.TradeType) {
		case TradeBuy, TradeRightsIssue:
			totalCost := t.Amount + t.Fee // 买入费用资本化
			if method == CostMethodFIFO {
				lots = append(lots, fifoLot{txID: t.ID, shares: t.Shares, unitCost: safeDiv(totalCost, t.Shares)})
			}
			shares += t.Shares
			costAmount += totalCost

		case TradeSell:
			qty := t.Shares
			if shares+1e-6 < qty {
				return NewIAError(ErrNegativeShares, "流水 #%d 卖出超过当时持仓", t.ID)
			}
			gross := t.Amount // price×shares
			var costRemoved float64
			var lotIDs []int64
			if method == CostMethodFIFO {
				remaining := qty
				for remaining > 1e-9 && len(lots) > 0 {
					lot := &lots[0]
					use := math.Min(lot.shares, remaining)
					costRemoved += use * lot.unitCost
					if use >= lot.shares-1e-9 {
						lotIDs = append(lotIDs, lot.txID)
						lots = lots[1:]
					} else {
						lot.shares -= use
						lotIDs = append(lotIDs, lot.txID)
					}
					remaining -= use
				}
			} else {
				avg := safeDiv(costAmount, shares)
				costRemoved = avg * qty
				shares -= qty
				costAmount -= costRemoved
			}
			net := round4(gross - costRemoved - t.Fee - t.Tax)
			lotRef := any(int64(0))
			if len(lotIDs) > 0 {
				lotRef = lotIDs[0]
			}
			realizedLots = append(realizedLots, RealizedPnL{
				AccountID: accountID, AssetID: assetID, TransactionID: t.ID,
				LotTransactionID: lotRef.(int64), Method: string(method),
				Shares: qty, SellPrice: t.Price, CostPrice: round6(safeDiv(costRemoved, qty)),
				GrossPnL: round4(gross - costRemoved), Fee: t.Fee, Tax: t.Tax, NetPnL: net, RealizedAt: realizedAt,
			})
			sellResults[t.ID] = [2]any{string(method), net}
			if method == CostMethodFIFO {
				shares -= qty
			}

		case TradeDividendReinv:
			shares += t.Shares
			costAmount += t.Amount // 视同零费买入
			if method == CostMethodFIFO {
				lots = append(lots, fifoLot{txID: t.ID, shares: t.Shares, unitCost: safeDiv(t.Amount, t.Shares)})
			}

		case TradeBonusShares:
			shares += t.Shares // 送转股：份额增加，成本不变

		case TradeDividendCash, TradeInterest:
			// 现金流收益，不影响持仓成本；统计走聚合查询
		}

		// 浮点误差收敛
		if math.Abs(shares) < 1e-6 {
			shares = 0
		}
		if math.Abs(costAmount) < 1e-4 {
			costAmount = 0
		}
		if shares <= 0 && method == CostMethodWA {
			costAmount = 0
		}
	}

	locked := lockedShares(now, asset, shares)
	pos := &Position{
		AccountID: accountID, AssetID: assetID,
		Shares: round6(shares), AvailableShares: round6(shares - locked), LockedShares: round6(locked),
		AvgCost: round6(safeDiv(costAmount, shares)), CostAmount: round4(costAmount),
		Currency: asset.Currency, Status: "holding",
	}
	if pos.Shares <= 0 {
		pos.Status = "closed"
		pos.AvgCost, pos.CostAmount = 0, 0
	}
	if err = s.repo.UpsertPosition(ctx, pos); err != nil {
		return err
	}
	if err = s.repo.ReplaceRealizedPnL(ctx, accountID, assetID, realizedLots); err != nil {
		return err
	}
	for txID, res := range sellResults {
		m, _ := res[0].(string)
		pnl, _ := res[1].(float64)
		if err = s.repo.UpdateTxResult(ctx, txID, m, pnl); err != nil {
			return err
		}
	}
	return nil
}

// getPosRow 读取当前持仓行（不存在返回 nil,nil）
func (s *Service) getPosRow(ctx context.Context, accountID, assetID string) (*Position, error) {
	const q = `SELECT shares, available_shares FROM zzz_ia_positions WHERE account_id = ? AND asset_id = ?`
	var shares, avail float64
	err := s.repo.db.QueryRowContext(ctx, q, accountID, assetID).Scan(&shares, &avail)
	if err == sql.ErrNoRows {
		return &Position{}, nil
	}
	if err != nil {
		return nil, err
	}
	return &Position{Shares: shares, AvailableShares: avail}, nil
}

// lockedShares 计算锁定份额：理财在封闭期内（起息日+lock_days 未到）全额锁定
func lockedShares(now time.Time, asset *Asset, total float64) float64 {
	if asset.LockDays <= 0 || asset.StartDate == "" || total <= 0 {
		return 0
	}
	start, err := time.ParseInLocation(dateLayout, asset.StartDate, time.Local)
	if err != nil {
		return 0
	}
	lockEnd := start.AddDate(0, 0, asset.LockDays)
	if now.Before(lockEnd) {
		return total
	}
	return 0
}

// ============================================================================
// 持仓查询
// ============================================================================

// ListPositions 查询持仓并计算市值/浮盈
func (s *Service) ListPositions(ctx context.Context, req *ListPositionsReq) ([]PositionView, error) {
	positions, err := s.repo.ListPositions(ctx, req)
	if err != nil {
		return nil, err
	}
	out := make([]PositionView, 0, len(positions))
	for i := range positions {
		p := &positions[i]
		v := PositionView{Position: *p}
		price := p.LastPrice
		v.PriceValued = price > 0
		if price <= 0 {
			price = p.AvgCost // 无行情按成本兜底
		}
		v.MarketValue = round4(price * p.Shares)
		v.UnrealizedPnL = round4((price - p.AvgCost) * p.Shares)
		if p.CostAmount > 0 {
			v.PnLRatio = round6(v.UnrealizedPnL / p.CostAmount)
		}
		out = append(out, v)
	}
	return out, nil
}

// ============================================================================
// 总览
// ============================================================================

// Overview 全局与分账户汇总（含汇率折算）
func (s *Service) Overview(ctx context.Context) (*OverviewResp, error) {
	type aggKey struct{ accountID, ccy string }
	aggs := make(map[aggKey]*OverviewSummary)
	getAgg := func(k aggKey) *OverviewSummary {
		if aggs[k] == nil {
			aggs[k] = &OverviewSummary{}
		}
		return aggs[k]
	}

	// 持仓成本与市值
	positions, err := s.repo.ListPositions(ctx, &ListPositionsReq{Status: "holding"})
	if err != nil {
		return nil, err
	}
	for i := range positions {
		p := &positions[i]
		k := aggKey{p.AccountID, p.Currency}
		a := getAgg(k)
		a.TotalCost += p.CostAmount
		price := p.LastPrice
		if price <= 0 {
			price = p.AvgCost
		}
		a.TotalValue += price * p.Shares
	}

	// 现金余额
	balances, err := s.repo.CashBalances(ctx)
	if err != nil {
		return nil, err
	}
	for _, b := range balances {
		getAgg(aggKey{b.AccountID, b.Currency}).CashBalance = b.Balance
	}

	// 已实现收益 / 分红 / 利息 / 税
	incomes, err := s.repo.IncomeAggs(ctx)
	if err != nil {
		return nil, err
	}
	for _, it := range incomes {
		a := getAgg(aggKey{it.AccountID, it.Currency})
		a.DividendIncome = it.DividendNet
		a.InterestIncome = it.InterestIncome
		a.TaxPaid = it.TaxPaid
	}
	realized, err := s.repo.RealizedAggs(ctx)
	if err != nil {
		return nil, err
	}
	for _, it := range realized {
		getAgg(aggKey{it.AccountID, it.Currency}).RealizedPnL = it.NetPnL
	}

	// 独立费用
	fees, err := s.repo.FeeSums(ctx)
	if err != nil {
		return nil, err
	}
	for _, f := range fees {
		getAgg(aggKey{f.AccountID, f.Currency}).FeeTotal = f.Total
	}

	// 收尾计算
	rates, err := s.repo.LatestFxRates(ctx)
	if err != nil {
		return nil, err
	}
	toCNY := func(ccy string, v float64) float64 {
		if ccy == "CNY" || v == 0 {
			return v
		}
		if r, ok := rates["CNY->"+ccy]; ok {
			return v * r
		}
		return 0 // 缺汇率时不折算
	}

	resp := &OverviewResp{Accounts: make(map[string]OverviewSummary)}
	for k, a := range aggs {
		a.TotalPnL = round4(a.TotalValue - a.TotalCost + a.RealizedPnL + a.DividendIncome + a.InterestIncome - a.FeeTotal - a.TaxPaid)
		if a.TotalCost > 0 {
			a.PnLRatio = round6(a.TotalPnL / a.TotalCost)
		}
		resp.Accounts[k.accountID] = *a
	}

	// 全局按币种折叠后再汇总（跨币种需经汇率折算才有意义）
	byCcy := make(map[string]*OverviewSummary)
	for k, a := range aggs {
		g := byCcy[k.ccy]
		if g == nil {
			g = &OverviewSummary{}
			byCcy[k.ccy] = g
		}
		g.TotalCost += a.TotalCost
		g.TotalValue += a.TotalValue
		g.TotalPnL += a.TotalPnL
		g.RealizedPnL += a.RealizedPnL
		g.DividendIncome += a.DividendIncome
		g.InterestIncome += a.InterestIncome
		g.TaxPaid += a.TaxPaid
		g.FeeTotal += a.FeeTotal
		g.CashBalance += a.CashBalance
	}
	resp.Grand.ByCurrency = make(map[string]OverviewSummary, len(byCcy))
	for ccy, g := range byCcy {
		if g.TotalCost > 0 {
			g.PnLRatio = round6(g.TotalPnL / g.TotalCost)
		}
		resp.Grand.ByCurrency[ccy] = *g
		// CNY 口径全局汇总（缺汇率的币种不参与折算）
		resp.Grand.TotalValueCNY += toCNY(ccy, g.TotalValue) + toCNY(ccy, g.CashBalance)
		resp.Grand.TotalCost += toCNY(ccy, g.TotalCost)
		resp.Grand.TotalPnL += toCNY(ccy, g.TotalPnL)
		resp.Grand.RealizedPnL += toCNY(ccy, g.RealizedPnL)
		resp.Grand.DividendIncome += toCNY(ccy, g.DividendIncome)
		resp.Grand.InterestIncome += toCNY(ccy, g.InterestIncome)
		resp.Grand.TaxPaid += toCNY(ccy, g.TaxPaid)
		resp.Grand.FeeTotal += toCNY(ccy, g.FeeTotal)
		resp.Grand.CashBalance += toCNY(ccy, g.CashBalance)
	}
	if resp.Grand.TotalCost > 0 {
		resp.Grand.PnLRatio = round6(resp.Grand.TotalPnL / resp.Grand.TotalCost)
	}
	return resp, nil
}

// ============================================================================
// 定投计划
// ============================================================================

var validDCACycles = map[string]bool{string(DCADaily): true, string(DCAWeekly): true, string(DCABiWeekly): true, string(DCAMonthly): true}

// CreateDCAPlan 创建定投计划
func (s *Service) CreateDCAPlan(ctx context.Context, req *CreateDCAPlanReq) (*DCAPlan, error) {
	if !validDCACycles[req.Cycle] {
		return nil, NewIAError(ErrInvalidDCACycle)
	}
	if (req.Cycle == string(DCAWeekly) || req.Cycle == string(DCABiWeekly)) && req.ExecDay > 7 {
		return nil, NewIAError(ErrInvalidDCACycle, "周定投执行日应为 1-7（周一至周日）")
	}
	if _, err := s.repo.GetAccount(ctx, req.AccountID); err != nil {
		if err == sql.ErrNoRows {
			return nil, NewIAError(ErrAccountNotFound)
		}
		return nil, err
	}
	if _, err := s.repo.GetAsset(ctx, req.AssetID); err != nil {
		if err == sql.ErrNoRows {
			return nil, NewIAError(ErrAssetNotFound)
		}
		return nil, err
	}
	start := time.Now().Format(dateLayout)
	if req.StartDate != "" {
		if !isDateStr(req.StartDate) {
			return nil, NewIAError(ErrInvalidDate)
		}
		start = req.StartDate
	}
	end := ""
	if req.EndDate != "" {
		if !isDateStr(req.EndDate) {
			return nil, NewIAError(ErrInvalidDate)
		}
		end = req.EndDate
	}
	p := &DCAPlan{
		Name: req.Name, AccountID: req.AccountID, AssetID: req.AssetID,
		Amount: req.Amount, Cycle: req.Cycle, ExecDay: req.ExecDay,
		StartDate: start, EndDate: end, NextExecDate: nextExecDate(start, req.Cycle, req.ExecDay),
		Status: string(PlanActive), AutoBuy: req.AutoBuy, Remark: req.Remark,
	}
	return s.repo.CreateDCAPlan(ctx, p)
}

// UpdateDCAPlan 更新定投计划
func (s *Service) UpdateDCAPlan(ctx context.Context, id string, req *UpdateDCAPlanReq) (*DCAPlan, error) {
	old, err := s.repo.GetDCAPlan(ctx, id)
	if err == sql.ErrNoRows {
		return nil, NewIAError(ErrDCAPlanNotFound)
	}
	if err != nil {
		return nil, err
	}
	if old.Status == string(PlanFinished) {
		return nil, NewIAError(ErrPlanFinished)
	}
	if req.Cycle != nil && !validDCACycles[*req.Cycle] {
		return nil, NewIAError(ErrInvalidDCACycle)
	}
	if req.Status != nil {
		st := PlanStatus(*req.Status)
		if st != PlanActive && st != PlanPaused && st != PlanFinished {
			return nil, NewIAError(ErrInvalidTxStatus, "无效的计划状态")
		}
	}
	updated, err := s.repo.UpdateDCAPlan(ctx, id, req)
	if err != nil {
		return nil, err
	}
	// 周期/执行日变更后重算下次执行日
	if (req.Cycle != nil || req.ExecDay != nil) && updated.Status == string(PlanActive) {
		next := nextExecDate(time.Now().Format(dateLayout), updated.Cycle, updated.ExecDay)
		if next != updated.NextExecDate {
			if err = s.repo.UpdateDCANextExec(ctx, id, next); err != nil {
				return nil, err
			}
			updated.NextExecDate = next
		}
	}
	return updated, nil
}

// GetDCAPlan 定投计划详情
func (s *Service) GetDCAPlan(ctx context.Context, id string) (*DCAPlan, error) {
	p, err := s.repo.GetDCAPlan(ctx, id)
	if err == sql.ErrNoRows {
		return nil, NewIAError(ErrDCAPlanNotFound)
	}
	return p, err
}

// DeleteDCAPlan 删除定投计划
func (s *Service) DeleteDCAPlan(ctx context.Context, id string) error {
	res, err := s.repo.db.ExecContext(ctx, `DELETE FROM zzz_ia_dca_plans WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return NewIAError(ErrDCAPlanNotFound)
	}
	return nil
}

// ListDCAPlans 分页查询定投计划
func (s *Service) ListDCAPlans(ctx context.Context, req *ListDCAPlansReq) ([]DCAPlan, bool, int, error) {
	return s.repo.ListDCAPlans(ctx, req)
}

// nextExecDate 计算下一次执行日期
func nextExecDate(from, cycle string, execDay int) string {
	base, err := time.ParseInLocation(dateLayout, from, time.Local)
	if err != nil {
		base = truncateDate(time.Now())
	}
	today := truncateDate(time.Now())
	switch DCACycle(cycle) {
	case DCADaily:
		if base.After(today) {
			return base.Format(dateLayout)
		}
		return today.AddDate(0, 0, 1).Format(dateLayout)
	case DCAWeekly, DCABiWeekly:
		step := 7
		if DCACycle(cycle) == DCABiWeekly {
			step = 14
		}
		target := base
		for i := 0; i < step*3; i++ {
			if target.After(today) && weekdayNum(target) == execDay {
				return target.Format(dateLayout)
			}
			target = target.AddDate(0, 0, 1)
		}
		return target.Format(dateLayout)
	case DCAMonthly:
		y, m, _ := today.Date()
		cand := time.Date(y, m, execDay, 0, 0, 0, 0, today.Location())
		if !cand.After(today) {
			cand = cand.AddDate(0, 1, 0)
		}
		return cand.Format(dateLayout)
	}
	return from
}

// ============================================================================
// 汇率
// ============================================================================

// UpsertFxRate 录入/更新汇率
func (s *Service) UpsertFxRate(ctx context.Context, req *UpsertFxRateReq) error {
	if strings.EqualFold(req.BaseCcy, req.QuoteCcy) {
		return NewIAError(ErrInvalidCurrencyCode, "基准与报价币种不能相同")
	}
	date := req.RateDate
	if date == "" {
		date = time.Now().Format(dateLayout)
	} else if !isDateStr(date) {
		return NewIAError(ErrInvalidDate)
	}
	source := req.Source
	if source == "" {
		source = "manual"
	}
	return s.repo.UpsertFxRate(ctx, &FxRate{
		BaseCcy: strings.ToUpper(req.BaseCcy), QuoteCcy: strings.ToUpper(req.QuoteCcy),
		Rate: req.Rate, RateDate: date, Source: source,
	})
}

// ListFxRates 查询汇率
func (s *Service) ListFxRates(ctx context.Context, req *ListFxRatesReq) ([]FxRate, error) {
	return s.repo.ListFxRates(ctx, req)
}

// ============================================================================
// 资产配比
// ============================================================================

var validAllocCategories = map[string]bool{string(AllocStock): true, string(AllocETF): true, string(AllocFund): true, string(AllocDeposit): true, string(AllocWealth): true, string(AllocCash): true}

// CreateAllocation 创建配置目标项
func (s *Service) CreateAllocation(ctx context.Context, req *CreateAllocationReq) (*AllocationItem, error) {
	if !validAllocCategories[req.Category] {
		return nil, NewIAError(ErrInvalidAssetType, "无效的配置类别")
	}
	if req.TargetRatio <= 0 || req.TargetRatio >= 1 {
		return nil, NewIAError(ErrInvalidRatio)
	}
	if req.AccountID != "" {
		if _, err := s.repo.GetAccount(ctx, req.AccountID); err != nil {
			if err == sql.ErrNoRows {
				return nil, NewIAError(ErrAccountNotFound)
			}
			return nil, err
		}
	}
	item := &AllocationItem{
		AccountID: req.AccountID, Category: req.Category,
		TargetRatio: req.TargetRatio, Tolerance: req.Tolerance, Remark: req.Remark,
	}
	created, err := s.repo.CreateAllocation(ctx, item)
	if err == nil {
		sum, existCat, sumErr := s.repo.SumActiveRatios(ctx, req.AccountID, "")
		if sumErr == nil && existCat == req.Category && sum > 1.0001 {
			_ = s.repo.DeleteAllocation(ctx, created.ID)
			return nil, NewIAError(ErrCategoryConflict, "同范围内类别 %s 已存在", req.Category)
		}
		if sumErr == nil && sum > 1.0001 {
			_ = s.repo.DeleteAllocation(ctx, created.ID)
			return nil, NewIAError(ErrAllocationSumExceed)
		}
	}
	return created, err
}

// UpdateAllocation 更新配置目标项
func (s *Service) UpdateAllocation(ctx context.Context, id string, req *UpdateAllocationReq) (*AllocationItem, error) {
	old, err := s.repo.GetAllocation(ctx, id)
	if err == sql.ErrNoRows {
		return nil, NewIAError(ErrAllocationNotFound)
	}
	if err != nil {
		return nil, err
	}
	if req.TargetRatio != nil && (*req.TargetRatio <= 0 || *req.TargetRatio >= 1) {
		return nil, NewIAError(ErrInvalidRatio)
	}
	updated, err := s.repo.UpdateAllocation(ctx, id, req)
	if err != nil {
		return nil, err
	}
	sum, _, sumErr := s.repo.SumActiveRatios(ctx, old.AccountID, "")
	if sumErr == nil && sum > 1.0001 {
		return nil, NewIAError(ErrAllocationSumExceed)
	}
	return updated, nil
}

// DeleteAllocation 删除配置目标项
func (s *Service) DeleteAllocation(ctx context.Context, id string) error {
	res, err := s.repo.db.ExecContext(ctx, `DELETE FROM zzz_ia_allocations WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return NewIAError(ErrAllocationNotFound)
	}
	return nil
}

// ListAllocations 配置列表
func (s *Service) ListAllocations(ctx context.Context, req *ListAllocationsReq) ([]AllocationItem, error) {
	return s.repo.ListAllocations(ctx, req)
}

// allocCategoryOf 标的类型 → 配置类别
func allocCategoryOf(assetType string) AllocCategory {
	switch AssetType(assetType) {
	case AssetTypeStock:
		return AllocStock
	case AssetTypeETF:
		return AllocETF
	case AssetTypeFund:
		return AllocFund
	case AssetTypeDeposit:
		return AllocDeposit
	case AssetTypeWealth:
		return AllocWealth
	}
	return ""
}

// AllocationReview 实际配比 vs 目标配置（CNY 口径）
func (s *Service) AllocationReview(ctx context.Context, accountID string) (*AllocationReviewResp, error) {
	rates, err := s.repo.LatestFxRates(ctx)
	if err != nil {
		return nil, err
	}
	toCNY := func(ccy string, v float64) float64 {
		if ccy == "CNY" {
			return v
		}
		if r, ok := rates["CNY->"+ccy]; ok {
			return v * r
		}
		return 0
	}

	actual := make(map[AllocCategory]float64)
	positions, err := s.repo.ListPositions(ctx, &ListPositionsReq{AccountID: accountID, Status: "holding"})
	if err != nil {
		return nil, err
	}
	for i := range positions {
		p := &positions[i]
		cat := allocCategoryOf(p.Asset.AssetType)
		if cat == "" {
			continue
		}
		price := p.LastPrice
		if price <= 0 {
			price = p.AvgCost
		}
		actual[cat] += toCNY(p.Currency, price*p.Shares)
	}
	balances, err := s.repo.CashBalances(ctx)
	if err != nil {
		return nil, err
	}
	for _, b := range balances {
		if accountID != "" && b.AccountID != accountID {
			continue
		}
		actual[AllocCash] += toCNY(b.Currency, b.Balance)
	}

	targets, err := s.repo.ListAllocations(ctx, &ListAllocationsReq{AccountID: accountID, Status: "active"})
	if err != nil {
		return nil, err
	}

	resp := &AllocationReviewResp{AccountID: accountID, Items: []AllocationReviewItem{}, Uncovered: []string{}}
	totalValue := 0.0
	for _, v := range actual {
		totalValue += v
	}
	targetSet := make(map[AllocCategory]bool)
	for _, t := range targets {
		targetSet[AllocCategory(t.Category)] = true
		val := actual[AllocCategory(t.Category)]
		actualRatio := 0.0
		if totalValue > 0 {
			actualRatio = val / totalValue
		}
		dev := round6(actualRatio - t.TargetRatio)
		tol := t.Tolerance
		if tol <= 0 {
			tol = 0.05
		}
		resp.Items = append(resp.Items, AllocationReviewItem{
			Category: t.Category, TargetRatio: t.TargetRatio,
			ActualRatio: round6(actualRatio), ValueCNY: round4(val),
			Deviation: dev, Breach: math.Abs(dev) > tol,
		})
	}
	for cat, val := range actual {
		if val > 0 && !targetSet[cat] {
			resp.Uncovered = append(resp.Uncovered, string(cat))
		}
	}
	return resp, nil
}

// ============================================================================
// 内部工具
// ============================================================================

const dateLayout = "2006-01-02"

// todayStr 当天日期字符串
func todayStr() string { return time.Now().Format(dateLayout) }

// nonNil 确保 JSON 序列化输出 [] 而非 null
func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// isDateStr 校验 YYYY-MM-DD 格式
func isDateStr(s string) bool {
	_, err := time.Parse(dateLayout, s)
	return err == nil
}

// parseDateTime 解析 RFC3339 或 "YYYY-MM-DD[ HH:MM:SS]"
func parseDateTime(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", dateLayout} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, NewIAError(ErrInvalidDate)
}

// truncateDate 截断到日
func truncateDate(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// weekdayNum ISO 星期：1=周一 … 7=周日
func weekdayNum(t time.Time) int {
	w := int(t.Weekday())
	if w == 0 {
		return 7
	}
	return w
}

// safeDiv 安全除法（分母为 0 返回 0）
func safeDiv(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

// round4/round6 金额与份额精度处理
func round4(v float64) float64 { return math.Round(v*1e4) / 1e4 }
func round6(v float64) float64 { return math.Round(v*1e6) / 1e6 }
