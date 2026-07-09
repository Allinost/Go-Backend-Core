package migrate

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type SchemaMigrator struct {
	adapter       DBAdapter
	migrationsDir string
	tableName     string
}

func NewSchemaMigrator(adapter DBAdapter, migrationsDir string) *SchemaMigrator {
	return &SchemaMigrator{
		adapter:       adapter,
		migrationsDir: migrationsDir,
		tableName:     "_migrations",
	}
}

func (m *SchemaMigrator) Init(ctx context.Context) error {
	query := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		version INT NOT NULL,
		name VARCHAR(255) NOT NULL,
		description TEXT,
		applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		checksum VARCHAR(64) NOT NULL,
		PRIMARY KEY (version)
	)`, m.adapter.Quote(m.tableName))

	if m.adapter.Type() == DBTypeMySQL {
		query += " ENGINE=InnoDB DEFAULT CHARSET=utf8mb4"
	}

	return m.adapter.Exec(ctx, query)
}

func (m *SchemaMigrator) Applied(ctx context.Context) ([]SchemaEntry, error) {
	entries := map[int]*SchemaEntry{}
	if err := m.adapter.SelectAll(ctx, m.tableName, nil, "", 1000, func(batch []map[string]any) error {
		for _, row := range batch {
			entry := &SchemaEntry{}
			if v, ok := row["version"]; ok {
				switch val := v.(type) {
				case int64:
					entry.Version = int(val)
				case float64:
					entry.Version = int(val)
				}
			}
			if v, ok := row["name"]; ok {
				entry.Name = fmt.Sprintf("%v", v)
			}
			if v, ok := row["description"]; ok {
				entry.Description = fmt.Sprintf("%v", v)
			}
			if v, ok := row["checksum"]; ok {
				entry.Checksum = fmt.Sprintf("%v", v)
			}
			if v, ok := row["applied_at"]; ok {
				entry.AppliedAt = time.Now()
				_ = v
			}
			entries[entry.Version] = entry
		}
		return nil
	}); err != nil {
		return nil, err
	}

	var result []SchemaEntry
	for _, e := range entries {
		result = append(result, *e)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Version < result[j].Version
	})
	return result, nil
}

func (m *SchemaMigrator) Pending(ctx context.Context) ([]MigrationFile, error) {
	migrations, err := m.loadMigrations()
	if err != nil {
		return nil, err
	}

	applied, err := m.Applied(ctx)
	if err != nil {
		return nil, err
	}

	appliedMap := make(map[int]bool)
	for _, a := range applied {
		appliedMap[a.Version] = true
	}

	var pending []MigrationFile
	for _, mig := range migrations {
		if !appliedMap[mig.Version] {
			pending = append(pending, mig)
		}
	}

	return pending, nil
}

func (m *SchemaMigrator) Apply(ctx context.Context, mig MigrationFile) error {
	applied, err := m.Applied(ctx)
	if err != nil {
		return err
	}
	for _, a := range applied {
		if a.Version == mig.Version {
			return fmt.Errorf("migrate: 迁移 %d 已应用", mig.Version)
		}
	}

	if err := m.adapter.ExecRaw(ctx, mig.Content); err != nil {
		return fmt.Errorf("migrate: 应用迁移 %s 失败: %w", mig.Filename, err)
	}

	checksum := fmt.Sprintf("%x", sha256.Sum256([]byte(mig.Content)))

	entry := map[string]any{
		"version":     mig.Version,
		"name":        mig.Filename,
		"description": mig.Description,
		"applied_at":  time.Now(),
		"checksum":    checksum,
	}

	return m.adapter.InsertBatch(ctx, m.tableName, []string{"version", "name", "description", "applied_at", "checksum"}, []map[string]any{entry})
}

func (m *SchemaMigrator) ApplyAll(ctx context.Context) ([]MigrationFile, error) {
	pending, err := m.Pending(ctx)
	if err != nil {
		return nil, err
	}

	var applied []MigrationFile
	for _, mig := range pending {
		if err := m.Apply(ctx, mig); err != nil {
			return applied, fmt.Errorf("migrate: 应用迁移 %d (%s) 失败: %w", mig.Version, mig.Filename, err)
		}
		applied = append(applied, mig)
	}

	return applied, nil
}

func (m *SchemaMigrator) Status(ctx context.Context) (*MigrateStatus, error) {
	allFiles, err := m.loadMigrations()
	if err != nil {
		return nil, err
	}

	applied, err := m.Applied(ctx)
	if err != nil {
		return nil, err
	}

	appliedMap := make(map[int]*SchemaEntry)
	for i, a := range applied {
		appliedMap[a.Version] = &applied[i]
	}

	status := &MigrateStatus{
		Total:      len(allFiles),
		Applied:    len(applied),
		Pending:    len(allFiles) - len(applied),
		Migrations: make([]MigrationStatus, len(allFiles)),
	}

	for i, f := range allFiles {
		entry, isApplied := appliedMap[f.Version]
		s := MigrationStatus{
			Version:     f.Version,
			Filename:    f.Filename,
			Description: f.Description,
			Applied:     isApplied,
		}
		if isApplied && entry != nil {
			s.AppliedAt = entry.AppliedAt
			s.Checksum = entry.Checksum
		}
		status.Migrations[i] = s
	}

	return status, nil
}

type MigrationFile struct {
	Version     int
	Filename    string
	Description string
	Content     string
}

type MigrateStatus struct {
	Total      int               `json:"total"`
	Applied    int               `json:"applied"`
	Pending    int               `json:"pending"`
	Migrations []MigrationStatus `json:"migrations"`
}

type MigrationStatus struct {
	Version     int       `json:"version"`
	Filename    string    `json:"filename"`
	Description string    `json:"description"`
	Applied     bool      `json:"applied"`
	AppliedAt   time.Time `json:"applied_at,omitempty"`
	Checksum    string    `json:"checksum,omitempty"`
}

func (m *SchemaMigrator) loadMigrations() ([]MigrationFile, error) {
	entries, err := os.ReadDir(m.migrationsDir)
	if err != nil {
		return nil, fmt.Errorf("migrate: 读取迁移目录失败: %w", err)
	}

	var migrations []MigrationFile
	for _, entry := range entries {
		if entry.IsDir() || !stringsHasSuffix(entry.Name(), ".sql") {
			continue
		}

		content, err := os.ReadFile(filepath.Join(m.migrationsDir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("migrate: 读取迁移文件 %s 失败: %w", entry.Name(), err)
		}

		version, desc := parseMigrationFilename(entry.Name())

		migrations = append(migrations, MigrationFile{
			Version:     version,
			Filename:    entry.Name(),
			Description: desc,
			Content:     string(content),
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

func parseMigrationFilename(name string) (int, string) {
	name = strings.TrimSuffix(name, ".sql")
	parts := strings.SplitN(name, "_", 2)
	version := 0
	fmt.Sscanf(parts[0], "%d", &version)
	desc := ""
	if len(parts) > 1 {
		desc = strings.ReplaceAll(parts[1], "_", " ")
	}
	return version, desc
}

func (m *SchemaMigrator) CreateMigrationFile(name string) (string, error) {
	files, err := m.loadMigrations()
	if err != nil {
		return "", err
	}

	nextVer := 1
	for _, f := range files {
		if f.Version >= nextVer {
			nextVer = f.Version + 1
		}
	}

	safeName := strings.ReplaceAll(strings.ToLower(name), " ", "_")
	filename := fmt.Sprintf("%03d_%s.sql", nextVer, safeName)
	filepath := filepath.Join(m.migrationsDir, filename)

	content := fmt.Sprintf("-- Migration %d: %s\n-- Created at %s\n\n", nextVer, name, time.Now().Format(time.RFC3339))
	if err := os.WriteFile(filepath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("migrate: 创建迁移文件失败: %w", err)
	}

	return filepath, nil
}

func SchemaDump(ctx context.Context, adapter DBAdapter) ([]byte, error) {
	tables, err := adapter.GetTables(ctx)
	if err != nil {
		return nil, err
	}

	var schemas []*TableSchema
	for _, table := range tables {
		schema, err := adapter.GetTableSchema(ctx, table)
		if err != nil {
			continue
		}
		schemas = append(schemas, schema)
	}

	return json.MarshalIndent(schemas, "", "  ")
}
