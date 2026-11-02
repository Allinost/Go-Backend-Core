package goodser

import "fmt"

// Goodser 模块错误码范围：20000-29999
const (
	ErrInventoryNotFound  = 20001
	ErrProductNotFound    = 20002
	ErrOrderNotFound      = 20003
	ErrInboundLogNotFound = 20004
	ErrTagNotFound        = 20005
	ErrStatusCodeNotFound = 20006
	ErrWhitelistNotFound  = 20007

	ErrInvalidZone       = 20100
	ErrInvalidStatusCode = 20101
	ErrInvalidQuantity   = 20102
	ErrDuplicateTagName  = 20103
	ErrDuplicateCode     = 20104
	ErrDuplicateOpenid   = 20105

	ErrInsufficientStock    = 20200
	ErrInvalidOrderStatus   = 20201
	ErrReserveConvertFail   = 20202
	ErrOrderItemsRequired   = 20203
	ErrProductInventory     = 20204
	ErrProductInUse         = 20205
	ErrReturnNotAllowed     = 20206
	ErrReturnQtyExceeded    = 20207
	ErrReturnNotFound       = 20208
	ErrInvalidReturnReason  = 20209
	ErrDictionaryNotFound   = 20210
	ErrDictionaryKind       = 20211
	ErrDictionaryInUse      = 20212
	ErrComboNotFound        = 20213
	ErrComboItemsRequired   = 20214
	ErrInvalidDimension     = 20215
	ErrCustomerItemsNeeded  = 20216
	ErrBundleNotFound       = 20217
	ErrBundleMainRequired   = 20218
	ErrBundleMainNotAllowed = 20219
	ErrBundleSubRequired    = 20220
	ErrBundleSubNotAllowed  = 20221
	ErrBundleRule           = 20222

	ErrImageUpload     = 20300
	ErrImageNotFound   = 20301
	ErrStatusCodeInUse = 20302
)

var errMsgs = map[int]string{
	ErrInventoryNotFound:    "库存目录不存在",
	ErrProductNotFound:      "商品不存在",
	ErrOrderNotFound:        "出库单不存在",
	ErrInboundLogNotFound:   "入库日志不存在",
	ErrTagNotFound:          "标签不存在",
	ErrStatusCodeNotFound:   "状态码不存在",
	ErrWhitelistNotFound:    "白名单条目不存在",
	ErrInvalidZone:          "无效的区域编码",
	ErrInvalidStatusCode:    "无效的状态编码",
	ErrInvalidQuantity:      "无效的数量",
	ErrDuplicateTagName:     "标签名已存在",
	ErrDuplicateCode:        "编码已存在",
	ErrDuplicateOpenid:      "OpenID 已存在",
	ErrInsufficientStock:    "库存不足",
	ErrInvalidOrderStatus:   "无效的单据状态",
	ErrReserveConvertFail:   "预留单转换失败",
	ErrOrderItemsRequired:   "单据商品明细不能为空",
	ErrProductInventory:     "商品不属于当前库存目录",
	ErrProductInUse:         "商品仍有库存或锁定数量，无法删除",
	ErrReturnNotAllowed:     "当前出库单不允许退货",
	ErrReturnQtyExceeded:    "退货数量超过可退数量",
	ErrReturnNotFound:       "退货单不存在",
	ErrInvalidReturnReason:  "无效的退货原因",
	ErrDictionaryNotFound:   "字典项不存在",
	ErrDictionaryKind:       "无效的字典类型",
	ErrDictionaryInUse:      "字典项正在被使用，无法删除",
	ErrComboNotFound:        "组合包不存在",
	ErrComboItemsRequired:   "组合包组件不能为空",
	ErrInvalidDimension:     "商品维度不合法",
	ErrCustomerItemsNeeded:  "客户订单请求缺少商品明细",
	ErrBundleNotFound:       "捆不存在",
	ErrBundleMainRequired:   "捆必须配置主商品清单或主商品筛选条件",
	ErrBundleMainNotAllowed: "主商品不在捆允许范围内",
	ErrBundleSubRequired:    "捆必须配置副区商品",
	ErrBundleSubNotAllowed:  "副区商品不在捆配置中",
	ErrBundleRule:           "捆规则校验失败",
	ErrImageUpload:          "图片上传失败",
	ErrImageNotFound:        "图片不存在",
	ErrStatusCodeInUse:      "状态编码正在被使用，无法删除",
}

// GoodserError 模块级错误
type GoodserError struct {
	Code    int
	Message string
}

func (e *GoodserError) Error() string {
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

func NewGoodserError(code int, args ...interface{}) *GoodserError {
	msg := errMsgs[code]
	if len(args) > 0 {
		// 现有错误消息没有格式化占位符，额外参数作为补充说明拼接，避免出现 %!(EXTRA ...)
		msg = msg + "：" + fmt.Sprint(args...)
	}
	return &GoodserError{Code: code, Message: msg}
}
