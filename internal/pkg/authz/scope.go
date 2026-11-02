// Package authz 定义集成作用域与匹配规则，供 HTTP 中间件、Action Gateway 共用。
package authz

import "strings"

// 集成作用域约定：
//
//	owner:read              读取库存、订单、退货、统计
//	owner:write             库存、订单、退货等写操作
//	customer:read           客户服务只读查询
//	customer:order_request  客户提交订单请求（不直接改库存）
//	integration:manage      管理连接器、事件与集成配置
const (
	ScopeOwnerRead         = "owner:read"
	ScopeOwnerWrite        = "owner:write"
	ScopeCustomerRead      = "customer:read"
	ScopeCustomerOrderReq  = "customer:order_request"
	ScopeIntegrationManage = "integration:manage"
)

// Has 判断作用域字符串是否满足任一 required。
// 支持 *、*:*、namespace:* 以及逗号/空格/分号分隔的多个作用域。
func Has(raw string, required ...string) bool {
	if len(required) == 0 {
		return true
	}
	for _, scope := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ' ' || r == ';'
	}) {
		scope = strings.TrimSpace(scope)
		if scope == "" {
			continue
		}
		if scope == "*" || scope == "*:*" {
			return true
		}
		for _, need := range required {
			if scope == need {
				return true
			}
			if strings.HasSuffix(scope, ":*") {
				prefix := strings.TrimSuffix(scope, "*")
				if strings.HasPrefix(need, prefix) {
					return true
				}
			}
		}
	}
	return false
}
