package fileref

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Allinost/go-backend-core/internal/pkg/logger"
)

const (
	tableName       = "file_references"
	targetTableName = "scan_targets"
)

// MySQLStore 基于 MySQL 的文件引用存储
type MySQLStore struct {
	db *sql.DB
}

func NewMySQLStore(db *sql.DB) *MySQLStore {
	return &MySQLStore{db: db}
}

// execDDL 执行一条 DDL 并记录精确 SQL（出错时方便排查）
func (m *MySQLStore) execDDL(ctx context.Context, label, ddl string) error {
	logger.Info().Str("ddl", ddl).Msg("fileref: 执行 " + label)
	_, err := m.db.ExecContext(ctx, ddl)
	if err != nil {
		logger.Error().Str("ddl", ddl).Err(err).Msg("fileref: " + label + " 失败")
		return fmt.Errorf("%s 失败: %w", label, err)
	}
	return nil
}

func (m *MySQLStore) Init(ctx context.Context) error {
	// 记录当前连接的数据库
	var dbName string
	if err := m.db.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&dbName); err != nil {
		logger.Warn().Err(err).Msg("fileref: 无法获取当前数据库名")
		dbName = "unknown"
	} else {
		logger.Info().Str("database", dbName).Msg("fileref: 当前 MySQL 数据库")
	}

	// 创建 file_references 表
	if err := m.execDDL(ctx, "创建 file_references 表", fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		storage_name VARCHAR(64) NOT NULL DEFAULT 'rustfs',
		object_key VARCHAR(500) NOT NULL,
		module_name VARCHAR(64) NOT NULL,
		table_name VARCHAR(64) NOT NULL DEFAULT '',
		record_id VARCHAR(64) NOT NULL DEFAULT '',
		column_name VARCHAR(64) NOT NULL DEFAULT '',
		reference_type VARCHAR(32) NOT NULL DEFAULT 'image',
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		INDEX idx_storage_object (storage_name, object_key(255)),
		INDEX idx_module_ref (module_name, table_name, record_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`, tableName)); err != nil {
		return err
	}

	// 创建 scan_targets 表
	if err := m.execDDL(ctx, "创建 scan_targets 表", fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		storage_name VARCHAR(64) NOT NULL DEFAULT 'rustfs',
		table_name VARCHAR(64) NOT NULL,
		column_name VARCHAR(64) NOT NULL,
		module_name VARCHAR(64) NOT NULL DEFAULT '',
		reference_type VARCHAR(32) NOT NULL DEFAULT 'image',
		enabled TINYINT(1) NOT NULL DEFAULT 1,
		description VARCHAR(255) NOT NULL DEFAULT '',
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		UNIQUE INDEX idx_unique_target (storage_name, table_name, column_name)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`, targetTableName)); err != nil {
		return err
	}

	// 验证表是否真的创建成功
	for _, tbl := range []string{tableName, targetTableName} {
		var cnt int
		if err := m.db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = ? AND table_name = ?",
			dbName, tbl).Scan(&cnt); err != nil {
			logger.Warn().Str("table", tbl).Err(err).Msg("fileref: 验证表存在性失败")
		} else if cnt == 0 {
			logger.Error().Str("database", dbName).Str("table", tbl).Msg("fileref: 表创建后仍不存在！请检查 MySQL 权限和数据库名称")
		} else {
			logger.Info().Str("table", tbl).Msg("fileref: 表已确认存在")
		}
	}

	// 插入默认扫描目标（首次启动）
	seed := fmt.Sprintf(`INSERT IGNORE INTO %s (storage_name, table_name, column_name, module_name, reference_type, description) VALUES
		('rustfs', 'zzz_goodser_products', 'image_url', 'goodser', 'image', '商品主图'),
		('rustfs', 'zzz_goodser_products', 'images', 'goodser', 'image', '商品图片列表')`, targetTableName)
	if _, err := m.db.ExecContext(ctx, seed); err != nil {
		logger.Warn().Err(err).Msg("fileref: 插入默认扫描目标失败")
	} else {
		logger.Info().Msg("fileref: 默认扫描目标已插入")
	}

	logger.Info().Msg("fileref: 数据表初始化完成")
	return nil
}

func (m *MySQLStore) Insert(ctx context.Context, refs []ReferenceRecord) error {
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

	_, err := m.db.ExecContext(ctx, query, valueArgs...)
	if err != nil {
		return fmt.Errorf("插入文件引用失败: %w", err)
	}
	return nil
}

func (m *MySQLStore) DeleteByID(ctx context.Context, id int64) error {
	_, err := m.db.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE id = ?", tableName), id)
	if err != nil {
		return fmt.Errorf("删除文件引用失败 (id=%d): %w", id, err)
	}
	return nil
}

func (m *MySQLStore) DeleteByRecord(ctx context.Context, moduleName, tableName, recordID string) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE module_name = ? AND table_name = ? AND record_id = ?", tableName)
	_, err := m.db.ExecContext(ctx, query, moduleName, tableName, recordID)
	if err != nil {
		return fmt.Errorf("删除记录文件引用失败: %w", err)
	}
	return nil
}

func (m *MySQLStore) List(ctx context.Context, filter ReferenceFilter) ([]ReferenceRecord, int64, error) {
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
	if err := m.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
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

	rows, err := m.db.QueryContext(ctx, query, listArgs...)
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

func (m *MySQLStore) AllKeys(ctx context.Context, storageName string) ([]string, error) {
	query := fmt.Sprintf("SELECT DISTINCT object_key FROM %s WHERE storage_name = ?", tableName)
	if storageName == "" {
		query = fmt.Sprintf("SELECT DISTINCT object_key FROM %s", tableName)
	}

	rows, err := m.db.QueryContext(ctx, query)
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

func (m *MySQLStore) Stats(ctx context.Context) (*UsageStats, error) {
	stats := &UsageStats{}

	// 总引用数
	if err := m.db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)).Scan(&stats.TotalReferences); err != nil {
		return nil, fmt.Errorf("统计引用总数失败: %w", err)
	}

	// 独立文件数
	if err := m.db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(DISTINCT storage_name, object_key) FROM %s", tableName)).Scan(&stats.TotalFiles); err != nil {
		return nil, fmt.Errorf("统计文件数失败: %w", err)
	}

	// 按模块统计
	mRows, err := m.db.QueryContext(ctx, fmt.Sprintf("SELECT module_name, COUNT(*) as cnt FROM %s GROUP BY module_name ORDER BY cnt DESC", tableName))
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
	sRows, err := m.db.QueryContext(ctx, fmt.Sprintf("SELECT storage_name, COUNT(*) as cnt FROM %s GROUP BY storage_name ORDER BY cnt DESC", tableName))
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

func (m *MySQLStore) InsertScanTarget(ctx context.Context, t *ScanTarget) error {
	now := time.Now()
	query := fmt.Sprintf(`INSERT INTO %s (storage_name, table_name, column_name, module_name, reference_type, enabled, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		targetTableName)
	_, err := m.db.ExecContext(ctx, query,
		t.StorageName, t.TableName, t.ColumnName, t.ModuleName, t.ReferenceType, t.Enabled, t.Description, now, now)
	if err != nil {
		return fmt.Errorf("插入扫描目标失败: %w", err)
	}
	return nil
}

func (m *MySQLStore) UpdateScanTarget(ctx context.Context, t *ScanTarget) error {
	query := fmt.Sprintf(`UPDATE %s SET storage_name=?, table_name=?, column_name=?, module_name=?, reference_type=?, enabled=?, description=?, updated_at=? WHERE id=?`,
		targetTableName)
	_, err := m.db.ExecContext(ctx, query,
		t.StorageName, t.TableName, t.ColumnName, t.ModuleName, t.ReferenceType, t.Enabled, t.Description, time.Now(), t.ID)
	if err != nil {
		return fmt.Errorf("更新扫描目标失败: %w", err)
	}
	return nil
}

func (m *MySQLStore) DeleteScanTarget(ctx context.Context, id int64) error {
	_, err := m.db.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE id=?", targetTableName), id)
	if err != nil {
		return fmt.Errorf("删除扫描目标失败: %w", err)
	}
	return nil
}

func (m *MySQLStore) ListScanTargets(ctx context.Context, storageName string, enabledOnly bool) ([]ScanTarget, error) {
	var conditions []string
	var args []interface{}

	if storageName != "" {
		conditions = append(conditions, "storage_name = ?")
		args = append(args, storageName)
	}
	if enabledOnly {
		conditions = append(conditions, "enabled = 1")
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf("SELECT id, storage_name, table_name, column_name, module_name, reference_type, enabled, description, created_at, updated_at FROM %s%s ORDER BY module_name, table_name, column_name",
		targetTableName, whereClause)

	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询扫描目标失败: %w", err)
	}
	defer rows.Close()

	var targets []ScanTarget
	for rows.Next() {
		var t ScanTarget
		var enabled int
		if err := rows.Scan(&t.ID, &t.StorageName, &t.TableName, &t.ColumnName,
			&t.ModuleName, &t.ReferenceType, &enabled, &t.Description, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("扫描扫描目标记录失败: %w", err)
		}
		t.Enabled = enabled == 1
		targets = append(targets, t)
	}
	return targets, nil
}

func (m *MySQLStore) Close() error {
	return nil
}
