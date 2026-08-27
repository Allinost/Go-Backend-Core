// Package ia 数据访问层 — MySQL
package ia

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	tableAccounts  = "zzz_ia_accounts"
	tableAssets    = "zzz_ia_assets"
	tableCashFlows = "zzz_ia_cash_flows"
	tableTxs       = "zzz_ia_transactions"
	tableFees      = "zzz_ia_fees"
	tablePositions = "zzz_ia_positions"
	tablePrices    = "zzz_ia_price_history"
	tableSnaps     = "zzz_ia_portfolio_snapshots"
	tableRealized  = "zzz_ia_realized_pnl"
	tableDCA       = "zzz_ia_dca_plans"
	tableFx        = "zzz_ia_fx_rates"
	tableAlloc     = "zzz_ia_allocations"
	tableAlerts    = "zzz_ia_price_alerts"
)

// Repository 数据访问层，封装 MySQL 操作
type Repository struct {
	db *sql.DB
}

// NewRepository 创建 Repository
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// ============================================================================
// 账户
// ============================================================================

const accountCols = `id, name, type, broker, currency, cost_method, status, remark, created_at, updated_at`

// CreateAccount 创建账户
func (r *Repository) CreateAccount(ctx context.Context, req *CreateAccountReq) (*Account, error) {
	id := uuid.NewString()
	currency := req.Currency
	if currency == "" {
		currency = "CNY"
	}
	method := req.CostMethod
	if method == "" {
		method = string(CostMethodWA)
	}
	_, err := r.db.ExecContext(ctx,
		fmt.Sprintf(`INSERT INTO %s (id, name, type, broker, currency, cost_method, status, remark) VALUES (?,?,?,?,?,?, 'active', ?)`, tableAccounts),
		id, req.Name, req.Type, req.Broker, currency, method, req.Remark)
	if err != nil {
		return nil, err
	}
	return r.GetAccount(ctx, id)
}

// GetAccount 按 ID 获取账户
func (r *Repository) GetAccount(ctx context.Context, id string) (*Account, error) {
	row := r.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT %s FROM %s WHERE id = ?`, accountCols, tableAccounts), id)
	return scanAccount(row)
}

// UpdateAccount 更新账户（部分更新）
func (r *Repository) UpdateAccount(ctx context.Context, id string, req *UpdateAccountReq) (*Account, error) {
	var sets []string
	var args []any
	appendSet := func(col string, v any) { sets = append(sets, col+" = ?"); args = append(args, v) }
	if req.Name != nil {
		appendSet("name", *req.Name)
	}
	if req.Type != nil {
		appendSet("type", *req.Type)
	}
	if req.Broker != nil {
		appendSet("broker", *req.Broker)
	}
	if req.Currency != nil {
		appendSet("currency", *req.Currency)
	}
	if req.CostMethod != nil {
		appendSet("cost_method", *req.CostMethod)
	}
	if req.Status != nil {
		appendSet("status", *req.Status)
	}
	if req.Remark != nil {
		appendSet("remark", *req.Remark)
	}
	if len(sets) == 0 {
		return r.GetAccount(ctx, id)
	}
	args = append(args, id)
	_, err := r.db.ExecContext(ctx,
		fmt.Sprintf(`UPDATE %s SET %s WHERE id = ?`, tableAccounts, strings.Join(sets, ", ")), args...)
	if err != nil {
		return nil, err
	}
	return r.GetAccount(ctx, id)
}

// DeleteAccount 删除账户
func (r *Repository) DeleteAccount(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s WHERE id = ?`, tableAccounts), id)
	return err
}

// ListAccounts 分页查询账户
func (r *Repository) ListAccounts(ctx context.Context, req *ListAccountsReq) ([]Account, bool, int, error) {
	conds := []string{"1=1"}
	var cargs []any
	if req.Type != "" {
		conds = append(conds, "type = ?")
		cargs = append(cargs, req.Type)
	}
	if req.Status != "" {
		conds = append(conds, "status = ?")
		cargs = append(cargs, req.Status)
	}
	if req.Keyword != "" {
		conds = append(conds, "(name LIKE ? OR broker LIKE ?)")
		kw := "%" + req.Keyword + "%"
		cargs = append(cargs, kw, kw)
	}
	w := strings.Join(conds, " AND ")

	var total int
	if err := r.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE %s`, tableAccounts, w), cargs...).Scan(&total); err != nil {
		return nil, false, 0, err
	}
	q := fmt.Sprintf(`SELECT %s FROM %s WHERE %s ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		accountCols, tableAccounts, w)
	rows, err := r.db.QueryContext(ctx, q, appendInt(cargs, req.PageSize+1, req.Offset())...)
	if err != nil {
		return nil, false, 0, err
	}
	defer rows.Close()
	items, err := scanAccounts(rows)
	if err != nil {
		return nil, false, 0, err
	}
	hasMore := len(items) > req.PageSize
	if hasMore {
		items = items[:req.PageSize]
	}
	return items, hasMore, total, nil
}

func scanAccount(row *sql.Row) (*Account, error) {
	var a Account
	err := row.Scan(&a.ID, &a.Name, &a.Type, &a.Broker, &a.Currency, &a.CostMethod, &a.Status, &a.Remark, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func scanAccounts(rows *sql.Rows) ([]Account, error) {
	defer rows.Close()
	var items []Account
	for rows.Next() {
		var a Account
		if err := rows.Scan(&a.ID, &a.Name, &a.Type, &a.Broker, &a.Currency, &a.CostMethod, &a.Status, &a.Remark, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	return items, rows.Err()
}

// ============================================================================
// 标的
// ============================================================================

const assetCols = `id, code, name, asset_type, sub_type, market, currency, tracking_index,
	annual_rate, start_date, maturity_date, lock_days, risk_level, issuer, status, remark, created_at, updated_at`

// CreateAsset 创建标的
func (r *Repository) CreateAsset(ctx context.Context, req *CreateAssetReq) (*Asset, error) {
	id := uuid.NewString()
	currency := req.Currency
	if currency == "" {
		currency = "CNY"
	}
	_, err := r.db.ExecContext(ctx,
		fmt.Sprintf(`INSERT INTO %s (id, code, name, asset_type, sub_type, market, currency, tracking_index,
			annual_rate, start_date, maturity_date, lock_days, risk_level, issuer, status, remark)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?, 'active', ?)`, tableAssets),
		id, req.Code, req.Name, req.AssetType, req.SubType, req.Market, currency, req.TrackingIndex,
		req.AnnualRate, nullStr(req.StartDate), nullStr(req.MaturityDate), req.LockDays, req.RiskLevel, req.Issuer, req.Remark)
	if err != nil {
		if isDuplicateErr(err) {
			return nil, NewIAError(ErrDuplicateAsset)
		}
		return nil, err
	}
	return r.GetAsset(ctx, id)
}

// GetAsset 按 ID 获取标的
func (r *Repository) GetAsset(ctx context.Context, id string) (*Asset, error) {
	row := r.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT %s FROM %s WHERE id = ?`, assetCols, tableAssets), id)
	return scanAsset(row)
}

// AssetExistsByCode 检查 code+market 是否已存在
func (r *Repository) AssetExistsByCode(ctx context.Context, code, market string) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE code = ? AND market = ?`, tableAssets), code, market).Scan(&n)
	return n > 0, err
}

// UpdateAsset 更新标的（部分更新）
func (r *Repository) UpdateAsset(ctx context.Context, id string, req *UpdateAssetReq) (*Asset, error) {
	var sets []string
	var args []any
	appendSet := func(col string, v any) { sets = append(sets, col+" = ?"); args = append(args, v) }
	if req.Code != nil {
		appendSet("code", *req.Code)
	}
	if req.Name != nil {
		appendSet("name", *req.Name)
	}
	if req.AssetType != nil {
		appendSet("asset_type", *req.AssetType)
	}
	if req.SubType != nil {
		appendSet("sub_type", *req.SubType)
	}
	if req.Market != nil {
		appendSet("market", *req.Market)
	}
	if req.Currency != nil {
		appendSet("currency", *req.Currency)
	}
	if req.TrackingIndex != nil {
		appendSet("tracking_index", *req.TrackingIndex)
	}
	if req.AnnualRate != nil {
		appendSet("annual_rate", *req.AnnualRate)
	}
	if req.StartDate != nil {
		appendSet("start_date", nullStr(*req.StartDate))
	}
	if req.MaturityDate != nil {
		appendSet("maturity_date", nullStr(*req.MaturityDate))
	}
	if req.LockDays != nil {
		appendSet("lock_days", *req.LockDays)
	}
	if req.RiskLevel != nil {
		appendSet("risk_level", *req.RiskLevel)
	}
	if req.Issuer != nil {
		appendSet("issuer", *req.Issuer)
	}
	if req.Status != nil {
		appendSet("status", *req.Status)
	}
	if req.Remark != nil {
		appendSet("remark", *req.Remark)
	}
	if len(sets) == 0 {
		return r.GetAsset(ctx, id)
	}
	args = append(args, id)
	_, err := r.db.ExecContext(ctx,
		fmt.Sprintf(`UPDATE %s SET %s WHERE id = ?`, tableAssets, strings.Join(sets, ", ")), args...)
	if err != nil {
		return nil, err
	}
	return r.GetAsset(ctx, id)
}

// DeleteAsset 删除标的
func (r *Repository) DeleteAsset(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s WHERE id = ?`, tableAssets), id)
	return err
}

// ListAssets 分页查询标的
func (r *Repository) ListAssets(ctx context.Context, req *ListAssetsReq) ([]Asset, bool, int, error) {
	conds := []string{"1=1"}
	var cargs []any
	if req.AssetType != "" {
		conds = append(conds, "asset_type = ?")
		cargs = append(cargs, req.AssetType)
	}
	if req.Market != "" {
		conds = append(conds, "market = ?")
		cargs = append(cargs, req.Market)
	}
	if req.Status != "" {
		conds = append(conds, "status = ?")
		cargs = append(cargs, req.Status)
	}
	if req.Keyword != "" {
		conds = append(conds, "(code LIKE ? OR name LIKE ? OR issuer LIKE ?)")
		kw := "%" + req.Keyword + "%"
		cargs = append(cargs, kw, kw, kw)
	}
	w := strings.Join(conds, " AND ")

	var total int
	if err := r.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE %s`, tableAssets, w), cargs...).Scan(&total); err != nil {
		return nil, false, 0, err
	}
	q := fmt.Sprintf(`SELECT %s FROM %s WHERE %s ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		assetCols, tableAssets, w)
	rows, err := r.db.QueryContext(ctx, q, appendInt(cargs, req.PageSize+1, req.Offset())...)
	if err != nil {
		return nil, false, 0, err
	}
	defer rows.Close()
	items, err := scanAssets(rows)
	if err != nil {
		return nil, false, 0, err
	}
	hasMore := len(items) > req.PageSize
	if hasMore {
		items = items[:req.PageSize]
	}
	return items, hasMore, total, nil
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func scanAsset(row *sql.Row) (*Asset, error) {
	var a Asset
	var subType, trackIdx, startDate, maturityDate, riskLevel, issuer, remark sql.NullString
	var rate sql.NullFloat64
	var lockDays sql.NullInt64
	err := row.Scan(&a.ID, &a.Code, &a.Name, &a.AssetType, &subType, &a.Market, &a.Currency, &trackIdx,
		&rate, &startDate, &maturityDate, &lockDays, &riskLevel, &issuer, &a.Status, &remark, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	a.SubType = subType.String
	a.TrackingIndex = trackIdx.String
	a.AnnualRate = rate.Float64
	a.StartDate = startDate.String
	a.MaturityDate = maturityDate.String
	a.LockDays = int(lockDays.Int64)
	a.RiskLevel = riskLevel.String
	a.Issuer = issuer.String
	a.Remark = remark.String
	return &a, nil
}

func scanAssets(rows *sql.Rows) ([]Asset, error) {
	defer rows.Close()
	var items []Asset
	for rows.Next() {
		var a Asset
		var subType, trackIdx, startDate, maturityDate, riskLevel, issuer, remark sql.NullString
		var rate sql.NullFloat64
		var lockDays sql.NullInt64
		if err := rows.Scan(&a.ID, &a.Code, &a.Name, &a.AssetType, &subType, &a.Market, &a.Currency, &trackIdx,
			&rate, &startDate, &maturityDate, &lockDays, &riskLevel, &issuer, &a.Status, &remark, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		a.SubType = subType.String
		a.TrackingIndex = trackIdx.String
		a.AnnualRate = rate.Float64
		a.StartDate = startDate.String
		a.MaturityDate = maturityDate.String
		a.LockDays = int(lockDays.Int64)
		a.RiskLevel = riskLevel.String
		a.Issuer = issuer.String
		a.Remark = remark.String
		items = append(items, a)
	}
	return items, rows.Err()
}

// ============================================================================
// 交易流水
// ============================================================================

const txCols = `id, account_id, asset_id, trade_type, shares, price, amount, fee, fee_detail, tax,
	trade_at, settle_date, status, method, realized_pnl, remark, created_at, updated_at`

// CreateTransaction 创建交易流水
func (r *Repository) CreateTransaction(ctx context.Context, t *Transaction) (*Transaction, error) {
	res, err := r.db.ExecContext(ctx,
		fmt.Sprintf(`INSERT INTO %s (account_id, asset_id, trade_type, shares, price, amount, fee, fee_detail, tax,
			trade_at, settle_date, status, remark) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`, tableTxs),
		t.AccountID, t.AssetID, t.TradeType, t.Shares, t.Price, t.Amount, t.Fee, nullStr(t.FeeDetail), t.Tax,
		t.TradeAt, nullStr(t.SettleDate), t.Status, t.Remark)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return r.GetTransaction(ctx, id)
}

// GetTransaction 按 ID 获取交易流水
func (r *Repository) GetTransaction(ctx context.Context, id int64) (*Transaction, error) {
	row := r.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT %s FROM %s WHERE id = ?`, txCols, tableTxs), id)
	return scanTx(row)
}

// UpdateTxStatus 更新交易状态（在途确认）
func (r *Repository) UpdateTxStatus(ctx context.Context, id int64, status string) error {
	_, err := r.db.ExecContext(ctx,
		fmt.Sprintf(`UPDATE %s SET status = ?, settle_date = COALESCE(settle_date, CURDATE()) WHERE id = ?`, tableTxs), status, id)
	return err
}

// UpdateTxResult 回放引擎回写卖出结算结果
func (r *Repository) UpdateTxResult(ctx context.Context, id int64, method string, pnl float64) error {
	_, err := r.db.ExecContext(ctx,
		fmt.Sprintf(`UPDATE %s SET method = ?, realized_pnl = ? WHERE id = ?`, tableTxs), method, pnl, id)
	return err
}

// DeleteTransaction 删除交易流水
func (r *Repository) DeleteTransaction(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s WHERE id = ?`, tableTxs), id)
	return err
}

// ListTransactions 分页查询交易流水
func (r *Repository) ListTransactions(ctx context.Context, req *ListTransactionsReq) ([]Transaction, bool, int, error) {
	conds := []string{"1=1"}
	var cargs []any
	if req.AccountID != "" {
		conds = append(conds, "account_id = ?")
		cargs = append(cargs, req.AccountID)
	}
	if req.AssetID != "" {
		conds = append(conds, "asset_id = ?")
		cargs = append(cargs, req.AssetID)
	}
	if req.TradeType != "" {
		conds = append(conds, "trade_type = ?")
		cargs = append(cargs, req.TradeType)
	}
	if req.Status != "" {
		conds = append(conds, "status = ?")
		cargs = append(cargs, req.Status)
	}
	if req.Start != "" {
		conds = append(conds, "trade_at >= ?")
		cargs = append(cargs, req.Start+" 00:00:00")
	}
	if req.End != "" {
		conds = append(conds, "trade_at <= ?")
		cargs = append(cargs, req.End+" 23:59:59")
	}
	w := strings.Join(conds, " AND ")

	var total int
	if err := r.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE %s`, tableTxs, w), cargs...).Scan(&total); err != nil {
		return nil, false, 0, err
	}
	q := fmt.Sprintf(`SELECT %s FROM %s WHERE %s ORDER BY trade_at DESC, id DESC LIMIT ? OFFSET ?`,
		txCols, tableTxs, w)
	rows, err := r.db.QueryContext(ctx, q, appendInt(cargs, req.PageSize+1, req.Offset())...)
	if err != nil {
		return nil, false, 0, err
	}
	defer rows.Close()

	items, err := scanTxs(rows)
	if err != nil {
		return nil, false, 0, err
	}
	hasMore := len(items) > req.PageSize
	if hasMore {
		items = items[:req.PageSize]
	}
	return items, hasMore, total, nil
}

// ListConfirmedByAcctAsset 按时间正序返回某账户某标的全部已确认流水（重放数据源）
func (r *Repository) ListConfirmedByAcctAsset(ctx context.Context, accountID, assetID string) ([]Transaction, error) {
	q := fmt.Sprintf(`SELECT %s FROM %s WHERE account_id = ? AND asset_id = ? AND status = 'confirmed'
		ORDER BY trade_at ASC, id ASC`, txCols, tableTxs)
	rows, err := r.db.QueryContext(ctx, q, accountID, assetID)
	if err != nil {
		return nil, err
	}
	return scanTxs(rows)
}

func scanTx(row *sql.Row) (*Transaction, error) {
	var t Transaction
	var feeDetail, settleDate, method, remark sql.NullString
	var realized sql.NullFloat64
	err := row.Scan(&t.ID, &t.AccountID, &t.AssetID, &t.TradeType, &t.Shares, &t.Price, &t.Amount, &t.Fee,
		&feeDetail, &t.Tax, &t.TradeAt, &settleDate, &t.Status, &method, &realized, &remark, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	t.FeeDetail = feeDetail.String
	t.SettleDate = settleDate.String
	t.Method = method.String
	t.RealizedPnL = realized.Float64
	t.Remark = remark.String
	return &t, nil
}

func scanTxs(rows *sql.Rows) ([]Transaction, error) {
	defer rows.Close()
	var items []Transaction
	for rows.Next() {
		var t Transaction
		var feeDetail, settleDate, method, remark sql.NullString
		var realized sql.NullFloat64
		if err := rows.Scan(&t.ID, &t.AccountID, &t.AssetID, &t.TradeType, &t.Shares, &t.Price, &t.Amount, &t.Fee,
			&feeDetail, &t.Tax, &t.TradeAt, &settleDate, &t.Status, &method, &realized, &remark, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		t.FeeDetail = feeDetail.String
		t.SettleDate = settleDate.String
		t.Method = method.String
		t.RealizedPnL = realized.Float64
		t.Remark = remark.String
		items = append(items, t)
	}
	return items, rows.Err()
}

// ============================================================================
// 持仓（重放引擎写入）
// ============================================================================

// UpsertPosition 写入重放结果；保留已有 last_price 字段不被覆盖
func (r *Repository) UpsertPosition(ctx context.Context, p *Position) error {
	_, err := r.db.ExecContext(ctx, fmt.Sprintf(`INSERT INTO %s
		(account_id, asset_id, shares, available_shares, locked_shares, avg_cost, cost_amount, currency, status)
		VALUES (?,?,?,?,?,?,?,?,?)
		ON DUPLICATE KEY UPDATE shares = VALUES(shares), available_shares = VALUES(available_shares),
		locked_shares = VALUES(locked_shares), avg_cost = VALUES(avg_cost), cost_amount = VALUES(cost_amount),
		currency = VALUES(currency), status = VALUES(status)`, tablePositions),
		p.AccountID, p.AssetID, p.Shares, p.AvailableShares, p.LockedShares, p.AvgCost, p.CostAmount, p.Currency, p.Status)
	return err
}

const posCols = `p.id, p.account_id, p.asset_id, p.shares, p.available_shares, p.locked_shares,
	p.avg_cost, p.cost_amount, p.last_price, p.last_price_at, p.last_price_source,
	COALESCE(p.target_price,0), COALESCE(p.stop_loss_price,0), p.target_set_at,
	p.currency, p.status, p.created_at, p.updated_at`

// ListPositions 查询持仓（联表带出标的信息与类型过滤）
func (r *Repository) ListPositions(ctx context.Context, req *ListPositionsReq) ([]Position, error) {
	status := req.Status
	if status == "" {
		status = "holding"
	}
	conds := []string{"p.status = ?"}
	var args []any
	args = append(args, status)
	if req.AccountID != "" {
		conds = append(conds, "p.account_id = ?")
		args = append(args, req.AccountID)
	}
	if req.AssetType != "" {
		conds = append(conds, "a.asset_type = ?")
		args = append(args, req.AssetType)
	}
	q := fmt.Sprintf(`SELECT %s, a.code, a.name, a.asset_type, a.market, a.sub_type FROM %s p
		JOIN %s a ON a.id = p.asset_id WHERE %s ORDER BY p.cost_amount DESC`,
		posCols, tablePositions, tableAssets, strings.Join(conds, " AND "))
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []Position
	for rows.Next() {
		var p Position
		var lastPrice sql.NullFloat64
		var lastPriceAt sql.NullTime
		var lastSrc sql.NullString
		var targetSetAt sql.NullTime
		var asset Asset
		var subType sql.NullString
		if err := rows.Scan(&p.ID, &p.AccountID, &p.AssetID, &p.Shares, &p.AvailableShares, &p.LockedShares,
			&p.AvgCost, &p.CostAmount, &lastPrice, &lastPriceAt, &lastSrc,
			&p.TargetPrice, &p.StopLossPrice, &targetSetAt,
			&p.Currency, &p.Status,
			&p.CreatedAt, &p.UpdatedAt,
			&asset.ID, &asset.Code, &asset.Name, &asset.AssetType, &asset.Market, &subType); err != nil {
			return nil, err
		}
		p.LastPrice = lastPrice.Float64
		if lastPriceAt.Valid {
			t := lastPriceAt.Time
			p.LastPriceAt = &t
		}
		if targetSetAt.Valid {
			t := targetSetAt.Time
			p.TargetSetAt = &t
		}
		p.LastPriceSrc = lastSrc.String
		asset.SubType = subType.String
		p.Asset = &asset
		items = append(items, p)
	}
	return items, rows.Err()
}

// PositionExists 判断账户下是否存在持仓或已确认交易（删除账户前置校验）
func (r *Repository) PositionExists(ctx context.Context, accountID string) (bool, error) {
	var n int
	if err := r.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE account_id = ? AND shares <> 0`, tablePositions), accountID).Scan(&n); err != nil {
		return true, err
	}
	if n > 0 {
		return true, nil
	}
	err := r.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE account_id = ?`, tableTxs), accountID).Scan(&n)
	return n > 0, err
}

// ============================================================================
// 已实现收益台账（重放引擎整批重建）
// ============================================================================

// ReplaceRealizedPnL 重建某账户某标的的已实现收益台账（事务内先删后插）
func (r *Repository) ReplaceRealizedPnL(ctx context.Context, accountID, assetID string, lots []RealizedPnL) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx,
		fmt.Sprintf(`DELETE FROM %s WHERE account_id = ? AND asset_id = ?`, tableRealized), accountID, assetID); err != nil {
		return err
	}
	if len(lots) > 0 {
		var sb strings.Builder
		var args []any
		sb.WriteString(fmt.Sprintf(`INSERT INTO %s (account_id, asset_id, transaction_id, lot_transaction_id, method,
			shares, sell_price, cost_price, gross_pnl, fee, tax, net_pnl, realized_at) VALUES `, tableRealized))
		for i, l := range lots {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString("(?,?,?,?,?,?,?,?,?,?,?,?,?)")
			lotID := any(nil)
			if l.LotTransactionID > 0 {
				lotID = l.LotTransactionID
			}
			args = append(args, l.AccountID, l.AssetID, l.TransactionID, lotID, l.Method,
				l.Shares, l.SellPrice, l.CostPrice, l.GrossPnL, l.Fee, l.Tax, l.NetPnL, l.RealizedAt)
		}
		if _, err = tx.ExecContext(ctx, sb.String(), args...); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ============================================================================
// 资金流水
// ============================================================================

// CreateCashFlow 创建资金流水
func (r *Repository) CreateCashFlow(ctx context.Context, c *CashFlow) (*CashFlow, error) {
	res, err := r.db.ExecContext(ctx,
		fmt.Sprintf(`INSERT INTO %s (account_id, flow_type, amount, currency, flow_date, counter_account_id, remark)
			VALUES (?,?,?,?,?,?,?)`, tableCashFlows),
		c.AccountID, c.FlowType, c.Amount, c.Currency, c.FlowDate, nullStr(c.CounterAcctID), c.Remark)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	var out CashFlow
	var counterAcct, remark sql.NullString
	err = r.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT id, account_id, flow_type, amount, currency, flow_date, counter_account_id, remark, created_at
			FROM %s WHERE id = ?`, tableCashFlows), id).
		Scan(&out.ID, &out.AccountID, &out.FlowType, &out.Amount, &out.Currency, &out.FlowDate, &counterAcct, &remark, &out.CreatedAt)
	if err != nil {
		return nil, err
	}
	out.CounterAcctID = counterAcct.String
	out.Remark = remark.String
	return &out, nil
}

// DeleteCashFlow 删除资金流水
func (r *Repository) DeleteCashFlow(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s WHERE id = ?`, tableCashFlows), id)
	return err
}

// ListCashFlows 分页查询资金流水
func (r *Repository) ListCashFlows(ctx context.Context, req *ListCashFlowsReq) ([]CashFlow, bool, int, error) {
	conds := []string{"1=1"}
	var cargs []any
	if req.AccountID != "" {
		conds = append(conds, "account_id = ?")
		cargs = append(cargs, req.AccountID)
	}
	if req.FlowType != "" {
		conds = append(conds, "flow_type = ?")
		cargs = append(cargs, req.FlowType)
	}
	if req.Start != "" {
		conds = append(conds, "flow_date >= ?")
		cargs = append(cargs, req.Start)
	}
	if req.End != "" {
		conds = append(conds, "flow_date <= ?")
		cargs = append(cargs, req.End)
	}
	w := strings.Join(conds, " AND ")

	var total int
	if err := r.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE %s`, tableCashFlows, w), cargs...).Scan(&total); err != nil {
		return nil, false, 0, err
	}
	rows, err := r.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT id, account_id, flow_type, amount, currency, flow_date, counter_account_id, remark, created_at
			FROM %s WHERE %s ORDER BY flow_date DESC, id DESC LIMIT ? OFFSET ?`, tableCashFlows, w),
		appendInt(cargs, req.PageSize+1, req.Offset())...)
	if err != nil {
		return nil, false, 0, err
	}
	defer rows.Close()

	var items []CashFlow
	for rows.Next() {
		var c CashFlow
		var counterAcct, remark sql.NullString
		if err := rows.Scan(&c.ID, &c.AccountID, &c.FlowType, &c.Amount, &c.Currency, &c.FlowDate, &counterAcct, &remark, &c.CreatedAt); err != nil {
			return nil, false, 0, err
		}
		c.CounterAcctID = counterAcct.String
		c.Remark = remark.String
		items = append(items, c)
	}
	if err = rows.Err(); err != nil {
		return nil, false, 0, err
	}
	hasMore := len(items) > req.PageSize
	if hasMore {
		items = items[:req.PageSize]
	}
	return items, hasMore, total, nil
}

// CashBalanceRow 现金余额聚合行
type CashBalanceRow struct {
	AccountID string
	Currency  string
	Balance   float64 // 入金+划入-出金-划出
}

// CashBalances 全部账户现金余额（入金-出金净额）
func (r *Repository) CashBalances(ctx context.Context) ([]CashBalanceRow, error) {
	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`SELECT account_id, currency,
		SUM(CASE WHEN flow_type IN ('deposit','transfer_in') THEN amount ELSE -amount END)
		FROM %s GROUP BY account_id, currency`, tableCashFlows))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []CashBalanceRow
	for rows.Next() {
		var it CashBalanceRow
		if err := rows.Scan(&it.AccountID, &it.Currency, &it.Balance); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// ============================================================================
// 独立费用
// ============================================================================

// CreateFee 创建费用记录
func (r *Repository) CreateFee(ctx context.Context, f *FeeRecord) (*FeeRecord, error) {
	res, err := r.db.ExecContext(ctx,
		fmt.Sprintf(`INSERT INTO %s (account_id, asset_id, fee_type, amount, currency, charge_date, period, remark)
			VALUES (?,?,?,?,?,?,?,?)`, tableFees),
		f.AccountID, nullStr(f.AssetID), f.FeeType, f.Amount, f.Currency, f.ChargeDate, nullStr(f.Period), f.Remark)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	var out FeeRecord
	var assetID, period, remark sql.NullString
	err = r.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT id, account_id, asset_id, fee_type, amount, currency, charge_date, period, remark, created_at
			FROM %s WHERE id = ?`, tableFees), id).
		Scan(&out.ID, &out.AccountID, &assetID, &out.FeeType, &out.Amount, &out.Currency, &out.ChargeDate, &period, &remark, &out.CreatedAt)
	if err != nil {
		return nil, err
	}
	out.AssetID = assetID.String
	out.Period = period.String
	out.Remark = remark.String
	return &out, nil
}

// DeleteFee 删除费用记录
func (r *Repository) DeleteFee(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s WHERE id = ?`, tableFees), id)
	return err
}

// ListFees 分页查询费用记录
func (r *Repository) ListFees(ctx context.Context, req *ListFeesReq) ([]FeeRecord, bool, int, error) {
	conds := []string{"1=1"}
	var cargs []any
	if req.AccountID != "" {
		conds = append(conds, "account_id = ?")
		cargs = append(cargs, req.AccountID)
	}
	if req.AssetID != "" {
		conds = append(conds, "asset_id = ?")
		cargs = append(cargs, req.AssetID)
	}
	if req.FeeType != "" {
		conds = append(conds, "fee_type = ?")
		cargs = append(cargs, req.FeeType)
	}
	if req.Start != "" {
		conds = append(conds, "charge_date >= ?")
		cargs = append(cargs, req.Start)
	}
	if req.End != "" {
		conds = append(conds, "charge_date <= ?")
		cargs = append(cargs, req.End)
	}
	w := strings.Join(conds, " AND ")

	var total int
	if err := r.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE %s`, tableFees, w), cargs...).Scan(&total); err != nil {
		return nil, false, 0, err
	}
	rows, err := r.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT id, account_id, asset_id, fee_type, amount, currency, charge_date, period, remark, created_at
			FROM %s WHERE %s ORDER BY charge_date DESC, id DESC LIMIT ? OFFSET ?`, tableFees, w),
		appendInt(cargs, req.PageSize+1, req.Offset())...)
	if err != nil {
		return nil, false, 0, err
	}
	defer rows.Close()

	var items []FeeRecord
	for rows.Next() {
		var f FeeRecord
		var assetID, period, remark sql.NullString
		if err := rows.Scan(&f.ID, &f.AccountID, &assetID, &f.FeeType, &f.Amount, &f.Currency, &f.ChargeDate, &period, &remark, &f.CreatedAt); err != nil {
			return nil, false, 0, err
		}
		f.AssetID = assetID.String
		f.Period = period.String
		f.Remark = remark.String
		items = append(items, f)
	}
	if err = rows.Err(); err != nil {
		return nil, false, 0, err
	}
	hasMore := len(items) > req.PageSize
	if hasMore {
		items = items[:req.PageSize]
	}
	return items, hasMore, total, nil
}

// FeeSumRow 费用汇总行
type FeeSumRow struct {
	AccountID string
	Currency  string
	Total     float64
}

// FeeSums 各账户累计费用
func (r *Repository) FeeSums(ctx context.Context) ([]FeeSumRow, error) {
	rows, err := r.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT account_id, currency, SUM(amount) FROM %s GROUP BY account_id, currency`, tableFees))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []FeeSumRow
	for rows.Next() {
		var it FeeSumRow
		if err := rows.Scan(&it.AccountID, &it.Currency, &it.Total); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// ============================================================================
// 定投计划
// ============================================================================

const dcaCols = `id, name, account_id, asset_id, amount, cycle, exec_day, start_date, end_date, next_exec_date, status, auto_buy, remark, created_at, updated_at`

// CreateDCAPlan 创建定投计划
func (r *Repository) CreateDCAPlan(ctx context.Context, p *DCAPlan) (*DCAPlan, error) {
	p.ID = uuid.NewString()
	_, err := r.db.ExecContext(ctx,
		fmt.Sprintf(`INSERT INTO %s (id, name, account_id, asset_id, amount, cycle, exec_day, start_date, end_date, next_exec_date, status, auto_buy, remark)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`, tableDCA),
		p.ID, p.Name, p.AccountID, p.AssetID, p.Amount, p.Cycle, p.ExecDay,
		p.StartDate, nullStr(p.EndDate), p.NextExecDate, p.Status, p.AutoBuy, p.Remark)
	if err != nil {
		return nil, err
	}
	return r.GetDCAPlan(ctx, p.ID)
}

// GetDCAPlan 获取定投计划
func (r *Repository) GetDCAPlan(ctx context.Context, id string) (*DCAPlan, error) {
	row := r.db.QueryRowContext(ctx, fmt.Sprintf(`SELECT %s FROM %s WHERE id = ?`, dcaCols, tableDCA), id)
	return scanDCA(row)
}

// UpdateDCAPlan 更新定投计划
func (r *Repository) UpdateDCAPlan(ctx context.Context, id string, req *UpdateDCAPlanReq) (*DCAPlan, error) {
	var sets []string
	var args []any
	appendSet := func(col string, v any) { sets = append(sets, col+" = ?"); args = append(args, v) }
	if req.Name != nil {
		appendSet("name", *req.Name)
	}
	if req.Amount != nil {
		appendSet("amount", *req.Amount)
	}
	if req.Cycle != nil {
		appendSet("cycle", *req.Cycle)
	}
	if req.ExecDay != nil {
		appendSet("exec_day", *req.ExecDay)
	}
	if req.EndDate != nil {
		appendSet("end_date", nullStr(*req.EndDate))
	}
	if req.Status != nil {
		appendSet("status", *req.Status)
	}
	if req.AutoBuy != nil {
		appendSet("auto_buy", *req.AutoBuy)
	}
	if req.Remark != nil {
		appendSet("remark", *req.Remark)
	}
	if len(sets) > 0 {
		args = append(args, id)
		if _, err := r.db.ExecContext(ctx,
			fmt.Sprintf(`UPDATE %s SET %s WHERE id = ?`, tableDCA, strings.Join(sets, ", ")), args...); err != nil {
			return nil, err
		}
	}
	return r.GetDCAPlan(ctx, id)
}

// UpdateDCANextExec 更新定投计划下次执行日期
func (r *Repository) UpdateDCANextExec(ctx context.Context, id, date string) error {
	_, err := r.db.ExecContext(ctx,
		fmt.Sprintf(`UPDATE %s SET next_exec_date = ? WHERE id = ?`, tableDCA), date, id)
	return err
}

// DeleteDCAPlan 删除定投计划
func (r *Repository) DeleteDCAPlan(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s WHERE id = ?`, tableDCA), id)
	return err
}

// ListDCAPlans 分页查询定投计划
func (r *Repository) ListDCAPlans(ctx context.Context, req *ListDCAPlansReq) ([]DCAPlan, bool, int, error) {
	conds := []string{"1=1"}
	var cargs []any
	if req.AccountID != "" {
		conds = append(conds, "account_id = ?")
		cargs = append(cargs, req.AccountID)
	}
	if req.AssetID != "" {
		conds = append(conds, "asset_id = ?")
		cargs = append(cargs, req.AssetID)
	}
	if req.Status != "" {
		conds = append(conds, "status = ?")
		cargs = append(cargs, req.Status)
	}
	w := strings.Join(conds, " AND ")

	var total int
	if err := r.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE %s`, tableDCA, w), cargs...).Scan(&total); err != nil {
		return nil, false, 0, err
	}
	rows, err := r.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT %s FROM %s WHERE %s ORDER BY created_at DESC LIMIT ? OFFSET ?`, dcaCols, tableDCA, w),
		appendInt(cargs, req.PageSize+1, req.Offset())...)
	if err != nil {
		return nil, false, 0, err
	}
	defer rows.Close()

	var items []DCAPlan
	for rows.Next() {
		var p DCAPlan
		var endDate, remark sql.NullString
		var autoBuy int
		if err := rows.Scan(&p.ID, &p.Name, &p.AccountID, &p.AssetID, &p.Amount, &p.Cycle, &p.ExecDay,
			&p.StartDate, &endDate, &p.NextExecDate, &p.Status, &autoBuy, &remark, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, false, 0, err
		}
		p.EndDate = endDate.String
		p.AutoBuy = autoBuy == 1
		p.Remark = remark.String
		items = append(items, p)
	}
	if err = rows.Err(); err != nil {
		return nil, false, 0, err
	}
	hasMore := len(items) > req.PageSize
	if hasMore {
		items = items[:req.PageSize]
	}
	return items, hasMore, total, nil
}

func scanDCA(row *sql.Row) (*DCAPlan, error) {
	var p DCAPlan
	var endDate, remark sql.NullString
	var autoBuy int
	err := row.Scan(&p.ID, &p.Name, &p.AccountID, &p.AssetID, &p.Amount, &p.Cycle, &p.ExecDay,
		&p.StartDate, &endDate, &p.NextExecDate, &p.Status, &autoBuy, &remark, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	p.EndDate = endDate.String
	p.AutoBuy = autoBuy == 1
	p.Remark = remark.String
	return &p, nil
}

func scanAlloc(row *sql.Row) (*AllocationItem, error) {
	var a AllocationItem
	var acctID, remark sql.NullString
	err := row.Scan(&a.ID, &acctID, &a.Category, &a.TargetRatio, &a.Tolerance, &a.Status, &remark, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	a.AccountID = acctID.String
	a.Remark = remark.String
	return &a, nil
}

// ============================================================================
// 汇率
// ============================================================================

// UpsertFxRate 录入/更新汇率（同币对同日期覆盖）
func (r *Repository) UpsertFxRate(ctx context.Context, f *FxRate) error {
	_, err := r.db.ExecContext(ctx, fmt.Sprintf(`INSERT INTO %s (base_ccy, quote_ccy, rate, rate_date, source)
		VALUES (?,?,?,?,?)
		ON DUPLICATE KEY UPDATE rate = VALUES(rate), source = VALUES(source)`, tableFx),
		f.BaseCcy, f.QuoteCcy, f.Rate, f.RateDate, f.Source)
	return err
}

// ListFxRates 查询汇率记录
func (r *Repository) ListFxRates(ctx context.Context, req *ListFxRatesReq) ([]FxRate, error) {
	conds := []string{"1=1"}
	var args []any
	if req.BaseCcy != "" {
		conds = append(conds, "base_ccy = ?")
		args = append(args, req.BaseCcy)
	}
	if req.QuoteCcy != "" {
		conds = append(conds, "quote_ccy = ?")
		args = append(args, req.QuoteCcy)
	}
	if req.Start != "" {
		conds = append(conds, "rate_date >= ?")
		args = append(args, req.Start)
	}
	if req.End != "" {
		conds = append(conds, "rate_date <= ?")
		args = append(args, req.End)
	}
	rows, err := r.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT id, base_ccy, quote_ccy, rate, rate_date, source FROM %s WHERE %s ORDER BY rate_date DESC, id DESC LIMIT 500`,
			tableFx, strings.Join(conds, " AND ")), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []FxRate
	for rows.Next() {
		var f FxRate
		if err := rows.Scan(&f.ID, &f.BaseCcy, &f.QuoteCcy, &f.Rate, &f.RateDate, &f.Source); err != nil {
			return nil, err
		}
		items = append(items, f)
	}
	return items, rows.Err()
}

// LatestFxRates 每个币对的最新汇率：map["USD->CNY"] = rate
func (r *Repository) LatestFxRates(ctx context.Context) (map[string]float64, error) {
	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`SELECT f.base_ccy, f.quote_ccy, f.rate FROM %s f
		JOIN (SELECT base_ccy, quote_ccy, MAX(CONCAT(rate_date, LPAD(id, 10, '0'))) AS mk FROM %s GROUP BY base_ccy, quote_ccy) m
		ON f.base_ccy = m.base_ccy AND f.quote_ccy = m.quote_ccy AND CONCAT(f.rate_date, LPAD(f.id, 10, '0')) = m.mk`,
		tableFx, tableFx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]float64)
	for rows.Next() {
		var base, quote string
		var rate float64
		if err := rows.Scan(&base, &quote, &rate); err != nil {
			return nil, err
		}
		out[base+"->"+quote] = rate
	}
	return out, rows.Err()
}

// ============================================================================
// 资产配比目标
// ============================================================================

const allocCols = `id, account_id, category, target_ratio, tolerance, status, remark, created_at, updated_at`

// CreateAllocation 创建配置目标项
func (r *Repository) CreateAllocation(ctx context.Context, a *AllocationItem) (*AllocationItem, error) {
	a.ID = uuid.NewString()
	_, err := r.db.ExecContext(ctx,
		fmt.Sprintf(`INSERT INTO %s (id, account_id, category, target_ratio, tolerance, status, remark)
			VALUES (?,?,?,?,?, 'active', ?)`, tableAlloc),
		a.ID, nullStr(a.AccountID), a.Category, a.TargetRatio, a.Tolerance, a.Remark)
	if err != nil {
		return nil, err
	}
	return r.GetAllocation(ctx, a.ID)
}

// GetAllocation 获取配置目标项
func (r *Repository) GetAllocation(ctx context.Context, id string) (*AllocationItem, error) {
	row := r.db.QueryRowContext(ctx, fmt.Sprintf(`SELECT %s FROM %s WHERE id = ?`, allocCols, tableAlloc), id)
	return scanAlloc(row)
}

// UpdateAllocation 更新配置目标项
func (r *Repository) UpdateAllocation(ctx context.Context, id string, req *UpdateAllocationReq) (*AllocationItem, error) {
	var sets []string
	var args []any
	if req.TargetRatio != nil {
		sets = append(sets, "target_ratio = ?")
		args = append(args, *req.TargetRatio)
	}
	if req.Tolerance != nil {
		sets = append(sets, "tolerance = ?")
		args = append(args, *req.Tolerance)
	}
	if req.Status != nil {
		sets = append(sets, "status = ?")
		args = append(args, *req.Status)
	}
	if req.Remark != nil {
		sets = append(sets, "remark = ?")
		args = append(args, *req.Remark)
	}
	if len(sets) > 0 {
		args = append(args, id)
		if _, err := r.db.ExecContext(ctx,
			fmt.Sprintf(`UPDATE %s SET %s WHERE id = ?`, tableAlloc, strings.Join(sets, ", ")), args...); err != nil {
			return nil, err
		}
	}
	return r.GetAllocation(ctx, id)
}

// DeleteAllocation 删除配置目标项
func (r *Repository) DeleteAllocation(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s WHERE id = ?`, tableAlloc), id)
	return err
}

// ListAllocations 查询配置目标列表
func (r *Repository) ListAllocations(ctx context.Context, req *ListAllocationsReq) ([]AllocationItem, error) {
	conds := []string{scopeCond(req.AccountID)}
	var args []any
	if req.AccountID != "" {
		args = append(args, req.AccountID)
	}
	if req.Status != "" {
		conds = append(conds, "status = ?")
		args = append(args, req.Status)
	}
	rows, err := r.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT %s FROM %s WHERE %s ORDER BY category`, allocCols, tableAlloc, strings.Join(conds, " AND ")), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []AllocationItem
	for rows.Next() {
		var a AllocationItem
		var acctID, remark sql.NullString
		if err := rows.Scan(&a.ID, &acctID, &a.Category, &a.TargetRatio, &a.Tolerance, &a.Status, &remark, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		a.AccountID = acctID.String
		a.Remark = remark.String
		items = append(items, a)
	}
	return items, rows.Err()
}

// scopeCond 配置范围条件：指定账户或全局（account_id IS NULL）
func scopeCond(accountID string) string {
	if accountID == "" {
		return "account_id IS NULL"
	}
	return "account_id = ?"
}

// SumActiveRatios 同 scope 下活跃目标比例之和（校验不超 100%）
func (r *Repository) SumActiveRatios(ctx context.Context, accountID, excludeCategory string) (float64, string, error) {
	conds := []string{scopeCond(accountID), "status = 'active'"}
	var args []any
	if accountID != "" {
		args = append(args, accountID)
	}
	if excludeCategory != "" {
		conds = append(conds, "category <> ?")
		args = append(args, excludeCategory)
	}
	var sum float64
	var existingCategory string
	err := r.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT COALESCE(SUM(target_ratio),0), COALESCE(MAX(category),'') FROM %s WHERE %s`,
			tableAlloc, strings.Join(conds, " AND ")), args...).Scan(&sum, &existingCategory)
	return sum, existingCategory, err
}

// ============================================================================
// 收益聚合查询（总览用）
// ============================================================================

// IncomeAggRow 交易收益聚合行：按账户+币种统计分红税后、税款、利息
type IncomeAggRow struct {
	AccountID      string
	Currency       string
	DividendNet    float64 // 现金分红税后净额
	TaxPaid        float64 // 红利税合计
	InterestIncome float64 // 利息合计
}

// IncomeAggs 交易收益聚合（仅 confirmed）
func (r *Repository) IncomeAggs(ctx context.Context) ([]IncomeAggRow, error) {
	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT t.account_id, a.currency,
			SUM(CASE WHEN t.trade_type = 'dividend_cash' THEN GREATEST(t.amount - t.tax, 0) ELSE 0 END) AS dividend_net,
			SUM(t.tax) AS tax_paid,
			SUM(CASE WHEN t.trade_type = 'interest' THEN t.amount ELSE 0 END) AS interest
		FROM %s t JOIN %s a ON a.id = t.asset_id
		WHERE t.status = 'confirmed'
		GROUP BY t.account_id, a.currency`, tableTxs, tableAssets))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []IncomeAggRow
	for rows.Next() {
		var it IncomeAggRow
		if err := rows.Scan(&it.AccountID, &it.Currency, &it.DividendNet, &it.TaxPaid, &it.InterestIncome); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// RealizedAggRow 已实现收益聚合行
type RealizedAggRow struct {
	AccountID string
	Currency  string
	NetPnL    float64
}

// RealizedAggs 各账户已实现收益合计
func (r *Repository) RealizedAggs(ctx context.Context) ([]RealizedAggRow, error) {
	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT rp.account_id, a.currency, SUM(rp.net_pnl)
		FROM %s rp JOIN %s a ON a.id = rp.asset_id
		GROUP BY rp.account_id, a.currency`, tableRealized, tableAssets))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []RealizedAggRow
	for rows.Next() {
		var it RealizedAggRow
		if err := rows.Scan(&it.AccountID, &it.Currency, &it.NetPnL); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// ============================================================================
// 行情与净值历史
// ============================================================================

// PriceHistoryRow 价格/净值历史行
type PriceHistoryRow struct {
	AssetID string  `json:"asset_id"`
	NavDate string  `json:"nav_date"`
	Nav     float64 `json:"nav"`
	Source  string  `json:"source"`
}

// QuotableAsset 需要拉取行情的标的摘要
type QuotableAsset struct {
	ID        string `db:"id"`
	Code      string `db:"code"`
	Market    string `db:"market"`
	Currency  string `db:"currency"`
	AssetType string `db:"asset_type"`
	SubType   string `db:"sub_type"`
}

// ListQuotableAssets 查询有持仓的股票/ETF/基金标的（去重）
func (r *Repository) ListQuotableAssets(ctx context.Context) ([]QuotableAsset, error) {
	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`SELECT DISTINCT a.id, a.code, a.market, a.currency, a.asset_type, COALESCE(a.sub_type,'')
		FROM %s a JOIN %s p ON p.asset_id = a.id AND p.status = 'holding'
		WHERE a.asset_type IN ('stock','etf','fund') AND a.status = 'active'`, tableAssets, tablePositions))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []QuotableAsset
	for rows.Next() {
		var it QuotableAsset
		if err := rows.Scan(&it.ID, &it.Code, &it.Market, &it.Currency, &it.AssetType, &it.SubType); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// UpsertPrice 写入价格/净值历史（同标的同日覆盖）
func (r *Repository) UpsertPrice(ctx context.Context, assetID, navDate string, nav float64, source string) error {
	_, err := r.db.ExecContext(ctx, fmt.Sprintf(`INSERT INTO %s (asset_id, nav_date, nav, source)
		VALUES (?,?,?,?) ON DUPLICATE KEY UPDATE nav = VALUES(nav), source = VALUES(source)`, tablePrices),
		assetID, navDate, nav, source)
	return err
}

// ListPriceHistory 查询标的价格历史（默认最近 500 条）
func (r *Repository) ListPriceHistory(ctx context.Context, assetID string, limit int) ([]PriceHistoryRow, error) {
	if limit <= 0 || limit > 1000 {
		limit = 500
	}
	rows, err := r.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT asset_id, nav_date, nav, source FROM %s WHERE asset_id = ? ORDER BY nav_date DESC LIMIT ?`, tablePrices),
		assetID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []PriceHistoryRow
	for rows.Next() {
		var it PriceHistoryRow
		if err := rows.Scan(&it.AssetID, &it.NavDate, &it.Nav, &it.Source); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// UpdatePositionPriceByAsset 按标的批量刷新持仓最新价（跨账户）
func (r *Repository) UpdatePositionPriceByAsset(ctx context.Context, assetID string, price float64, source string) error {
	_, err := r.db.ExecContext(ctx, fmt.Sprintf(`UPDATE %s SET last_price = ?, last_price_source = ?, last_price_at = NOW()
		WHERE asset_id = ? AND status = 'holding'`, tablePositions), price, source, assetID)
	return err
}

// SetPositionTarget 设置/清除持仓追踪目标与止损参考价（重放引擎不覆盖这两个字段）
func (r *Repository) SetPositionTarget(ctx context.Context, accountID, assetID string, targetPrice, stopLossPrice float64) error {
	target := any(nil)
	stop := any(nil)
	if targetPrice > 0 {
		target = targetPrice
	}
	if stopLossPrice > 0 {
		stop = stopLossPrice
	}
	setAt := any(nil)
	if targetPrice > 0 || stopLossPrice > 0 {
		setAt = time.Now()
	}
	res, err := r.db.ExecContext(ctx, fmt.Sprintf(`UPDATE %s
		SET target_price = ?, stop_loss_price = ?, target_set_at = ?
		WHERE account_id = ? AND asset_id = ? AND status = 'holding'`, tablePositions),
		target, stop, setAt, accountID, assetID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return NewIAError(ErrPositionNotFound)
	}
	return nil
}

// ============================================================================
// 自动计息
// ============================================================================

// InterestAsset 计息资产与持仓摘要
type InterestAsset struct {
	AccountID    string  `db:"account_id"`
	AssetID      string  `db:"asset_id"`
	Code         string  `db:"code"`
	Name         string  `db:"name"`
	AnnualRate   float64 `db:"annual_rate"`
	StartDate    string  `db:"start_date"`
	MaturityDate string  `db:"maturity_date"`
	CostAmount   float64 `db:"cost_amount"`
	Shares       float64 `db:"shares"`
}

// ListInterestAssets 查询需自动计息的存款/理财持仓（约定利率>0、已起息、未到期）
func (r *Repository) ListInterestAssets(ctx context.Context, today string) ([]InterestAsset, error) {
	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`SELECT p.account_id, a.id, a.code, a.name,
			a.annual_rate, COALESCE(a.start_date,''), COALESCE(a.maturity_date,''), p.cost_amount, p.shares
		FROM %s a JOIN %s p ON p.asset_id = a.id AND p.status = 'holding' AND p.cost_amount > 0
		WHERE a.asset_type IN ('deposit','wealth') AND a.annual_rate > 0 AND a.status = 'active'
		AND a.start_date IS NOT NULL AND a.start_date <= ?
		AND (a.maturity_date IS NULL OR a.maturity_date >= ?)`, tableAssets, tablePositions), today, today)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []InterestAsset
	for rows.Next() {
		var it InterestAsset
		if err := rows.Scan(&it.AccountID, &it.AssetID, &it.Code, &it.Name, &it.AnnualRate,
			&it.StartDate, &it.MaturityDate, &it.CostAmount, &it.Shares); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// InterestAutoExistsToday 判断某持仓当日是否已生成过自动计息流水（幂等防重）
func (r *Repository) InterestAutoExistsToday(ctx context.Context, accountID, assetID, date string) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s
		WHERE account_id = ? AND asset_id = ? AND trade_type = 'interest' AND remark = 'auto-accrual'
		AND trade_at >= ? AND trade_at < DATE_ADD(?, INTERVAL 1 DAY)`, tableTxs),
		accountID, assetID, date+" 00:00:00", date).Scan(&n)
	return n > 0, err
}

// SumAssetInterest 汇总某持仓累计利息（含手工与自动）
func (r *Repository) SumAssetInterest(ctx context.Context, accountID, assetID string) (float64, error) {
	var sum sql.NullFloat64
	err := r.db.QueryRowContext(ctx, fmt.Sprintf(`SELECT SUM(amount) FROM %s
		WHERE account_id = ? AND asset_id = ? AND trade_type = 'interest' AND status = 'confirmed'`,
		tableTxs), accountID, assetID).Scan(&sum)
	return sum.Float64, err
}

// ============================================================================
// 组合快照
// ============================================================================

// SnapshotUpsert 写入每日组合快照（同日同 scope 覆盖）
func (r *Repository) SnapshotUpsert(ctx context.Context, s *PortfolioSnapshot) error {
	acctID := any(nil)
	if s.AccountID != "" {
		acctID = s.AccountID
	}
	_, err := r.db.ExecContext(ctx, fmt.Sprintf(`INSERT INTO %s
		(snap_date, account_id, total_cost, total_value, total_pnl, cash_amount, by_type, by_currency)
		VALUES (?,?,?,?,?,?,?,?)
		ON DUPLICATE KEY UPDATE total_cost = VALUES(total_cost), total_value = VALUES(total_value),
		total_pnl = VALUES(total_pnl), cash_amount = VALUES(cash_amount),
		by_type = VALUES(by_type), by_currency = VALUES(by_currency)`, tableSnaps),
		s.SnapDate, acctID, s.TotalCost, s.TotalValue, s.TotalPnL, s.CashAmount,
		nullStr(s.ByType), nullStr(s.ByCurrency))
	return err
}

// ============================================================================
// 到期检测
// ============================================================================

// MaturingAsset 即将到期资产行
type MaturingAsset struct {
	AccountID    string `db:"account_id"`
	AssetID      string `db:"asset_id"`
	Code         string `db:"code"`
	Name         string `db:"name"`
	AssetType    string `db:"asset_type"`
	MaturityDate string `db:"maturity_date"`
	DaysLeft     int    `db:"days_left"`
}

// ListMaturingAssets 查询 N 天内到期的存款/理财（有持仓）
func (r *Repository) ListMaturingAssets(ctx context.Context, withinDays int) ([]MaturingAsset, error) {
	if withinDays <= 0 {
		withinDays = 7
	}
	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`SELECT p.account_id, a.id, a.code, a.name, a.asset_type,
			DATE_FORMAT(a.maturity_date, '%%Y-%%m-%%d'), DATEDIFF(a.maturity_date, CURDATE())
		FROM %s a JOIN %s p ON p.asset_id = a.id AND p.status = 'holding'
		WHERE a.status = 'active' AND a.maturity_date IS NOT NULL
		AND a.maturity_date BETWEEN CURDATE() AND DATE_ADD(CURDATE(), INTERVAL ? DAY)
		ORDER BY a.maturity_date`, tableAssets, tablePositions), withinDays)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []MaturingAsset
	for rows.Next() {
		var it MaturingAsset
		if err := rows.Scan(&it.AccountID, &it.AssetID, &it.Code, &it.Name, &it.AssetType,
			&it.MaturityDate, &it.DaysLeft); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// ============================================================================
// 分析报表聚合查询
// ============================================================================

// ListSnapshotsRange 查询快照序列（accountID 为空 = 全局行，升序）
func (r *Repository) ListSnapshotsRange(ctx context.Context, accountID, start, end string) ([]PortfolioSnapshot, error) {
	conds := []string{scopeCond(accountID)}
	var args []any
	if accountID != "" {
		args = append(args, accountID)
	}
	if start != "" {
		conds = append(conds, "snap_date >= ?")
		args = append(args, start)
	}
	if end != "" {
		conds = append(conds, "snap_date <= ?")
		args = append(args, end)
	}
	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`SELECT snap_date, account_id, total_cost, total_value, total_pnl, cash_amount
		FROM %s WHERE %s ORDER BY snap_date ASC`, tableSnaps, strings.Join(conds, " AND ")), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []PortfolioSnapshot
	for rows.Next() {
		var s PortfolioSnapshot
		var acctID sql.NullString
		if err := rows.Scan(&s.SnapDate, &acctID, &s.TotalCost, &s.TotalValue, &s.TotalPnL, &s.CashAmount); err != nil {
			return nil, err
		}
		s.AccountID = acctID.String
		items = append(items, s)
	}
	return items, rows.Err()
}

// LatestSnapshotBefore 取某日之前（含）最近一份快照，无则返回 nil
func (r *Repository) LatestSnapshotBefore(ctx context.Context, accountID, date string) (*PortfolioSnapshot, error) {
	var row *sql.Row
	if accountID != "" {
		row = r.db.QueryRowContext(ctx,
			fmt.Sprintf(`SELECT snap_date, COALESCE(total_value,0), COALESCE(total_pnl,0) FROM %s
				WHERE account_id = ? AND snap_date <= ? ORDER BY snap_date DESC LIMIT 1`, tableSnaps),
			accountID, date)
	} else {
		row = r.db.QueryRowContext(ctx,
			fmt.Sprintf(`SELECT snap_date, COALESCE(total_value,0), COALESCE(total_pnl,0) FROM %s
				WHERE account_id IS NULL AND snap_date <= ? ORDER BY snap_date DESC LIMIT 1`, tableSnaps),
			date)
	}
	var s PortfolioSnapshot
	var val, pnl float64
	if err := row.Scan(&s.SnapDate, &val, &pnl); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	s.AccountID = accountID
	s.TotalValue, s.TotalPnL = val, pnl
	return &s, nil
}

// CashFlowDateRow 资金流原始行（收益率计算用，含币种便于折算）
type CashFlowDateRow struct {
	Date     string
	Currency string
	FlowType string
	Amount   float64
}

// ListCashFlowsRange 查询区间内全部资金流水（不分页，升序）
func (r *Repository) ListCashFlowsRange(ctx context.Context, accountID, start, end string) ([]CashFlowDateRow, error) {
	conds := []string{"1=1"}
	var args []any
	if accountID != "" {
		conds = append(conds, "account_id = ?")
		args = append(args, accountID)
	}
	if start != "" {
		conds = append(conds, "flow_date >= ?")
		args = append(args, start)
	}
	if end != "" {
		conds = append(conds, "flow_date <= ?")
		args = append(args, end)
	}
	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`SELECT flow_date, currency, flow_type, amount
		FROM %s WHERE %s ORDER BY flow_date ASC`, tableCashFlows, strings.Join(conds, " AND ")), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []CashFlowDateRow
	for rows.Next() {
		var it CashFlowDateRow
		if err := rows.Scan(&it.Date, &it.Currency, &it.FlowType, &it.Amount); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// RealizedLotsByAsset 读取某持仓已实现收益台账（按写入顺序）
func (r *Repository) RealizedLotsByAsset(ctx context.Context, accountID, assetID string) ([]RealizedPnL, error) {
	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`SELECT id, account_id, asset_id, transaction_id,
		COALESCE(lot_transaction_id,0), method, shares, sell_price, cost_price, gross_pnl, fee, tax, net_pnl,
		DATE_FORMAT(realized_at, '%%Y-%%m-%%d')
		FROM %s WHERE account_id = ? AND asset_id = ? ORDER BY id ASC`, tableRealized), accountID, assetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []RealizedPnL
	for rows.Next() {
		var l RealizedPnL
		if err := rows.Scan(&l.ID, &l.AccountID, &l.AssetID, &l.TransactionID, &l.LotTransactionID, &l.Method,
			&l.Shares, &l.SellPrice, &l.CostPrice, &l.GrossPnL, &l.Fee, &l.Tax, &l.NetPnL, &l.RealizedAt); err != nil {
			return nil, err
		}
		items = append(items, l)
	}
	return items, rows.Err()
}

// ReviewAssetRow 复盘标的行（有买卖记录的账户+标的组合）
type ReviewAssetRow struct {
	AccountID string
	AssetID   string
	Code      string
	Name      string
	AssetType string
	Currency  string
}

// ListReviewedAssets 有已确认买卖流水的标的清单
func (r *Repository) ListReviewedAssets(ctx context.Context, accountID string) ([]ReviewAssetRow, error) {
	conds := []string{"t.status = 'confirmed'", "t.trade_type IN ('buy','sell','rights_issue','dividend_reinvest')"}
	var args []any
	if accountID != "" {
		conds = append(conds, "t.account_id = ?")
		args = append(args, accountID)
	}
	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`SELECT DISTINCT t.account_id, a.id, a.code, a.name, a.asset_type, a.currency
		FROM %s t JOIN %s a ON a.id = t.asset_id WHERE %s`, tableTxs, tableAssets, strings.Join(conds, " AND ")), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []ReviewAssetRow
	for rows.Next() {
		var it ReviewAssetRow
		if err := rows.Scan(&it.AccountID, &it.AssetID, &it.Code, &it.Name, &it.AssetType, &it.Currency); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// TradeFeeYearRow 交易费用年度聚合行
type TradeFeeYearRow struct {
	Year int     `db:"year"`
	Fee  float64 `db:"fee"`
	Tax  float64 `db:"tax"`
}

// TradeFeeSums 交易费税按年度汇总（仅 confirmed）
func (r *Repository) TradeFeeSums(ctx context.Context, accountID, start, end string) ([]TradeFeeYearRow, error) {
	conds := []string{"status = 'confirmed'"}
	var args []any
	if accountID != "" {
		conds = append(conds, "account_id = ?")
		args = append(args, accountID)
	}
	if start != "" {
		conds = append(conds, "trade_at >= ?")
		args = append(args, start+" 00:00:00")
	}
	if end != "" {
		conds = append(conds, "trade_at <= ?")
		args = append(args, end+" 23:59:59")
	}
	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`SELECT YEAR(trade_at), SUM(fee), SUM(tax) FROM %s
		WHERE %s GROUP BY YEAR(trade_at) ORDER BY YEAR(trade_at)`, tableTxs, strings.Join(conds, " AND ")), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []TradeFeeYearRow
	for rows.Next() {
		var it TradeFeeYearRow
		if err := rows.Scan(&it.Year, &it.Fee, &it.Tax); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// StandaloneFeeAggRow 独立费用聚合行（类型+年度）
type StandaloneFeeAggRow struct {
	FeeType string  `db:"fee_type"`
	Year    int     `db:"year"`
	Total   float64 `db:"total"`
}

// StandaloneFeeAggs 独立费用按类型与年度汇总
func (r *Repository) StandaloneFeeAggs(ctx context.Context, accountID, start, end string) ([]StandaloneFeeAggRow, error) {
	conds := []string{"1=1"}
	var args []any
	if accountID != "" {
		conds = append(conds, "account_id = ?")
		args = append(args, accountID)
	}
	if start != "" {
		conds = append(conds, "charge_date >= ?")
		args = append(args, start)
	}
	if end != "" {
		conds = append(conds, "charge_date <= ?")
		args = append(args, end)
	}
	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`SELECT fee_type, YEAR(charge_date), SUM(amount) FROM %s
		WHERE %s GROUP BY fee_type, YEAR(charge_date) ORDER BY YEAR(charge_date), fee_type`,
		tableFees, strings.Join(conds, " AND ")), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []StandaloneFeeAggRow
	for rows.Next() {
		var it StandaloneFeeAggRow
		if err := rows.Scan(&it.FeeType, &it.Year, &it.Total); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// DividendAssetAggRow 分红聚合行（标的+年度，税前/税/净额与笔数）
type DividendAssetAggRow struct {
	AccountID string  `db:"account_id"`
	AssetID   string  `db:"asset_id"`
	Code      string  `db:"code"`
	Name      string  `db:"name"`
	Currency  string  `db:"currency"`
	Year      int     `db:"year"`
	Gross     float64 `db:"gross"`
	Tax       float64 `db:"tax"`
	Net       float64 `db:"net"`
	Count     int     `db:"count"`
}

// DividendAggs 现金分红按标的+年度汇总
func (r *Repository) DividendAggs(ctx context.Context, accountID, start, end string) ([]DividendAssetAggRow, error) {
	conds := []string{"t.status = 'confirmed'", "t.trade_type = 'dividend_cash'"}
	var args []any
	if accountID != "" {
		conds = append(conds, "t.account_id = ?")
		args = append(args, accountID)
	}
	if start != "" {
		conds = append(conds, "t.trade_at >= ?")
		args = append(args, start+" 00:00:00")
	}
	if end != "" {
		conds = append(conds, "t.trade_at <= ?")
		args = append(args, end+" 23:59:59")
	}
	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`SELECT t.account_id, a.id, a.code, a.name, a.currency,
		YEAR(t.trade_at), SUM(t.amount), SUM(t.tax), SUM(GREATEST(t.amount - t.tax, 0)), COUNT(*)
		FROM %s t JOIN %s a ON a.id = t.asset_id WHERE %s
		GROUP BY t.account_id, a.id, a.code, a.name, a.currency, YEAR(t.trade_at)
		ORDER BY a.code, YEAR(t.trade_at)`, tableTxs, tableAssets, strings.Join(conds, " AND ")), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []DividendAssetAggRow
	for rows.Next() {
		var it DividendAssetAggRow
		if err := rows.Scan(&it.AccountID, &it.AssetID, &it.Code, &it.Name, &it.Currency,
			&it.Year, &it.Gross, &it.Tax, &it.Net, &it.Count); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// ReinvestCount 统计红利再投笔数
func (r *Repository) ReinvestCount(ctx context.Context, accountID, start, end string) (int, error) {
	conds := []string{"status = 'confirmed'", "trade_type = 'dividend_reinvest'"}
	var args []any
	if accountID != "" {
		conds = append(conds, "account_id = ?")
		args = append(args, accountID)
	}
	if start != "" {
		conds = append(conds, "trade_at >= ?")
		args = append(args, start+" 00:00:00")
	}
	if end != "" {
		conds = append(conds, "trade_at <= ?")
		args = append(args, end+" 23:59:59")
	}
	var n int
	err := r.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE %s`, tableTxs, strings.Join(conds, " AND ")), args...).Scan(&n)
	return n, err
}

// ============================================================================
// 价格告警
// ============================================================================

const alertCols = `id, account_id, asset_id, alert_type, target_price, status, triggered_at, triggered_price, note, created_at, updated_at`

// CreateAlert 创建价格告警规则
func (r *Repository) CreateAlert(ctx context.Context, a *PriceAlert) (*PriceAlert, error) {
	a.ID = uuid.NewString()
	_, err := r.db.ExecContext(ctx,
		fmt.Sprintf(`INSERT INTO %s (id, account_id, asset_id, alert_type, target_price, status, note)
			VALUES (?,?,?,?,?, 'active', ?)`, tableAlerts),
		a.ID, a.AccountID, a.AssetID, a.AlertType, a.TargetPrice, a.Note)
	if err != nil {
		return nil, err
	}
	return r.GetAlert(ctx, a.ID)
}

// GetAlert 获取告警规则
func (r *Repository) GetAlert(ctx context.Context, id string) (*PriceAlert, error) {
	row := r.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT %s FROM %s WHERE id = ?`, alertCols, tableAlerts), id)
	return scanAlert(row)
}

// UpdateAlert 更新告警规则（部分更新）
func (r *Repository) UpdateAlert(ctx context.Context, id string, req *UpdateAlertReq) (*PriceAlert, error) {
	var sets []string
	var args []any
	if req.TargetPrice != nil {
		sets = append(sets, "target_price = ?")
		args = append(args, *req.TargetPrice)
	}
	if req.Status != nil {
		sets = append(sets, "status = ?")
		args = append(args, *req.Status)
		if *req.Status != string(AlertTriggered) {
			// 离开触发态时清空触发痕迹
			sets = append(sets, "triggered_at = NULL, triggered_price = NULL")
		}
	}
	if req.Note != nil {
		sets = append(sets, "note = ?")
		args = append(args, *req.Note)
	}
	if len(sets) > 0 {
		args = append(args, id)
		if _, err := r.db.ExecContext(ctx,
			fmt.Sprintf(`UPDATE %s SET %s WHERE id = ?`, tableAlerts, strings.Join(sets, ", ")), args...); err != nil {
			return nil, err
		}
	}
	return r.GetAlert(ctx, id)
}

// DeleteAlert 删除告警规则
func (r *Repository) DeleteAlert(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s WHERE id = ?`, tableAlerts), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return NewIAError(ErrAlertNotFound)
	}
	return nil
}

// ListAlerts 分页查询告警规则（联表带出标的）
func (r *Repository) ListAlerts(ctx context.Context, req *ListAlertsReq) ([]PriceAlert, bool, int, error) {
	conds := []string{"1=1"}
	var cargs []any
	if req.AccountID != "" {
		conds = append(conds, "a.account_id = ?")
		cargs = append(cargs, req.AccountID)
	}
	if req.AssetID != "" {
		conds = append(conds, "a.asset_id = ?")
		cargs = append(cargs, req.AssetID)
	}
	if req.Status != "" {
		conds = append(conds, "a.status = ?")
		cargs = append(cargs, req.Status)
	}
	w := strings.Join(conds, " AND ")

	var total int
	if err := r.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM %s a WHERE %s`, tableAlerts, w), cargs...).Scan(&total); err != nil {
		return nil, false, 0, err
	}
	rows, err := r.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT %s, s.code, s.name FROM %s a JOIN %s s ON s.id = a.asset_id WHERE %s
			ORDER BY a.created_at DESC LIMIT ? OFFSET ?`,
			alertCols, tableAlerts, tableAssets, w),
		appendInt(cargs, req.PageSize+1, req.Offset())...)
	if err != nil {
		return nil, false, 0, err
	}
	defer rows.Close()

	var items []PriceAlert
	for rows.Next() {
		var a PriceAlert
		var triggeredAt sql.NullTime
		var triggeredPrice sql.NullFloat64
		var note sql.NullString
		var asset Asset
		if err := rows.Scan(&a.ID, &a.AccountID, &a.AssetID, &a.AlertType, &a.TargetPrice, &a.Status,
			&triggeredAt, &triggeredPrice, &note, &a.CreatedAt, &a.UpdatedAt,
			&asset.ID, &asset.Code, &asset.Name); err != nil {
			return nil, false, 0, err
		}
		if triggeredAt.Valid {
			t := triggeredAt.Time
			a.TriggeredAt = &t
		}
		a.TriggeredPrice = triggeredPrice.Float64
		a.Note = note.String
		asset.SubType = ""
		a.Asset = &asset
		items = append(items, a)
	}
	if err = rows.Err(); err != nil {
		return nil, false, 0, err
	}
	hasMore := len(items) > req.PageSize
	if hasMore {
		items = items[:req.PageSize]
	}
	return items, hasMore, total, nil
}

// ActiveAlertsWithAsset 查询全部 active 告警及其标的信息与最新价
type ActiveAlertRow struct {
	Alert     PriceAlert
	Code      string
	Name      string
	LastPrice float64
}

// ListActiveAlerts 查询全部活跃告警（联持仓取最新价）
func (r *Repository) ListActiveAlerts(ctx context.Context) ([]ActiveAlertRow, error) {
	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`SELECT al.id, al.account_id, al.asset_id, al.alert_type,
			al.target_price, al.status, al.triggered_at, al.triggered_price, al.note, al.created_at, al.updated_at,
			s.code, s.name, COALESCE(p.last_price, 0)
		FROM %s al
		JOIN %s s ON s.id = al.asset_id
		LEFT JOIN %s p ON p.account_id = al.account_id AND p.asset_id = al.asset_id AND p.status = 'holding'
		WHERE al.status = 'active'`, tableAlerts, tableAssets, tablePositions))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []ActiveAlertRow
	for rows.Next() {
		var it ActiveAlertRow
		var triggeredAt sql.NullTime
		var triggeredPrice sql.NullFloat64
		var note sql.NullString
		if err := rows.Scan(&it.Alert.ID, &it.Alert.AccountID, &it.Alert.AssetID, &it.Alert.AlertType,
			&it.Alert.TargetPrice, &it.Alert.Status, &triggeredAt, &triggeredPrice, &note,
			&it.Alert.CreatedAt, &it.Alert.UpdatedAt,
			&it.Code, &it.Name, &it.LastPrice); err != nil {
			return nil, err
		}
		it.Alert.Note = note.String
		items = append(items, it)
	}
	return items, rows.Err()
}

// MarkAlertTriggered 标记告警已触发（乐观锁：仅 active 可转 triggered）
func (r *Repository) MarkAlertTriggered(ctx context.Context, id string, price float64) (bool, error) {
	res, err := r.db.ExecContext(ctx,
		fmt.Sprintf(`UPDATE %s SET status = 'triggered', triggered_at = NOW(), triggered_price = ?
			WHERE id = ? AND status = 'active'`, tableAlerts), price, id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func scanAlert(row *sql.Row) (*PriceAlert, error) {
	var a PriceAlert
	var triggeredAt sql.NullTime
	var triggeredPrice sql.NullFloat64
	var note sql.NullString
	err := row.Scan(&a.ID, &a.AccountID, &a.AssetID, &a.AlertType, &a.TargetPrice, &a.Status,
		&triggeredAt, &triggeredPrice, &note, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if triggeredAt.Valid {
		t := triggeredAt.Time
		a.TriggeredAt = &t
	}
	a.TriggeredPrice = triggeredPrice.Float64
	a.Note = note.String
	return &a, nil
}

// TxTupleExists 按元组查重（导入防重复）：账户+标的+类型+成交日+份额+价格
func (r *Repository) TxTupleExists(ctx context.Context, accountID, assetID, tradeType string, tradeAt time.Time, shares, price float64) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s
		WHERE account_id = ? AND asset_id = ? AND trade_type = ? AND shares = ? AND price = ?
		AND trade_at >= ? AND trade_at < DATE_ADD(?, INTERVAL 1 DAY)`,
		tableTxs), accountID, assetID, tradeType, shares, price,
		tradeAt.Format("2006-01-02 15:04:05"), tradeAt.Format("2006-01-02 15:04:05")).Scan(&n)
	return n > 0, err
}

// ============================================================================
// 内部工具
// ============================================================================

// appendInt 追加 int 参数
func appendInt(args []any, vals ...int) []any {
	for _, v := range vals {
		args = append(args, v)
	}
	return args
}

// isDuplicateErr 判断唯一键冲突错误
func isDuplicateErr(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "duplicate")
}

// marshalFeeDetail 序列化费用明细 JSON
func marshalFeeDetail(m map[string]float64) string {
	if len(m) == 0 {
		return ""
	}
	b, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	return string(b)
}
