package ops

import "time"

// 维护模式
const (
	MaintenanceModeShutdown        = "shutdown"
	MaintenanceModeMigrateReturn   = "migrate_return"
	MaintenanceModeMigrateNoReturn = "migrate_no_return"
)

// 维护状态
const (
	MaintenanceStatusInProgress = "in_progress"
	MaintenanceStatusCompleted  = "completed"
	MaintenanceStatusAborted    = "aborted"
)

var ValidMaintenanceModes = map[string]bool{
	MaintenanceModeShutdown:        true,
	MaintenanceModeMigrateReturn:   true,
	MaintenanceModeMigrateNoReturn: true,
}

// Maintenance L1/L2 手动维护记录
type Maintenance struct {
	ID                int64      `json:"id"`
	Mode              string     `json:"mode"`
	Status            string     `json:"status"`
	SourceNode        string     `json:"source_node"`
	TargetNode        string     `json:"target_node"`
	SwitchedToL2      bool       `json:"switched_to_l2"`
	SyncConfirmed     bool       `json:"sync_confirmed"`
	SyncBackConfirmed bool       `json:"sync_back_confirmed"`
	Note              string     `json:"note"`
	StartedBy         string     `json:"started_by"`
	StartedAt         time.Time  `json:"started_at"`
	EndedBy           string     `json:"ended_by"`
	EndedAt           *time.Time `json:"ended_at,omitempty"`
	EndNote           string     `json:"end_note"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// StartMaintenanceReq 开始维护请求
type StartMaintenanceReq struct {
	Mode          string `json:"mode" binding:"required"`
	SyncConfirmed bool   `json:"sync_confirmed"`
	Note          string `json:"note"`
}

// EndMaintenanceReq 结束维护请求
type EndMaintenanceReq struct {
	SwitchedToL2      *bool  `json:"switched_to_l2"`
	SyncBackConfirmed bool   `json:"sync_back_confirmed"`
	Note              string `json:"note"`
}

// MaintenanceStatusResp 当前维护状态
type MaintenanceStatusResp struct {
	Active  *Maintenance `json:"active"`
	Node    string       `json:"node"`
	Modes   []string     `json:"modes"`
	Warning string       `json:"warning,omitempty"`
}
