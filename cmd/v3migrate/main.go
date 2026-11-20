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
	flagSet := flag.NewFlagSet(cmd, flag.ExitOnError)
	flagSet.StringVar(&file, "file", "backups/v3_bootstrap.json", "dump/load 文件")
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

	type pendingRow struct {
		table string
		row   map[string]any
	}
	var pending []pendingRow
	inserted := 0
	failed := 0
	typeCache := make(map[string]map[string]string)

	for _, spec := range tableSpecs {
		for _, row := range dumpData[spec.Source] {
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
	fmt.Printf("导入完成：inserted=%d failed=%d pending=%d\n", inserted, failed, len(pending))
	return nil
}

func verify(ctx context.Context, mysqlDB, pgDB *sql.DB) error {
	var bad int
	for _, spec := range tableSpecs {
		v2, err := countRows(ctx, mysqlDB, "`"+spec.Source+"`")
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
	fmt.Println("校验通过：所有 allowlist 表行数一致")
	return nil
}

func insertRow(ctx context.Context, pgDB *sql.DB, table string, row map[string]any, typeCache map[string]map[string]string) (bool, error) {
	types, ok := typeCache[table]
	if !ok {
		var err error
		_, types, err = pgColumns(ctx, pgDB, table)
		if err != nil {
			return false, err
		}
		typeCache[table] = types
	}
	keys := make([]string, 0, len(row))
	for k := range row {
		if _, ok := types[k]; ok {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return false, nil
	}
	sort.Strings(keys)
	placeholders := make([]string, len(keys))
	args := make([]any, len(keys))
	for i, k := range keys {
		placeholders[i] = "$" + strconv.Itoa(i+1)
		args[i] = normalizeValue(row[k], types[k])
	}
	query := "INSERT INTO \"" + table + "\" (" + quoteCols(keys) + ") VALUES (" + strings.Join(placeholders, ", ") + ") ON CONFLICT DO NOTHING"
	res, err := pgDB.ExecContext(ctx, query, args...)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func pgColumns(ctx context.Context, db *sql.DB, table string) ([]string, map[string]string, error) {
	rows, err := db.QueryContext(ctx, `SELECT column_name, udt_name FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = $1`, table)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	types := make(map[string]string)
	var cols []string
	for rows.Next() {
		var name, udt string
		if err := rows.Scan(&name, &udt); err != nil {
			return nil, nil, err
		}
		cols = append(cols, name)
		types[name] = udt
	}
	return cols, types, rows.Err()
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
