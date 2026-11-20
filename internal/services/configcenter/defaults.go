package configcenter

// DefaultSpecs 返回 V3.0 核心配置项。模块迁移时继续向 Registry 注册自己的 Spec。
func DefaultSpecs() []Spec {
	zero := 0.0
	minOne := 1.0
	maxHundred := 100.0
	maxRate := 100000.0
	maxRetention := 3650.0
	maxUpload := 1024.0

	return []Spec{
		{Key: "server.public_base_url", Module: "server", Type: TypeString, Default: "", Reloadable: true, Group: "server", Description: "对外访问地址，用于第三方登录回调等"},
		{Key: "server.mode", Module: "server", Type: TypeString, Default: "debug", Enum: []string{"debug", "release", "test"}, Reloadable: false, Group: "server", Description: "Gin 运行模式"},
		{Key: "log.level", Module: "log", Type: TypeString, Default: "info", Enum: []string{"debug", "info", "warn", "error"}, Reloadable: true, Group: "log", Description: "日志级别"},
		{Key: "log.format", Module: "log", Type: TypeString, Default: "json", Enum: []string{"json", "text"}, Reloadable: true, Group: "log", Description: "日志格式"},
		{Key: "cache.default_ttl", Module: "cache", Type: TypeDuration, Default: "5m", Reloadable: true, Group: "cache", Description: "默认缓存 TTL"},
		{Key: "scheduler.worker_concurrency", Module: "scheduler", Type: TypeInt, Default: 5, Min: &minOne, Max: &maxHundred, Reloadable: true, Group: "scheduler", Description: "调度器并发数"},
		{Key: "integration.rate_limit.requests_per_minute", Module: "integration", Type: TypeInt, Default: 600, Min: &minOne, Max: &maxRate, Reloadable: true, Group: "integration", Description: "集成动作/MCP 每 Key 每分钟限流"},
		{Key: "integration.retention_days", Module: "integration", Type: TypeInt, Default: 30, Min: &zero, Max: &maxRetention, Reloadable: true, Group: "integration", Description: "已投递事件保留天数"},
		{Key: "security.upload_max_mb", Module: "security", Type: TypeInt, Default: 100, Min: &minOne, Max: &maxUpload, Reloadable: true, Group: "security", Description: "上传文件大小上限（MB）"},
		{Key: "auth.allow_registration", Module: "auth", Type: TypeBool, Default: false, Reloadable: true, Group: "auth", Description: "允许自助注册；默认关闭，由管理员创建用户"},
		{Key: "auth.login_max_failures", Module: "auth", Type: TypeInt, Default: 5, Min: &minOne, Max: &maxHundred, Reloadable: true, Group: "auth", Description: "登录连续失败多少次数后锁定"},
		{Key: "auth.login_lock_minutes", Module: "auth", Type: TypeInt, Default: 15, Min: &minOne, Max: &maxRetention, Reloadable: true, Group: "auth", Description: "登录锁定分钟数"},
		{Key: "goodser.jd.username", Module: "goodser", Type: TypeString, Default: "", Reloadable: false, Group: "business", Description: "京东账号（数据库保存，不写 env）"},
		{Key: "goodser.jd.password", Module: "goodser", Type: TypeSecret, Default: "", Sensitive: true, Reloadable: false, Group: "business", Description: "京东密码（加密保存，不写 env）"},
	}
}
