# 模块开发 SOP 自检清单

> 适用于新模块与老模块重构（feature-V2 P8 之后）。执行 `make sop-check` 自动检查第 1、2、3、4、5 项。

## 1. 建表与命名

- [ ] 表结构写在 `migrations/mysql/NNN_*.sql`，模块内**不得**再有 `initTables()`
- [ ] 表名使用 `zzz_<模块名>_<实体>` 前缀（历史表已由 `009_rename_legacy_tables.sql` 统一）
- [ ] DDL 注明表用途与字段含义；新表登记进 developmentManager 数据字典（可扫描导入）
- [ ] 迁移只追加新文件，不修改已应用版本

## 2. 代码质量

- [ ] `gofmt -l` 无输出
- [ ] `go vet ./...` 通过
- [ ] `go test -short ./...` 通过（在线联调用 `testing.Short()` 跳过）
- [ ] 关键分支有单测：校验、状态机、纯函数、错误码映射
- [ ] 错误码段已登记，不与其他模块冲突

## 3. 模块文档

- [ ] `docs/modules/<name>/index.md`
- [ ] `01-architecture-overview.md`
- [ ] `02-data-model.md`（表名与迁移文件对应）
- [ ] `03-api-design.md`（路由、参数、错误码）
- [ ] `04-module-structure.md`（文件组织、生命周期、已知限制）
- [ ] 文档与代码同提交；结构性变更同步更新，不保留已删除代码的行号引用

## 4. Swagger

- [ ] 每个注册路由都有 `@Router` 注解（含 REST 与 legacy 双入口时使用多条 `@Router`）
- [ ] `make swagger` 成功，`api/swagger/swagger.json` 覆盖模块全部路径
- [ ] 请求体/响应使用真实模型（`response.Response{data=...}`）

## 5. 关联与统一管理

- [ ] 项目登记在 developmentManager 台账（基座/模板/模块/容器项目类型正确）
- [ ] 模块文档与历史需求已登记资产索引
- [ ] 历史模块表已登记数据字典
- [ ] 前端模块在 Go-Frontend-Core 登记页面与菜单

## 6. 开发环境（仅当前 Windows 设备）

- [ ] 命令使用 Git Bash；复杂命令写入 `.sh` 脚本执行
- [ ] 后端启动前 `export EASY_TIER_NAS_URL=et.hailong.site`
- [ ] 迁移通过 `POST /api/v1/migrate/schema/migrate` 应用

## 自动检查

```bash
make sop-check
# 或
bash scripts/sop-check.sh
```

检查项：gofmt、`initTables` 残留、模块文档索引、`go vet`、`go test -short`。
