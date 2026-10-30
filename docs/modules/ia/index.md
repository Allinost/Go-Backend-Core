# 投资分析模块开发文档

## 文档目录

| 文件 | 内容 |
|------|------|
| [01-architecture-overview.md](01-architecture-overview.md) | 模块定位、架构总览、核心能力、命名空间、依赖关系 |
| [02-data-model.md](02-data-model.md) | 13 张 `zzz_ia_*` 表设计、Go 模型、DTO |
| [03-api-design.md](03-api-design.md) | 账户/标的/交易/持仓、分析报表、筛选、任务等全部 API |
| [04-module-structure.md](04-module-structure.md) | 文件组织、分层实现、定时任务、quote 子包、错误码 |

## 关键设计决策

1. **持仓重放引擎**：持仓不靠增量更新，而是由交易记录**全量重放**推导（`service.go` 的 `replay`）。删除/修改交易后重放修正持仓，保证账实一致；成本核算支持 **WA（加权平均）/ FIFO 双法**，按账户配置选择。
2. **双法已实现收益**：`zzz_ia_realized_pnl` 记录每笔卖出的已实现收益，FIFO 记录批次来源（`lot_transaction_id`）。
3. **统一迁移**：13 张 `zzz_ia_*` 表已收敛到 `migrations/mysql/008_legacy_modules_schema.sql`（P8），模块启动不再自建表；存量列兼容逻辑已并入建表 DDL。
4. **原生 SQL 而非 GORM**：repository 层使用 `database/sql` + 手写 SQL，model 用 `db:`/`json:` tag（无 gorm tag）。
5. **行情源可插拔**：`quote.Provider` 接口 + `Registry` 注册表，东方财富/天天基金/免费汇率三类上游，全部经 `pkg/net` 带**重试与断路器**的 HTTP 客户端；主行情与全市场筛选使用**独立断路器**（互不拖累）。
6. **内部调度器 + 集中调度注册**：实际任务由模块内 ticker 驱动（工作日行情 30min、每日计息/快照/到期/风险巡检），同时向 centralized scheduler 注册占位任务，仅保证管理面可见性。
7. **可选依赖降级**：webhook、WebSocket Hub 未注册时不致命（仅告警日志），模块可独立运行。
8. **at-most-once 每日任务**：内存 `dailyDone` 标记当日已执行，任务 5 分钟超时 + panic recover，失败次日重试或手动触发（`/tasks/*` 接口）。
