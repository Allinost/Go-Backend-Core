# V3 部署手册（Go-Backend-Core）

> 适用：单机 PostgreSQL 18、Docker/二进制部署；L1 主机、L2 备份、ACloud 云服务器。

## 1. 前置

- Go 1.24+（构建机）
- PostgreSQL 18，数据库 `GoBackendCoreDatabase`
- Redis（L1/L2 本地）
- RustFS（固定在 ACloud）
- 反向代理/证书（1Panel 或 OpenResty，域名待定）

## 2. 关键 env

```env
APP_PROFILE=host
APP_DATABASE_POSTGRES_MAIN_HOST=${L1_INTRANET}
APP_DATABASE_POSTGRES_MAIN_PORT=58432
APP_DATABASE_POSTGRES_MAIN_USER=GoBackendCoreDatabase
APP_DATABASE_POSTGRES_MAIN_PASSWORD=...
APP_DATABASE_POSTGRES_MAIN_DBNAME=GoBackendCoreDatabase
APP_DATABASE_POSTGRES_MAIN_SEARCH_PATH=gobackendcore,public
APP_DATABASE_FAILOVER_MODE=manual
APP_CRYPTO_MASTER_KEY_HEX=...
# V3 默认不加载 V2 MySQL 业务模块，也不连接 MySQL
APP_LEGACY_MODULES_ENABLED=false
APP_DATABASE_MYSQL_MAIN_ENABLED=false
```

- `search_path` 必须包含专用 schema；应用账号无需 `public` 建表权限。
- `APP_CRYPTO_MASTER_KEY_HEX` 用于配置中心敏感项和主机凭据；丢失将无法解密。
- L2 使用 `APP_PROFILE=backup`，PG/Redis 指向 `${L2_INTRANET}`。
- RustFS 始终在 ACloud，不随 L1/L2 切换。

## 3. 迁移

```http
POST /api/v1/migrate/schema/migrate
GET  /api/v1/migrate/schema/migrations
```

或启动后由 migrate 模块自动创建 schema 与 `_migrations`，再执行接口。

生产必须 `APP_DATABASE_AUTO_MIGRATE=false`，Schema 唯一由 `migrations/postgres` 管理。

## 4. 初始化管理员

```powershell
$env:V3_ADMIN_PASSWORD='至少8位密码'
go run ./cmd/v3admin -username admin -nickname 管理员
```

- 默认关闭自助注册；需要时在配置中心打开 `auth.allow_registration`。
- 登录失败锁定：`auth.login_max_failures` / `auth.login_lock_minutes`。

## 5. V3 台账

```powershell
go run ./cmd/v3ledger -dry-run
go run ./cmd/v3ledger
```

导入 61 任务/61 依赖，按 `code` 幂等。

## 6. V2 数据 bootstrap

```powershell
go run ./cmd/v3migrate plan
go run ./cmd/v3migrate dump -file backups/v3_bootstrap.json
go run ./cmd/v3migrate rehearsal -file backups/v3_bootstrap.json -schema v3_rehearsal_check
go run ./cmd/v3migrate load -file backups/v3_bootstrap.json
go run ./cmd/v3migrate verify
go run ./cmd/v3migrate report -out reports/v3_bootstrap_report.md
```

- 先演练，再正式 load。
- allowlist 外业务表默认拒绝。
- 行数/孤儿关联/FK 由 verify/rehearsal 校验。

## 7. 健康检查

```http
GET /api/v1/health/ping
GET /api/v1/health/health
GET /api/v1/health/metrics
GET /api/v1/health/modules
```

- 主 API：`:29090`
- 调试/Swagger/health：`:29092`
- 反向代理只需暴露 `:29090` 的业务 API 与必要静态资源。

## 8. 备份

```bash
export PGHOST=... PGPORT=... PGUSER=... PGDATABASE=... PGPASSWORD=...
./scripts/pg-backup.sh
```

详见 `docs/runbooks/v3-backup-restore.md`。

## 9. 维护切换

```http
GET  /api/v1/ops/maintenance
POST /api/v1/ops/maintenance/start
POST /api/v1/ops/maintenance/end
```

- `shutdown`：停 L1，直接回 L1。
- `migrate_return`：L1→L2 同步确认，结束前 L2→L1 反向同步确认。
- `migrate_no_return`：L2 长期运行，L1 退役。
- 不做运行中热切连接；L2 上以 `APP_PROFILE=backup` 启动 V3。

## 10. 多主机台账

```http
GET/POST /api/v1/ops/hosts
GET/PUT/DELETE /api/v1/ops/hosts/:id
POST /api/v1/ops/hosts/:id/test
GET /api/v1/ops/hosts/:id/status
```

- 凭据 AES-GCM 加密，接口只回 `credential_hint`。
- V3.0 状态采集只执行固定白名单命令；known_hosts 指纹校验后续补。
