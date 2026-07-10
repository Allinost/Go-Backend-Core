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

	ErrInsufficientStock  = 20200
	ErrInvalidOrderStatus = 20201
	ErrReserveConvertFail = 20202

	ErrImageUpload   = 20300
	ErrImageNotFound = 20301
)

var errMsgs = map[int]string{
	ErrInventoryNotFound:  "库存目录不存在",
	ErrProductNotFound:    "商品不存在",
	ErrOrderNotFound:      "出库单不存在",
	ErrInboundLogNotFound: "入库日志不存在",
	ErrTagNotFound:        "标签不存在",
	ErrStatusCodeNotFound: "状态码不存在",
	ErrWhitelistNotFound:  "白名单条目不存在",
	ErrInvalidZone:        "无效的区域编码",
	ErrInvalidStatusCode:  "无效的状态编码",
	ErrInvalidQuantity:    "无效的数量",
	ErrDuplicateTagName:   "标签名已存在",
	ErrDuplicateCode:      "编码已存在",
	ErrDuplicateOpenid:    "OpenID 已存在",
	ErrInsufficientStock:  "库存不足",
	ErrInvalidOrderStatus: "无效的出库单状态",
	ErrReserveConvertFail: "预留单转换失败",
	ErrImageUpload:        "图片上传失败",
	ErrImageNotFound:      "图片不存在",
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
		msg = fmt.Sprintf(msg, args...)
	}
	return &GoodserError{Code: code, Message: msg}
}
