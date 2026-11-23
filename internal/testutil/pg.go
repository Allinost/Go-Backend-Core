package testutil

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/Allinost/go-backend-core/internal/services/migrate"
)

var schemaSeq atomic.Uint64

// OpenPG 连接测试 PostgreSQL 并创建临时 schema。
// 未设置 TEST_DATABASE_URL 时跳过测试。
func OpenPG(t *testing.T) *sql.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("testutil: open postgres: %v", err)
	}
	// 测试使用单连接，确保 SET search_path 对所有查询生效。
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		t.Fatalf("testutil: ping postgres: %v", err)
	}
	schema := fmt.Sprintf("test_%d_%d_%d", time.Now().UnixNano(), os.Getpid(), schemaSeq.Add(1))
	if _, err := db.Exec(`CREATE SCHEMA ` + quoteIdent(schema)); err != nil {
		_ = db.Close()
		t.Fatalf("testutil: create schema: %v", err)
	}
	if _, err := db.Exec(`SET search_path TO ` + quoteIdent(schema) + `, public`); err != nil {
		_, _ = db.Exec(`DROP SCHEMA ` + quoteIdent(schema) + ` CASCADE`)
		_ = db.Close()
		t.Fatalf("testutil: set search_path: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(`DROP SCHEMA ` + quoteIdent(schema) + ` CASCADE`)
		_ = db.Close()
	})
	return db
}

// ApplyMigrations 按文件名顺序执行 migrations/postgres/*.sql。
// 迁移文件使用 IF NOT EXISTS/OR REPLACE，可重复执行。
func ApplyMigrations(t *testing.T, db *sql.DB) error {
	t.Helper()
	dir := migrationsDir(t)
	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	if len(files) == 0 {
		return fmt.Errorf("testutil: no migrations found in %s", dir)
	}
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		for _, stmt := range migrate.SplitSQL(string(content)) {
			if _, err := db.Exec(stmt); err != nil {
				return fmt.Errorf("testutil: apply %s: %w", filepath.Base(file), err)
			}
		}
	}
	return nil
}

func migrationsDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("testutil: cannot resolve source path")
	}
	// internal/testutil/pg.go -> repo root
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	return filepath.Join(root, "migrations", "postgres")
}

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
