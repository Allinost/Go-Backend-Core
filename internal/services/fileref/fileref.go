package fileref

import (
	"context"
	"time"
)

// ReferenceRecord 文件引用记录，各业务模块在存储文件 URL 时注册一条记录
type ReferenceRecord struct {
	ID            int64     `json:"id"`
	StorageName   string    `json:"storage_name"`
	ObjectKey     string    `json:"object_key"`
	ModuleName    string    `json:"module_name"`
	TableName     string    `json:"table_name,omitempty"`
	RecordID      string    `json:"record_id,omitempty"`
	ColumnName    string    `json:"column_name,omitempty"`
	ReferenceType string    `json:"reference_type,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// ReferenceFilter 引用查询过滤条件
type ReferenceFilter struct {
	StorageName string
	ObjectKey   string
	ModuleName  string
	TableName   string
	RecordID    string
	RefType     string
	Offset      int
	Limit       int
}

// ScanTarget 启发层扫描目标配置
type ScanTarget struct {
	ID            int64     `json:"id"`
	StorageName   string    `json:"storage_name"`
	TableName     string    `json:"table_name"`
	ColumnName    string    `json:"column_name"`
	ModuleName    string    `json:"module_name,omitempty"`
	ReferenceType string    `json:"reference_type,omitempty"`
	Enabled       bool      `json:"enabled"`
	Description   string    `json:"description,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Store 文件引用存储接口，支持 MySQL / 内存 等实现
type Store interface {
	Init(ctx context.Context) error
	Insert(ctx context.Context, refs []ReferenceRecord) error
	DeleteByID(ctx context.Context, id int64) error
	DeleteByRecord(ctx context.Context, moduleName, tableName, recordID string) error
	List(ctx context.Context, filter ReferenceFilter) ([]ReferenceRecord, int64, error)
	AllKeys(ctx context.Context, storageName string) ([]string, error)
	Stats(ctx context.Context) (*UsageStats, error)
	Close() error

	// ScanTarget 管理
	InsertScanTarget(ctx context.Context, t *ScanTarget) error
	UpdateScanTarget(ctx context.Context, t *ScanTarget) error
	DeleteScanTarget(ctx context.Context, id int64) error
	ListScanTargets(ctx context.Context, storageName string, enabledOnly bool) ([]ScanTarget, error)
}

// UsageStats 引用统计
type UsageStats struct {
	TotalReferences int64         `json:"total_references"`
	TotalFiles      int64         `json:"total_files"`
	ByModule        []ModuleStat  `json:"by_module"`
	ByStorage       []StorageStat `json:"by_storage"`
}

type ModuleStat struct {
	ModuleName string `json:"module_name"`
	Count      int64  `json:"count"`
}

type StorageStat struct {
	StorageName string `json:"storage_name"`
	Count       int64  `json:"count"`
}

// Service 文件引用业务服务，各业务模块通过此服务注册文件引用
type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) Init(ctx context.Context) error {
	return s.store.Init(ctx)
}

func (s *Service) Reinit(ctx context.Context) error {
	return s.store.Init(ctx)
}

func (s *Service) Register(ctx context.Context, refs []ReferenceRecord) error {
	return s.store.Insert(ctx, refs)
}

func (s *Service) Remove(ctx context.Context, id int64) error {
	return s.store.DeleteByID(ctx, id)
}

func (s *Service) RemoveByRecord(ctx context.Context, moduleName, tableName, recordID string) error {
	return s.store.DeleteByRecord(ctx, moduleName, tableName, recordID)
}

func (s *Service) List(ctx context.Context, filter ReferenceFilter) ([]ReferenceRecord, int64, error) {
	if filter.Limit <= 0 || filter.Limit > 1000 {
		filter.Limit = 100
	}
	return s.store.List(ctx, filter)
}

func (s *Service) AllKeys(ctx context.Context, storageName string) ([]string, error) {
	return s.store.AllKeys(ctx, storageName)
}

func (s *Service) Stats(ctx context.Context) (*UsageStats, error) {
	return s.store.Stats(ctx)
}

func (s *Service) Close() error {
	return s.store.Close()
}

func (s *Service) AddScanTarget(ctx context.Context, t *ScanTarget) error {
	if t.StorageName == "" {
		t.StorageName = "rustfs"
	}
	if t.ReferenceType == "" {
		t.ReferenceType = "image"
	}
	return s.store.InsertScanTarget(ctx, t)
}

func (s *Service) UpdateScanTarget(ctx context.Context, t *ScanTarget) error {
	return s.store.UpdateScanTarget(ctx, t)
}

func (s *Service) RemoveScanTarget(ctx context.Context, id int64) error {
	return s.store.DeleteScanTarget(ctx, id)
}

func (s *Service) ListScanTargets(ctx context.Context, storageName string, enabledOnly bool) ([]ScanTarget, error) {
	return s.store.ListScanTargets(ctx, storageName, enabledOnly)
}
