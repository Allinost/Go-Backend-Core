# 文档索引

> Go-Backend-Core 项目的统一文档导航入口。
> **本目录（`docs/`）是文档的唯一事实源**；[`docs-page/`](docs-page/) 仅为 Jekyll 导航站点，正文不再维护（见文末说明）。

---

## 1. 架构与设计

| 文档 | 内容 |
|------|------|
| [architecture.md](architecture.md) | 整体架构图、启动流程、模块体系（15 个模块）、目录结构 |
| [tech-stack.md](tech-stack.md) | 技术选型与依赖说明 |
| [module-design.md](module-design.md) | 模块接口、注册表机制、三层结构、各模块设计（95KB，最详尽） |
| [data-model.md](data-model.md) | 数据库策略、表设计、命名规范 |
| [deployment.md](deployment.md) | 部署方案与环境管理 |
| [test.md](test.md) | 测试规范（单元/集成/覆盖率） |
| [development-plan.md](development-plan.md) | 开发规划与阶段安排 |
| [module-sop.md](module-sop.md) | 模块开发 SOP 自检清单（建表/测试/Swagger/文档） |
| [social-login.md](social-login.md) | 第三方 OAuth 登录接入方案 |
| [login3rd-plan.md](login3rd-plan.md) | 第三方登录模块实施计划 |

---

## 2. 业务模块文档

按模块目录组织，标准套件为 `index + 01~04`（架构总览 / 数据模型 / API 设计 / 模块结构）：

| 模块 | 代码位置 | 路由前缀 | 文档 |
|------|----------|----------|------|
| 投资分析 | `zzz/ia` | `/api/v1/zzz-ia` | [modules/ia/](modules/ia/) ✅ |
| 第三方登录 | `zzz/login3rd` | `/api/v1/login3rd` | [modules/login3rd/](modules/login3rd/) ✅ |
| 店铺采集 | `zzz/shopchecker` | `/api/v1/shopchecker` | [modules/shopChecker/](modules/shopChecker/) ✅ |
| 商品服务 | `zzz/goodser` | `/api/v1/goodser` | [modules/goodser/dev/](modules/goodser/dev/) ✅ |
| 项目开发管理 | `zzz/developmentManager` | `/api/v1/developmentmanager` | [modules/developmentManager/](modules/developmentManager/) ✅（P1 台账 + P2 部署资产 + P3 数据字典 + P4 需求管理 + P5 生成流程 + P6 前端 Web；含容器项目与容器管理页，规划见 [plans/feature-V2-project-dev-management.md](plans/feature-V2-project-dev-management.md)） |
| 服务导航 | `internal/modules/servicenav` | `/api/v1/servicenav` | [modules/servicenav/](modules/servicenav/) ✅ |
| Webhook | `internal/modules/webhook` | `/api/v1/webhook` | [modules/webhook/](modules/webhook/) ✅ |
| 认证 / 运维 / 迁移等 | `internal/modules/{auth,ops,migrate,s0,s1,s2}` | `/api/v1/{name}` | ⬜ 暂无独立文档，见 [API/index.md](API/index.md) |

---

## 3. API 文档

- **交互式 API 文档**：启动服务后访问 `http://localhost:29092/swagger/index.html`（调试端口，支持环境预设切换）
- **静态索引**：[API/index.md](API/index.md)（通用约定、统一响应格式、错误码、模块路由表）

---

## 4. 过程文档

| 目录 | 内容 |
|------|------|
| [release/](release/) | 发布记录（P0~P17 里程碑清单） |
| [weekly/](weekly/) | 周报与迭代总结 |
| [docs-page/](docs-page/) | Jekyll 导航站点（仅导航，正文已指向本文档） |

---

## 5. 维护约定

1. **单一事实源**：文档正文只在本目录（`docs/`）维护；`docs-page/` 下的同名文档是说明页，不放正文。
2. **模块文档规范**：新模块按 `docs/modules/{name}/` 建目录，包含：
   - `index.md` — 文档目录 + 关键设计决策
   - `01-architecture-overview.md` — 模块定位、分层架构、命名空间、依赖关系
   - `02-data-model.md` — 表结构、Go 模型、DTO
   - `03-api-design.md` — 路由表、请求/响应、业务错误码
   - `04-module-structure.md` — 文件组织、生命周期、已知限制
3. **同步要求**：模块代码发生结构性变更（新增路由、改表、改错误码）时同步更新对应文档。
4. **新增文档时**：在本索引的对应表格中登记。
5. **开发 SOP（2026-10-29 起生效）**：所有项目开发按 **用户描述 → 规划 → 开发 → 文档** 四段式执行：
   - **用户描述**：需求点以原文记录、不改写（留档于规划文档的需求记录节）；
   - **规划**：讨论定稿、逐项决策落入决策表，**经用户核对确认后才进入开发**；
   - **开发**：分阶段实施、每阶段可独立验收、小步提交可追溯；
   - **文档**：与代码同提交（模块四件套 / 数据字典 / Swagger / 发布说明），不留事后补。
   - 首个执行范例：[plans/feature-V2-project-dev-management.md](plans/feature-V2-project-dev-management.md)（§0.1 SOP、§0.2 原始需求、§0.3 追溯表）。
   - 正式细则：[module-sop.md](module-sop.md)（**v1.0，2026-10-30 生效**），含四阶段流程、**缺陷修复流程**、编码/测试/Swagger/文档/台账标准与例外规则。
   - 自检门禁：`make sop-check`（gofmt / initTables / 文档索引 / vet / 短测）。
6. **开发环境（Windows）**：命令行统一使用 **Git Bash**（`& "C:\Program Files\Git\bin\bash.exe" -lc "<cmd>"`），不再用 PowerShell 执行开发命令。
   - 原因：PowerShell 默认编码/重定向会破坏中文（`Set-Content` 写出 ANSI、`>` 重定向乱码），且不支持三元运算符；Git Bash 原生 UTF-8，`go`/`git`/`grep`/`sed` 等命令与 Linux CI 一致。
   - 约定：写中文文件一律用编辑器工具或 heredoc（`cat > file <<'EOF'`）；禁止用 `Set-Content`/`Out-File` 改写含中文的源码与文档。
   - 注意：从 PowerShell 调用 bash 时，`-lc` 的参数用**单引号**包裹，避免 `$?`、`$变量` 被 PowerShell 提前展开（双引号下会得到 `True`/空值而不是脚本结果）。
   - 复杂命令（含管道、正则、内嵌引号、中文）**写入临时 `.sh` 脚本后执行 `bash script.sh`**，避免 PowerShell 与 bash 双层引号/转义差异导致命令被拆断。
   - **本机联调启动（仅适用于当前这台 Windows 设备，即 iMac 上的 Windows 环境）**：后端需先 `export EASY_TIER_NAS_URL=et.hailong.site` 再 `go run ./cmd/server`（`.env` 中 MySQL/Redis 地址引用该变量）；前端在 Go-Frontend-Core 执行 `npm run dev`（3000，Vite 代理 `/api` → `localhost:29090`）。其他设备/环境需按各自 `.env` 与网络条件调整，不可照搬该地址。
