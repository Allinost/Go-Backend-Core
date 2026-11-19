package auth

import "sync/atomic"

// 默认关闭自助注册；部署后由配置中心 auth.allow_registration 覆盖。
var allowRegistration atomic.Bool

func SetAllowRegistration(allow bool) {
	allowRegistration.Store(allow)
}

func AllowRegistration() bool {
	return allowRegistration.Load()
}
