# 第三方登录模块开发文档

## 文档目录

| 文件 | 内容 |
|------|------|
| [01-architecture-overview.md](01-architecture-overview.md) | 模块定位、三种登录方式、会话健康检查降级链、依赖关系 |
| [02-data-model.md](02-data-model.md) | `zzz_login3rd_platforms` / `zzz_login3rd_accounts` 表、Go 模型、DTO |
| [03-api-design.md](03-api-design.md) | 平台/账号/登录/会话/远程浏览器全部 API 与错误码 |
| [04-module-structure.md](04-module-structure.md) | 文件组织、Provider 接口、chromedp 浏览器层、加密与安全 |

## 关键设计决策

1. **平台可配置**：平台元数据（登录页、表单选择器、成功判定 Cookie）存 `zzz_login3rd_platforms` 表并 seed，新增平台主要改数据而非代码；7 个平台开箱即用（jd/qixian/jdm/taobao/pdd/_1688/tjupt）。
2. **三种登录方式**：`auto`（chromedp 填表自动登录）、`remote`（远程/本地浏览器人工登录，WS 推截图 + 指令驱动）、`manual`（手动登录抓 Cookie）。
3. **密码 AES-256-GCM 加密入库**：密钥取 `crypto.master_key_hex`（hex 解码），为空时回退 32 字节零密钥（仅非生产）；密文字段 `json:"-"` 不回传 API。
4. **会话健康检查降级链**：`verify → refresh → auto login → 远程登录链接 + webhook 告警`，由 centralized scheduler 按 24h 执行（`login3rd_session_check` 任务）。
5. **Provider 接口抽象**：`AutoLogin / Verify / Refresh` 三方法；京东不支持密码登录与 refresh，实现直接返回 `ErrNotSupported`，`jd/qixian/jdm` 三平台复用同一实例。
6. **跨模块服务暴露**：`GetService()` 供 `shopchecker` 做会话注入、失效同步（`MarkExpired`）与采集成功后会话延寿（`TouchSession`）。
7. **远程会话短时效**：token 会话 5 分钟过期，Webhook 告警附带一次性登录链接。
8. **原生 SQL 而非 GORM**：DDL 内联在 `module.go`，model 用 `db:`/`json:` tag。
