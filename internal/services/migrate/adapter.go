package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type MySQLAdapter struct {
	db   *sql.DB
	name string
}

func (a *MySQLAdapter) Type() DBType { return DBTypeMySQL }
func (a *MySQLAdapter) Ping(ctx context.Context) error {
	return a.db.PingContext(ctx)
}
func (a *MySQLAdapter) GetTables(ctx context.Context) ([]string, error) {
	rows, err := a.db.QueryContext(ctx, "SHOW TABLES")
	if err != nil {
		return nil, fmt.Errorf("migrate: 查询表列表失败: %w", err)
	}
	defer rows.Close()
	var tables []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		tables = append(tables, t)
	}
	return tables, rows.Err()
}
func (a *MySQLAdapter) GetTableSchema(ctx context.Context, table string) (*TableSchema, error) {
	cols, err := a.GetColumns(ctx, table)
	if err != nil {
		return nil, err
	}
	return &TableSchema{Name: table, Columns: cols}, nil
}
func (a *MySQLAdapter) GetColumns(ctx context.Context, table string) ([]ColumnInfo, error) {
	rows, err := a.db.QueryContext(ctx, fmt.Sprintf("SHOW COLUMNS FROM %s", a.Quote(table)))
	if err != nil {
		return nil, fmt.Errorf("migrate: 查询表 %s 列信息失败: %w", table, err)
	}
	defer rows.Close()
	var cols []ColumnInfo
	for rows.Next() {
		var field, colType, null, key, extra string
		var defaultVal sql.NullString
		if err := rows.Scan(&field, &colType, &null, &key, &defaultVal, &extra); err != nil {
			return nil, err
		}
		cols = append(cols, ColumnInfo{
			Name:     field,
			Type:     colType,
			Nullable: null == "YES",
		})
	}
	return cols, rows.Err()
}
func (a *MySQLAdapter) SelectAll(ctx context.Context, table string, columns []string, where string, batchSize int, fn func(rows []map[string]any) error) error {
	if batchSize <= 0 {
		batchSize = 1000
	}
	colList := "*"
	if len(columns) > 0 {
		colList = strings.Join(quoteAll(columns, a), ", ")
	}
	whereClause := ""
	if where != "" {
		whereClause = " WHERE " + where
	}
	offset := 0
	for {
		query := fmt.Sprintf("SELECT %s FROM %s%s LIMIT %d OFFSET %d", colList, a.Quote(table), whereClause, batchSize, offset)
		qRows, err := a.db.QueryContext(ctx, query)
		if err != nil {
			return fmt.Errorf("migrate: 查询失败: %w", err)
		}
		cols, err := qRows.Columns()
		if err != nil {
			qRows.Close()
			return err
		}
		var batch []map[string]any
		for qRows.Next() {
			vals := make([]any, len(cols))
			valPtrs := make([]any, len(cols))
			for i := range vals {
				valPtrs[i] = &vals[i]
			}
			if err := qRows.Scan(valPtrs...); err != nil {
				qRows.Close()
				return err
			}
			row := make(map[string]any)
			for i, col := range cols {
				row[col] = vals[i]
			}
			batch = append(batch, row)
		}
		qRows.Close()
		if len(batch) == 0 {
			break
		}
		if err := fn(batch); err != nil {
			return err
		}
		offset += batchSize
		if len(batch) < batchSize {
			break
		}
	}
	return nil
}
func (a *MySQLAdapter) InsertBatch(ctx context.Context, table string, columns []string, rows []map[string]any) error {
	if len(rows) == 0 {
		return nil
	}
	if len(columns) == 0 {
		for k := range rows[0] {
			columns = append(columns, k)
		}
	}
	colList := strings.Join(quoteAll(columns, a), ", ")
	var placeholders []string
	var args []any
	for _, row := range rows {
		var ph []string
		for _, col := range columns {
			ph = append(ph, "?")
			args = append(args, row[col])
		}
		placeholders = append(placeholders, "("+strings.Join(ph, ",")+")")
	}
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES %s", a.Quote(table), colList, strings.Join(placeholders, ","))
	_, err := a.db.ExecContext(ctx, query, args...)
	return err
}
func (a *MySQLAdapter) Exec(ctx context.Context, sql string, args ...any) error {
	_, err := a.db.ExecContext(ctx, sql, args...)
	return err
}
func (a *MySQLAdapter) ExecRaw(ctx context.Context, sql string) error {
	for _, stmt := range splitSQL(sql) {
		trimmed := strings.TrimSpace(stmt)
		if trimmed == "" {
			continue
		}
		if _, err := a.db.ExecContext(ctx, trimmed); err != nil {
			return fmt.Errorf("migrate: 执行 SQL 失败: %s: %w", truncateStr(trimmed, 80), err)
		}
	}
	return nil
}
func (a *MySQLAdapter) Truncate(ctx context.Context, table string) error {
	_, err := a.db.ExecContext(ctx, fmt.Sprintf("TRUNCATE TABLE %s", a.Quote(table)))
	return err
}
func (a *MySQLAdapter) CreateTableIfNotExists(ctx context.Context, schema *TableSchema) error {
	var defs []string
	for _, col := range schema.Columns {
		def := a.Quote(col.Name) + " " + col.Type
		if !col.Nullable {
			def += " NOT NULL"
		}
		defs = append(defs, def)
	}
	query := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n  %s\n) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4", a.Quote(schema.Name), strings.Join(defs, ",\n  "))
	_, err := a.db.ExecContext(ctx, query)
	return err
}
func (a *MySQLAdapter) Quote(name string) string { return "`" + name + "`" }

type PostgresAdapter struct {
	db   *pgxpool.Pool
	name string
}

func (a *PostgresAdapter) Type() DBType { return DBTypePostgres }
func (a *PostgresAdapter) Ping(ctx context.Context) error {
	return a.db.Ping(ctx)
}
func (a *PostgresAdapter) GetTables(ctx context.Context) ([]string, error) {
	rows, err := a.db.Query(ctx, "SELECT tablename FROM pg_catalog.pg_tables WHERE schemaname = 'public' ORDER BY tablename")
	if err != nil {
		return nil, fmt.Errorf("migrate: 查询表列表失败: %w", err)
	}
	defer rows.Close()
	var tables []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		tables = append(tables, t)
	}
	return tables, nil
}
func (a *PostgresAdapter) GetTableSchema(ctx context.Context, table string) (*TableSchema, error) {
	cols, err := a.GetColumns(ctx, table)
	if err != nil {
		return nil, err
	}
	return &TableSchema{Name: table, Columns: cols}, nil
}
func (a *PostgresAdapter) GetColumns(ctx context.Context, table string) ([]ColumnInfo, error) {
	rows, err := a.db.Query(ctx, `SELECT column_name, data_type, is_nullable FROM information_schema.columns WHERE table_name = $1 AND table_schema = 'public' ORDER BY ordinal_position`, table)
	if err != nil {
		return nil, fmt.Errorf("migrate: 查询表 %s 列信息失败: %w", table, err)
	}
	defer rows.Close()
	var cols []ColumnInfo
	for rows.Next() {
		var name, colType, nullable string
		if err := rows.Scan(&name, &colType, &nullable); err != nil {
			return nil, err
		}
		cols = append(cols, ColumnInfo{
			Name:     name,
			Type:     colType,
			Nullable: nullable == "YES",
		})
	}
	return cols, nil
}
func (a *PostgresAdapter) SelectAll(ctx context.Context, table string, columns []string, where string, batchSize int, fn func(rows []map[string]any) error) error {
	if batchSize <= 0 {
		batchSize = 1000
	}
	colList := "*"
	if len(columns) > 0 {
		colList = strings.Join(quoteAll(columns, a), ", ")
	}
	whereClause := ""
	if where != "" {
		whereClause = " WHERE " + where
	}
	offset := 0
	for {
		query := fmt.Sprintf("SELECT %s FROM %s%s ORDER BY 1 LIMIT %d OFFSET %d", colList, a.Quote(table), whereClause, batchSize, offset)
		qRows, err := a.db.Query(ctx, query)
		if err != nil {
			return fmt.Errorf("migrate: 查询失败: %w", err)
		}
		fds := qRows.FieldDescriptions()
		colNames := make([]string, len(fds))
		for i, fd := range fds {
			colNames[i] = string(fd.Name)
		}
		var batch []map[string]any
		for qRows.Next() {
			vals, err := qRows.Values()
			if err != nil {
				qRows.Close()
				return err
			}
			row := make(map[string]any)
			for i, val := range vals {
				row[colNames[i]] = val
			}
			batch = append(batch, row)
		}
		qRows.Close()
		if len(batch) == 0 {
			break
		}
		if err := fn(batch); err != nil {
			return err
		}
		offset += batchSize
		if len(batch) < batchSize {
			break
		}
	}
	return nil
}
func (a *PostgresAdapter) InsertBatch(ctx context.Context, table string, columns []string, rows []map[string]any) error {
	if len(rows) == 0 {
		return nil
	}
	if len(columns) == 0 {
		for k := range rows[0] {
			columns = append(columns, k)
		}
	}
	colList := strings.Join(quoteAll(columns, a), ", ")
	for _, row := range rows {
		var ph []string
		var args []any
		for j, col := range columns {
			ph = append(ph, fmt.Sprintf("$%d", j+1))
			args = append(args, row[col])
		}
		query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", a.Quote(table), colList, strings.Join(ph, ","))
		if _, err := a.db.Exec(ctx, query, args...); err != nil {
			return err
		}
	}
	return nil
}
func (a *PostgresAdapter) Exec(ctx context.Context, sql string, args ...any) error {
	_, err := a.db.Exec(ctx, sql, args...)
	return err
}
func (a *PostgresAdapter) ExecRaw(ctx context.Context, sql string) error {
	for _, stmt := range splitSQL(sql) {
		trimmed := strings.TrimSpace(stmt)
		if trimmed == "" {
			continue
		}
		if _, err := a.db.Exec(ctx, trimmed); err != nil {
			return fmt.Errorf("migrate: 执行 SQL 失败: %s: %w", truncateStr(trimmed, 80), err)
		}
	}
	return nil
}
func (a *PostgresAdapter) Truncate(ctx context.Context, table string) error {
	_, err := a.db.Exec(ctx, fmt.Sprintf("TRUNCATE TABLE %s", a.Quote(table)))
	return err
}
func (a *PostgresAdapter) CreateTableIfNotExists(ctx context.Context, schema *TableSchema) error {
	var defs []string
	for _, col := range schema.Columns {
		pgType := mysqlTypeToPostgres(col.Type)
		def := a.Quote(col.Name) + " " + pgType
		if !col.Nullable {
			def += " NOT NULL"
		}
		defs = append(defs, def)
	}
	query := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n  %s\n)", a.Quote(schema.Name), strings.Join(defs, ",\n  "))
	_, err := a.db.Exec(ctx, query)
	return err
}
func (a *PostgresAdapter) Quote(name string) string { return "\"" + name + "\"" }

func quoteAll(names []string, a DBAdapter) []string {
	res := make([]string, len(names))
	for i, n := range names {
		res[i] = a.Quote(n)
	}
	return res
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
