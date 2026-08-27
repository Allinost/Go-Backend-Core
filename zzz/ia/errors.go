// Package ia 错误码和错误类型定义（42xxx 段）
package ia

import "fmt"

const (
	// 资源不存在 (42001-42099)
	ErrAccountNotFound     = 42001
	ErrAssetNotFound       = 42002
	ErrTransactionNotFound = 42003
	ErrCashFlowNotFound    = 42004
	ErrFeeNotFound         = 42005
	ErrPositionNotFound    = 42006
	ErrDCAPlanNotFound     = 42007
	ErrAllocationNotFound  = 42008
	ErrFxRateNotFound      = 42009

	// 参数校验错误 (42100-42199)
	ErrInvalidAccountType  = 42100
	ErrInvalidCostMethod   = 42101
	ErrInvalidAssetType    = 42102
	ErrInvalidMarket       = 42103
	ErrInvalidTradeType    = 42104
	ErrInvalidTxStatus     = 42105
	ErrInvalidFlowType     = 42106
	ErrInvalidFeeType      = 42107
	ErrInvalidDCACycle     = 42108
	ErrInvalidDate         = 42109
	ErrInvalidRatio        = 42110
	ErrInvalidCurrencyCode = 42111

	// 业务规则冲突 (42200-42299)
	ErrDuplicateAsset      = 42200 // 标的 code+market 重复
	ErrAccountHasPositions = 42201 // 账户下有持仓/流水，禁止删除
	ErrInsufficientShares  = 42202 // 卖出份额超过持仓
	ErrNegativeShares      = 42203 // 重放后份额为负，流水异常
	ErrPlanFinished        = 42204 // 定投计划已结束
	ErrAllocationSumExceed = 42205 // 同 scope 目标比例之和超过 100%
	ErrCategoryConflict    = 42206 // 同 scope 下类别重复
	ErrTransferSameAccount = 42207 // 划转双方账户相同

	// 任务与外部依赖 (42300-42399)
	ErrQuoteUnavailable = 42300 // 行情数据源未就绪
	ErrTaskFailed       = 42301 // 定时任务执行失败
	ErrInsufficientData = 42302 // 分析数据不足

	// 告警与导入 (42400-42499)
	ErrAlertNotFound   = 42400 // 告警规则不存在
	ErrInvalidAlert    = 42401 // 无效的告警类型或状态
	ErrImportFormat    = 42402 // CSV 格式错误
	ErrImportNoAccount = 42403 // 导入账户不存在
)

var extraMsgs = map[int]string{
	ErrQuoteUnavailable: "行情数据源未初始化",
	ErrTaskFailed:       "任务执行失败",
	ErrInsufficientData: "分析数据不足",
	ErrAlertNotFound:    "告警规则不存在",
	ErrInvalidAlert:     "无效的告警配置",
	ErrImportFormat:     "CSV 对账单格式错误（需含表头：trade_date,trade_type,code,market,...）",
	ErrImportNoAccount:  "导入目标账户不存在",
}

// errMsgs 错误码 → 中文消息映射
var errMsgs = map[int]string{
	ErrAccountNotFound:     "账户不存在",
	ErrAssetNotFound:       "标的不存在",
	ErrTransactionNotFound: "交易记录不存在",
	ErrCashFlowNotFound:    "资金流水不存在",
	ErrFeeNotFound:         "费用记录不存在",
	ErrPositionNotFound:    "持仓不存在",
	ErrDCAPlanNotFound:     "定投计划不存在",
	ErrAllocationNotFound:  "配置目标不存在",
	ErrFxRateNotFound:      "汇率记录不存在",

	ErrInvalidAccountType:  "无效的账户类型",
	ErrInvalidCostMethod:   "无效的成本核算方法（wa/fifo）",
	ErrInvalidAssetType:    "无效的资产类型",
	ErrInvalidMarket:       "无效的市场",
	ErrInvalidTradeType:    "无效的交易类型",
	ErrInvalidTxStatus:     "无效的交易状态",
	ErrInvalidFlowType:     "无效的资金流水类型",
	ErrInvalidFeeType:      "无效的费用类型",
	ErrInvalidDCACycle:     "无效的定投周期",
	ErrInvalidDate:         "日期格式错误，应为 YYYY-MM-DD",
	ErrInvalidRatio:        "比例取值错误，应为 0-1 之间",
	ErrInvalidCurrencyCode: "无效的币种代码",

	ErrDuplicateAsset:      "标的已存在（code+market 重复）",
	ErrAccountHasPositions: "账户存在持仓或流水，禁止删除",
	ErrInsufficientShares:  "卖出/转出份额超过当前持仓",
	ErrNegativeShares:      "交易重放后份额为负，请检查历史流水",
	ErrPlanFinished:        "定投计划已结束，禁止操作",
	ErrAllocationSumExceed: "同范围内目标配置比例之和不能超过 100%",
	ErrCategoryConflict:    "同范围内该配置类别已存在",
	ErrTransferSameAccount: "划转双方账户不能相同",
}

// IAError 业务错误，携带错误码和消息
type IAError struct {
	Code    int
	Message string
}

// Error 实现 error 接口
func (e *IAError) Error() string {
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

// NewIAError 创建业务错误。
// args 为空时使用默认错误文案；args[0] 作为格式串覆盖默认文案，其余参数填充占位符：
//
//	NewIAError(ErrAssetNotFound)                          → "标的不存在"
//	NewIAError(ErrNegativeShares, "流水 #%d 异常", txID)   → "流水 #12 异常"
func NewIAError(code int, args ...interface{}) *IAError {
	msg, ok := errMsgs[code]
	if !ok {
		msg = extraMsgs[code]
	}
	if len(args) > 0 {
		if f, isStr := args[0].(string); isStr {
			msg = fmt.Sprintf(f, args[1:]...)
		}
	}
	return &IAError{Code: code, Message: msg}
}
