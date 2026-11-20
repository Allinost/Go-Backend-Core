package developmentmanager

import (
	"encoding/json"
	"time"
)

// ---------- 枚举 ----------

var ValidIterationStatuses = map[string]bool{"planned": true, "active": true, "closed": true}

var ValidRiskProbability = map[string]bool{"low": true, "medium": true, "high": true}
var ValidRiskImpact = map[string]bool{"low": true, "medium": true, "high": true}
var ValidRiskLevels = map[string]bool{"low": true, "medium": true, "high": true, "critical": true}
var ValidRiskStatuses = map[string]bool{"open": true, "mitigating": true, "closed": true, "accepted": true}

var ValidReleaseTypes = map[string]bool{"major": true, "minor": true, "patch": true, "hotfix": true}
var ValidReleaseStatuses = map[string]bool{"planned": true, "deploying": true, "deployed": true, "rolled_back": true}

var ValidCommentTargets = map[string]bool{"requirement": true, "task": true, "risk": true, "release": true}
var ValidTaskAssetRelations = map[string]bool{"input": true, "output": true, "evidence": true}

// ---------- 实体 ----------

type Iteration struct {
	ID            int64      `json:"id"`
	ProjectID     int64      `json:"project_id"`
	MilestoneID   *int64     `json:"milestone_id,omitempty"`
	Name          string     `json:"name"`
	Goal          string     `json:"goal"`
	StartDate     *time.Time `json:"start_date,omitempty"`
	EndDate       *time.Time `json:"end_date,omitempty"`
	Status        string     `json:"status"`
	CapacityHours float64    `json:"capacity_hours"`
	Deleted       bool       `json:"deleted"`
	DeletedAt     *time.Time `json:"deleted_at,omitempty"`
	CreatedBy     string     `json:"created_by"`
	UpdatedBy     string     `json:"updated_by"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type Risk struct {
	ID          int64      `json:"id"`
	ProjectID   int64      `json:"project_id"`
	MilestoneID *int64     `json:"milestone_id,omitempty"`
	TaskID      *int64     `json:"task_id,omitempty"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Probability string     `json:"probability"`
	Impact      string     `json:"impact"`
	Level       string     `json:"level"`
	Mitigation  string     `json:"mitigation"`
	Owner       string     `json:"owner"`
	Status      string     `json:"status"`
	DueDate     *time.Time `json:"due_date,omitempty"`
	Deleted     bool       `json:"deleted"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
	CreatedBy   string     `json:"created_by"`
	UpdatedBy   string     `json:"updated_by"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type Release struct {
	ID            int64      `json:"id"`
	ProjectID     int64      `json:"project_id"`
	MilestoneID   *int64     `json:"milestone_id,omitempty"`
	Version       string     `json:"version"`
	Name          string     `json:"name"`
	ReleaseType   string     `json:"release_type"`
	Status        string     `json:"status"`
	ReleasedAt    *time.Time `json:"released_at,omitempty"`
	ChangeSummary string     `json:"change_summary"`
	RollbackPlan  string     `json:"rollback_plan"`
	DeployedBy    string     `json:"deployed_by"`
	Deleted       bool       `json:"deleted"`
	DeletedAt     *time.Time `json:"deleted_at,omitempty"`
	CreatedBy     string     `json:"created_by"`
	UpdatedBy     string     `json:"updated_by"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type Baseline struct {
	ID          int64           `json:"id"`
	ProjectID   int64           `json:"project_id"`
	MilestoneID *int64          `json:"milestone_id,omitempty"`
	Name        string          `json:"name"`
	Snapshot    json.RawMessage `json:"snapshot"`
	Note        string          `json:"note"`
	CreatedBy   string          `json:"created_by"`
	CreatedAt   time.Time       `json:"created_at"`
}

type Comment struct {
	ID         int64     `json:"id"`
	ProjectID  int64     `json:"project_id"`
	TargetType string    `json:"target_type"`
	TargetID   int64     `json:"target_id"`
	Content    string    `json:"content"`
	Operator   string    `json:"operator"`
	CreatedAt  time.Time `json:"created_at"`
}

type ChecklistItem struct {
	ID        int64     `json:"id"`
	TaskID    int64     `json:"task_id"`
	Title     string    `json:"title"`
	Done      bool      `json:"done"`
	OrderNo   int       `json:"order_no"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Label struct {
	ID        int64     `json:"id"`
	ProjectID int64     `json:"project_id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type TaskLabel struct {
	ID        int64     `json:"id"`
	TaskID    int64     `json:"task_id"`
	LabelID   int64     `json:"label_id"`
	CreatedAt time.Time `json:"created_at"`
}

type TaskAsset struct {
	ID           int64     `json:"id"`
	TaskID       int64     `json:"task_id"`
	AssetID      int64     `json:"asset_id"`
	RelationType string    `json:"relation_type"`
	CreatedAt    time.Time `json:"created_at"`
}

// ---------- 请求 ----------

type CreateIterationReq struct {
	MilestoneID   *int64  `json:"milestone_id"`
	Name          string  `json:"name" binding:"required"`
	Goal          string  `json:"goal"`
	StartDate     string  `json:"start_date"`
	EndDate       string  `json:"end_date"`
	Status        string  `json:"status"`
	CapacityHours float64 `json:"capacity_hours"`
}

type UpdateIterationReq struct {
	MilestoneID   *int64   `json:"milestone_id"`
	Name          *string  `json:"name"`
	Goal          *string  `json:"goal"`
	StartDate     *string  `json:"start_date"`
	EndDate       *string  `json:"end_date"`
	Status        *string  `json:"status"`
	CapacityHours *float64 `json:"capacity_hours"`
}

type CreateRiskReq struct {
	MilestoneID *int64 `json:"milestone_id"`
	TaskID      *int64 `json:"task_id"`
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
	Probability string `json:"probability"`
	Impact      string `json:"impact"`
	Level       string `json:"level"`
	Mitigation  string `json:"mitigation"`
	Owner       string `json:"owner"`
	Status      string `json:"status"`
	DueDate     string `json:"due_date"`
}

type UpdateRiskReq struct {
	MilestoneID *int64  `json:"milestone_id"`
	TaskID      *int64  `json:"task_id"`
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Probability *string `json:"probability"`
	Impact      *string `json:"impact"`
	Level       *string `json:"level"`
	Mitigation  *string `json:"mitigation"`
	Owner       *string `json:"owner"`
	Status      *string `json:"status"`
	DueDate     *string `json:"due_date"`
}

type CreateReleaseReq struct {
	MilestoneID   *int64 `json:"milestone_id"`
	Version       string `json:"version" binding:"required"`
	Name          string `json:"name" binding:"required"`
	ReleaseType   string `json:"release_type"`
	Status        string `json:"status"`
	ReleasedAt    string `json:"released_at"`
	ChangeSummary string `json:"change_summary"`
	RollbackPlan  string `json:"rollback_plan"`
	DeployedBy    string `json:"deployed_by"`
}

type UpdateReleaseReq struct {
	MilestoneID   *int64  `json:"milestone_id"`
	Version       *string `json:"version"`
	Name          *string `json:"name"`
	ReleaseType   *string `json:"release_type"`
	Status        *string `json:"status"`
	ReleasedAt    *string `json:"released_at"`
	ChangeSummary *string `json:"change_summary"`
	RollbackPlan  *string `json:"rollback_plan"`
	DeployedBy    *string `json:"deployed_by"`
}

type CreateBaselineReq struct {
	MilestoneID *int64 `json:"milestone_id"`
	Name        string `json:"name" binding:"required"`
	Note        string `json:"note"`
}

type CreateCommentReq struct {
	TargetType string `json:"target_type" binding:"required"`
	TargetID   int64  `json:"target_id" binding:"required"`
	Content    string `json:"content" binding:"required"`
}

type CreateTaskCommentReq struct {
	Content string `json:"content" binding:"required"`
}

type CreateChecklistReq struct {
	Title   string `json:"title" binding:"required"`
	Done    bool   `json:"done"`
	OrderNo int    `json:"order_no"`
}

type UpdateChecklistReq struct {
	Title   *string `json:"title"`
	Done    *bool   `json:"done"`
	OrderNo *int    `json:"order_no"`
}

type CreateLabelReq struct {
	Name  string `json:"name" binding:"required"`
	Color string `json:"color"`
}

type AddTaskLabelReq struct {
	LabelID int64 `json:"label_id" binding:"required"`
}

type AddTaskAssetReq struct {
	AssetID      int64  `json:"asset_id" binding:"required"`
	RelationType string `json:"relation_type"`
}

type ListProjectItemsReq struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Status   string `form:"status"`
	Keyword  string `form:"keyword"`
	Deleted  *bool  `form:"deleted"`
}

func (r *ListProjectItemsReq) Normalize() {
	if r.Page < 1 {
		r.Page = 1
	}
	if r.PageSize < 1 || r.PageSize > 100 {
		r.PageSize = 20
	}
}

func (r *ListProjectItemsReq) Offset() int { return (r.Page - 1) * r.PageSize }
