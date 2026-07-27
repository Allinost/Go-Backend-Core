package shopchecker

import "fmt"

const (
	ErrProductNotFound     = 30001
	ErrPriceRecordNotFound = 30002
	ErrNotifLogNotFound    = 30003

	ErrInvalidURL      = 30100
	ErrInvalidPlatform = 30101
	ErrInvalidInterval = 30102
	ErrInvalidCronExpr = 30103

	ErrFetchFailed = 30200
)

var errMsgs = map[int]string{
	ErrProductNotFound:     "商品不存在",
	ErrPriceRecordNotFound: "价格记录不存在",
	ErrNotifLogNotFound:    "通知记录不存在",
	ErrInvalidURL:          "无效的商品 URL",
	ErrInvalidPlatform:     "不支持的平台",
	ErrInvalidInterval:     "无效的检查间隔",
	ErrInvalidCronExpr:     "无效的 Cron 表达式",
	ErrFetchFailed:         "价格抓取失败",
}

type ShopCheckerError struct {
	Code    int
	Message string
}

func (e *ShopCheckerError) Error() string {
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

func NewShopCheckerError(code int, args ...any) *ShopCheckerError {
	msg := errMsgs[code]
	if len(args) > 0 {
		msg = fmt.Sprintf(msg, args...)
	}
	return &ShopCheckerError{Code: code, Message: msg}
}