package response

import (
	"net/http"

	"github.com/Allinost/go-backend-core/internal/config"
	appErr "github.com/Allinost/go-backend-core/internal/pkg/errors"
	"github.com/gin-gonic/gin"
)

// Response 统一 JSON 响应结构
type Response struct {
	Code    int         `json:"code"`              // 业务错误码（0 表示成功）
	Message string      `json:"message"`           // 提示信息
	Data    interface{} `json:"data,omitempty"`     // 响应数据
	Detail  string      `json:"detail,omitempty"`  // 错误详情（仅开发环境可见）
	TraceID string      `json:"trace_id,omitempty"`// 请求追踪 ID
}

// Success 成功响应
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    appErr.CodeSuccess,
		Message: appErr.CodeMsg(appErr.CodeSuccess),
		Data:    data,
	})
}

// SuccessWithMsg 成功响应（自定义提示文案）
func SuccessWithMsg(c *gin.Context, data interface{}, msg string) {
	c.JSON(http.StatusOK, Response{
		Code:    appErr.CodeSuccess,
		Message: msg,
		Data:    data,
	})
}

// Fail 失败响应（使用 AppError）
func Fail(c *gin.Context, err *appErr.AppError) {
	httpStatus := appErr.CodeHTTPStatus(err.Code)

	resp := Response{
		Code:    err.Code,
		Message: err.Message,
	}

	// 开发环境下暴露错误详情
	if config.Get() != nil && config.Get().Server.Mode != "release" {
		resp.Detail = err.Detail
		if err.Err != nil {
			resp.Detail = err.Err.Error()
		}
	}

	c.AbortWithStatusJSON(httpStatus, resp)
}

// FailCode 直接使用错误码响应
func FailCode(c *gin.Context, code int) {
	Fail(c, appErr.New(code, ""))
}

// ParamErr 参数错误的快捷方式
func ParamErr(c *gin.Context, detail string) {
	err := appErr.New(appErr.CodeParamErr, "").WithDetail(detail)
	Fail(c, err)
}
