package authz

import "testing"

func TestHas(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		required []string
		want     bool
	}{
		{"空要求默认放行", "owner:read", nil, true},
		{"精确匹配", "owner:read,owner:write", []string{ScopeOwnerWrite}, true},
		{"任一匹配", "customer:read", []string{ScopeOwnerWrite, ScopeCustomerRead}, true},
		{"通配全部", "*:*", []string{ScopeOwnerWrite}, true},
		{"星号全部", "*", []string{ScopeCustomerOrderReq}, true},
		{"命名空间通配", "owner:*", []string{ScopeOwnerRead}, true},
		{"命名空间不越界", "customer:*", []string{ScopeOwnerRead}, false},
		{"分号分隔", "owner:read;integration:manage", []string{ScopeIntegrationManage}, true},
		{"无权限", "customer:read", []string{ScopeOwnerWrite}, false},
		{"空作用域", "", []string{ScopeOwnerRead}, false},
	}
	for _, c := range cases {
		if got := Has(c.raw, c.required...); got != c.want {
			t.Errorf("%s: Has(%q, %v)=%v，期望 %v", c.name, c.raw, c.required, got, c.want)
		}
	}
}
