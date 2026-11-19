package fileref

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Allinost/go-backend-core/internal/pkg/dbutil"
	"github.com/Allinost/go-backend-core/internal/pkg/logger"
)

const (
	tableName       = "file_references"
	targetTableName = "scan_targets"
)

// PostgresStore 基于 PostgreSQL 的文件引用存储
type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (m *PostgresStore) queryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return m.db.QueryContext(ctx, dbutil.Rebind(query), args...)
}

func (m *PostgresStore) queryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return m.db.QueryRowContext(ctx, dbutil.Rebind(query), args...)
}

func (m *PostgresStore) execContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return m.db.ExecContext(ctx, dbutil.Rebind(query), args...)
}

func (m *PostgresStore) Init(ctx context.Context) error {
	for _, tbl := range []string{tableName, targetTableName} {
		var exists bool
		if err := m.queryRowContext(ctx, `SELECT to_regclass($1) IS NOT NULL`, tbl).Scan(&exists); err != nil {
			return fmt.Errorf("检查 %s 失败: %w", tbl, err)
		}
		if !exists {
			return fmt.Errorf("%s 表不存在，请先执行 PostgreSQL 迁移", tbl)
		}
	}

	// 首次启动插入默认扫描目标；表由 migrations 管理，运行时只做幂等 seed。
	seed := fmt.Sprintf(`INSERT INTO %s (storage_name, table_name, column_name, module_name, reference_type, description) VALUES
		('rustfs', 'zzz_goodser_products', 'image_url', 'goodser', 'image', '商品主图'),
		('rustfs', 'zzz_goodser_products', 'images', 'goodser', 'image', '商品图片列表')
		ON CONFLICT (storage_name, table_name, column_name) DO NOTHING`, targetTableName)
	if _, err := m.execContext(ctx, seed); err != nil {
		logger.Warn().Err(err).Msg("fileref: 插入默认扫描目标失败")
	} else {
		logger.Info().Msg("fileref: Postgres 表检查完成")
	}
	return nil
}

func (m *PostgresStore) Insert(ctx context.Context, refs []ReferenceRecord) error {
	if len(refs) == 0 {
		return nil
	}
	now := time.Now()
	valueStrings := make([]string, 0, len(refs))
	valueArgs := make([]interface{}, 0, len(refs)*8)

	for _, r := range refs {
		valueStrings = append(valueStrings, "(?, ?, ?, ?, ?, ?, ?, ?, ?)")
		storageName := r.StorageName
		if storageName == "" {
			storageName = "rustfs"
		}
		refType := r.ReferenceType
		if refType == "" {
			refType = "image"
		}
		valueArgs = append(valueArgs, storageName, r.ObjectKey, r.ModuleName,
			r.TableName, r.RecordID, r.ColumnName, refType, now, now)
	}

	query := fmt.Sprintf(`INSERT INTO %s (storage_name, object_key, module_name, table_name, record_id, column_name, reference_type, created_at, updated_at) VALUES %s`,
		tableName, strings.Join(valueStrings, ","))

	_, err := m.execContext(ctx, query, valueArgs...)
	if err != nil {
		return fmt.Errorf("插入文件引用失败: %w", err)
	}
	return nil
}

func (m *PostgresStore) DeleteByID(ctx context.Context, id int64) error {
	_, err := m.execContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE id = ?", tableName), id)
	if err != nil {
		return fmt.Errorf("删除文件引用失败 (id=%d): %w", id, err)
	}
	return nil
}

func (m *PostgresStore) DeleteByRecord(ctx context.Context, moduleName, recordTable, recordID string) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE module_name = ? AND table_name = ? AND record_id = ?", tableName)
	_, err := m.execContext(ctx, query, moduleName, recordTable, recordID)
	if err != nil {
		return fmt.Errorf("删除记录文件引用失败: %w", err)
	}
	return nil
}

func (m *PostgresStore) List(ctx context.Context, filter ReferenceFilter) ([]ReferenceRecord, int64, error) {
	var conditions []string
	var args []interface{}

	if filter.StorageName != "" {
		conditions = append(conditions, "storage_name = ?")
		args = append(args, filter.StorageName)
	}
	if filter.ObjectKey != "" {
		conditions = append(conditions, "object_key = ?")
		args = append(args, filter.ObjectKey)
	}
	if filter.ModuleName != "" {
		conditions = append(conditions, "module_name = ?")
		args = append(args, filter.ModuleName)
	}
	if filter.TableName != "" {
		conditions = append(conditions, "table_name = ?")
		args = append(args, filter.TableName)
	}
	if filter.RecordID != "" {
		conditions = append(conditions, "record_id = ?")
		args = append(args, filter.RecordID)
	}
	if filter.RefType != "" {
		conditions = append(conditions, "reference_type = ?")
		args = append(args, filter.RefType)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}

	// 计数
	var total int64
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s%s", tableName, whereClause)
	if err := m.queryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("查询引用计数失败: %w", err)
	}

	// 列表
	offset := filter.Offset
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}

	query := fmt.Sprintf("SELECT id, storage_name, object_key, module_name, table_name, record_id, column_name, reference_type, created_at, updated_at FROM %s%s ORDER BY id DESC LIMIT ? OFFSET ?",
		tableName, whereClause)
	listArgs := append(args, limit, offset)

	rows, err := m.queryContext(ctx, query, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("查询文件引用列表失败: %w", err)
	}
	defer rows.Close()

	var records []ReferenceRecord
	for rows.Next() {
		var r ReferenceRecord
		if err := rows.Scan(&r.ID, &r.StorageName, &r.ObjectKey, &r.ModuleName,
			&r.TableName, &r.RecordID, &r.ColumnName, &r.ReferenceType,
			&r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("扫描引用记录失败: %w", err)
		}
		records = append(records, r)
	}

	return records, total, nil
}

func (m *PostgresStore) AllKeys(ctx context.Context, storageName string) ([]string, error) {
	query := fmt.Sprintf("SELECT DISTINCT object_key FROM %s WHERE storage_name = ?", tableName)
	args := []any{storageName}
	if storageName == "" {
		query = fmt.Sprintf("SELECT DISTINCT object_key FROM %s", tableName)
		args = nil
	}

	rows, err := m.queryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询所有引用文件键失败: %w", err)
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("扫描文件键失败: %w", err)
		}
		keys = append(keys, key)
	}
	return keys, nil
}

func (m *PostgresStore) Stats(ctx context.Context) (*UsageStats, error) {
	stats := &UsageStats{}

	// 总引用数
	if err := m.queryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)).Scan(&stats.TotalReferences); err != nil {
		return nil, fmt.Errorf("统计引用总数失败: %w", err)
	}

	// 独立文件数
	if err := m.queryRowContext(ctx, fmt.Sprintf("SELECT COUNT(DISTINCT (storage_name, object_key)) FROM %s", tableName)).Scan(&stats.TotalFiles); err != nil {
		return nil, fmt.Errorf("统计文件数失败: %w", err)
	}

	// 按模块统计
	mRows, err := m.queryContext(ctx, fmt.Sprintf("SELECT module_name, COUNT(*) as cnt FROM %s GROUP BY module_name ORDER BY cnt DESC", tableName))
	if err != nil {
		return nil, fmt.Errorf("统计模块引用失败: %w", err)
	}
	defer mRows.Close()
	for mRows.Next() {
		var ms ModuleStat
		if err := mRows.Scan(&ms.ModuleName, &ms.Count); err != nil {
			return nil, fmt.Errorf("扫描模块统计失败: %w", err)
		}
		stats.ByModule = append(stats.ByModule, ms)
	}

	// 按存储统计
	sRows, err := m.queryContext(ctx, fmt.Sprintf("SELECT storage_name, COUNT(*) as cnt FROM %s GROUP BY storage_name ORDER BY cnt DESC", tableName))
	if err != nil {
		return nil, fmt.Errorf("统计存储引用失败: %w", err)
	}
	defer sRows.Close()
	for sRows.Next() {
		var ss StorageStat
		if err := sRows.Scan(&ss.StorageName, &ss.Count); err != nil {
			return nil, fmt.Errorf("扫描存储统计失败: %w", err)
		}
		stats.ByStorage = append(stats.ByStorage, ss)
	}

	return stats, nil
}

func (m *PostgresStore) InsertScanTarget(ctx context.Context, t *ScanTarget) error {
	now := time.Now()
	query := fmt.Sprintf(`INSERT INTO %s (storage_name, table_name, column_name, module_name, reference_type, enabled, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		targetTableName)
	_, err := m.execContext(ctx, query,
		t.StorageName, t.TableName, t.ColumnName, t.ModuleName, t.ReferenceType, t.Enabled, t.Description, now, now)
	if err != nil {
		return fmt.Errorf("插入扫描目标失败: %w", err)
	}
	return nil
}

func (m *PostgresStore) UpdateScanTarget(ctx context.Context, t *ScanTarget) error {
	query := fmt.Sprintf(`UPDATE %s SET storage_name=?, table_name=?, column_name=?, module_name=?, reference_type=?, enabled=?, description=?, updated_at=? WHERE id=?`,
		targetTableName)
	_, err := m.execContext(ctx, query,
		t.StorageName, t.TableName, t.ColumnName, t.ModuleName, t.ReferenceType, t.Enabled, t.Description, time.Now(), t.ID)
	if err != nil {
		return fmt.Errorf("更新扫描目标失败: %w", err)
	}
	return nil
}

func (m *PostgresStore) DeleteScanTarget(ctx context.Context, id int64) error {
	_, err := m.execContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE id=?", targetTableName), id)
	if err != nil {
		return fmt.Errorf("删除扫描目标失败: %w", err)
	}
	return nil
}

func (m *PostgresStore) ListScanTargets(ctx context.Context, storageName string, enabledOnly bool) ([]ScanTarget, error) {
	var conditions []string
	var args []interface{}

	if storageName != "" {
		conditions = append(conditions, "storage_name = ?")
		args = append(args, storageName)
	}
	if enabledOnly {
		conditions = append(conditions, "enabled = true")
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf("SELECT id, storage_name, table_name, column_name, module_name, reference_type, enabled, description, created_at, updated_at FROM %s%s ORDER BY module_name, table_name, column_name",
		targetTableName, whereClause)

	rows, err := m.queryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询扫描目标失败: %w", err)
	}
	defer rows.Close()

	var targets []ScanTarget
	for rows.Next() {
		var t ScanTarget
		var enabled bool
		if err := rows.Scan(&t.ID, &t.StorageName, &t.TableName, &t.ColumnName,
			&t.ModuleName, &t.ReferenceType, &enabled, &t.Description, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("扫描扫描目标记录失败: %w", err)
		}
		t.Enabled = enabled
		targets = append(targets, t)
	}
	return targets, nil
}

func (m *PostgresStore) Close() error {
	return nil
}
