package developmentmanager

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

const (
	tableDBTable  = "zzz_development_manager_db_table"
	tableDBColumn = "zzz_development_manager_db_column"

	dbTableColumns = "t.id, t.project_id, t.db_type, t.table_schema, t.table_name, t.purpose, t.status, " +
		"t.module_name, t.created_note, " +
		"(SELECT COUNT(*) FROM zzz_development_manager_db_column c WHERE c.table_id = t.id), " +
		"t.created_by, t.updated_by, t.created_at, t.updated_at"

	dbColumnColumns = "id, table_id, column_name, data_type, nullable, default_value, meaning, example, " +
		"is_pk, is_index, note, created_at, updated_at"
)

func scanDBTable(row rowScanner) (*DBTable, error) {
	var t DBTable
	if err := row.Scan(&t.ID, &t.ProjectID, &t.DBType, &t.TableSchema, &t.TableName, &t.Purpose,
		&t.Status, &t.ModuleName, &t.CreatedNote, &t.ColumnCount, &t.CreatedBy, &t.UpdatedBy,
		&t.CreatedAt, &t.UpdatedAt); err != nil {
		return nil, err
	}
	return &t, nil
}

func scanDBColumn(row rowScanner) (*DBColumn, error) {
	var c DBColumn
	var defaultValue sql.NullString
	if err := row.Scan(&c.ID, &c.TableID, &c.ColumnName, &c.DataType, &c.Nullable, &defaultValue,
		&c.Meaning, &c.Example, &c.IsPK, &c.IsIndex, &c.Note, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, err
	}
	if defaultValue.Valid {
		v := defaultValue.String
		c.DefaultValue = &v
	}
	return &c, nil
}

// CurrentSchema 当前连接默认库名
func (r *Repository) CurrentSchema(ctx context.Context) (string, error) {
	var schema sql.NullString
	if err := r.db.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&schema); err != nil {
		return "", err
	}
	return schema.String, nil
}

// ListTables 查询项目数据表（可按状态/模块/关键词筛选）
func (r *Repository) ListTables(ctx context.Context, projectID int64, req *ListTablesReq) ([]DBTable, error) {
	query := "SELECT " + dbTableColumns + " FROM " + tableDBTable + " t WHERE t.project_id = ?"
	args := []any{projectID}
	if req.Status != "" {
		query += " AND t.status = ?"
		args = append(args, req.Status)
	}
	if req.Module != "" {
		query += " AND t.module_name = ?"
		args = append(args, req.Module)
	}
	if req.Keyword != "" {
		query += " AND (t.table_name LIKE ? OR t.purpose LIKE ?)"
		like := "%" + req.Keyword + "%"
		args = append(args, like, like)
	}
	query += " ORDER BY t.id ASC"
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []DBTable
	for rows.Next() {
		t, err := scanDBTable(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *t)
	}
	return items, rows.Err()
}

// GetTable 按 ID 查询数据表
func (r *Repository) GetTable(ctx context.Context, id int64) (*DBTable, error) {
	t, err := scanDBTable(r.db.QueryRowContext(ctx,
		"SELECT "+dbTableColumns+" FROM "+tableDBTable+" t WHERE t.id = ?", id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, NewDevelopmentManagerError(ErrTableNotFound)
		}
		return nil, err
	}
	return t, nil
}

// GetTableByName 按项目 + 库 + 表名查询
func (r *Repository) GetTableByName(ctx context.Context, projectID int64, schema, name string) (*DBTable, error) {
	t, err := scanDBTable(r.db.QueryRowContext(ctx,
		"SELECT "+dbTableColumns+" FROM "+tableDBTable+
			" t WHERE t.project_id = ? AND t.table_schema = ? AND t.table_name = ?",
		projectID, schema, name))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, NewDevelopmentManagerError(ErrTableNotFound)
		}
		return nil, err
	}
	return t, nil
}

// CreateTable 新建数据表登记
func (r *Repository) CreateTable(ctx context.Context, t *DBTable) (*DBTable, error) {
	res, err := r.db.ExecContext(ctx,
		"INSERT INTO "+tableDBTable+
			" (project_id, db_type, table_schema, table_name, purpose, status, module_name, created_note, created_by, updated_by)"+
			" VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		t.ProjectID, t.DBType, t.TableSchema, t.TableName, t.Purpose, t.Status, t.ModuleName,
		t.CreatedNote, t.CreatedBy, t.UpdatedBy)
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate entry") {
			return nil, NewDevelopmentManagerError(ErrDuplicateTable)
		}
		return nil, err
	}
	id, _ := res.LastInsertId()
	return r.GetTable(ctx, id)
}

// UpdateTable 全量更新数据表登记
func (r *Repository) UpdateTable(ctx context.Context, t *DBTable) (*DBTable, error) {
	if _, err := r.db.ExecContext(ctx,
		"UPDATE "+tableDBTable+" SET db_type=?, table_schema=?, table_name=?, purpose=?, status=?, "+
			"module_name=?, created_note=?, updated_by=? WHERE id=?",
		t.DBType, t.TableSchema, t.TableName, t.Purpose, t.Status, t.ModuleName, t.CreatedNote,
		t.UpdatedBy, t.ID); err != nil {
		if strings.Contains(err.Error(), "Duplicate entry") {
			return nil, NewDevelopmentManagerError(ErrDuplicateTable)
		}
		return nil, err
	}
	return r.GetTable(ctx, t.ID)
}

// DeleteTableTx 事务内删除字段并删除数据表登记（硬删除）
func (r *Repository) DeleteTableTx(ctx context.Context, tableID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		"DELETE FROM "+tableDBColumn+" WHERE table_id = ?", tableID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		"DELETE FROM "+tableDBTable+" WHERE id = ?", tableID); err != nil {
		return err
	}
	return tx.Commit()
}

// ListColumns 查询表字段（主键优先，其次登记顺序）
func (r *Repository) ListColumns(ctx context.Context, tableID int64) ([]DBColumn, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT "+dbColumnColumns+" FROM "+tableDBColumn+
			" WHERE table_id = ? ORDER BY is_pk DESC, id ASC", tableID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []DBColumn
	for rows.Next() {
		c, err := scanDBColumn(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *c)
	}
	return items, rows.Err()
}

// GetColumn 按 ID 查询字段
func (r *Repository) GetColumn(ctx context.Context, id int64) (*DBColumn, error) {
	c, err := scanDBColumn(r.db.QueryRowContext(ctx,
		"SELECT "+dbColumnColumns+" FROM "+tableDBColumn+" WHERE id = ?", id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, NewDevelopmentManagerError(ErrColumnNotFound)
		}
		return nil, err
	}
	return c, nil
}

// GetColumnByName 按表 + 字段名查询
func (r *Repository) GetColumnByName(ctx context.Context, tableID int64, name string) (*DBColumn, error) {
	c, err := scanDBColumn(r.db.QueryRowContext(ctx,
		"SELECT "+dbColumnColumns+" FROM "+tableDBColumn+" WHERE table_id = ? AND column_name = ?",
		tableID, name))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, NewDevelopmentManagerError(ErrColumnNotFound)
		}
		return nil, err
	}
	return c, nil
}

// CreateColumn 新建字段
func (r *Repository) CreateColumn(ctx context.Context, c *DBColumn) (*DBColumn, error) {
	res, err := r.db.ExecContext(ctx,
		"INSERT INTO "+tableDBColumn+
			" (table_id, column_name, data_type, nullable, default_value, meaning, example, is_pk, is_index, note)"+
			" VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		c.TableID, c.ColumnName, c.DataType, c.Nullable, c.DefaultValue, c.Meaning, c.Example,
		c.IsPK, c.IsIndex, c.Note)
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate entry") {
			return nil, NewDevelopmentManagerError(ErrDuplicateColumn)
		}
		return nil, err
	}
	id, _ := res.LastInsertId()
	return r.GetColumn(ctx, id)
}

// UpdateColumn 全量更新字段
func (r *Repository) UpdateColumn(ctx context.Context, c *DBColumn) (*DBColumn, error) {
	if _, err := r.db.ExecContext(ctx,
		"UPDATE "+tableDBColumn+" SET column_name=?, data_type=?, nullable=?, default_value=?, meaning=?, "+
			"example=?, is_pk=?, is_index=?, note=? WHERE id=?",
		c.ColumnName, c.DataType, c.Nullable, c.DefaultValue, c.Meaning, c.Example, c.IsPK,
		c.IsIndex, c.Note, c.ID); err != nil {
		if strings.Contains(err.Error(), "Duplicate entry") {
			return nil, NewDevelopmentManagerError(ErrDuplicateColumn)
		}
		return nil, err
	}
	return r.GetColumn(ctx, c.ID)
}

// DeleteColumn 删除字段（硬删除）
func (r *Repository) DeleteColumn(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM "+tableDBColumn+" WHERE id = ?", id)
	return err
}

// ScanTableColumns 从 information_schema 扫描指定表的字段
func (r *Repository) ScanTableColumns(ctx context.Context, schema, table string) ([]ScannedColumn, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COLUMN_DEFAULT, COLUMN_KEY, COLUMN_COMMENT "+
			"FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? ORDER BY ORDINAL_POSITION",
		schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ScannedColumn
	for rows.Next() {
		var name, dataType, nullable, columnKey, comment string
		var defaultValue sql.NullString
		if err := rows.Scan(&name, &dataType, &nullable, &defaultValue, &columnKey, &comment); err != nil {
			return nil, err
		}
		sc := ScannedColumn{
			Name:     name,
			DataType: dataType,
			Nullable: strings.EqualFold(nullable, "YES"),
			IsPK:     strings.EqualFold(columnKey, "PRI"),
			IsIndex:  columnKey == "PRI" || columnKey == "UNI" || columnKey == "MUL",
			Comment:  comment,
		}
		if defaultValue.Valid {
			v := defaultValue.String
			sc.DefaultValue = &v
		}
		items = append(items, sc)
	}
	return items, rows.Err()
}

// UpsertScannedColumn 扫描导入字段：新字段插入，已有字段更新类型/可空/默认值/主键/索引，但保留人工填写的含义
func (r *Repository) UpsertScannedColumn(ctx context.Context, tableID int64, c *ScannedColumn) (bool, error) {
	res, err := r.db.ExecContext(ctx,
		"INSERT INTO "+tableDBColumn+
			" (table_id, column_name, data_type, nullable, default_value, meaning, is_pk, is_index)"+
			" VALUES (?, ?, ?, ?, ?, ?, ?, ?)"+
			" ON DUPLICATE KEY UPDATE data_type=VALUES(data_type), nullable=VALUES(nullable),"+
			" default_value=VALUES(default_value), meaning=IF(meaning = '', VALUES(meaning), meaning),"+
			" is_pk=VALUES(is_pk), is_index=VALUES(is_index)",
		tableID, c.Name, c.DataType, c.Nullable, c.DefaultValue, c.Comment, c.IsPK, c.IsIndex)
	if err != nil {
		return false, err
	}
	affected, _ := res.RowsAffected()
	return affected == 1, nil
}
