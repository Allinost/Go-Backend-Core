package apierror

const (
	CodeSuccess    = 0
	CodeUnknown    = -1
	CodeSystemErr  = 1000
	CodeParamErr   = 1001
	CodeNotFound   = 1002
	CodeUnauth     = 1003
	CodeForbidden  = 1004
	CodeRateLimit  = 1005
	CodeConflict   = 1006
	CodeTimeout    = 1007
	CodeBadGateway = 1008
)

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

func CodeMsg(code int) string {
	if msg, ok := codeMessages[code]; ok {
		return msg
	}
	return "未知错误"
}
