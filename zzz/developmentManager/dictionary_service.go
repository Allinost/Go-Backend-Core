package developmentmanager

import (
	"context"
	"regexp"
	"strings"
)

// identifierRe 库名/表名/字段名格式：字母、数字、下划线
var identifierRe = regexp.MustCompile(`^[A-Za-z0-9_]{1,128}$`)

// ---------- 数据表 ----------

// ListTables 查询项目数据表字典
func (s *Service) ListTables(ctx context.Context, projectID int64, req *ListTablesReq) ([]DBTable, error) {
	if _, err := s.requireProject(ctx, projectID); err != nil {
		return nil, err
	}
	if req.Status != "" && !ValidTableStatuses[req.Status] {
		return nil, NewDevelopmentManagerError(ErrInvalidTableStatus)
	}
	return s.repo.ListTables(ctx, projectID, req)
}

// GetTable 查询数据表详情
func (s *Service) GetTable(ctx context.Context, id int64) (*DBTable, error) {
	return s.repo.GetTable(ctx, id)
}

// normalizeTableReq 校验并归一化数据表请求（库名缺省取当前连接默认库）
func (s *Service) normalizeTableReq(ctx context.Context, req *CreateTableReq) (*DBTable, error) {
	dbType := strings.TrimSpace(req.DBType)
	if dbType == "" {
		dbType = DBTypeMySQL
	}
	if !ValidDBTypes[dbType] {
		return nil, NewDevelopmentManagerError(ErrInvalidDBType)
	}
	schema := strings.TrimSpace(req.TableSchema)
	if schema != "" && !identifierRe.MatchString(schema) {
		return nil, NewDevelopmentManagerError(ErrInvalidTableName)
	}
	name := strings.TrimSpace(req.TableName)
	if !identifierRe.MatchString(name) {
		return nil, NewDevelopmentManagerError(ErrInvalidTableName)
	}
	if schema == "" {
		current, err := s.repo.CurrentSchema(ctx)
		if err != nil {
			return nil, err
		}
		if current == "" {
			return nil, NewDevelopmentManagerError(ErrInvalidTableName).WithDetail("无法解析当前库名，请显式指定 table_schema")
		}
		schema = current
	}
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = TableStatusActive
	}
	if !ValidTableStatuses[status] {
		return nil, NewDevelopmentManagerError(ErrInvalidTableStatus)
	}
	return &DBTable{
		DBType:      dbType,
		TableSchema: schema,
		TableName:   name,
		Purpose:     strings.TrimSpace(req.Purpose),
		Status:      status,
		ModuleName:  strings.TrimSpace(req.ModuleName),
		CreatedNote: strings.TrimSpace(req.CreatedNote),
	}, nil
}

// CreateTable 登记数据表
func (s *Service) CreateTable(ctx context.Context, projectID int64, req *CreateTableReq, actor string) (*DBTable, error) {
	if _, err := s.requireProject(ctx, projectID); err != nil {
		return nil, err
	}
	t, err := s.normalizeTableReq(ctx, req)
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.GetTableByName(ctx, projectID, t.TableSchema, t.TableName); err == nil {
		return nil, NewDevelopmentManagerError(ErrDuplicateTable)
	} else if !isDMCode(err, ErrTableNotFound) {
		return nil, err
	}
	t.ProjectID = projectID
	t.CreatedBy = actor
	t.UpdatedBy = actor
	return s.repo.CreateTable(ctx, t)
}

// UpdateTable 全量更新数据表登记
func (s *Service) UpdateTable(ctx context.Context, id int64, req *UpdateTableReq, actor string) (*DBTable, error) {
	existing, err := s.repo.GetTable(ctx, id)
	if err != nil {
		return nil, err
	}
	t, err := s.normalizeTableReq(ctx, req)
	if err != nil {
		return nil, err
	}
	if dup, err := s.repo.GetTableByName(ctx, existing.ProjectID, t.TableSchema, t.TableName); err == nil && dup.ID != id {
		return nil, NewDevelopmentManagerError(ErrDuplicateTable)
	} else if err != nil && !isDMCode(err, ErrTableNotFound) {
		return nil, err
	}
	t.ID = id
	t.ProjectID = existing.ProjectID
	t.CreatedBy = existing.CreatedBy
	t.UpdatedBy = actor
	return s.repo.UpdateTable(ctx, t)
}

// DeleteTable 删除数据表登记及其字段（硬删除）
func (s *Service) DeleteTable(ctx context.Context, id int64) error {
	if _, err := s.repo.GetTable(ctx, id); err != nil {
		return err
	}
	return s.repo.DeleteTableTx(ctx, id)
}

// ---------- 字段 ----------

// ListColumns 查询表字段字典
func (s *Service) ListColumns(ctx context.Context, tableID int64) ([]DBColumn, error) {
	if _, err := s.repo.GetTable(ctx, tableID); err != nil {
		return nil, err
	}
	return s.repo.ListColumns(ctx, tableID)
}

// normalizeColumnReq 校验并归一化字段请求
func normalizeColumnReq(req *CreateColumnReq) (*DBColumn, error) {
	name := strings.TrimSpace(req.ColumnName)
	if !identifierRe.MatchString(name) {
		return nil, NewDevelopmentManagerError(ErrInvalidColumnName)
	}
	dataType := strings.TrimSpace(req.DataType)
	if dataType == "" || len(dataType) > 128 {
		return nil, NewDevelopmentManagerError(ErrInvalidColumnName)
	}
	nullable := true
	if req.Nullable != nil {
		nullable = *req.Nullable
	}
	return &DBColumn{
		ColumnName:   name,
		DataType:     dataType,
		Nullable:     nullable,
		DefaultValue: req.DefaultValue,
		Meaning:      strings.TrimSpace(req.Meaning),
		Example:      strings.TrimSpace(req.Example),
		IsPK:         req.IsPK,
		IsIndex:      req.IsIndex,
		Note:         strings.TrimSpace(req.Note),
	}, nil
}

// CreateColumn 新增字段
func (s *Service) CreateColumn(ctx context.Context, tableID int64, req *CreateColumnReq) (*DBColumn, error) {
	if _, err := s.repo.GetTable(ctx, tableID); err != nil {
		return nil, err
	}
	c, err := normalizeColumnReq(req)
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.GetColumnByName(ctx, tableID, c.ColumnName); err == nil {
		return nil, NewDevelopmentManagerError(ErrDuplicateColumn)
	} else if !isDMCode(err, ErrColumnNotFound) {
		return nil, err
	}
	c.TableID = tableID
	return s.repo.CreateColumn(ctx, c)
}

// UpdateColumn 全量更新字段
func (s *Service) UpdateColumn(ctx context.Context, id int64, req *UpdateColumnReq) (*DBColumn, error) {
	existing, err := s.repo.GetColumn(ctx, id)
	if err != nil {
		return nil, err
	}
	c, err := normalizeColumnReq(req)
	if err != nil {
		return nil, err
	}
	if dup, err := s.repo.GetColumnByName(ctx, existing.TableID, c.ColumnName); err == nil && dup.ID != id {
		return nil, NewDevelopmentManagerError(ErrDuplicateColumn)
	} else if err != nil && !isDMCode(err, ErrColumnNotFound) {
		return nil, err
	}
	c.ID = id
	c.TableID = existing.TableID
	return s.repo.UpdateColumn(ctx, c)
}

// DeleteColumn 删除字段（硬删除）
func (s *Service) DeleteColumn(ctx context.Context, id int64) error {
	if _, err := s.repo.GetColumn(ctx, id); err != nil {
		return err
	}
	return s.repo.DeleteColumn(ctx, id)
}

// ScanImportColumns 从 information_schema 扫描表字段并导入/更新字典（保留人工填写的含义）
func (s *Service) ScanImportColumns(ctx context.Context, tableID int64) (*ScanImportResp, error) {
	t, err := s.repo.GetTable(ctx, tableID)
	if err != nil {
		return nil, err
	}
	if t.DBType != DBTypeMySQL {
		return nil, NewDevelopmentManagerError(ErrScanUnsupported)
	}
	schema := t.TableSchema
	if schema == "" {
		current, err := s.repo.CurrentSchema(ctx)
		if err != nil {
			return nil, err
		}
		if current == "" {
			return nil, NewDevelopmentManagerError(ErrInvalidTableName).WithDetail("表未登记库名且无法解析当前库")
		}
		schema = current
	}
	cols, err := s.repo.ScanTableColumns(ctx, schema, t.TableName)
	if err != nil {
		return nil, NewDevelopmentManagerError(ErrScanFailed).WithDetail(err.Error())
	}
	if len(cols) == 0 {
		return nil, NewDevelopmentManagerError(ErrScanFailed).WithDetail(
			"information_schema 中未找到表 " + schema + "." + t.TableName)
	}

	resp := &ScanImportResp{TableID: tableID, Scanned: len(cols)}
	for _, col := range cols {
		created, err := s.repo.UpsertScannedColumn(ctx, tableID, &col)
		if err != nil {
			return nil, err
		}
		if created {
			resp.Created++
		} else {
			resp.Updated++
		}
	}
	return resp, nil
}
