# V3 监控与告警

## 1. 组件

| 组件 | 端口 | 说明 |
|---|---|---|
| `/metrics` | 29092 | Go 应用 Prometheus 指标 |
| Prometheus | 9090 | 指标采集与告警规则 |
| postgres-exporter | 9187 | PG 连接/复制/死锁指标 |
| Grafana | 3000 | 面板 |

启动：

```bash
docker compose -f docker-compose.monitoring.yml up -d
```

PG exporter DSN 通过环境变量注入，**不要把生产密码写进 compose 文件**：

```bash
export PG_EXPORTER_DSN='postgresql://user:pass@host:5432/db?sslmode=disable'
```

## 2. 采集

`deploy/prometheus/prometheus.yml` 已包含：

- `prometheus`
- `go-backend-core` → `host.docker.internal:29092/metrics`
- `postgres` → `postgres-exporter:9187`

告警规则：`deploy/prometheus/alerts.yml`。

## 3. 告警项

| 告警 | 条件 | 级别 |
|---|---|---|
| AppDown | `up{job="go-backend-core"}==0` 2m | critical |
| AppHigh5xxRate | 5xx 比例 > 5% 5m | warning |
| AppDBPoolExhausted | in_use > 90% open 5m | warning |
| PostgresDown | `pg_up==0` 1m | critical |
| PostgresConnectionsHigh | 连接使用率 > 80% 5m | warning |
| PostgresReplicationLag | lag > 60s 2m | critical |
| PostgresDeadlocks | 10m 内出现死锁 | warning |
| PostgresSlowQueries | `pg_stat_statements` 耗时上升 | warning |

## 4. 面板

Grafana 数据源与 dashboard 已由 `deploy/grafana/provisioning` 自动加载。

建议看板：

- 应用：QPS、p95、5xx、goroutines、DB 连接池
- PG：连接数、事务/秒、死锁、复制延迟、数据库大小
- 主机：CPU/内存/磁盘（Node Exporter，可选）

## 5. 慢查询

启用 `pg_stat_statements` 后：

```sql
SELECT query, calls, total_exec_time, mean_exec_time
FROM pg_stat_statements
ORDER BY total_exec_time DESC
LIMIT 20;
```

结合 `scripts/pg-perf.sh` 的 EXPLAIN 输出定位索引问题。

## 6. 注意

- 监控 compose 与应用 compose 分开，端口不要直接暴露公网。
- Grafana 默认 `admin/admin` 仅用于首次启动，上线前必须修改。
- `PG_EXPORTER_DSN` 使用只读监控账号更安全。
