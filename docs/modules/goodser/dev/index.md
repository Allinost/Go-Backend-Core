# Goodser Phase 3 开发文档

## 文档目录

| 文件 | 内容 |
|------|------|
| [01-architecture-overview.md](01-architecture-overview.md) | 架构总览、目录结构、依赖关系、命名隔离策略 |
| [02-data-model.md](02-data-model.md) | 9 张数据库表设计、Go 数据模型、Rust→Go 类型映射 |
| [03-api-design.md](03-api-design.md) | Legacy 和 RESTful 两端 API 定义、路由注册方式 |
| [04-module-structure.md](04-module-structure.md) | Go 模块文件组织、三层架构、关键业务逻辑实现要点 |
| [05-implementation-plan.md](05-implementation-plan.md) | 实施步骤、工作量估算、配置变更、main.go 注册 |
| [06-rust-to-go-migration.md](06-rust-to-go-migration.md) | 代码迁移对照、数据库差异、测试迁移方法 |

## 关键决策

1. **代码放置在 `zzz/` 目录**: 与框架 `internal/`、`pkg/` 平级，标识为"实际业务代码"
2. **数据库/表添加 `zzz_` 前缀**: 区分业务数据库与框架数据库
3. **模块名 `goodser`**: 路由自动挂载至 `/api/v1/goodser/...`
4. **独立数据库 `zzz_goodser`**: 与框架库 `go_backend_core` 物理隔离
5. **兼容 Legacy 端点**: 微信小程序前端无需修改即可切换至 Go 后端

## 前置依赖

- [x] Phase 0: 项目脚手架
- [x] Phase 1: 核心基础设施（数据库连接池、缓存、日志、配置等）
- [ ] Phase 2: 社交登录（可选依赖，Goodser 白名单模式可独立运行）

## P1.5 交付状态（2026-11）

- 退货单：独立单据、多商品多选、商品级回库开关、仅退款/退货退款/运损/其他原因，确认与取消接口已完成；Web 与小程序均已接入。
- 统计：`GET /api/v1/zzz-goodser/inventories/:id/stats?days=30` 返回库存数量与价值、订单/退货状态数、按天趋势、系列分布、热销商品；集成动作 `goodser.stats.summary` 同步开放。
- 小程序：新增 `pages/returns/index`、`pages/returns/create`、`pages/stats/index`，出库单详情可直接申请退货并查看退货记录。
- Web：Goodser 模块新增「统计」Tab。

## 快速开始

```bash
# 1. 创建数据库
mysql -e "CREATE DATABASE IF NOT EXISTS zzz_goodser"
mysql zzz_goodser < migrations/mysql/002_zzz_goodser_init.up.sql

# 2. 启动服务
go run cmd/server/main.go

# 3. 验证
curl http://localhost:29090/api/v1/goodser/legacy/loadInventories
```
