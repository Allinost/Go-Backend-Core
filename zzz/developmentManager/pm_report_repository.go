package developmentmanager

import (
	"context"
	"time"
)

// Dashboard 项目仪表盘。
func (r *PMRepository) Dashboard(ctx context.Context, projectID int64) (*ProjectDashboard, error) {
	d := &ProjectDashboard{ProjectID: projectID}
	err := r.queryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM `+tableMilestone+` WHERE project_id = ? AND deleted = false),
			(SELECT COUNT(*) FROM `+tableMilestone+` WHERE project_id = ? AND status = 'active' AND deleted = false),
			COUNT(*),
			COUNT(*) FILTER (WHERE status = 'todo'),
			COUNT(*) FILTER (WHERE status = 'doing'),
			COUNT(*) FILTER (WHERE status = 'blocked'),
			COUNT(*) FILTER (WHERE status = 'review'),
			COUNT(*) FILTER (WHERE status = 'done'),
			COUNT(*) FILTER (WHERE status = 'cancelled'),
			COALESCE(SUM(estimate_hours),0),
			COALESCE(SUM(actual_hours),0)
		FROM `+tableTask+` WHERE project_id = ? AND deleted = false`,
		projectID, projectID, projectID).Scan(
		&d.Milestones, &d.ActiveMilestones, &d.Tasks, &d.Todo, &d.Doing, &d.Blocked,
		&d.Review, &d.Done, &d.Cancelled, &d.EstimateHours, &d.ActualHours)
	if err != nil {
		return nil, err
	}
	if d.Tasks > 0 {
		d.Percent = float64(d.Done) / float64(d.Tasks) * 100
	}
	_ = r.queryRow(ctx, `SELECT COUNT(*), COUNT(*) FILTER (WHERE level = 'critical')
		FROM `+tableRisk+` WHERE project_id = ? AND deleted = false AND status IN ('open','mitigating')`, projectID).
		Scan(&d.RisksOpen, &d.RisksCritical)
	_ = r.queryRow(ctx, `SELECT COUNT(*) FROM `+tableRelease+` WHERE project_id = ? AND deleted = false`, projectID).Scan(&d.Releases)
	_ = r.queryRow(ctx, `SELECT COUNT(*) FROM `+tableIteration+` WHERE project_id = ? AND status = 'active' AND deleted = false`, projectID).Scan(&d.ActiveIterations)
	return d, nil
}

// Board 看板：按状态分组。
func (r *PMRepository) Board(ctx context.Context, projectID int64) ([]BoardColumn, error) {
	rows, err := r.query(ctx, "SELECT "+taskColumns+" FROM "+tableTask+
		" WHERE project_id = ? AND deleted = false ORDER BY order_no ASC, id ASC LIMIT 1000", projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	order := []string{TaskStatusTodo, TaskStatusDoing, TaskStatusBlocked, TaskStatusReview, TaskStatusDone, TaskStatusCancelled}
	group := make(map[string][]Task, len(order))
	for rows.Next() {
		item, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		group[item.Status] = append(group[item.Status], *item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	columns := make([]BoardColumn, 0, len(order))
	for _, status := range order {
		tasks := group[status]
		if tasks == nil {
			tasks = []Task{}
		}
		columns = append(columns, BoardColumn{Status: status, Tasks: tasks})
	}
	return columns, nil
}

// Roadmap 路线图：里程碑 + 进度。
func (r *PMRepository) Roadmap(ctx context.Context, projectID int64) ([]RoadmapItem, error) {
	items, _, _, err := r.ListMilestones(ctx, projectID, &ListMilestonesReq{Page: 1, PageSize: 100})
	if err != nil {
		return nil, err
	}
	out := make([]RoadmapItem, 0, len(items))
	for _, m := range items {
		p, err := r.MilestoneProgress(ctx, m.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, RoadmapItem{Milestone: m, Progress: *p})
	}
	return out, nil
}

// WeeklyReport 指定时间范围的周报。
func (r *PMRepository) WeeklyReport(ctx context.Context, projectID int64, from, to time.Time) (*WeeklyReport, error) {
	report := &WeeklyReport{ProjectID: projectID, From: from, To: to}
	if err := r.queryRow(ctx, `SELECT COUNT(*) FROM `+tableTask+`
		WHERE project_id = ? AND deleted = false AND created_at >= ? AND created_at < ?`, projectID, from, to).Scan(&report.CreatedTasks); err != nil {
		return nil, err
	}
	rows, err := r.query(ctx, "SELECT "+taskColumns+" FROM "+tableTask+
		" WHERE project_id = ? AND deleted = false AND done_at >= ? AND done_at < ? ORDER BY done_at ASC", projectID, from, to)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		item, err := scanTask(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		report.DoneList = append(report.DoneList, *item)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	report.DoneTasks = len(report.DoneList)
	if err := r.queryRow(ctx, `SELECT COALESCE(SUM(hours),0) FROM `+tableTaskLog+`
		WHERE task_id IN (SELECT id FROM `+tableTask+` WHERE project_id = ?) AND created_at >= ? AND created_at < ?`,
		projectID, from, to).Scan(&report.LogHours); err != nil {
		return nil, err
	}
	_ = r.queryRow(ctx, `SELECT COUNT(*) FROM `+tableRisk+` WHERE project_id = ? AND created_at >= ? AND created_at < ?`, projectID, from, to).Scan(&report.RisksOpened)
	_ = r.queryRow(ctx, `SELECT COUNT(*) FROM `+tableRisk+` WHERE project_id = ? AND status = 'closed' AND updated_at >= ? AND updated_at < ?`, projectID, from, to).Scan(&report.RisksClosed)
	_ = r.queryRow(ctx, `SELECT COUNT(*) FROM `+tableRelease+` WHERE project_id = ? AND released_at >= ? AND released_at < ?`, projectID, from, to).Scan(&report.Releases)
	return report, nil
}
