package developmentmanager

import "time"

// 数据字典：数据库类型
const (
	DBTypeMySQL    = "mysql"
	DBTypePostgres = "postgres"
)

// 数据字典：表状态
const (
	TableStatusActive     = "active"     // 使用中
	TableStatusDeprecated = "deprecated" // 已废弃
	TableStatusDropped    = "dropped"    // 已删除
)

// ValidDBTypes 数据库类型合法集合
var ValidDBTypes = map[string]bool{DBTypeMySQL: true, DBTypePostgres: true}

// ValidTableStatuses 表状态合法集合
var ValidTableStatuses = map[string]bool{
	TableStatusActive:     true,
	TableStatusDeprecated: true,
	TableStatusDropped:    true,
}

// DBTable 数据表登记
type DBTable struct {
	ID          int64     `json:"id"`
	ProjectID   int64     `json:"project_id"`
	DBType      string    `json:"db_type"`
	TableSchema string    `json:"table_schema"`
	TableName   string    `json:"table_name"`
	Purpose     string    `json:"purpose"`
	Status      string    `json:"status"`
	ModuleName  string    `json:"module_name"`
	CreatedNote string    `json:"created_note"`
	ColumnCount int       `json:"column_count"`
	CreatedBy   string    `json:"created_by"`
	UpdatedBy   string    `json:"updated_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// DBColumn 字段字典
type DBColumn struct {
	ID           int64     `json:"id"`
	TableID      int64     `json:"table_id"`
	ColumnName   string    `json:"column_name"`
	DataType     string    `json:"data_type"`
	Nullable     bool      `json:"nullable"`
	DefaultValue *string   `json:"default_value"`
	Meaning      string    `json:"meaning"`
	Example      string    `json:"example"`
	IsPK         bool      `json:"is_pk"`
	IsIndex      bool      `json:"is_index"`
	Note         string    `json:"note"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ScannedColumn information_schema 扫描出的字段原始信息
type ScannedColumn struct {
	Name         string
	DataType     string
	Nullable     bool
	DefaultValue *string
	IsPK         bool
	IsIndex      bool
	Comment      string
}

// ScanImportResp 扫描导入结果
type ScanImportResp struct {
	TableID int64 `json:"table_id"`
	Scanned int   `json:"scanned"`
	Created int   `json:"created"`
	Updated int   `json:"updated"`
}

// ---------- 请求结构 ----------

// CreateTableReq 新建/更新数据表请求
type CreateTableReq struct {
	DBType      string `json:"db_type"`
	TableSchema string `json:"table_schema"`
	TableName   string `json:"table_name" binding:"required"`
	Purpose     string `json:"purpose"`
	Status      string `json:"status"`
	ModuleName  string `json:"module_name"`
	CreatedNote string `json:"created_note"`
}

// UpdateTableReq 更新数据表请求（全量）
type UpdateTableReq = CreateTableReq

// CreateColumnReq 新建/更新字段请求
type CreateColumnReq struct {
	ColumnName   string  `json:"column_name" binding:"required"`
	DataType     string  `json:"data_type" binding:"required"`
	Nullable     *bool   `json:"nullable"`
	DefaultValue *string `json:"default_value"`
	Meaning      string  `json:"meaning"`
	Example      string  `json:"example"`
	IsPK         bool    `json:"is_pk"`
	IsIndex      bool    `json:"is_index"`
	Note         string  `json:"note"`
}

// UpdateColumnReq 更新字段请求（全量）
type UpdateColumnReq = CreateColumnReq

// ListTablesReq 数据表列表筛选
type ListTablesReq struct {
	Status  string `form:"status"`
	Module  string `form:"module_name"`
	Keyword string `form:"keyword"`
}
