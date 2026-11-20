package developmentmanager

import "time"

// ProjectDashboard 项目仪表盘汇总。
type ProjectDashboard struct {
	ProjectID        int64   `json:"project_id"`
	Milestones       int     `json:"milestones"`
	ActiveMilestones int     `json:"active_milestones"`
	Tasks            int     `json:"tasks"`
	Todo             int     `json:"todo"`
	Doing            int     `json:"doing"`
	Blocked          int     `json:"blocked"`
	Review           int     `json:"review"`
	Done             int     `json:"done"`
	Cancelled        int     `json:"cancelled"`
	EstimateHours    float64 `json:"estimate_hours"`
	ActualHours      float64 `json:"actual_hours"`
	RisksOpen        int     `json:"risks_open"`
	RisksCritical    int     `json:"risks_critical"`
	Releases         int     `json:"releases"`
	ActiveIterations int     `json:"active_iterations"`
	Percent          float64 `json:"percent"`
}

// BoardColumn 看板列。
type BoardColumn struct {
	Status string `json:"status"`
	Tasks  []Task `json:"tasks"`
}

// RoadmapItem 路线图条目。
type RoadmapItem struct {
	Milestone Milestone         `json:"milestone"`
	Progress  MilestoneProgress `json:"progress"`
}

// WeeklyReport 周报。
type WeeklyReport struct {
	ProjectID    int64     `json:"project_id"`
	From         time.Time `json:"from"`
	To           time.Time `json:"to"`
	CreatedTasks int       `json:"created_tasks"`
	DoneTasks    int       `json:"done_tasks"`
	LogHours     float64   `json:"log_hours"`
	RisksOpened  int       `json:"risks_opened"`
	RisksClosed  int       `json:"risks_closed"`
	Releases     int       `json:"releases"`
	DoneList     []Task    `json:"done_tasks_list"`
}
