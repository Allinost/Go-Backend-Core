package developmentmanager

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strings"
)

const (
	tableIteration = "zzz_development_manager_iteration"
	tableRisk      = "zzz_development_manager_risk"
	tableRelease   = "zzz_development_manager_release"
	tableBaseline  = "zzz_development_manager_baseline"
	tableComment   = "zzz_development_manager_comment"
	tableChecklist = "zzz_development_manager_task_checklist"
	tableLabel     = "zzz_development_manager_label"
	tableTaskLabel = "zzz_development_manager_task_label"
	tableTaskAsset = "zzz_development_manager_task_asset"

	iterationColumns = `id, project_id, milestone_id, name, COALESCE(goal,''), start_date, end_date,
		status, capacity_hours, deleted, deleted_at, created_by, updated_by, created_at, updated_at`
	riskColumns = `id, project_id, milestone_id, task_id, title, COALESCE(description,''), probability,
		impact, level, COALESCE(mitigation,''), owner, status, due_date, deleted, deleted_at,
		created_by, updated_by, created_at, updated_at`
	releaseColumns = `id, project_id, milestone_id, version, name, release_type, status, released_at,
		COALESCE(change_summary,''), COALESCE(rollback_plan,''), deployed_by, deleted, deleted_at,
		created_by, updated_by, created_at, updated_at`
	baselineColumns = `id, project_id, milestone_id, name, snapshot, COALESCE(note,''), created_by, created_at`
)

func scanIteration(row rowScanner) (*Iteration, error) {
	var m Iteration
	var goal string
	var milestoneID sql.NullInt64
	var startDate, endDate, deletedTime sql.NullTime
	if err := row.Scan(&m.ID, &m.ProjectID, &milestoneID, &m.Name, &goal, &startDate, &endDate,
		&m.Status, &m.CapacityHours, &m.Deleted, &deletedTime, &m.CreatedBy, &m.UpdatedBy, &m.CreatedAt, &m.UpdatedAt); err != nil {
		return nil, err
	}
	m.Goal = goal
	m.MilestoneID = nullInt64(milestoneID)
	m.StartDate = nullTime(startDate)
	m.EndDate = nullTime(endDate)
	m.DeletedAt = nullTime(deletedTime)
	return &m, nil
}

func scanRisk(row rowScanner) (*Risk, error) {
	var m Risk
	var description, mitigation string
	var milestoneID, taskID sql.NullInt64
	var dueDate, deletedTime sql.NullTime
	if err := row.Scan(&m.ID, &m.ProjectID, &milestoneID, &taskID, &m.Title, &description,
		&m.Probability, &m.Impact, &m.Level, &mitigation, &m.Owner, &m.Status, &dueDate,
		&m.Deleted, &deletedTime, &m.CreatedBy, &m.UpdatedBy, &m.CreatedAt, &m.UpdatedAt); err != nil {
		return nil, err
	}
	m.Description = description
	m.Mitigation = mitigation
	m.MilestoneID = nullInt64(milestoneID)
	m.TaskID = nullInt64(taskID)
	m.DueDate = nullTime(dueDate)
	m.DeletedAt = nullTime(deletedTime)
	return &m, nil
}

func scanRelease(row rowScanner) (*Release, error) {
	var m Release
	var changeSummary, rollbackPlan string
	var milestoneID sql.NullInt64
	var releasedAt, deletedTime sql.NullTime
	if err := row.Scan(&m.ID, &m.ProjectID, &milestoneID, &m.Version, &m.Name, &m.ReleaseType, &m.Status,
		&releasedAt, &changeSummary, &rollbackPlan, &m.DeployedBy, &m.Deleted, &deletedTime,
		&m.CreatedBy, &m.UpdatedBy, &m.CreatedAt, &m.UpdatedAt); err != nil {
		return nil, err
	}
	m.MilestoneID = nullInt64(milestoneID)
	m.ReleasedAt = nullTime(releasedAt)
	m.ChangeSummary = changeSummary
	m.RollbackPlan = rollbackPlan
	m.DeletedAt = nullTime(deletedTime)
	return &m, nil
}

func scanBaseline(row rowScanner) (*Baseline, error) {
	var m Baseline
	var milestoneID sql.NullInt64
	var note string
	var snapshot []byte
	if err := row.Scan(&m.ID, &m.ProjectID, &milestoneID, &m.Name, &snapshot, &note, &m.CreatedBy, &m.CreatedAt); err != nil {
		return nil, err
	}
	m.MilestoneID = nullInt64(milestoneID)
	m.Snapshot = json.RawMessage(snapshot)
	m.Note = note
	return &m, nil
}

// ---------- Iteration ----------

func (r *PMRepository) ListIterations(ctx context.Context, projectID int64, req *ListProjectItemsReq) ([]Iteration, bool, int, error) {
	req.Normalize()
	conditions := []string{"project_id = ?"}
	args := []any{projectID}
	if req.Deleted == nil || !*req.Deleted {
		conditions = append(conditions, "deleted = false")
	} else {
		conditions = append(conditions, "deleted = true")
	}
	if req.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, req.Status)
	}
	if req.Keyword != "" {
		like := "%" + req.Keyword + "%"
		conditions = append(conditions, "(name ILIKE ? OR COALESCE(goal,'') ILIKE ?)")
		args = append(args, like, like)
	}
	where := strings.Join(conditions, " AND ")
	var total int
	if err := r.queryRow(ctx, "SELECT COUNT(*) FROM "+tableIteration+" WHERE "+where, args...).Scan(&total); err != nil {
		return nil, false, 0, err
	}
	rows, err := r.query(ctx, "SELECT "+iterationColumns+" FROM "+tableIteration+" WHERE "+where+" ORDER BY id DESC LIMIT ? OFFSET ?",
		append(args, req.PageSize+1, req.Offset())...)
	if err != nil {
		return nil, false, 0, err
	}
	defer rows.Close()
	var items []Iteration
	for rows.Next() {
		item, err := scanIteration(rows)
		if err != nil {
			return nil, false, 0, err
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, false, 0, err
	}
	hasMore := len(items) > req.PageSize
	if hasMore {
		items = items[:req.PageSize]
	}
	return items, hasMore, total, nil
}

func (r *PMRepository) GetIteration(ctx context.Context, id int64) (*Iteration, error) {
	m, err := scanIteration(r.queryRow(ctx, "SELECT "+iterationColumns+" FROM "+tableIteration+" WHERE id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, NewDevelopmentManagerError(ErrIterationNotFound)
	}
	return m, err
}

func (r *PMRepository) CreateIteration(ctx context.Context, m *Iteration) (*Iteration, error) {
	var id int64
	err := r.queryRow(ctx, `INSERT INTO `+tableIteration+`
		(project_id, milestone_id, name, goal, start_date, end_date, status, capacity_hours, created_by, updated_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		m.ProjectID, m.MilestoneID, m.Name, m.Goal, m.StartDate, m.EndDate, m.Status, m.CapacityHours, m.CreatedBy, m.UpdatedBy).Scan(&id)
	if err != nil {
		return nil, err
	}
	return r.GetIteration(ctx, id)
}

func (r *PMRepository) UpdateIteration(ctx context.Context, id int64, updates map[string]any) error {
	return r.updateAdvanced(ctx, tableIteration, id, updates,
		map[string]bool{"milestone_id": true, "name": true, "goal": true, "start_date": true, "end_date": true, "status": true, "capacity_hours": true, "updated_by": true},
		ErrIterationNotFound)
}

func (r *PMRepository) DeleteIteration(ctx context.Context, id int64) error {
	return r.softDeleteAdvanced(ctx, tableIteration, id, ErrIterationNotFound)
}

// ---------- Risk ----------

func (r *PMRepository) ListRisks(ctx context.Context, projectID int64, req *ListProjectItemsReq) ([]Risk, bool, int, error) {
	req.Normalize()
	conditions := []string{"project_id = ?"}
	args := []any{projectID}
	if req.Deleted == nil || !*req.Deleted {
		conditions = append(conditions, "deleted = false")
	} else {
		conditions = append(conditions, "deleted = true")
	}
	if req.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, req.Status)
	}
	if req.Keyword != "" {
		like := "%" + req.Keyword + "%"
		conditions = append(conditions, "(title ILIKE ? OR COALESCE(description,'') ILIKE ?)")
		args = append(args, like, like)
	}
	where := strings.Join(conditions, " AND ")
	var total int
	if err := r.queryRow(ctx, "SELECT COUNT(*) FROM "+tableRisk+" WHERE "+where, args...).Scan(&total); err != nil {
		return nil, false, 0, err
	}
	rows, err := r.query(ctx, "SELECT "+riskColumns+" FROM "+tableRisk+" WHERE "+where+" ORDER BY id DESC LIMIT ? OFFSET ?",
		append(args, req.PageSize+1, req.Offset())...)
	if err != nil {
		return nil, false, 0, err
	}
	defer rows.Close()
	var items []Risk
	for rows.Next() {
		item, err := scanRisk(rows)
		if err != nil {
			return nil, false, 0, err
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, false, 0, err
	}
	hasMore := len(items) > req.PageSize
	if hasMore {
		items = items[:req.PageSize]
	}
	return items, hasMore, total, nil
}

func (r *PMRepository) GetRisk(ctx context.Context, id int64) (*Risk, error) {
	m, err := scanRisk(r.queryRow(ctx, "SELECT "+riskColumns+" FROM "+tableRisk+" WHERE id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, NewDevelopmentManagerError(ErrRiskNotFound)
	}
	return m, err
}

func (r *PMRepository) CreateRisk(ctx context.Context, m *Risk) (*Risk, error) {
	var id int64
	err := r.queryRow(ctx, `INSERT INTO `+tableRisk+`
		(project_id, milestone_id, task_id, title, description, probability, impact, level, mitigation, owner, status, due_date, created_by, updated_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		m.ProjectID, m.MilestoneID, m.TaskID, m.Title, m.Description, m.Probability, m.Impact, m.Level,
		m.Mitigation, m.Owner, m.Status, m.DueDate, m.CreatedBy, m.UpdatedBy).Scan(&id)
	if err != nil {
		return nil, err
	}
	return r.GetRisk(ctx, id)
}

func (r *PMRepository) UpdateRisk(ctx context.Context, id int64, updates map[string]any) error {
	return r.updateAdvanced(ctx, tableRisk, id, updates,
		map[string]bool{"milestone_id": true, "task_id": true, "title": true, "description": true, "probability": true, "impact": true, "level": true, "mitigation": true, "owner": true, "status": true, "due_date": true, "updated_by": true},
		ErrRiskNotFound)
}

func (r *PMRepository) DeleteRisk(ctx context.Context, id int64) error {
	return r.softDeleteAdvanced(ctx, tableRisk, id, ErrRiskNotFound)
}

// ---------- Release ----------

func (r *PMRepository) ListReleases(ctx context.Context, projectID int64, req *ListProjectItemsReq) ([]Release, bool, int, error) {
	req.Normalize()
	conditions := []string{"project_id = ?"}
	args := []any{projectID}
	if req.Deleted == nil || !*req.Deleted {
		conditions = append(conditions, "deleted = false")
	} else {
		conditions = append(conditions, "deleted = true")
	}
	if req.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, req.Status)
	}
	if req.Keyword != "" {
		like := "%" + req.Keyword + "%"
		conditions = append(conditions, "(version ILIKE ? OR name ILIKE ?)")
		args = append(args, like, like)
	}
	where := strings.Join(conditions, " AND ")
	var total int
	if err := r.queryRow(ctx, "SELECT COUNT(*) FROM "+tableRelease+" WHERE "+where, args...).Scan(&total); err != nil {
		return nil, false, 0, err
	}
	rows, err := r.query(ctx, "SELECT "+releaseColumns+" FROM "+tableRelease+" WHERE "+where+" ORDER BY id DESC LIMIT ? OFFSET ?",
		append(args, req.PageSize+1, req.Offset())...)
	if err != nil {
		return nil, false, 0, err
	}
	defer rows.Close()
	var items []Release
	for rows.Next() {
		item, err := scanRelease(rows)
		if err != nil {
			return nil, false, 0, err
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, false, 0, err
	}
	hasMore := len(items) > req.PageSize
	if hasMore {
		items = items[:req.PageSize]
	}
	return items, hasMore, total, nil
}

func (r *PMRepository) GetRelease(ctx context.Context, id int64) (*Release, error) {
	m, err := scanRelease(r.queryRow(ctx, "SELECT "+releaseColumns+" FROM "+tableRelease+" WHERE id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, NewDevelopmentManagerError(ErrReleaseNotFound)
	}
	return m, err
}

func (r *PMRepository) CreateRelease(ctx context.Context, m *Release) (*Release, error) {
	var id int64
	err := r.queryRow(ctx, `INSERT INTO `+tableRelease+`
		(project_id, milestone_id, version, name, release_type, status, released_at, change_summary, rollback_plan, deployed_by, created_by, updated_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		m.ProjectID, m.MilestoneID, m.Version, m.Name, m.ReleaseType, m.Status, m.ReleasedAt,
		m.ChangeSummary, m.RollbackPlan, m.DeployedBy, m.CreatedBy, m.UpdatedBy).Scan(&id)
	if err != nil {
		return nil, err
	}
	return r.GetRelease(ctx, id)
}

func (r *PMRepository) UpdateRelease(ctx context.Context, id int64, updates map[string]any) error {
	return r.updateAdvanced(ctx, tableRelease, id, updates,
		map[string]bool{"milestone_id": true, "version": true, "name": true, "release_type": true, "status": true, "released_at": true, "change_summary": true, "rollback_plan": true, "deployed_by": true, "updated_by": true},
		ErrReleaseNotFound)
}

func (r *PMRepository) DeleteRelease(ctx context.Context, id int64) error {
	return r.softDeleteAdvanced(ctx, tableRelease, id, ErrReleaseNotFound)
}

// ---------- Baseline ----------

func (r *PMRepository) ListBaselines(ctx context.Context, projectID int64, limit int) ([]Baseline, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := r.query(ctx, "SELECT "+baselineColumns+" FROM "+tableBaseline+" WHERE project_id = ? ORDER BY id DESC LIMIT ?", projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Baseline
	for rows.Next() {
		item, err := scanBaseline(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (r *PMRepository) GetBaseline(ctx context.Context, id int64) (*Baseline, error) {
	m, err := scanBaseline(r.queryRow(ctx, "SELECT "+baselineColumns+" FROM "+tableBaseline+" WHERE id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, NewDevelopmentManagerError(ErrBaselineNotFound)
	}
	return m, err
}

// CreateBaseline 生成当前项目里程碑/任务快照。
func (r *PMRepository) CreateBaseline(ctx context.Context, m *Baseline) (*Baseline, error) {
	var snapshot []byte
	err := r.queryRow(ctx, `
		SELECT jsonb_build_object(
			'milestones', (SELECT COALESCE(jsonb_agg(to_jsonb(x)), '[]'::jsonb) FROM `+tableMilestone+` x WHERE x.project_id = ? AND x.deleted = false),
			'tasks', (SELECT COALESCE(jsonb_agg(to_jsonb(y)), '[]'::jsonb) FROM `+tableTask+` y WHERE y.project_id = ? AND y.deleted = false)
		)::text`, m.ProjectID, m.ProjectID).Scan(&snapshot)
	if err != nil {
		return nil, err
	}
	var id int64
	err = r.queryRow(ctx, `INSERT INTO `+tableBaseline+`
		(project_id, milestone_id, name, snapshot, note, created_by) VALUES (?, ?, ?, ?, ?, ?) RETURNING id`,
		m.ProjectID, m.MilestoneID, m.Name, string(snapshot), m.Note, m.CreatedBy).Scan(&id)
	if err != nil {
		return nil, err
	}
	return r.GetBaseline(ctx, id)
}

func (r *PMRepository) DeleteBaseline(ctx context.Context, id int64) error {
	res, err := r.exec(ctx, "DELETE FROM "+tableBaseline+" WHERE id = ?", id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return NewDevelopmentManagerError(ErrBaselineNotFound)
	}
	return nil
}

// ---------- Comment ----------

func (r *PMRepository) ListComments(ctx context.Context, projectID int64, targetType string, targetID int64) ([]Comment, error) {
	rows, err := r.query(ctx, `SELECT id, project_id, target_type, target_id, content, operator, created_at
		FROM `+tableComment+` WHERE project_id = ? AND target_type = ? AND target_id = ? ORDER BY id DESC LIMIT 500`,
		projectID, targetType, targetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Comment
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.ProjectID, &c.TargetType, &c.TargetID, &c.Content, &c.Operator, &c.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

func (r *PMRepository) CreateComment(ctx context.Context, c *Comment) (*Comment, error) {
	var id int64
	err := r.queryRow(ctx, `INSERT INTO `+tableComment+`
		(project_id, target_type, target_id, content, operator) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		c.ProjectID, c.TargetType, c.TargetID, c.Content, c.Operator).Scan(&id)
	if err != nil {
		return nil, err
	}
	var out Comment
	if err := r.queryRow(ctx, `SELECT id, project_id, target_type, target_id, content, operator, created_at
		FROM `+tableComment+` WHERE id = ?`, id).Scan(&out.ID, &out.ProjectID, &out.TargetType, &out.TargetID, &out.Content, &out.Operator, &out.CreatedAt); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *PMRepository) DeleteComment(ctx context.Context, id int64) error {
	res, err := r.exec(ctx, "DELETE FROM "+tableComment+" WHERE id = ?", id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return NewDevelopmentManagerError(ErrCommentNotFound)
	}
	return nil
}

// ---------- Checklist ----------

func (r *PMRepository) ListChecklist(ctx context.Context, taskID int64) ([]ChecklistItem, error) {
	rows, err := r.query(ctx, `SELECT id, task_id, title, done, order_no, created_at, updated_at
		FROM `+tableChecklist+` WHERE task_id = ? ORDER BY order_no ASC, id ASC`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []ChecklistItem
	for rows.Next() {
		var c ChecklistItem
		if err := rows.Scan(&c.ID, &c.TaskID, &c.Title, &c.Done, &c.OrderNo, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

func (r *PMRepository) CreateChecklistItem(ctx context.Context, c *ChecklistItem) (*ChecklistItem, error) {
	var id int64
	err := r.queryRow(ctx, `INSERT INTO `+tableChecklist+` (task_id, title, done, order_no) VALUES (?, ?, ?, ?) RETURNING id`,
		c.TaskID, c.Title, c.Done, c.OrderNo).Scan(&id)
	if err != nil {
		return nil, err
	}
	var out ChecklistItem
	if err := r.queryRow(ctx, `SELECT id, task_id, title, done, order_no, created_at, updated_at FROM `+tableChecklist+` WHERE id = ?`, id).
		Scan(&out.ID, &out.TaskID, &out.Title, &out.Done, &out.OrderNo, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *PMRepository) UpdateChecklistItem(ctx context.Context, id int64, updates map[string]any) error {
	return r.updateAdvanced(ctx, tableChecklist, id, updates,
		map[string]bool{"title": true, "done": true, "order_no": true}, ErrChecklistNotFound)
}

func (r *PMRepository) DeleteChecklistItem(ctx context.Context, id int64) error {
	res, err := r.exec(ctx, "DELETE FROM "+tableChecklist+" WHERE id = ?", id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return NewDevelopmentManagerError(ErrChecklistNotFound)
	}
	return nil
}

// ---------- Label ----------

func (r *PMRepository) ListLabels(ctx context.Context, projectID int64) ([]Label, error) {
	rows, err := r.query(ctx, `SELECT id, project_id, name, color, created_at, updated_at
		FROM `+tableLabel+` WHERE project_id = ? ORDER BY name ASC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Label
	for rows.Next() {
		var l Label
		if err := rows.Scan(&l.ID, &l.ProjectID, &l.Name, &l.Color, &l.CreatedAt, &l.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, l)
	}
	return items, rows.Err()
}

func (r *PMRepository) CreateLabel(ctx context.Context, l *Label) (*Label, error) {
	var id int64
	err := r.queryRow(ctx, `INSERT INTO `+tableLabel+` (project_id, name, color) VALUES (?, ?, ?) RETURNING id`,
		l.ProjectID, l.Name, l.Color).Scan(&id)
	if err != nil {
		if isDuplicateKey(err) {
			return nil, NewDevelopmentManagerError(ErrLabelExists)
		}
		return nil, err
	}
	var out Label
	if err := r.queryRow(ctx, `SELECT id, project_id, name, color, created_at, updated_at FROM `+tableLabel+` WHERE id = ?`, id).
		Scan(&out.ID, &out.ProjectID, &out.Name, &out.Color, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *PMRepository) DeleteLabel(ctx context.Context, id int64) error {
	res, err := r.exec(ctx, "DELETE FROM "+tableLabel+" WHERE id = ?", id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return NewDevelopmentManagerError(ErrLabelNotFound)
	}
	return nil
}

// ---------- Task label ----------

func (r *PMRepository) ListTaskLabels(ctx context.Context, taskID int64) ([]Label, error) {
	rows, err := r.query(ctx, `SELECT l.id, l.project_id, l.name, l.color, l.created_at, l.updated_at
		FROM `+tableTaskLabel+` tl JOIN `+tableLabel+` l ON l.id = tl.label_id WHERE tl.task_id = ? ORDER BY l.name`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Label
	for rows.Next() {
		var l Label
		if err := rows.Scan(&l.ID, &l.ProjectID, &l.Name, &l.Color, &l.CreatedAt, &l.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, l)
	}
	return items, rows.Err()
}

func (r *PMRepository) AddTaskLabel(ctx context.Context, taskID, labelID int64) (*TaskLabel, error) {
	var id int64
	err := r.queryRow(ctx, `INSERT INTO `+tableTaskLabel+` (task_id, label_id) VALUES (?, ?) RETURNING id`, taskID, labelID).Scan(&id)
	if err != nil {
		if isDuplicateKey(err) {
			return nil, NewDevelopmentManagerError(ErrTaskLabelExists)
		}
		return nil, err
	}
	var t TaskLabel
	if err := r.queryRow(ctx, `SELECT id, task_id, label_id, created_at FROM `+tableTaskLabel+` WHERE id = ?`, id).
		Scan(&t.ID, &t.TaskID, &t.LabelID, &t.CreatedAt); err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *PMRepository) RemoveTaskLabel(ctx context.Context, taskID, labelID int64) error {
	res, err := r.exec(ctx, `DELETE FROM `+tableTaskLabel+` WHERE task_id = ? AND label_id = ?`, taskID, labelID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return NewDevelopmentManagerError(ErrTaskLabelNotFound)
	}
	return nil
}

// ---------- Task asset ----------

func (r *PMRepository) ListTaskAssets(ctx context.Context, taskID int64) ([]TaskAsset, error) {
	rows, err := r.query(ctx, `SELECT id, task_id, asset_id, relation_type, created_at
		FROM `+tableTaskAsset+` WHERE task_id = ? ORDER BY id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []TaskAsset
	for rows.Next() {
		var a TaskAsset
		if err := rows.Scan(&a.ID, &a.TaskID, &a.AssetID, &a.RelationType, &a.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	return items, rows.Err()
}

func (r *PMRepository) AddTaskAsset(ctx context.Context, a *TaskAsset) (*TaskAsset, error) {
	var id int64
	err := r.queryRow(ctx, `INSERT INTO `+tableTaskAsset+` (task_id, asset_id, relation_type) VALUES (?, ?, ?) RETURNING id`,
		a.TaskID, a.AssetID, a.RelationType).Scan(&id)
	if err != nil {
		if isDuplicateKey(err) {
			return nil, NewDevelopmentManagerError(ErrTaskAssetExists)
		}
		return nil, err
	}
	a.ID = id
	return a, nil
}

func (r *PMRepository) RemoveTaskAsset(ctx context.Context, taskID, assetID int64) error {
	res, err := r.exec(ctx, `DELETE FROM `+tableTaskAsset+` WHERE task_id = ? AND asset_id = ?`, taskID, assetID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return NewDevelopmentManagerError(ErrTaskAssetNotFound)
	}
	return nil
}

// ---------- 通用更新 ----------

func (r *PMRepository) updateAdvanced(ctx context.Context, table string, id int64, updates map[string]any, allowed map[string]bool, notFound int) error {
	keys := make([]string, 0, len(updates))
	for k := range updates {
		if allowed[k] {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return nil
	}
	sort.Strings(keys)
	sets := make([]string, 0, len(keys)+1)
	args := make([]any, 0, len(keys)+1)
	for _, k := range keys {
		sets = append(sets, k+" = ?")
		args = append(args, updates[k])
	}
	if allowed["updated_by"] || table == tableChecklist {
		sets = append(sets, "updated_at = now()")
	}
	args = append(args, id)
	query := "UPDATE " + table + " SET " + strings.Join(sets, ", ") + " WHERE id = ?"
	if table != tableChecklist {
		query += " AND deleted = false"
	}
	res, err := r.exec(ctx, query, args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return NewDevelopmentManagerError(notFound)
	}
	return nil
}

func (r *PMRepository) softDeleteAdvanced(ctx context.Context, table string, id int64, notFound int) error {
	res, err := r.exec(ctx, "UPDATE "+table+" SET deleted = true, deleted_at = now(), updated_at = now() WHERE id = ? AND deleted = false", id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return NewDevelopmentManagerError(notFound)
	}
	return nil
}
