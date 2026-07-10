package apierror

// 预定义的 API 错误码常量
const (
	CodeSuccess    = 0    // 成功
	CodeUnknown    = -1   // 未知错误
	CodeSystemErr  = 1000 // 系统内部错误
	CodeParamErr   = 1001 // 请求参数错误
	CodeNotFound   = 1002 // 资源不存在
	CodeUnauth     = 1003 // 未登录/未授权
	CodeForbidden  = 1004 // 权限不足
	CodeRateLimit  = 1005 // 请求频率限制
	CodeConflict   = 1006 // 资源冲突（已存在）
	CodeTimeout    = 1007 // 请求超时
	CodeBadGateway = 1008 // 上游服务不可达
)

// codeMessages 错误码到默认消息的映射
var codeMessages = map[int]string{
	CodeSuccess:    "success",
	CodeUnknown:    "系统繁忙，请稍后再试",
	CodeSystemErr:  "系统内部错误",
	CodeParamErr:   "请求参数错误",
	CodeNotFound:   "资源不存在",
	CodeUnauth:     "请先登录",
	CodeForbidden:  "权限不足",
	CodeRateLimit:  "请求过于频繁，请稍后再试",
	CodeConflict:   "资源已存在",
	CodeTimeout:    "请求超时",
	CodeBadGateway: "上游服务不可达",
}

// CodeMsg 根据错误码获取对应的中文错误消息
func CodeMsg(code int) string {
	if msg, ok := codeMessages[code]; ok {
		return msg
	}
	return "未知错误"
}
