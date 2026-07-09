package migrate

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testRow struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type testDB struct {
	tables  map[string][]map[string]any
	schemas map[string]*TableSchema
	seq     int64
}

func newTestDB() *testDB {
	return &testDB{
		tables:  make(map[string][]map[string]any),
		schemas: make(map[string]*TableSchema),
	}
}

func (d *testDB) Type() DBType                   { return DBTypeMySQL }
func (d *testDB) Ping(ctx context.Context) error { return nil }
func (d *testDB) GetTables(ctx context.Context) ([]string, error) {
	var tables []string
	for t := range d.tables {
		tables = append(tables, t)
	}
	return tables, nil
}
func (d *testDB) GetTableSchema(ctx context.Context, table string) (*TableSchema, error) {
	if s, ok := d.schemas[table]; ok {
		return s, nil
	}
	return &TableSchema{Name: table, Columns: []ColumnInfo{
		{Name: "id", Type: "BIGINT", Nullable: false},
		{Name: "name", Type: "VARCHAR(255)", Nullable: true},
	}}, nil
}
func (d *testDB) GetColumns(ctx context.Context, table string) ([]ColumnInfo, error) {
	s, err := d.GetTableSchema(ctx, table)
	if err != nil {
		return nil, err
	}
	return s.Columns, nil
}
func (d *testDB) SelectAll(ctx context.Context, table string, columns []string, where string, batchSize int, fn func(rows []map[string]any) error) error {
	rows := d.tables[table]
	if rows == nil {
		return nil
	}
	return fn(rows)
}
func (d *testDB) InsertBatch(ctx context.Context, table string, columns []string, rows []map[string]any) error {
	d.tables[table] = append(d.tables[table], rows...)
	return nil
}
func (d *testDB) Exec(ctx context.Context, sql string, args ...any) error {
	d.extractSchemaFromSQL(sql)
	return nil
}
func (d *testDB) ExecRaw(ctx context.Context, sql string) error {
	d.extractSchemaFromSQL(sql)
	return nil
}

func (d *testDB) extractSchemaFromSQL(sql string) {
	upper := strings.ToUpper(strings.TrimSpace(sql))
	if strings.HasPrefix(upper, "CREATE TABLE") {
		nameStart := strings.Index(upper, "TABLE")
		if nameStart < 0 {
			return
		}
		rest := sql[nameStart+5:]
		rest = strings.TrimSpace(rest)
		if strings.HasPrefix(rest, "IF NOT EXISTS") {
			rest = strings.TrimSpace(rest[13:])
		}
		var tableName string
		for i, c := range rest {
			if c == '(' || c == ' ' || c == '\n' || c == '\t' {
				tableName = strings.Trim(strings.TrimSpace(rest[:i]), "`\"'")
				break
			}
		}
		if tableName == "" {
			tableName = strings.Trim(strings.TrimSpace(rest), "`\"'")
		}
		if _, ok := d.schemas[tableName]; !ok {
			d.schemas[tableName] = &TableSchema{Name: tableName}
		}
	}
}
func (d *testDB) Truncate(ctx context.Context, table string) error {
	delete(d.tables, table)
	return nil
}
func (d *testDB) CreateTableIfNotExists(ctx context.Context, schema *TableSchema) error {
	d.schemas[schema.Name] = schema
	return nil
}
func (d *testDB) Quote(name string) string { return "`" + name + "`" }
func (d *testDB) Name() string             { return "test" }

func TestDumpJSON(t *testing.T) {
	db := newTestDB()
	db.tables["users"] = []map[string]any{
		{"id": int64(1), "name": "Alice"},
		{"id": int64(2), "name": "Bob"},
	}
	db.tables["roles"] = []map[string]any{
		{"id": int64(1), "name": "admin"},
	}

	result, err := Dump(context.Background(), db, DumpOptions{
		Tables: []string{"users", "roles"},
		Format: FormatJSON,
	})
	require.NoError(t, err)
	assert.Equal(t, FormatJSON, result.Format)
	assert.Equal(t, 2, result.TableCount)
	assert.Equal(t, int64(3), result.RowCount)
	assert.Contains(t, string(result.Data), "Alice")
	assert.Contains(t, string(result.Data), "admin")
}

func TestDumpCSV(t *testing.T) {
	db := newTestDB()
	db.tables["users"] = []map[string]any{
		{"id": int64(1), "name": "Alice"},
		{"id": int64(2), "name": "Bob"},
	}

	result, err := Dump(context.Background(), db, DumpOptions{
		Tables: []string{"users"},
		Format: FormatCSV,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(2), result.RowCount)
	assert.Contains(t, string(result.Data), "Alice")
}

func TestDumpSQL(t *testing.T) {
	db := newTestDB()
	db.tables["users"] = []map[string]any{
		{"id": int64(1), "name": "Alice"},
		{"id": int64(2), "name": "Bob"},
	}

	result, err := Dump(context.Background(), db, DumpOptions{
		Tables: []string{"users"},
		Format: FormatSQL,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(2), result.RowCount)
	assert.Contains(t, string(result.Data), "INSERT INTO")
	assert.Contains(t, string(result.Data), "Alice")
}

func TestDumpSchemaOnly(t *testing.T) {
	db := newTestDB()
	db.tables["users"] = []map[string]any{
		{"id": int64(1), "name": "Alice"},
	}

	result, err := Dump(context.Background(), db, DumpOptions{
		Tables:     []string{"users"},
		Format:     FormatJSON,
		SchemaOnly: true,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(0), result.RowCount)
}

func TestRestoreJSON(t *testing.T) {
	db := newTestDB()
	data := []byte(`{"users":[{"id":1,"name":"Alice"},{"id":2,"name":"Bob"}]}`)

	result, err := Restore(context.Background(), db, data, RestoreOptions{Format: FormatJSON})
	require.NoError(t, err)
	assert.Equal(t, int64(2), result.RowCount)
	assert.Len(t, db.tables["users"], 2)
}

func TestRestoreJSONWithTruncate(t *testing.T) {
	db := newTestDB()
	db.tables["users"] = []map[string]any{{"id": int64(99), "name": "Old"}}
	data := []byte(`{"users":[{"id":1,"name":"New"}]}`)

	result, err := Restore(context.Background(), db, data, RestoreOptions{
		Format:   FormatJSON,
		Truncate: true,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.RowCount)
	assert.Len(t, db.tables["users"], 1)
	assert.Equal(t, "New", db.tables["users"][0]["name"])
}

func TestRestoreSQL(t *testing.T) {
	db := newTestDB()
	data := []byte(`CREATE TABLE IF NOT EXISTS users (id INT, name VARCHAR(255));
INSERT INTO users (id, name) VALUES (1, 'Alice');
INSERT INTO users (id, name) VALUES (2, 'Bob');`)

	result, err := Restore(context.Background(), db, data, RestoreOptions{Format: FormatSQL})
	require.NoError(t, err)
	assert.Equal(t, 3, result.TableCount)
}

func TestTransfer(t *testing.T) {
	src := newTestDB()
	src.tables["users"] = []map[string]any{
		{"id": int64(1), "name": "Alice"},
		{"id": int64(2), "name": "Bob"},
	}

	dst := newTestDB()

	result, err := Transfer(context.Background(), src, dst, TransferOptions{
		Tables:      []string{"users"},
		CreateTable: true,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, result.TableCount)
	assert.Equal(t, int64(2), result.RowCount)
	assert.Len(t, dst.tables["users"], 2)
}

func TestDetectFormat(t *testing.T) {
	assert.Equal(t, FormatJSON, detectFormat([]byte(`[{"id":1}]`)))
	assert.Equal(t, FormatJSON, detectFormat([]byte(`{"key":"val"}`)))
	assert.Equal(t, FormatSQL, detectFormat([]byte("INSERT INTO users VALUES (1);")))
	assert.Equal(t, FormatSQL, detectFormat([]byte("CREATE TABLE users (id INT);")))
	assert.Equal(t, FormatSQL, detectFormat([]byte("-- SQL dump\nSELECT 1;")))
}

func TestSchemaMigratorInit(t *testing.T) {
	db := newTestDB()
	migrator := NewSchemaMigrator(db, ".")

	err := migrator.Init(context.Background())
	require.NoError(t, err)
	assert.Contains(t, db.schemas, "_migrations")
}

func TestFormatCSVValue(t *testing.T) {
	assert.Equal(t, "", formatCSVValue(nil))
	assert.Equal(t, "hello", formatCSVValue("hello"))
	assert.Equal(t, "\"he,llo\"", formatCSVValue("he,llo"))
	assert.Equal(t, "\"he\"\"llo\"", formatCSVValue("he\"llo"))
}

func TestFormatSQLValue(t *testing.T) {
	assert.Equal(t, "NULL", formatSQLValue(nil))
	assert.Equal(t, "42", formatSQLValue(42))
	assert.Equal(t, "'hello'", formatSQLValue("hello"))
	assert.Equal(t, "'it''s'", formatSQLValue("it's"))
	assert.Equal(t, "1", formatSQLValue(true))
	assert.Equal(t, "0", formatSQLValue(false))
}

func TestMySQLTypeToPostgres(t *testing.T) {
	assert.Equal(t, "BIGINT", mysqlTypeToPostgres("bigint(20)"))
	assert.Equal(t, "INTEGER", mysqlTypeToPostgres("int(11)"))
	assert.Equal(t, "TEXT", mysqlTypeToPostgres("varchar(255)"))
	assert.Equal(t, "TEXT", mysqlTypeToPostgres("text"))
	assert.Equal(t, "BYTEA", mysqlTypeToPostgres("blob"))
	assert.Equal(t, "TIMESTAMP", mysqlTypeToPostgres("datetime"))
	assert.Equal(t, "BOOLEAN", mysqlTypeToPostgres("tinyint(1)"))
	assert.Equal(t, "JSONB", mysqlTypeToPostgres("json"))
}

func TestCreateBackup(t *testing.T) {
	ctx := context.Background()
	db := newTestDB()
	db.tables["users"] = []map[string]any{
		{"id": int64(1), "name": "Alice"},
	}

	outDir := t.TempDir()
	meta, err := CreateBackup(ctx, db, BackupOptions{
		Tables:       []string{"users"},
		CompressAlgo: "gzip",
		OutputDir:    outDir,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), meta.RowCount)
	assert.True(t, meta.FileSize > 0)
	assert.True(t, meta.CreatedAt.Unix() > 0)
}

func TestListBackups(t *testing.T) {
	outDir := t.TempDir()

	db := newTestDB()
	db.tables["test"] = []map[string]any{{"id": int64(1)}}

	_, err := CreateBackup(context.Background(), db, BackupOptions{
		Tables:    []string{"test"},
		OutputDir: outDir,
	})
	require.NoError(t, err)

	backups, err := ListBackups(outDir)
	require.NoError(t, err)
	assert.NotEmpty(t, backups)
}

func TestMigrationFileParsing(t *testing.T) {
	ver, desc := parseMigrationFilename("001_create_users.sql")
	assert.Equal(t, 1, ver)
	assert.Equal(t, "create users", desc)

	ver, desc = parseMigrationFilename("002_init.sql")
	assert.Equal(t, 2, ver)
	assert.Equal(t, "init", desc)
}

func TestExtractTableFromInsert(t *testing.T) {
	assert.Equal(t, "users", extractTableFromInsert("INSERT INTO users (id) VALUES (1)"))
	assert.Equal(t, "users", extractTableFromInsert("INSERT INTO `users` (id) VALUES (1)"))
}
