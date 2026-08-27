// Package ia HTTP 处理器层
package ia

import (
	"database/sql"
	"errors"
	"io"
	"strconv"
	"strings"

	appErr "github.com/Allinost/go-backend-core/internal/pkg/errors"
	"github.com/Allinost/go-backend-core/internal/pkg/logger"
	"github.com/Allinost/go-backend-core/internal/pkg/response"
	pkgNet "github.com/Allinost/go-backend-core/pkg/net"
	"github.com/gin-gonic/gin"
)

// Handler HTTP 处理器
type Handler struct {
	svc *Service
}

// NewHandler 创建 Handler
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// ============================================================================
// 账户
// ============================================================================

// ListAccounts 账户列表
// @Summary      获取投资账户列表
// @Tags         zzz-ia
// @Produce      json
// @Param        page       query  int     false  "页码"
// @Param        page_size  query  int     false  "每页数量"
// @Param        type       query  string  false  "账户类型 broker/bank/fund/other"
// @Param        status     query  string  false  "状态 active/paused"
// @Param        keyword    query  string  false  "关键词（name/broker）"
// @Success      200  {object}  response.Response{data=PaginatedResp[Account]}
// @Router       /zzz-ia/accounts [get]
func (h *Handler) ListAccounts(c *gin.Context) {
	var req ListAccountsReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	req.Normalize()
	items, hasMore, total, err := h.svc.ListAccounts(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, PaginatedResp[Account]{Items: nonNil(items), HasMore: hasMore, Total: total})
}

// CreateAccount 创建账户
// @Summary      创建投资账户
// @Tags         zzz-ia
// @Accept       json
// @Produce      json
// @Param        body  body  CreateAccountReq  true  "账户信息"
// @Success      200  {object}  response.Response{data=Account}
// @Router       /zzz-ia/accounts [post]
func (h *Handler) CreateAccount(c *gin.Context) {
	var req CreateAccountReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	acct, err := h.svc.CreateAccount(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, acct)
}

// GetAccount 账户详情
// @Summary      获取账户详情
// @Tags         zzz-ia
// @Produce      json
// @Param        id  path  string  true  "账户 ID"
// @Success      200  {object}  response.Response{data=Account}
// @Router       /zzz-ia/accounts/{id} [get]
func (h *Handler) GetAccount(c *gin.Context) {
	acct, err := h.svc.GetAccount(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, acct)
}

// UpdateAccount 更新账户
// @Summary      更新账户
// @Tags         zzz-ia
// @Accept       json
// @Produce      json
// @Param        id    path  string            true  "账户 ID"
// @Param        body  body  UpdateAccountReq  true  "更新字段"
// @Success      200  {object}  response.Response{data=Account}
// @Router       /zzz-ia/accounts/{id} [put]
func (h *Handler) UpdateAccount(c *gin.Context) {
	var req UpdateAccountReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	acct, err := h.svc.UpdateAccount(c.Request.Context(), c.Param("id"), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, acct)
}

// DeleteAccount 删除账户
// @Summary      删除账户（存在持仓/交易时拒绝）
// @Tags         zzz-ia
// @Produce      json
// @Param        id  path  string  true  "账户 ID"
// @Success      200  {object}  response.Response
// @Router       /zzz-ia/accounts/{id} [delete]
func (h *Handler) DeleteAccount(c *gin.Context) {
	if err := h.svc.DeleteAccount(c.Request.Context(), c.Param("id")); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{})
}

// ============================================================================
// 标的
// ============================================================================

// ListAssets 标的列表
// @Summary      获取标的列表
// @Tags         zzz-ia
// @Produce      json
// @Param        asset_type  query  string  false  "类型 deposit/wealth/fund/stock/etf"
// @Param        market      query  string  false  "市场 CN-SH/CN-SZ/HK/US/OTC/BANK"
// @Param        status      query  string  false  "状态"
// @Param        keyword     query  string  false  "关键词（code/name/issuer）"
// @Success      200  {object}  response.Response{data=PaginatedResp[Asset]}
// @Router       /zzz-ia/assets [get]
func (h *Handler) ListAssets(c *gin.Context) {
	var req ListAssetsReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	req.Normalize()
	items, hasMore, total, err := h.svc.ListAssets(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, PaginatedResp[Asset]{Items: nonNil(items), HasMore: hasMore, Total: total})
}

// CreateAsset 创建标的
// @Summary      创建标的
// @Tags         zzz-ia
// @Accept       json
// @Produce      json
// @Param        body  body  CreateAssetReq  true  "标的信息"
// @Success      200  {object}  response.Response{data=Asset}
// @Router       /zzz-ia/assets [post]
func (h *Handler) CreateAsset(c *gin.Context) {
	var req CreateAssetReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	asset, err := h.svc.CreateAsset(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, asset)
}

// GetAsset 标的详情
// @Summary      获取标的详情
// @Tags         zzz-ia
// @Produce      json
// @Param        id  path  string  true  "标的 ID"
// @Success      200  {object}  response.Response{data=Asset}
// @Router       /zzz-ia/assets/{id} [get]
func (h *Handler) GetAsset(c *gin.Context) {
	asset, err := h.svc.GetAsset(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, asset)
}

// UpdateAsset 更新标的
// @Summary      更新标的
// @Tags         zzz-ia
// @Accept       json
// @Produce      json
// @Param        id    path  string          true  "标的 ID"
// @Param        body  body  UpdateAssetReq  true  "更新字段"
// @Success      200  {object}  response.Response{data=Asset}
// @Router       /zzz-ia/assets/{id} [put]
func (h *Handler) UpdateAsset(c *gin.Context) {
	var req UpdateAssetReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	asset, err := h.svc.UpdateAsset(c.Request.Context(), c.Param("id"), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, asset)
}

// DeleteAsset 删除标的
// @Summary      删除标的（存在持仓时拒绝）
// @Tags         zzz-ia
// @Produce      json
// @Param        id  path  string  true  "标的 ID"
// @Success      200  {object}  response.Response
// @Router       /zzz-ia/assets/{id} [delete]
func (h *Handler) DeleteAsset(c *gin.Context) {
	if err := h.svc.DeleteAsset(c.Request.Context(), c.Param("id")); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{})
}

// ============================================================================
// 资金流水
// ============================================================================

// CreateCashFlow 创建资金流水
// @Summary      记录入金/出金/划转
// @Tags         zzz-ia
// @Accept       json
// @Produce      json
// @Param        body  body  CreateCashFlowReq  true  "资金流水"
// @Success      200  {object}  response.Response{data=CashFlow}
// @Router       /zzz-ia/cash-flows [post]
func (h *Handler) CreateCashFlow(c *gin.Context) {
	var req CreateCashFlowReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	flow, err := h.svc.CreateCashFlow(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, flow)
}

// ListCashFlows 资金流水列表
// @Summary      查询资金流水
// @Tags         zzz-ia
// @Produce      json
// @Param        account_id  query  string  false  "账户 ID"
// @Param        flow_type   query  string  false  "类型 deposit/withdraw/transfer_in/transfer_out"
// @Param        start       query  string  false  "起始日期 YYYY-MM-DD"
// @Param        end         query  string  false  "结束日期 YYYY-MM-DD"
// @Success      200  {object}  response.Response{data=PaginatedResp[CashFlow]}
// @Router       /zzz-ia/cash-flows [get]
func (h *Handler) ListCashFlows(c *gin.Context) {
	var req ListCashFlowsReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	req.Normalize()
	items, hasMore, total, err := h.svc.ListCashFlows(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, PaginatedResp[CashFlow]{Items: nonNil(items), HasMore: hasMore, Total: total})
}

// DeleteCashFlow 删除资金流水
// @Summary      删除资金流水
// @Tags         zzz-ia
// @Produce      json
// @Param        id  path  int  true  "流水 ID"
// @Success      200  {object}  response.Response
// @Router       /zzz-ia/cash-flows/{id} [delete]
func (h *Handler) DeleteCashFlow(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.ParamErr(c, "无效 ID")
		return
	}
	if err = h.svc.DeleteCashFlow(c.Request.Context(), id); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{})
}

// ============================================================================
// 交易
// ============================================================================

// CreateTransaction 创建交易
// @Summary      记录买入/卖出/分红/送转等交易
// @Tags         zzz-ia
// @Accept       json
// @Produce      json
// @Param        body  body  CreateTransactionReq  true  "交易信息"
// @Success      200  {object}  response.Response{data=Transaction}
// @Router       /zzz-ia/transactions [post]
func (h *Handler) CreateTransaction(c *gin.Context) {
	var req CreateTransactionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	tx, err := h.svc.CreateTransaction(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, tx)
}

// ListTransactions 交易列表
// @Summary      查询交易流水
// @Tags         zzz-ia
// @Produce      json
// @Param        account_id  query  string  false  "账户 ID"
// @Param        asset_id    query  string  false  "标的 ID"
// @Param        trade_type  query  string  false  "类型 buy/sell/dividend_cash/dividend_reinvest/interest/bonus_shares/rights_issue"
// @Param        status      query  string  false  "状态 pending/confirmed"
// @Param        start       query  string  false  "起始日期"
// @Param        end         query  string  false  "结束日期"
// @Success      200  {object}  response.Response{data=PaginatedResp[Transaction]}
// @Router       /zzz-ia/transactions [get]
func (h *Handler) ListTransactions(c *gin.Context) {
	var req ListTransactionsReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	req.Normalize()
	items, hasMore, total, err := h.svc.ListTransactions(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, PaginatedResp[Transaction]{Items: nonNil(items), HasMore: hasMore, Total: total})
}

// GetTransaction 交易详情
// @Summary      获取交易详情
// @Tags         zzz-ia
// @Produce      json
// @Param        id  path  int  true  "交易 ID"
// @Success      200  {object}  response.Response{data=Transaction}
// @Router       /zzz-ia/transactions/{id} [get]
func (h *Handler) GetTransaction(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.ParamErr(c, "无效 ID")
		return
	}
	tx, err := h.svc.GetTransaction(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, tx)
}

// ConfirmTransaction 确认在途交易
// @Summary      确认在途交易（场外申购到账）
// @Tags         zzz-ia
// @Produce      json
// @Param        id  path  int  true  "交易 ID"
// @Success      200  {object}  response.Response
// @Router       /zzz-ia/transactions/{id}/confirm [post]
func (h *Handler) ConfirmTransaction(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.ParamErr(c, "无效 ID")
		return
	}
	if err = h.svc.ConfirmTransaction(c.Request.Context(), id); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{})
}

// DeleteTransaction 删除交易（自动重放修正持仓）
// @Summary      删除交易记录
// @Tags         zzz-ia
// @Produce      json
// @Param        id  path  int  true  "交易 ID"
// @Success      200  {object}  response.Response
// @Router       /zzz-ia/transactions/{id} [delete]
func (h *Handler) DeleteTransaction(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.ParamErr(c, "无效 ID")
		return
	}
	if err = h.svc.DeleteTransaction(c.Request.Context(), id); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{})
}

// ============================================================================
// 持仓与总览
// ============================================================================

// ListPositions 持仓列表
// @Summary      查询实时持仓（含市值/浮盈）
// @Tags         zzz-ia
// @Produce      json
// @Param        account_id  query  string  false  "账户 ID"
// @Param        asset_type  query  string  false  "资产类型"
// @Param        status      query  string  false  "holding/closed，默认 holding"
// @Success      200  {object}  response.Response{data=[]PositionView}
// @Router       /zzz-ia/positions [get]
func (h *Handler) ListPositions(c *gin.Context) {
	var req ListPositionsReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	items, err := h.svc.ListPositions(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, items)
}

// Overview 资产总览
// @Summary      全局与分账户资产总览
// @Tags         zzz-ia
// @Produce      json
// @Success      200  {object}  response.Response{data=OverviewResp}
// @Router       /zzz-ia/overview [get]
func (h *Handler) Overview(c *gin.Context) {
	resp, err := h.svc.Overview(c.Request.Context())
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, resp)
}

// ============================================================================
// 独立费用
// ============================================================================

// CreateFee 创建费用记录
// @Summary      记录独立费用（管理费/托管费等外扣费用）
// @Tags         zzz-ia
// @Accept       json
// @Produce      json
// @Param        body  body  CreateFeeReq  true  "费用信息"
// @Success      200  {object}  response.Response{data=FeeRecord}
// @Router       /zzz-ia/fees [post]
func (h *Handler) CreateFee(c *gin.Context) {
	var req CreateFeeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	currency := req.Currency
	if currency == "" {
		acct, err := h.svc.GetAccount(c.Request.Context(), req.AccountID)
		if err != nil {
			h.handleError(c, err)
			return
		}
		currency = acct.Currency
	}
	date := req.ChargeDate
	if date == "" {
		date = todayStr()
	}
	fee, err := h.svc.repo.CreateFee(c.Request.Context(), &FeeRecord{
		AccountID: req.AccountID, AssetID: req.AssetID, FeeType: req.FeeType,
		Amount: req.Amount, Currency: currency, ChargeDate: date, Period: req.Period, Remark: req.Remark,
	})
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, fee)
}

// ListFees 费用列表
// @Summary      查询独立费用
// @Tags         zzz-ia
// @Produce      json
// @Param        account_id  query  string  false  "账户 ID"
// @Param        fee_type    query  string  false  "费用类型"
// @Param        start       query  string  false  "起始日期"
// @Param        end         query  string  false  "结束日期"
// @Success      200  {object}  response.Response{data=PaginatedResp[FeeRecord]}
// @Router       /zzz-ia/fees [get]
func (h *Handler) ListFees(c *gin.Context) {
	var req ListFeesReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	req.Normalize()
	items, hasMore, total, err := h.svc.repo.ListFees(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, PaginatedResp[FeeRecord]{Items: nonNil(items), HasMore: hasMore, Total: total})
}

// DeleteFee 删除费用记录
// @Summary      删除费用记录
// @Tags         zzz-ia
// @Produce      json
// @Param        id  path  int  true  "费用 ID"
// @Success      200  {object}  response.Response
// @Router       /zzz-ia/fees/{id} [delete]
func (h *Handler) DeleteFee(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.ParamErr(c, "无效 ID")
		return
	}
	if err = h.svc.repo.DeleteFee(c.Request.Context(), id); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{})
}

// ============================================================================
// 定投计划
// ============================================================================

// ListDCAPlans 定投计划列表
// @Summary      查询定投计划
// @Tags         zzz-ia
// @Produce      json
// @Param        status  query  string  false  "active/paused/finished"
// @Success      200  {object}  response.Response{data=PaginatedResp[DCAPlan]}
// @Router       /zzz-ia/dca-plans [get]
func (h *Handler) ListDCAPlans(c *gin.Context) {
	var req ListDCAPlansReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	req.Normalize()
	items, hasMore, total, err := h.svc.ListDCAPlans(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, PaginatedResp[DCAPlan]{Items: nonNil(items), HasMore: hasMore, Total: total})
}

// CreateDCAPlan 创建定投计划
// @Summary      创建定投计划
// @Tags         zzz-ia
// @Accept       json
// @Produce      json
// @Param        body  body  CreateDCAPlanReq  true  "计划信息"
// @Success      200  {object}  response.Response{data=DCAPlan}
// @Router       /zzz-ia/dca-plans [post]
func (h *Handler) CreateDCAPlan(c *gin.Context) {
	var req CreateDCAPlanReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	plan, err := h.svc.CreateDCAPlan(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, plan)
}

// GetDCAPlan 定投计划详情
// @Summary      获取定投计划详情
// @Tags         zzz-ia
// @Produce      json
// @Param        id  path  string  true  "计划 ID"
// @Success      200  {object}  response.Response{data=DCAPlan}
// @Router       /zzz-ia/dca-plans/{id} [get]
func (h *Handler) GetDCAPlan(c *gin.Context) {
	plan, err := h.svc.GetDCAPlan(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, plan)
}

// UpdateDCAPlan 更新定投计划
// @Summary      更新定投计划
// @Tags         zzz-ia
// @Accept       json
// @Produce      json
// @Param        id    path  string            true  "计划 ID"
// @Param        body  body  UpdateDCAPlanReq  true  "更新字段"
// @Success      200  {object}  response.Response{data=DCAPlan}
// @Router       /zzz-ia/dca-plans/{id} [put]
func (h *Handler) UpdateDCAPlan(c *gin.Context) {
	var req UpdateDCAPlanReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	plan, err := h.svc.UpdateDCAPlan(c.Request.Context(), c.Param("id"), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, plan)
}

// DeleteDCAPlan 删除定投计划
// @Summary      删除定投计划
// @Tags         zzz-ia
// @Produce      json
// @Param        id  path  string  true  "计划 ID"
// @Success      200  {object}  response.Response
// @Router       /zzz-ia/dca-plans/{id} [delete]
func (h *Handler) DeleteDCAPlan(c *gin.Context) {
	if err := h.svc.DeleteDCAPlan(c.Request.Context(), c.Param("id")); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{})
}

// ============================================================================
// 汇率
// ============================================================================

// UpsertFxRate 录入/更新汇率
// @Summary      录入或更新汇率（同币对同日期覆盖）
// @Tags         zzz-ia
// @Accept       json
// @Produce      json
// @Param        body  body  UpsertFxRateReq  true  "汇率"
// @Success      200  {object}  response.Response
// @Router       /zzz-ia/fx-rates [put]
func (h *Handler) UpsertFxRate(c *gin.Context) {
	var req UpsertFxRateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.UpsertFxRate(c.Request.Context(), &req); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{})
}

// ListFxRates 汇率列表
// @Summary      查询汇率记录
// @Tags         zzz-ia
// @Produce      json
// @Param        base_ccy   query  string  false  "基准币种 CNY"
// @Param        quote_ccy  query  string  false  "报价币种 USD/HKD"
// @Success      200  {object}  response.Response{data=[]FxRate}
// @Router       /zzz-ia/fx-rates [get]
func (h *Handler) ListFxRates(c *gin.Context) {
	var req ListFxRatesReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	items, err := h.svc.ListFxRates(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, items)
}

// ============================================================================
// 资产配比
// ============================================================================

// ListAllocations 配置目标列表
// @Summary      查询资产配置目标
// @Tags         zzz-ia
// @Produce      json
// @Param        account_id  query  string  false  "账户 ID（空=全局）"
// @Success      200  {object}  response.Response{data=[]AllocationItem}
// @Router       /zzz-ia/allocations [get]
func (h *Handler) ListAllocations(c *gin.Context) {
	var req ListAllocationsReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	items, err := h.svc.ListAllocations(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, items)
}

// CreateAllocation 创建配置目标
// @Summary      创建资产配置目标项
// @Tags         zzz-ia
// @Accept       json
// @Produce      json
// @Param        body  body  CreateAllocationReq  true  "目标项"
// @Success      200  {object}  response.Response{data=AllocationItem}
// @Router       /zzz-ia/allocations [post]
func (h *Handler) CreateAllocation(c *gin.Context) {
	var req CreateAllocationReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.svc.CreateAllocation(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

// UpdateAllocation 更新配置目标
// @Summary      更新资产配置目标项
// @Tags         zzz-ia
// @Accept       json
// @Produce      json
// @Param        id    path  string               true  "目标项 ID"
// @Param        body  body  UpdateAllocationReq  true  "更新字段"
// @Success      200  {object}  response.Response{data=AllocationItem}
// @Router       /zzz-ia/allocations/{id} [put]
func (h *Handler) UpdateAllocation(c *gin.Context) {
	var req UpdateAllocationReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.svc.UpdateAllocation(c.Request.Context(), c.Param("id"), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

// DeleteAllocation 删除配置目标
// @Summary      删除资产配置目标项
// @Tags         zzz-ia
// @Produce      json
// @Param        id  path  string  true  "目标项 ID"
// @Success      200  {object}  response.Response
// @Router       /zzz-ia/allocations/{id} [delete]
func (h *Handler) DeleteAllocation(c *gin.Context) {
	if err := h.svc.DeleteAllocation(c.Request.Context(), c.Param("id")); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{})
}

// AllocationReview 配置回顾
// @Summary      实际配比 vs 目标配置对比（含偏差预警）
// @Tags         zzz-ia
// @Produce      json
// @Param        account_id  query  string  false  "账户 ID（空=全局）"
// @Success      200  {object}  response.Response{data=AllocationReviewResp}
// @Router       /zzz-ia/allocations/review [get]
func (h *Handler) AllocationReview(c *gin.Context) {
	resp, err := h.svc.AllocationReview(c.Request.Context(), c.Query("account_id"))
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, resp)
}

// UpdateLastPrice 手工估值
// @Summary      手工更新持仓最新价/净值（存款理财估值入口）
// @Tags         zzz-ia
// @Accept       json
// @Produce      json
// @Param        body  body  object{account_id=string,asset_id=string,price=number}  true  "估值信息"
// @Success      200  {object}  response.Response
// @Router       /zzz-ia/positions/valuation [put]
func (h *Handler) UpdateLastPrice(c *gin.Context) {
	var req struct {
		AccountID string  `json:"account_id" binding:"required"`
		AssetID   string  `json:"asset_id" binding:"required"`
		Price     float64 `json:"price" binding:"required,gt=0"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.UpdateLastPrice(c.Request.Context(), req.AccountID, req.AssetID, req.Price); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{})
}

// SetPositionTarget 设置持仓追踪目标
// @Summary      设置/调整持仓追踪目标价与止损参考价（买入后可随时修改）
// @Tags         zzz-ia
// @Accept       json
// @Produce      json
// @Param        body  body  SetPositionTargetReq  true  "目标设置（传 0 清除对应项）"
// @Success      200  {object}  response.Response
// @Router       /zzz-ia/positions/target [post]
func (h *Handler) SetPositionTarget(c *gin.Context) {
	var req SetPositionTargetReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.SetPositionTarget(c.Request.Context(), &req); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{})
}

// TradeAdvice 买卖建议
// @Summary      持仓买卖操作建议：买入锚定趋势与目标空间，卖出锚定买入成本/追踪目标/止损纪律
// @Description  规则维度：追踪目标达成止盈、止损参考价纪律、浮亏评估线、MA20/MA60 趋势、集中度与配置偏差再平衡；存款理财按到期日给持有建议
// @Tags         zzz-ia
// @Produce      json
// @Param        account_id  query  string  false  "账户 ID（空=全部账户）"
// @Success      200  {object}  response.Response{data=TradeAdviceResp}
// @Router       /zzz-ia/advice [get]
func (h *Handler) TradeAdvice(c *gin.Context) {
	res, err := h.svc.TradeAdvice(c.Request.Context(), c.Query("account_id"))
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, res)
}

// ============================================================================
// 行情与定时任务
// ============================================================================

// RefreshQuotes 手动刷新行情
// @Summary      拉取全部持仓标的最新行情并刷新市值
// @Tags         zzz-ia
// @Produce      json
// @Success      200  {object}  response.Response{data=RefreshResult}
// @Router       /zzz-ia/quotes/refresh [post]
func (h *Handler) RefreshQuotes(c *gin.Context) {
	res, err := h.svc.RefreshQuotes(c.Request.Context())
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, res)
}

// ListPriceHistory 价格历史查询
// @Summary      查询标的价格/净值历史
// @Tags         zzz-ia
// @Produce      json
// @Param        asset_id  query  string  true   "标的 ID"
// @Param        limit     query  int     false  "返回条数，默认 500，最大 1000"
// @Success      200  {object}  response.Response{data=[]PriceHistory}
// @Router       /zzz-ia/price-history [get]
func (h *Handler) ListPriceHistory(c *gin.Context) {
	assetID := c.Query("asset_id")
	if assetID == "" {
		response.ParamErr(c, "参数错误: 缺少 asset_id")
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "500"))
	items, err := h.svc.repo.ListPriceHistory(c.Request.Context(), assetID, limit)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, items)
}

// TriggerAccrue 手动触发计息
// @Summary      立即执行存款/理财自动计息（幂等）
// @Tags         zzz-ia
// @Produce      json
// @Success      200  {object}  response.Response{data=AccrueResult}
// @Router       /zzz-ia/tasks/accrue [post]
func (h *Handler) TriggerAccrue(c *gin.Context) {
	res, err := h.svc.AccrueInterest(c.Request.Context())
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, res)
}

// TriggerSnapshot 手动触发快照
// @Summary      立即生成当日组合快照（幂等覆盖）
// @Tags         zzz-ia
// @Produce      json
// @Success      200  {object}  response.Response{data=SnapshotResult}
// @Router       /zzz-ia/tasks/snapshot [post]
func (h *Handler) TriggerSnapshot(c *gin.Context) {
	res, err := h.svc.GenerateSnapshots(c.Request.Context())
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, res)
}

// TriggerMaturityCheck 手动触发生成到期检测
// @Summary      检测临近到期资产并推送提醒
// @Tags         zzz-ia
// @Produce      json
// @Success      200  {object}  response.Response{data=MaturityCheckResult}
// @Router       /zzz-ia/tasks/maturity-check [post]
func (h *Handler) TriggerMaturityCheck(c *gin.Context) {
	res, err := h.svc.CheckMaturities(c.Request.Context())
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, res)
}

// ============================================================================
// 分析报表
// ============================================================================

// AnalyticsEquityCurve 收益曲线
// @Summary      组合收益曲线（快照驱动），可选叠加基准指数归一化对比
// @Tags         zzz-ia
// @Produce      json
// @Param        account_id  query  string  false  "账户 ID（空=全局）"
// @Param        start       query  string  false  "起始日期 YYYY-MM-DD"
// @Param        end         query  string  false  "结束日期"
// @Param        benchmark   query  string  false  "基准：CSI300/CSI500/SSE 或 secid，如 1.000300"
// @Success      200  {object}  response.Response{data=EquityCurveResp}
// @Router       /zzz-ia/analytics/equity-curve [get]
func (h *Handler) AnalyticsEquityCurve(c *gin.Context) {
	var req AnalyticsReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	res, err := h.svc.EquityCurve(c.Request.Context(), &req, c.Query("benchmark"))
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, res)
}

// AnalyticsReturns 区间收益指标
// @Summary      区间收益率：简单收益 + Modified Dietz + 全周期 XIRR
// @Tags         zzz-ia
// @Produce      json
// @Param        account_id  query  string  false  "账户 ID（空=全局）"
// @Param        start       query  string  false  "起始日期（默认回溯一年）"
// @Param        end         query  string  false  "结束日期（默认今天）"
// @Success      200  {object}  response.Response{data=ReturnsResp}
// @Router       /zzz-ia/analytics/returns [get]
func (h *Handler) AnalyticsReturns(c *gin.Context) {
	var req AnalyticsReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	res, err := h.svc.ReturnsAnalytics(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, res)
}

// AnalyticsTradesReview 加仓减持复盘
// @Summary      每笔加仓摊薄效果/买入后表现、减持盈亏与机会成本、胜率盈亏比汇总
// @Tags         zzz-ia
// @Produce      json
// @Param        account_id  query  string  false  "账户 ID（空=全部账户）"
// @Success      200  {object}  response.Response{data=TradesReviewResp}
// @Router       /zzz-ia/analytics/trades-review [get]
func (h *Handler) AnalyticsTradesReview(c *gin.Context) {
	var req AnalyticsReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	res, err := h.svc.TradesReview(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, res)
}

// AnalyticsRisk 风险与集中度
// @Summary      最大回撤、年化波动率、单标的集中度（超阈值标记）、HHI
// @Tags         zzz-ia
// @Produce      json
// @Param        account_id  query  string  false  "账户 ID（空=全局）"
// @Param        start       query  string  false  "起始日期"
// @Param        end         query  string  false  "结束日期"
// @Param        threshold   query  number  false  "集中度预警阈值，默认 0.3"
// @Success      200  {object}  response.Response{data=RiskMetricsResp}
// @Router       /zzz-ia/analytics/risk [get]
func (h *Handler) AnalyticsRisk(c *gin.Context) {
	var req AnalyticsReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	threshold, _ := strconv.ParseFloat(c.DefaultQuery("threshold", "0.3"), 64)
	res, err := h.svc.RiskMetrics(c.Request.Context(), &req, threshold)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, res)
}

// AnalyticsDCASimulate 定投模拟
// @Summary      定投 vs 一次性投入模拟（基于标的历史K线）
// @Tags         zzz-ia
// @Produce      json
// @Param        asset_id  query  string  true   "标的 ID（股票/ETF）"
// @Param        cycle     query  string  true   "daily/weekly/biweekly/monthly"
// @Param        exec_day  query  int     true   "执行日（周几1-7或几号1-28）"
// @Param        amount    query  number  true   "每期金额"
// @Param        start     query  string  true   "开始日期 YYYY-MM-DD"
// @Param        end       query  string  false  "结束日期（默认今天）"
// @Success      200  {object}  response.Response{data=DCASimulateResp}
// @Router       /zzz-ia/analytics/dca-simulate [get]
func (h *Handler) AnalyticsDCASimulate(c *gin.Context) {
	var req DCASimulateReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	res, err := h.svc.DCASimulate(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, res)
}

// AnalyticsFees 费用分析
// @Summary      交易费税+独立费用年度分布、分类占比与资产侵蚀度
// @Tags         zzz-ia
// @Produce      json
// @Param        account_id  query  string  false  "账户 ID（空=全局）"
// @Param        start       query  string  false  "起始日期"
// @Param        end         query  string  false  "结束日期"
// @Success      200  {object}  response.Response{data=FeesReportResp}
// @Router       /zzz-ia/analytics/fees [get]
func (h *Handler) AnalyticsFees(c *gin.Context) {
	var req AnalyticsReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	res, err := h.svc.FeesReport(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, res)
}

// AnalyticsDividends 分红统计
// @Summary      分红标的排行、年度分布、税负合计与近12月股息率(YOC)
// @Tags         zzz-ia
// @Produce      json
// @Param        account_id  query  string  false  "账户 ID（空=全局）"
// @Param        start       query  string  false  "起始日期"
// @Param        end         query  string  false  "结束日期"
// @Success      200  {object}  response.Response{data=DividendReportResp}
// @Router       /zzz-ia/analytics/dividends [get]
func (h *Handler) AnalyticsDividends(c *gin.Context) {
	var req AnalyticsReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	res, err := h.svc.DividendReport(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, res)
}

// ============================================================================
// 价格告警（P5）
// ============================================================================

// ListAlerts 告警规则列表
// @Summary      查询价格告警规则
// @Tags         zzz-ia
// @Produce      json
// @Param        account_id  query  string  false  "账户 ID"
// @Param        asset_id    query  string  false  "标的 ID"
// @Param        status      query  string  false  "active/triggered/disabled"
// @Success      200  {object}  response.Response{data=PaginatedResp[PriceAlert]}
// @Router       /zzz-ia/alerts [get]
func (h *Handler) ListAlerts(c *gin.Context) {
	var req ListAlertsReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	req.Normalize()
	items, hasMore, total, err := h.svc.ListAlerts(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, PaginatedResp[PriceAlert]{Items: nonNil(items), HasMore: hasMore, Total: total})
}

// CreateAlert 创建价格告警
// @Summary      创建止盈/止损/到价告警规则
// @Tags         zzz-ia
// @Accept       json
// @Produce      json
// @Param        body  body  CreateAlertReq  true  "告警规则"
// @Success      200  {object}  response.Response{data=PriceAlert}
// @Router       /zzz-ia/alerts [post]
func (h *Handler) CreateAlert(c *gin.Context) {
	var req CreateAlertReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	alert, err := h.svc.CreateAlert(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, alert)
}

// GetAlert 告警详情
// @Summary      获取价格告警详情
// @Tags         zzz-ia
// @Produce      json
// @Param        id  path  string  true  "告警 ID"
// @Success      200  {object}  response.Response{data=PriceAlert}
// @Router       /zzz-ia/alerts/{id} [get]
func (h *Handler) GetAlert(c *gin.Context) {
	alert, err := h.svc.GetAlert(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, alert)
}

// UpdateAlert 更新告警
// @Summary      更新告警（status 置回 active 可重新武装）
// @Tags         zzz-ia
// @Accept       json
// @Produce      json
// @Param        id    path  string           true  "告警 ID"
// @Param        body  body  UpdateAlertReq   true  "更新字段"
// @Success      200  {object}  response.Response{data=PriceAlert}
// @Router       /zzz-ia/alerts/{id} [put]
func (h *Handler) UpdateAlert(c *gin.Context) {
	var req UpdateAlertReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	alert, err := h.svc.UpdateAlert(c.Request.Context(), c.Param("id"), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, alert)
}

// DeleteAlert 删除告警
// @Summary      删除价格告警
// @Tags         zzz-ia
// @Produce      json
// @Param        id  path  string  true  "告警 ID"
// @Success      200  {object}  response.Response
// @Router       /zzz-ia/alerts/{id} [delete]
func (h *Handler) DeleteAlert(c *gin.Context) {
	if err := h.svc.DeleteAlert(c.Request.Context(), c.Param("id")); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{})
}

// TriggerAlertCheck 手动检查价格告警
// @Summary      立即执行价格告警检查（幂等）
// @Tags         zzz-ia
// @Produce      json
// @Success      200  {object}  response.Response{data=AlertCheckResult}
// @Router       /zzz-ia/tasks/check-alerts [post]
func (h *Handler) TriggerAlertCheck(c *gin.Context) {
	res, err := h.svc.CheckPriceAlerts(c.Request.Context())
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, res)
}

// TriggerRiskCheck 手动风险巡检
// @Summary      立即执行集中度与配置偏差巡检（异常时推送 webhook）
// @Tags         zzz-ia
// @Produce      json
// @Success      200  {object}  response.Response{data=RiskMetricsResp}
// @Router       /zzz-ia/tasks/check-risk [post]
func (h *Handler) TriggerRiskCheck(c *gin.Context) {
	res, err := h.svc.CheckRiskAlerts(c.Request.Context())
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, res)
}

// ImportTransactions 导入对账单
// @Summary      导入券商对账单 CSV（multipart 文件 file 或 JSON csv 字段二选一）
// @Tags         zzz-ia
// @Accept       multipart/form-data
// @Produce      json
// @Param        account_id   formData  string  false  "目标账户 ID"
// @Param        auto_create  formData  bool    false  "缺失标的是否自动创建，默认 false"
// @Param        default_type formData  string  false  "自动创建默认类型 stock/fund/etf/deposit/wealth"
// @Param        file         formData  file    false  "CSV 文件"
// @Param        csv          formData  string  false  "或直接传 CSV 文本"
// @Success      200  {object}  response.Response{data=ImportResult}
// @Router       /zzz-ia/import/transactions [post]
func (h *Handler) ImportTransactions(c *gin.Context) {
	var req ImportTxReq

	if c.ContentType() != "" && strings.HasPrefix(c.ContentType(), "application/json") {
		// JSON 模式：csv 文本直传
		if err := c.ShouldBindJSON(&req); err != nil {
			response.ParamErr(c, "参数错误: "+err.Error())
			return
		}
	} else if file, ferr := c.FormFile("file"); ferr == nil && file != nil {
		// multipart 文件模式
		f, oerr := file.Open()
		if oerr != nil {
			response.ParamErr(c, "读取上传文件失败: "+oerr.Error())
			return
		}
		defer f.Close()
		buf := new(strings.Builder)
		if _, cerr := io.Copy(buf, f); cerr != nil {
			response.ParamErr(c, "读取文件内容失败: "+cerr.Error())
			return
		}
		req.CSV = buf.String()
		req.AccountID = c.PostForm("account_id")
		req.AutoCreate = c.PostForm("auto_create") == "true"
		req.DefaultType = c.PostForm("default_type")
	} else {
		// multipart 表单文本模式
		req.CSV = c.PostForm("csv")
		req.AccountID = c.PostForm("account_id")
		req.AutoCreate = c.PostForm("auto_create") == "true"
		req.DefaultType = c.PostForm("default_type")
	}

	if req.AccountID == "" {
		response.ParamErr(c, "参数错误: 缺少 account_id")
		return
	}
	res, ierr := h.svc.ImportTransactions(c.Request.Context(), &req)
	if ierr != nil {
		h.handleError(c, ierr)
		return
	}
	response.Success(c, res)
}

// ============================================================================
// 个股筛选与指数分析（P6）
// ============================================================================

// ScreenDynamicPotential 动态潜力个股筛选
// @Summary      两阶段动量+资金流潜力股筛选（快照预选→K线动量评分）
// @Tags         zzz-ia
// @Produce      json
// @Param        top        query  int  false  "返回条数，默认 20，最大 50"
// @Param        pool_size  query  int  false  "候选池大小，默认 50，最大 100"
// @Param        pages      query  int  false  "快照页数（每页100只），默认 2，最大 5"
// @Success      200  {object}  response.Response{data=DynamicPotentialResp}
// @Router       /zzz-ia/screen/dynamic-potential [get]
func (h *Handler) ScreenDynamicPotential(c *gin.Context) {
	var req DynamicPotentialReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	res, err := h.svc.DynamicPotential(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, res)
}

// ScreenQualityStocks 绩优个股筛选
// @Summary      ROE/增速/估值/市值多维基本面质量筛选与加权排名
// @Tags         zzz-ia
// @Produce      json
// @Param        top       query  int     false  "返回条数，默认 30"
// @Param        pages     query  int     false  "快照页数，默认 3"
// @Param        min_roe   query  number  false  "ROE 下限 %，默认 10"
// @Param        max_pe    query  number  false  "PE-TTM 上限，默认 35"
// @Param        max_pb    query  number  false  "PB 上限，默认 8"
// @Param        min_mv_yi query  number  false  "总市值下限（亿元），默认 50"
// @Success      200  {object}  response.Response{data=QualityStockResp}
// @Router       /zzz-ia/screen/quality-stocks [get]
func (h *Handler) ScreenQualityStocks(c *gin.Context) {
	var req QualityStockReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	res, err := h.svc.QualityStockScreen(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, res)
}

// AnalyticsIndexAnalysis 指数分析
// @Summary      指数均线系统/年内涨幅/窗口分位/年化收益波动回撤分析（多指数并列）
// @Tags         zzz-ia
// @Produce      json
// @Param        indexes  query  string  false  "逗号分隔：CSI300/CSI500/CSI1000/SSE/SZSE/GEM 或 secid，默认 CSI300,CSI500,SSE"
// @Param        years    query  int     false  "回看年数 1-10，默认 3"
// @Success      200  {object}  response.Response{data=IndexAnalysisResp}
// @Router       /zzz-ia/analytics/index-analysis [get]
func (h *Handler) AnalyticsIndexAnalysis(c *gin.Context) {
	var req IndexAnalysisReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	res, err := h.svc.IndexAnalysis(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, res)
}

// TriggerFxRefresh 手动刷新汇率
// @Summary      拉取 CNY→USD/HKD 最新汇率写入 fx_rates（每日 08:10 自动）
// @Tags         zzz-ia
// @Produce      json
// @Success      200  {object}  response.Response{data=FxRefreshResult}
// @Router       /zzz-ia/tasks/refresh-fx [post]
func (h *Handler) TriggerFxRefresh(c *gin.Context) {
	res, err := h.svc.RefreshFxRates(c.Request.Context())
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, res)
}

// handleError 统一错误处理：IAError → 业务错误码，其余 → 500
func (h *Handler) handleError(c *gin.Context, err error) {
	var iaErr *IAError
	if errors.As(err, &iaErr) {
		logger.Error().Err(err).Int("ia_code", iaErr.Code).Msg("ia 业务错误")
		response.Fail(c, appErr.New(iaErr.Code, iaErr.Message))
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		response.Fail(c, appErr.New(appErr.CodeNotFound, "资源不存在"))
		return
	}
	if errors.Is(err, pkgNet.ErrCircuitOpen) {
		logger.Warn().Err(err).Msg("ia 上游数据源熔断")
		response.Fail(c, appErr.New(ErrCircuitOpen, "上游数据源暂时不可用，请稍后重试"))
		return
	}
	logger.Error().Err(err).Msg("ia 服务内部错误")
	response.Fail(c, appErr.New(appErr.CodeSystemErr, "服务内部错误"))
}
