package developmentmanager

import "time"

// ---------- 枚举 ----------

// 里程碑状态
const (
	MilestoneStatusPlanned   = "planned"
	MilestoneStatusActive    = "active"
	MilestoneStatusBlocked   = "blocked"
	MilestoneStatusDone      = "done"
	MilestoneStatusCancelled = "cancelled"
)

// 任务类型
const (
	TaskTypeRefactor  = "refactor"
	TaskTypeFeature   = "feature"
	TaskTypeTest      = "test"
	TaskTypeDocs      = "docs"
	TaskTypeOps       = "ops"
	TaskTypeMigration = "migration"
	TaskTypeRisk      = "risk"
)

// 任务状态
const (
	TaskStatusTodo      = "todo"
	TaskStatusDoing     = "doing"
	TaskStatusBlocked   = "blocked"
	TaskStatusReview    = "review"
	TaskStatusDone      = "done"
	TaskStatusCancelled = "cancelled"
)

// 依赖类型
const (
	DependencyFS = "FS"
	DependencySS = "SS"
	DependencyFF = "FF"
	DependencySF = "SF"
)

var ValidMilestoneStatuses = map[string]bool{
	MilestoneStatusPlanned: true, MilestoneStatusActive: true, MilestoneStatusBlocked: true,
	MilestoneStatusDone: true, MilestoneStatusCancelled: true,
}

var ValidTaskTypes = map[string]bool{
	TaskTypeRefactor: true, TaskTypeFeature: true, TaskTypeTest: true, TaskTypeDocs: true,
	TaskTypeOps: true, TaskTypeMigration: true, TaskTypeRisk: true,
}

var ValidTaskStatuses = map[string]bool{
	TaskStatusTodo: true, TaskStatusDoing: true, TaskStatusBlocked: true,
	TaskStatusReview: true, TaskStatusDone: true, TaskStatusCancelled: true,
}

var ValidDependencyTypes = map[string]bool{
	DependencyFS: true, DependencySS: true, DependencyFF: true, DependencySF: true,
}

var taskTransitions = map[string]map[string]bool{
	TaskStatusTodo:      {TaskStatusDoing: true, TaskStatusBlocked: true, TaskStatusCancelled: true},
	TaskStatusDoing:     {TaskStatusReview: true, TaskStatusBlocked: true, TaskStatusDone: true, TaskStatusCancelled: true},
	TaskStatusBlocked:   {TaskStatusTodo: true, TaskStatusDoing: true, TaskStatusCancelled: true},
	TaskStatusReview:    {TaskStatusDoing: true, TaskStatusDone: true, TaskStatusCancelled: true},
	TaskStatusDone:      {TaskStatusDoing: true, TaskStatusCancelled: true},
	TaskStatusCancelled: {TaskStatusTodo: true},
}

// CanTransitionTask 判断任务状态是否允许流转。
func CanTransitionTask(from, to string) bool {
	next, ok := taskTransitions[from]
	return ok && next[to]
}

// ---------- 实体 ----------

// Milestone 里程碑/版本
type Milestone struct {
	ID         int64      `json:"id"`
	ProjectID  int64      `json:"project_id"`
	Version    string     `json:"version"`
	Name       string     `json:"name"`
	Goal       string     `json:"goal"`
	Status     string     `json:"status"`
	StartDate  *time.Time `json:"start_date,omitempty"`
	TargetDate *time.Time `json:"target_date,omitempty"`
	ReleasedAt *time.Time `json:"released_at,omitempty"`
	Owner      string     `json:"owner"`
	Deleted    bool       `json:"deleted"`
	DeletedAt  *time.Time `json:"deleted_at,omitempty"`
	CreatedBy  string     `json:"created_by"`
	UpdatedBy  string     `json:"updated_by"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// Task 任务/WBS
type Task struct {
	ID            int64      `json:"id"`
	ProjectID     int64      `json:"project_id"`
	MilestoneID   *int64     `json:"milestone_id,omitempty"`
	IterationID   *int64     `json:"iteration_id,omitempty"`
	RequirementID *int64     `json:"requirement_id,omitempty"`
	ParentID      *int64     `json:"parent_id,omitempty"`
	Code          string     `json:"code"`
	Title         string     `json:"title"`
	Description   string     `json:"description"`
	Type          string     `json:"type"`
	Status        string     `json:"status"`
	Priority      string     `json:"priority"`
	EstimateHours float64    `json:"estimate_hours"`
	ActualHours   float64    `json:"actual_hours"`
	Owner         string     `json:"owner"`
	StartDate     *time.Time `json:"start_date,omitempty"`
	DueDate       *time.Time `json:"due_date,omitempty"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	DoneAt        *time.Time `json:"done_at,omitempty"`
	OrderNo       int        `json:"order_no"`
	Deleted       bool       `json:"deleted"`
	DeletedAt     *time.Time `json:"deleted_at,omitempty"`
	CreatedBy     string     `json:"created_by"`
	UpdatedBy     string     `json:"updated_by"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// TaskDependency 任务依赖
type TaskDependency struct {
	ID              int64     `json:"id"`
	TaskID          int64     `json:"task_id"`
	DependsOnTaskID int64     `json:"depends_on_task_id"`
	DepType         string    `json:"dep_type"`
	LagDays         int       `json:"lag_days"`
	CreatedAt       time.Time `json:"created_at"`
}

// TaskLog 任务状态/工时日志
type TaskLog struct {
	ID         int64     `json:"id"`
	TaskID     int64     `json:"task_id"`
	FromStatus string    `json:"from_status"`
	ToStatus   string    `json:"to_status"`
	Hours      float64   `json:"hours"`
	Note       string    `json:"note"`
	Operator   string    `json:"operator"`
	CreatedAt  time.Time `json:"created_at"`
}

// MilestoneProgress 里程碑进度
type MilestoneProgress struct {
	MilestoneID   int64   `json:"milestone_id"`
	Total         int     `json:"total"`
	Todo          int     `json:"todo"`
	Doing         int     `json:"doing"`
	Blocked       int     `json:"blocked"`
	Review        int     `json:"review"`
	Done          int     `json:"done"`
	Cancelled     int     `json:"cancelled"`
	EstimateHours float64 `json:"estimate_hours"`
	ActualHours   float64 `json:"actual_hours"`
	Percent       float64 `json:"percent"`
}

// ---------- 请求 ----------

type ListMilestonesReq struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Status   string `form:"status"`
	Keyword  string `form:"keyword"`
	Deleted  *bool  `form:"deleted"`
}

func (r *ListMilestonesReq) Normalize() {
	if r.Page < 1 {
		r.Page = 1
	}
	if r.PageSize < 1 || r.PageSize > 100 {
		r.PageSize = 20
	}
}

func (r *ListMilestonesReq) Offset() int { return (r.Page - 1) * r.PageSize }

type CreateMilestoneReq struct {
	Version    string `json:"version" binding:"required"`
	Name       string `json:"name" binding:"required"`
	Goal       string `json:"goal"`
	Status     string `json:"status"`
	StartDate  string `json:"start_date"`
	TargetDate string `json:"target_date"`
	Owner      string `json:"owner"`
}

type UpdateMilestoneReq struct {
	Version    *string `json:"version"`
	Name       *string `json:"name"`
	Goal       *string `json:"goal"`
	Status     *string `json:"status"`
	StartDate  *string `json:"start_date"`
	TargetDate *string `json:"target_date"`
	ReleasedAt *string `json:"released_at"`
	Owner      *string `json:"owner"`
}

type ListTasksReq struct {
	Page          int    `form:"page"`
	PageSize      int    `form:"page_size"`
	MilestoneID   *int64 `form:"milestone_id"`
	IterationID   *int64 `form:"iteration_id"`
	RequirementID *int64 `form:"requirement_id"`
	ParentID      *int64 `form:"parent_id"`
	Status        string `form:"status"`
	Type          string `form:"type"`
	Priority      string `form:"priority"`
	Owner         string `form:"owner"`
	Keyword       string `form:"keyword"`
	Deleted       *bool  `form:"deleted"`
}

func (r *ListTasksReq) Normalize() {
	if r.Page < 1 {
		r.Page = 1
	}
	if r.PageSize < 1 || r.PageSize > 100 {
		r.PageSize = 20
	}
}

func (r *ListTasksReq) Offset() int { return (r.Page - 1) * r.PageSize }

type CreateTaskReq struct {
	MilestoneID   *int64   `json:"milestone_id"`
	IterationID   *int64   `json:"iteration_id"`
	RequirementID *int64   `json:"requirement_id"`
	ParentID      *int64   `json:"parent_id"`
	Code          string   `json:"code"`
	Title         string   `json:"title" binding:"required"`
	Description   string   `json:"description"`
	Type          string   `json:"type"`
	Status        string   `json:"status"`
	Priority      string   `json:"priority"`
	EstimateHours *float64 `json:"estimate_hours"`
	Owner         string   `json:"owner"`
	StartDate     string   `json:"start_date"`
	DueDate       string   `json:"due_date"`
	OrderNo       *int     `json:"order_no"`
}

type UpdateTaskReq struct {
	MilestoneID   *int64   `json:"milestone_id"`
	IterationID   *int64   `json:"iteration_id"`
	RequirementID *int64   `json:"requirement_id"`
	ParentID      *int64   `json:"parent_id"`
	Code          *string  `json:"code"`
	Title         *string  `json:"title"`
	Description   *string  `json:"description"`
	Type          *string  `json:"type"`
	Priority      *string  `json:"priority"`
	EstimateHours *float64 `json:"estimate_hours"`
	ActualHours   *float64 `json:"actual_hours"`
	Owner         *string  `json:"owner"`
	StartDate     *string  `json:"start_date"`
	DueDate       *string  `json:"due_date"`
	OrderNo       *int     `json:"order_no"`
}

type ChangeTaskStatusReq struct {
	Status string   `json:"status" binding:"required"`
	Hours  *float64 `json:"hours"`
	Note   string   `json:"note"`
}

type AddTaskLogReq struct {
	FromStatus string   `json:"from_status"`
	ToStatus   string   `json:"to_status"`
	Hours      *float64 `json:"hours"`
	Note       string   `json:"note"`
}

type AddTaskDependencyReq struct {
	DependsOnTaskID int64  `json:"depends_on_task_id" binding:"required"`
	DepType         string `json:"dep_type"`
	LagDays         int    `json:"lag_days"`
}

// ImportTaskItem 任务导入项，Code 作为幂等键。
type ImportTaskItem struct {
	MilestoneID   *int64   `json:"milestone_id"`
	IterationID   *int64   `json:"iteration_id"`
	RequirementID *int64   `json:"requirement_id"`
	ParentID      *int64   `json:"parent_id"`
	Code          string   `json:"code" binding:"required"`
	Title         string   `json:"title" binding:"required"`
	Description   string   `json:"description"`
	Type          string   `json:"type"`
	Status        string   `json:"status"`
	Priority      string   `json:"priority"`
	EstimateHours *float64 `json:"estimate_hours"`
	ActualHours   *float64 `json:"actual_hours"`
	Owner         string   `json:"owner"`
	StartDate     string   `json:"start_date"`
	DueDate       string   `json:"due_date"`
	OrderNo       *int     `json:"order_no"`
}

type ImportTasksReq struct {
	Tasks []ImportTaskItem `json:"tasks" binding:"required"`
}

type ImportTasksResp struct {
	Created int `json:"created"`
	Updated int `json:"updated"`
	Skipped int `json:"skipped"`
}
