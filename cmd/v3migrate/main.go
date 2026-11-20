// Command v3migrate V2 MySQL → V3 PostgreSQL 受控数据引导工具。
//
//	go run ./cmd/v3migrate plan
//	go run ./cmd/v3migrate dump -file backups/v3_bootstrap.json
//	go run ./cmd/v3migrate load -file backups/v3_bootstrap.json
//	go run ./cmd/v3migrate verify
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/Allinost/go-backend-core/internal/database/mysql"
	"github.com/Allinost/go-backend-core/internal/database/postgres"
	"github.com/Allinost/go-backend-core/internal/services/migrate"
)

type tableSpec struct {
	Source string
	Target string
}

// tableSpecs 只迁移 V3.0 需要的表；业务模块表（goodser/ia/login3rd/shopchecker）默认拒绝。
// Source 是 V2 表名，Target 是 V3 表名（部分 GORM 表已改名）。
var tableSpecs = []tableSpec{
	{"users", "users"}, {"role_models", "roles"}, {"permissions", "permissions"},
	{"role_permissions", "role_permissions"}, {"user_roles", "user_roles"}, {"api_keys", "api_keys"},
	{"tasks", "scheduler_tasks"}, {"task_logs", "scheduler_task_logs"},
	{"file_references", "file_references"}, {"scan_targets", "scan_targets"},
	{"audit_log", "audit_log"}, {"feature_flags", "feature_flags"},
	{"zzz_integration_connectors", "zzz_integration_connectors"},
	{"zzz_integration_outbox", "zzz_integration_outbox"},
	{"zzz_integration_action_logs", "zzz_integration_action_logs"},
	{"zzz_webhook_configs", "zzz_webhook_configs"},
	{"zzz_webhook_delivery_logs", "zzz_webhook_delivery_logs"},
	{"zzz_servicenav_type", "zzz_servicenav_type"}, {"zzz_servicenav_item", "zzz_servicenav_item"},
	{"zzz_development_manager_project", "zzz_development_manager_project"},
	{"zzz_development_manager_project_relation", "zzz_development_manager_project_relation"},
	{"zzz_development_manager_project_asset", "zzz_development_manager_project_asset"},
	{"zzz_development_manager_requirement", "zzz_development_manager_requirement"},
	{"zzz_development_manager_requirement_version", "zzz_development_manager_requirement_version"},
	{"zzz_development_manager_container", "zzz_development_manager_container"},
	{"zzz_development_manager_resource_alloc", "zzz_development_manager_resource_alloc"},
	{"zzz_development_manager_volume_mapping", "zzz_development_manager_volume_mapping"},
	{"zzz_development_manager_whitelist", "zzz_development_manager_whitelist"},
	{"zzz_development_manager_whitelist_member", "zzz_development_manager_whitelist_member"},
	{"zzz_development_manager_access_point", "zzz_development_manager_access_point"},
	{"zzz_development_manager_backup", "zzz_development_manager_backup"},
	{"zzz_development_manager_db_table", "zzz_development_manager_db_table"},
	{"zzz_development_manager_db_column", "zzz_development_manager_db_column"},
	{"zzz_development_manager_generation", "zzz_development_manager_generation"},
}

func sourceTables() []string {
	seen := make(map[string]bool)
	var out []string
	for _, s := range tableSpecs {
		if !seen[s.Source] {
			seen[s.Source] = true
			out = append(out, s.Source)
		}
	}
	return out
}

func existingSourceTables(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, `SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE()`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	have := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		have[strings.ToLower(name)] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var out []string
	for _, name := range sourceTables() {
		if have[strings.ToLower(name)] {
			out = append(out, name)
		}
	}
	return out, nil
}

func main() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "用法: v3migrate plan|dump|load|verify [-file path]")
	}
	flag.Parse()
	cmd := "plan"
	if flag.NArg() > 0 {
		cmd = flag.Arg(0)
	}
	file := ""
	out := "reports/v3_bootstrap_report.md"
	schema := "v3_rehearsal"
	keep := false
	flagSet := flag.NewFlagSet(cmd, flag.ExitOnError)
	flagSet.StringVar(&file, "file", "backups/v3_bootstrap.json", "dump/load 文件")
	flagSet.StringVar(&out, "out", out, "report 输出文件")
	flagSet.StringVar(&schema, "schema", "v3_rehearsal", "rehearsal 目标 schema")
	flagSet.BoolVar(&keep, "keep", false, "rehearsal 后保留 schema")
	_ = flagSet.Parse(flag.Args()[1:])

	cfg, err := config.Load("config.yaml")
	if err != nil {
		fail("加载配置失败: " + err.Error())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	mysqlCfg := cfg.Database.MySQL["main"]
	if mysqlCfg.DBName == "" {
		fail("MySQL main 未配置")
	}
	mysqlPool, err := mysql.NewPool(mysqlCfg, false)
	if err != nil {
		fail("连接 V2 MySQL 失败: " + err.Error())
	}
	defer mysqlPool.Close()
	pgPool, err := postgres.NewPool(cfg.Database.Postgres["main"], false)
	if err != nil {
		fail("连接 V3 PostgreSQL 失败: " + err.Error())
	}
	defer pgPool.Close()

	switch cmd {
	case "plan":
		if err := plan(ctx, mysqlPool.DB, pgPool.SQLDB); err != nil {
			fail(err.Error())
		}
	case "dump":
		if err := dump(ctx, mysqlPool.DB, file); err != nil {
			fail(err.Error())
		}
	case "load":
		if err := load(ctx, pgPool.SQLDB, file); err != nil {
			fail(err.Error())
		}
	case "verify":
		if err := verify(ctx, mysqlPool.DB, pgPool.SQLDB); err != nil {
			fail(err.Error())
		}
	case "rehearsal":
		if err := rehearsal(ctx, mysqlPool.DB, cfg, file, schema, keep); err != nil {
			fail(err.Error())
		}
	case "report":
		if err := report(ctx, mysqlPool.DB, pgPool.SQLDB, out); err != nil {
			fail(err.Error())
		}
	default:
		fail("未知命令: " + cmd)
	}
}

func plan(ctx context.Context, mysqlDB, pgDB *sql.DB) error {
	fmt.Printf("%-46s %-46s %10s %10s\n", "source", "target", "v2_rows", "v3_rows")
	var totalV2, totalV3 int64
	for _, spec := range tableSpecs {
		v2, err := countRows(ctx, mysqlDB, "`"+spec.Source+"`")
		if err != nil {
			fmt.Printf("%-46s %-46s %10s %10s  (V2 不存在)\n", spec.Source, spec.Target, "-", "-")
			continue
		}
		v3, _ := countRows(ctx, pgDB, `"`+spec.Target+`"`)
		totalV2 += v2
		totalV3 += v3
		fmt.Printf("%-46s %-46s %10d %10d\n", spec.Source, spec.Target, v2, v3)
	}
	fmt.Printf("%-46s %-46s %10d %10d\n", "TOTAL", "", totalV2, totalV3)
	return nil
}

func dump(ctx context.Context, mysqlDB *sql.DB, file string) error {
	tables, err := existingSourceTables(ctx, mysqlDB)
	if err != nil {
		return fmt.Errorf("读取 V2 表清单失败: %w", err)
	}
	if len(tables) == 0 {
		return fmt.Errorf("V2 没有可导出的 allowlist 表")
	}
	adapter := migrate.NewMySQLAdapter(mysqlDB)
	result, err := migrate.Dump(ctx, adapter, migrate.DumpOptions{
		Tables: tables, Format: migrate.FormatJSON, BatchSize: 1000,
	})
	if err != nil {
		return fmt.Errorf("导出失败: %w", err)
	}
	if err := os.MkdirAll(dirOf(file), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(file, result.Data, 0o644); err != nil {
		return err
	}
	fmt.Printf("导出完成：tables=%d rows=%d file=%s\n", result.TableCount, result.RowCount, file)
	return nil
}

func load(ctx context.Context, pgDB *sql.DB, file string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("读取 dump 失败: %w", err)
	}
	var dumpData map[string][]map[string]any
	if err := json.Unmarshal(data, &dumpData); err != nil {
		return fmt.Errorf("解析 dump 失败: %w", err)
	}
	return loadData(ctx, pgDB, dumpData)
}

func loadData(ctx context.Context, pgDB *sql.DB, dumpData map[string][]map[string]any) error {
	type pendingRow struct {
		table string
		row   map[string]any
	}
	// 这些表依赖用户/权限/任务 ID，先加载基础表，再按名称重映射。
	dependent := map[string]bool{
		"user_roles": true, "api_keys": true, "role_permissions": true, "scheduler_task_logs": true,
	}
	var pending []pendingRow
	var dependentRows []pendingRow
	inserted := 0
	failed := 0
	skipped := 0
	typeCache := make(map[string]colMeta)

	for _, spec := range tableSpecs {
		for _, row := range dumpData[spec.Source] {
			if dependent[spec.Target] {
				dependentRows = append(dependentRows, pendingRow{table: spec.Target, row: row})
				continue
			}
			ok, err := insertRow(ctx, pgDB, spec.Target, row, typeCache)
			if err != nil {
				if isForeignKeyError(err) {
					pending = append(pending, pendingRow{table: spec.Target, row: row})
				} else {
					failed++
					fmt.Printf("插入失败 %s: %v\n", spec.Target, err)
				}
				continue
			}
			if ok {
				inserted++
			}
		}
	}

	// 基础表加载完成后，按名称重映射依赖 ID。
	sourceUsers := idNameMap(dumpData["users"], "id", "username")
	sourcePermissions := idNameMap(dumpData["permissions"], "id", "name")
	sourceTasks := idNameMap(dumpData["tasks"], "id", "name")
	targetUsers := queryNameID(ctx, pgDB, "users", "username")
	targetPermissions := queryNameID(ctx, pgDB, "permissions", "name")
	targetTasks := queryNameID(ctx, pgDB, "scheduler_tasks", "name")
	for _, pr := range dependentRows {
		switch pr.table {
		case "user_roles", "api_keys":
			srcID := toInt64(pr.row["user_id"])
			name := sourceUsers[srcID]
			targetID, ok := targetUsers[name]
			if !ok || targetID == 0 {
				skipped++
				continue
			}
			pr.row["user_id"] = targetID
		case "role_permissions":
			srcID := toInt64(pr.row["permission_id"])
			name := sourcePermissions[srcID]
			targetID, ok := targetPermissions[name]
			if !ok || targetID == 0 {
				skipped++
				continue
			}
			pr.row["permission_id"] = targetID
		case "scheduler_task_logs":
			srcID := toInt64(pr.row["task_id"])
			name := sourceTasks[srcID]
			if name == "" {
				name = fmt.Sprint(pr.row["task_name"])
			}
			targetID, ok := targetTasks[name]
			if !ok || targetID == 0 {
				skipped++
				continue
			}
			pr.row["task_id"] = targetID
		}
		ok, err := insertRow(ctx, pgDB, pr.table, pr.row, typeCache)
		if err != nil {
			if isForeignKeyError(err) {
				pending = append(pending, pr)
			} else {
				failed++
				fmt.Printf("插入失败 %s: %v\n", pr.table, err)
			}
			continue
		}
		if ok {
			inserted++
		}
	}

	for pass := 0; pass < 3 && len(pending) > 0; pass++ {
		fmt.Printf("重试 FK 失败行：%d\n", len(pending))
		current := pending
		pending = nil
		for _, pr := range current {
			ok, err := insertRow(ctx, pgDB, pr.table, pr.row, typeCache)
			if err != nil {
				if isForeignKeyError(err) {
					pending = append(pending, pr)
				} else {
					failed++
					fmt.Printf("插入失败 %s: %v\n", pr.table, err)
				}
				continue
			}
			if ok {
				inserted++
			}
		}
	}
	for _, spec := range tableSpecs {
		_ = resetSequence(ctx, pgDB, spec.Target)
	}
	fmt.Printf("导入完成：inserted=%d failed=%d skipped=%d pending=%d\n", inserted, failed, skipped, len(pending))
	return nil
}

func report(ctx context.Context, mysqlDB, pgDB *sql.DB, out string) error {
	var b strings.Builder
	b.WriteString("# V3 Bootstrap Report\n\n")
	b.WriteString("Generated: " + time.Now().Format(time.RFC3339) + "\n\n")
	b.WriteString("| source | target | v2_rows | v3_rows | status |\n")
	b.WriteString("|---|---|---:|---:|---|\n")
	var totalV2, totalV3 int64
	var diffs int
	for _, spec := range tableSpecs {
		v2, err := sourceCount(ctx, mysqlDB, spec.Source)
		if err != nil {
			b.WriteString(fmt.Sprintf("| %s | %s | - | - | V2_MISSING |\n", spec.Source, spec.Target))
			continue
		}
		v3, err := countRows(ctx, pgDB, `"`+spec.Target+`"`)
		status := "OK"
		if err != nil {
			status = "TARGET_MISSING"
			diffs++
		} else if v2 != v3 {
			status = "DIFF"
			diffs++
		}
		totalV2 += v2
		totalV3 += v3
		b.WriteString(fmt.Sprintf("| %s | %s | %d | %d | %s |\n", spec.Source, spec.Target, v2, v3, status))
	}
	b.WriteString(fmt.Sprintf("\nTOTAL: v2=%d v3=%d diffs=%d\n", totalV2, totalV3, diffs))
	if err := os.MkdirAll(dirOf(out), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(out, []byte(b.String()), 0o644); err != nil {
		return err
	}
	fmt.Printf("报告已生成：%s (diffs=%d)\n", out, diffs)
	return nil
}

func rehearsal(ctx context.Context, mysqlDB *sql.DB, cfg *config.Config, file, schema string, keep bool) error {
	if schema == "public" || !strings.HasPrefix(schema, "v3_rehearsal") || !isIdent(schema) {
		return fmt.Errorf("rehearsal schema 必须是 v3_rehearsal* 的安全标识符")
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("读取 dump 失败: %w", err)
	}
	var dumpData map[string][]map[string]any
	if err := json.Unmarshal(data, &dumpData); err != nil {
		return fmt.Errorf("解析 dump 失败: %w", err)
	}

	pgCfg := cfg.Database.Postgres["main"]
	pgCfg.SearchPath = schema + ",public"
	pool, err := postgres.NewPool(pgCfg, false)
	if err != nil {
		return fmt.Errorf("连接 rehearsal schema 失败: %w", err)
	}
	defer pool.Close()

	migrator := migrate.NewSchemaMigrator(migrate.NewPostgresAdapter(pool.Pool), "migrations/postgres")
	// rehearsal 每次从空 schema 开始，避免上次残留影响校验。
	if _, err := pool.Exec(ctx, `DROP SCHEMA IF EXISTS "`+schema+`" CASCADE`); err != nil {
		return fmt.Errorf("清理 rehearsal schema 失败: %w", err)
	}
	if err := migrator.EnsureSchema(ctx, schema); err != nil {
		return err
	}
	if err := migrator.Init(ctx); err != nil {
		return err
	}
	applied, err := migrator.ApplyAll(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("rehearsal schema=%s migrations=%d\n", schema, len(applied))

	if err := loadData(ctx, pool.SQLDB, dumpData); err != nil {
		return err
	}
	if err := verify(ctx, mysqlDB, pool.SQLDB); err != nil {
		return err
	}
	if keep {
		fmt.Printf("rehearsal 通过，保留 schema=%s\n", schema)
		return nil
	}
	if _, err := pool.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`); err != nil {
		return fmt.Errorf("清理 rehearsal schema 失败: %w", err)
	}
	fmt.Printf("rehearsal 通过，已清理 schema=%s\n", schema)
	return nil
}

func isIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (i > 0 && r >= '0' && r <= '9') {
			continue
		}
		return false
	}
	return true
}

func verify(ctx context.Context, mysqlDB, pgDB *sql.DB) error {
	var bad int
	for _, spec := range tableSpecs {
		v2, err := sourceCount(ctx, mysqlDB, spec.Source)
		if err != nil {
			continue
		}
		v3, err := countRows(ctx, pgDB, `"`+spec.Target+`"`)
		if err != nil {
			fmt.Printf("MISSING  %-46s v2=%d v3=?\n", spec.Target, v2)
			bad++
			continue
		}
		if v2 != v3 {
			fmt.Printf("DIFF     %-46s v2=%d v3=%d\n", spec.Target, v2, v3)
			bad++
		}
	}
	if bad > 0 {
		fmt.Printf("校验失败：%d 张表不一致\n", bad)
		return fmt.Errorf("verify failed")
	}
	fmt.Println("校验通过：所有 allowlist 表行数一致（孤儿关联已排除）")
	return nil
}

// sourceCount 对 V2 孤儿关联做过滤，避免把已删除用户/任务的关联算入应迁移行数。
func sourceCount(ctx context.Context, db *sql.DB, table string) (int64, error) {
	var query string
	switch table {
	case "user_roles":
		query = `SELECT COUNT(*) FROM user_roles ur JOIN users u ON u.id = ur.user_id`
	case "api_keys":
		query = `SELECT COUNT(*) FROM api_keys k JOIN users u ON u.id = k.user_id`
	case "scheduler_task_logs", "task_logs":
		query = `SELECT COUNT(*) FROM task_logs tl JOIN tasks t ON t.id = tl.task_id`
	case "role_permissions":
		query = `SELECT COUNT(*) FROM role_permissions rp JOIN permissions p ON p.id = rp.permission_id`
	default:
		return countRows(ctx, db, "`"+table+"`")
	}
	var n int64
	err := db.QueryRowContext(ctx, query).Scan(&n)
	return n, err
}

type colMeta struct {
	types    map[string]string
	nullable map[string]bool
}

func insertRow(ctx context.Context, pgDB *sql.DB, table string, row map[string]any, typeCache map[string]colMeta) (bool, error) {
	meta, ok := typeCache[table]
	if !ok {
		var err error
		meta, err = pgColumns(ctx, pgDB, table)
		if err != nil {
			return false, err
		}
		typeCache[table] = meta
	}
	keys := make([]string, 0, len(row))
	for k := range row {
		if _, ok := meta.types[k]; !ok {
			continue
		}
		// NOT NULL 列遇到 NULL 时跳过，让数据库默认值生效（如 users.avatar_url）。
		if row[k] == nil && !meta.nullable[k] {
			continue
		}
		keys = append(keys, k)
	}
	if len(keys) == 0 {
		return false, nil
	}
	sort.Strings(keys)
	placeholders := make([]string, len(keys))
	args := make([]any, len(keys))
	for i, k := range keys {
		placeholders[i] = "$" + strconv.Itoa(i+1)
		args[i] = normalizeValue(row[k], meta.types[k])
	}
	query := "INSERT INTO \"" + table + "\" (" + quoteCols(keys) + ") VALUES (" + strings.Join(placeholders, ", ") + ") ON CONFLICT DO NOTHING"
	res, err := pgDB.ExecContext(ctx, query, args...)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func pgColumns(ctx context.Context, db *sql.DB, table string) (colMeta, error) {
	meta := colMeta{types: make(map[string]string), nullable: make(map[string]bool)}
	rows, err := db.QueryContext(ctx, `SELECT column_name, udt_name, is_nullable FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = $1`, table)
	if err != nil {
		return meta, err
	}
	defer rows.Close()
	for rows.Next() {
		var name, udt, nullable string
		if err := rows.Scan(&name, &udt, &nullable); err != nil {
			return meta, err
		}
		meta.types[name] = udt
		meta.nullable[name] = strings.EqualFold(nullable, "YES")
	}
	return meta, rows.Err()
}

func idNameMap(rows []map[string]any, idKey, nameKey string) map[int64]string {
	out := make(map[int64]string, len(rows))
	for _, row := range rows {
		out[toInt64(row[idKey])] = fmt.Sprint(row[nameKey])
	}
	return out
}

func queryNameID(ctx context.Context, db *sql.DB, table, nameCol string) map[string]int64 {
	out := make(map[string]int64)
	rows, err := db.QueryContext(ctx, fmt.Sprintf(`SELECT id, %s FROM "%s"`, nameCol, table))
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err == nil {
			out[name] = id
		}
	}
	return out
}

func toInt64(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case float64:
		return int64(x)
	case json.Number:
		n, _ := x.Int64()
		return n
	case []byte:
		n, _ := strconv.ParseInt(string(x), 10, 64)
		return n
	case string:
		n, _ := strconv.ParseInt(x, 10, 64)
		return n
	default:
		return 0
	}
}

func normalizeValue(v any, udt string) any {
	switch udt {
	case "bool":
		switch x := v.(type) {
		case bool:
			return x
		case int64:
			return x != 0
		case float64:
			return x != 0
		case []byte:
			s := strings.ToLower(strings.TrimSpace(string(x)))
			return s == "1" || s == "true" || s == "t"
		case string:
			s := strings.ToLower(strings.TrimSpace(x))
			return s == "1" || s == "true" || s == "t"
		}
	case "json", "jsonb":
		switch x := v.(type) {
		case []byte:
			return string(x)
		case string:
			return x
		default:
			b, _ := json.Marshal(x)
			return string(b)
		}
	case "bytea":
		if x, ok := v.([]byte); ok {
			return x
		}
	default:
		if x, ok := v.([]byte); ok {
			return string(x)
		}
	}
	return v
}

func resetSequence(ctx context.Context, db *sql.DB, table string) error {
	var seq sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT pg_get_serial_sequence($1, 'id')`, table).Scan(&seq); err != nil {
		return err
	}
	if !seq.Valid || seq.String == "" {
		return nil
	}
	_, err := db.ExecContext(ctx, fmt.Sprintf(
		`SELECT setval('%s', COALESCE((SELECT MAX(id) FROM "%s"), 0) + 1, false)`, seq.String, table))
	return err
}

func countRows(ctx context.Context, db *sql.DB, quoted string) (int64, error) {
	var n int64
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+quoted).Scan(&n)
	return n, err
}

func quoteCols(cols []string) string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = `"` + strings.ReplaceAll(c, `"`, `""`) + `"`
	}
	return strings.Join(out, ", ")
}

func isForeignKeyError(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "foreign key")
}

func dirOf(path string) string {
	if i := strings.LastIndexAny(path, `/\`); i >= 0 {
		return path[:i]
	}
	return "."
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, "v3migrate: "+msg)
	os.Exit(1)
}
