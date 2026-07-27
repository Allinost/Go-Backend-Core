# Webhook 公共模块开发文档

## 文档目录

| 文件 | 内容 |
|------|------|
| [01-architecture-overview.md](01-architecture-overview.md) | 模块定位、架构总览、支持类型、核心能力、集成方式 |
| [02-data-model.md](02-data-model.md) | 数据表设计（webhook_configs / delivery_logs / rate_limits）、Go 模型、DTO |
| [03-api-design.md](03-api-design.md) | 配置管理 API、投递 API、日志查询、统计 |
| [04-module-structure.md](04-module-structure.md) | 模块文件组织、Sender/限流/去重实现、业务模块集成示例 |

## 关键设计决策

1. **公共模块定位**：Webhook 作为公共服务模块，所有业务模块统一通过它发送 Webhook，不自建配置
2. **多类型支持**：内置飞书、钉钉、企微、GitHub、通用 HTTP 五种 Sender，可扩展
3. **安全治理**：敏感字段自动脱敏展示，支持签名/HMAC/BasicAuth 多种鉴权
4. **频率控制**：Redis 滑动窗口限流 + 内容去重，防止重复投递和 Webhook 被封
5. **白名单过滤**：关键词匹配，只有满足条件的消息才投递
6. **模块间集成**：通过 `modules.Get("webhook")` 获取 Service 实例，或通过 EventBus 异步解耦

## 对 ShopChecker 的影响

- ShopChecker 不再需要 `zzz_shopchecker_channels` 表，改用 `zzz_webhook_configs` + tags
- 商品自定义 Webhook 字段改为存储 `webhook_id` 数组，而非完整 URL
- ShopChecker 的降价通知通过 `webhook.Service.Send()` 实现，复用该模块的限流、重试、日志能力