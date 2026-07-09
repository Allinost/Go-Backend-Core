package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DBType string

const (
	DBTypeMySQL    DBType = "mysql"
	DBTypePostgres DBType = "postgres"
)

type Format string

const (
	FormatJSON Format = "json"
	FormatCSV  Format = "csv"
	FormatSQL  Format = "sql"
)

type ColumnInfo struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
}

type TableSchema struct {
	Name    string       `json:"name"`
	Columns []ColumnInfo `json:"columns"`
}

type DumpOptions struct {
	Tables     []string
	Format     Format
	Where      map[string]string
	BatchSize  int
	NoData     bool
	SchemaOnly bool
}

type DumpResult struct {
	Data       []byte
	Format     Format
	TableCount int
	RowCount   int64
	Tables     []string
}

type RestoreOptions struct {
	Format       Format
	Tables       []string
	Truncate     bool
	SkipExisting bool
	BatchSize    int
}

type RestoreResult struct {
	TableCount int
	RowCount   int64
	Errors     []string
}

type TransferOptions struct {
	Tables      []string
	BatchSize   int
	CreateTable bool
	DropTarget  bool
}

type TransferResult struct {
	TableCount int
	RowCount   int64
	Errors     []string
}

type BackupOptions struct {
	Tables       []string
	CompressAlgo string
	EncryptKey   string
	OutputDir    string
	Filename     string
}

type BackupMeta struct {
	Version      string    `json:"version"`
	CreatedAt    time.Time `json:"created_at"`
	DBType       string    `json:"db_type"`
	DBName       string    `json:"db_name"`
	Tables       []string  `json:"tables"`
	Format       string    `json:"format"`
	CompressAlgo string    `json:"compress_algo"`
	Encrypted    bool      `json:"encrypted"`
	RowCount     int64     `json:"row_count"`
	FileSize     int64     `json:"file_size"`
	Checksum     string    `json:"checksum"`
}

type SchemaEntry struct {
	Version     int       `json:"version"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	AppliedAt   time.Time `json:"applied_at"`
	Checksum    string    `json:"checksum"`
}

type DBAdapter interface {
	Type() DBType
	Ping(ctx context.Context) error
	GetTables(ctx context.Context) ([]string, error)
	GetTableSchema(ctx context.Context, table string) (*TableSchema, error)
	GetColumns(ctx context.Context, table string) ([]ColumnInfo, error)
	SelectAll(ctx context.Context, table string, columns []string, where string, batchSize int, fn func(rows []map[string]any) error) error
	InsertBatch(ctx context.Context, table string, columns []string, rows []map[string]any) error
	Exec(ctx context.Context, sql string, args ...any) error
	ExecRaw(ctx context.Context, sql string) error
	Truncate(ctx context.Context, table string) error
	CreateTableIfNotExists(ctx context.Context, schema *TableSchema) error
	Quote(name string) string
}

func NewMySQLAdapter(db *sql.DB) *MySQLAdapter {
	return &MySQLAdapter{db: db}
}

func NewPostgresAdapter(db *pgxpool.Pool) *PostgresAdapter {
	return &PostgresAdapter{db: db}
}

func mysqlTypeToPostgres(mysqlType string) string {
	t := strings.ToLower(mysqlType)
	switch {
	case t == "tinyint(1)" || t == "bool" || t == "boolean":
		return "BOOLEAN"
	case strings.HasPrefix(t, "bigint"):
		return "BIGINT"
	case strings.HasPrefix(t, "int") || strings.HasPrefix(t, "mediumint") || strings.HasPrefix(t, "smallint") || strings.HasPrefix(t, "tinyint"):
		return "INTEGER"
	case strings.HasPrefix(t, "decimal") || strings.HasPrefix(t, "numeric") || strings.HasPrefix(t, "float") || strings.HasPrefix(t, "double"):
		return "NUMERIC"
	case strings.Contains(t, "text") || strings.Contains(t, "char"):
		return "TEXT"
	case strings.Contains(t, "blob") || strings.Contains(t, "binary"):
		return "BYTEA"
	case strings.Contains(t, "datetime") || strings.Contains(t, "timestamp"):
		return "TIMESTAMP"
	case strings.Contains(t, "date"):
		return "DATE"
	case strings.Contains(t, "time"):
		return "TIME"
	case strings.Contains(t, "json"):
		return "JSONB"
	default:
		return "TEXT"
	}
}

func splitSQL(sql string) []string {
	statements := strings.Split(sql, ";")
	var result []string
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt != "" {
			result = append(result, stmt)
		}
	}
	return result
}

func detectFormat(data []byte) Format {
	trimmed := strings.TrimSpace(string(data))
	if len(trimmed) == 0 {
		return FormatJSON
	}
	if trimmed[0] == '[' || trimmed[0] == '{' {
		return FormatJSON
	}
	if strings.HasPrefix(trimmed, "INSERT INTO") || strings.HasPrefix(trimmed, "CREATE TABLE") || strings.HasPrefix(trimmed, "-- ") {
		return FormatSQL
	}
	if strings.Contains(trimmed, ",") && strings.Count(trimmed, "\n") > 0 {
		return FormatCSV
	}
	return FormatJSON
}

func extractTableNames(ddl string) []string {
	var tables []string
	lines := strings.Split(ddl, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(line), "INSERT INTO") {
			rest := strings.TrimPrefix(strings.TrimPrefix(strings.ToUpper(line), "INSERT INTO"), "INSERT  INTO")
			rest = strings.TrimSpace(line[len("INSERT INTO"):])
			rest = strings.TrimSpace(rest)
			if idx := strings.IndexAny(rest, " ("); idx > 0 {
				name := strings.Trim(rest[:idx], "`\"'")
				tables = append(tables, name)
			}
		}
	}
	return tables
}

func joinStrings(items []string, sep string) string {
	return strings.Join(items, sep)
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func uniqueStrings(slice []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range slice {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

func fmtMapKey(m map[string]string, key string) string {
	if v, ok := m[key]; ok {
		return v
	}
	return ""
}

func fmtHex(n int64) string {
	return fmt.Sprintf("%x", n)
}
