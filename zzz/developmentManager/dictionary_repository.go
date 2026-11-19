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
	if err := r.queryRow(ctx, "SELECT current_schema()").Scan(&schema); err != nil {
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
		query += " AND (t.table_name ILIKE ? OR t.purpose ILIKE ?)"
		like := "%" + req.Keyword + "%"
		args = append(args, like, like)
	}
	query += " ORDER BY t.id ASC"
	rows, err := r.query(ctx, query, args...)
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
	t, err := scanDBTable(r.queryRow(ctx,
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
	t, err := scanDBTable(r.queryRow(ctx,
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
	var id int64
	err := r.queryRow(ctx,
		"INSERT INTO "+tableDBTable+
			" (project_id, db_type, table_schema, table_name, purpose, status, module_name, created_note, created_by, updated_by)"+
			" VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id",
		t.ProjectID, t.DBType, t.TableSchema, t.TableName, t.Purpose, t.Status, t.ModuleName,
		t.CreatedNote, t.CreatedBy, t.UpdatedBy).Scan(&id)
	if err != nil {
		if isDuplicateKey(err) {
			return nil, NewDevelopmentManagerError(ErrDuplicateTable)
		}
		return nil, err
	}
	return r.GetTable(ctx, id)
}

// UpdateTable 全量更新数据表登记
func (r *Repository) UpdateTable(ctx context.Context, t *DBTable) (*DBTable, error) {
	if _, err := r.exec(ctx,
		"UPDATE "+tableDBTable+" SET db_type=?, table_schema=?, table_name=?, purpose=?, status=?, "+
			"module_name=?, created_note=?, updated_by=? WHERE id=?",
		t.DBType, t.TableSchema, t.TableName, t.Purpose, t.Status, t.ModuleName, t.CreatedNote,
		t.UpdatedBy, t.ID); err != nil {
		if isDuplicateKey(err) {
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

	if _, err := execTx(ctx, tx,
		"DELETE FROM "+tableDBColumn+" WHERE table_id = ?", tableID); err != nil {
		return err
	}
	if _, err := execTx(ctx, tx,
		"DELETE FROM "+tableDBTable+" WHERE id = ?", tableID); err != nil {
		return err
	}
	return tx.Commit()
}

// ListColumns 查询表字段（主键优先，其次登记顺序）
func (r *Repository) ListColumns(ctx context.Context, tableID int64) ([]DBColumn, error) {
	rows, err := r.query(ctx,
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
	c, err := scanDBColumn(r.queryRow(ctx,
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
	c, err := scanDBColumn(r.queryRow(ctx,
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
	var id int64
	err := r.queryRow(ctx,
		"INSERT INTO "+tableDBColumn+
			" (table_id, column_name, data_type, nullable, default_value, meaning, example, is_pk, is_index, note)"+
			" VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id",
		c.TableID, c.ColumnName, c.DataType, c.Nullable, c.DefaultValue, c.Meaning, c.Example,
		c.IsPK, c.IsIndex, c.Note).Scan(&id)
	if err != nil {
		if isDuplicateKey(err) {
			return nil, NewDevelopmentManagerError(ErrDuplicateColumn)
		}
		return nil, err
	}
	return r.GetColumn(ctx, id)
}

// UpdateColumn 全量更新字段
func (r *Repository) UpdateColumn(ctx context.Context, c *DBColumn) (*DBColumn, error) {
	if _, err := r.exec(ctx,
		"UPDATE "+tableDBColumn+" SET column_name=?, data_type=?, nullable=?, default_value=?, meaning=?, "+
			"example=?, is_pk=?, is_index=?, note=? WHERE id=?",
		c.ColumnName, c.DataType, c.Nullable, c.DefaultValue, c.Meaning, c.Example, c.IsPK,
		c.IsIndex, c.Note, c.ID); err != nil {
		if isDuplicateKey(err) {
			return nil, NewDevelopmentManagerError(ErrDuplicateColumn)
		}
		return nil, err
	}
	return r.GetColumn(ctx, c.ID)
}

// DeleteColumn 删除字段（硬删除）
func (r *Repository) DeleteColumn(ctx context.Context, id int64) error {
	_, err := r.exec(ctx, "DELETE FROM "+tableDBColumn+" WHERE id = ?", id)
	return err
}

// ScanTableColumns 从 information_schema/pg_catalog 扫描指定表的字段
func (r *Repository) ScanTableColumns(ctx context.Context, schema, table string) ([]ScannedColumn, error) {
	rows, err := r.query(ctx, `
		SELECT a.attname,
		       format_type(a.atttypid, a.atttypmod),
		       CASE WHEN a.attnotnull THEN 'NO' ELSE 'YES' END,
		       pg_get_expr(ad.adbin, ad.adrelid),
		       CASE
		         WHEN EXISTS (SELECT 1 FROM pg_index i WHERE i.indrelid = a.attrelid AND i.indisprimary AND a.attnum = ANY(i.indkey)) THEN 'PRI'
		         WHEN EXISTS (SELECT 1 FROM pg_index i WHERE i.indrelid = a.attrelid AND i.indisunique AND a.attnum = ANY(i.indkey)) THEN 'UNI'
		         WHEN EXISTS (SELECT 1 FROM pg_index i WHERE i.indrelid = a.attrelid AND a.attnum = ANY(i.indkey)) THEN 'MUL'
		         ELSE ''
		       END,
		       COALESCE(col_description(a.attrelid, a.attnum), '')
		FROM pg_attribute a
		JOIN pg_class c ON c.oid = a.attrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		LEFT JOIN pg_attrdef ad ON ad.adrelid = a.attrelid AND ad.adnum = a.attnum
		WHERE n.nspname = ? AND c.relname = ? AND a.attnum > 0 AND NOT a.attisdropped
		ORDER BY a.attnum`, schema, table)
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
	var inserted bool
	err := r.queryRow(ctx,
		"INSERT INTO "+tableDBColumn+
			" (table_id, column_name, data_type, nullable, default_value, meaning, is_pk, is_index)"+
			" VALUES (?, ?, ?, ?, ?, ?, ?, ?)"+
			" ON CONFLICT (table_id, column_name) DO UPDATE SET"+
			" data_type=EXCLUDED.data_type, nullable=EXCLUDED.nullable,"+
			" default_value=EXCLUDED.default_value,"+
			" meaning=CASE WHEN zzz_development_manager_db_column.meaning = '' THEN EXCLUDED.meaning ELSE zzz_development_manager_db_column.meaning END,"+
			" is_pk=EXCLUDED.is_pk, is_index=EXCLUDED.is_index, updated_at=now()"+
			" RETURNING (xmax = 0)",
		tableID, c.Name, c.DataType, c.Nullable, c.DefaultValue, c.Comment, c.IsPK, c.IsIndex).Scan(&inserted)
	if err != nil {
		return false, err
	}
	return inserted, nil
}
