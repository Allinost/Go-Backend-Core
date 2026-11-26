package dbutil

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestV3DialectGate 防止 V3 已迁移模块重新引入 MySQL 专有 SQL。
func TestV3DialectGate(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("无法定位测试文件")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))

	scopes := []string{
		"internal/modules/auth",
		"internal/modules/integration",
		"internal/modules/webhook",
		"internal/modules/servicenav",
		"internal/modules/ops",
		"internal/modules/settings",
		"internal/services/auth",
		"internal/services/scheduler",
		"internal/services/audit",
		"internal/services/feature",
		"internal/services/fileref",
		"internal/services/outbox",
		"internal/services/configcenter",
		"internal/services/migrate",
		"zzz/developmentManager",
		"zzz/goodser",
	}

	allowed := map[string]bool{
		"internal/services/audit/mysql.go":     true, // SQLite/MySQL 兼容测试存储
		"internal/services/feature/store.go":   true, // SQLite/MySQL 兼容测试存储
		"internal/services/migrate/adapter.go": true, // MySQL 归档/适配器
		"internal/services/migrate/dumper.go":  true, // MySQL dump 兼容
		"internal/services/migrate/schema.go":  true, // MySQL _migrations 兼容
	}

	forbidden := []string{
		"AUTO_INCREMENT",
		"ENGINE=InnoDB",
		"ON DUPLICATE KEY",
		"INSERT IGNORE",
		"REPLACE INTO",
		"SELECT DATABASE()",
		"JSON_CONTAINS",
		"FIND_IN_SET",
		"DATE_FORMAT(",
		"IFNULL(",
		"LastInsertId(",
		"deleted = 0",
		"deleted = 1",
		"deleted=0",
		"deleted=1",
		"enabled = 0",
		"enabled = 1",
		"enabled=0",
		"enabled=1",
	}

	for _, scope := range scopes {
		dir := filepath.Join(root, filepath.FromSlash(scope))
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			if allowed[rel] {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			content := string(data)
			for _, pattern := range forbidden {
				if strings.Contains(content, pattern) {
					t.Errorf("%s 仍包含 MySQL 方言 %q，请改用 PostgreSQL 写法", rel, pattern)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("扫描 %s 失败: %v", scope, err)
		}
	}
}
