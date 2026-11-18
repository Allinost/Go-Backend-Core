# V3 数据迁移规则

> 上游：[V3 SOP §8](../../docs/projects/go-backend-core/v3/SOP.md)

## 1. 原则

1. V2 老机器继续运行但**全面冻结**（只安全/数据修复）；V3 新机器独立，V3 是唯一开发线。
2. V3 是用户/RBAC/项目管理唯一事实源；bootstrap 一次性迁移，不做双向同步。
3. V3.0 只迁移基座/公共/PM 数据；业务模块数据不迁。
4. 业务模块按模块迁移（V3.1+），每个模块单独冻结/快照/校验/切路由。
5. 不做双写；不在 V2/V3 同时写同一业务数据。
6. 所有迁移可重复执行；失败可清空目标重来。
7. 迁移前必须备份；迁移后必须校验。

## 2. V3.0 allowlist

**允许：**

- `users`、`roles`、`permissions`、`role_permissions`、`user_roles`、`api_keys`
- `scheduler_tasks`、`scheduler_task_logs`（active + 近期）
- `file_references`、`scan_targets`
- `audit_log`（可选）、`feature_flags`
- `zzz_integration_connectors`、`zzz_integration_outbox`、`zzz_integration_action_logs`（近期）
- `zzz_webhook_configs`、`zzz_webhook_delivery_logs`（近期）
- `zzz_servicenav_item`、`zzz_servicenav_type`
- `zzz_development_manager_*`（15 张）
- `data/settings.json`

**禁止：**

- `zzz_goodser_*`
- `zzz_ia_*`
- `zzz_login3rd_*`
- `zzz_shopchecker_*`
- 任何未在 allowlist 的表

## 3. 步骤

1. `v3migrate plan`：确认表清单/行数/依赖顺序/大小。
2. 备份 V2；记录快照时间。
3. `v3migrate dump`：按表导出 NDJSON/CSV。
4. `v3migrate load`：按拓扑顺序 COPY/批量写入。
5. `setval` 重置序列；`ANALYZE`。
6. `v3migrate verify`：行数/主键/FK/JSON/时间/金额/抽样/序列。
7. 生成报告；人工签字。
8. 启动 V3；健康检查 + 冒烟。
9. 观察 7 天；保留 V2 快照。

## 4. 校验

| 项 | 标准 |
|---|---|
| 行数 | 完全一致 |
| 主键 | 无重复；`max(id)` < 序列 |
| 外键 | 0 孤儿 |
| JSON | 全部合法 |
| 时间 | UTC 语义一致 |
| 金额 | `SUM/MIN/MAX` 一致 |
| 抽样 | 每模块 5–10 条查询一致 |
| 序列 | `nextval` 正常 |

## 5. 回滚

- V3.0 切换窗口内：停 V3，恢复 V2 PM 写入，V3 PG 可清空重来。
- 切换后 7 天内：保留 V2 快照；不自动反向同步；如需回滚，人工评估丢失窗口。
- 业务模块迁移：保留 V2 模块数据 ≥ 7 天；路由可切回 V2。

## 6. 禁止

- 未备份执行迁移。
- 迁移业务模块表（V3.0）。
- 双写。
- 手工改数据不记录。
- 跳过校验/签字。
