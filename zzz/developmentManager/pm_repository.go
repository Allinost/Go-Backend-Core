package developmentmanager

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Allinost/go-backend-core/internal/pkg/dbutil"
)

const (
	tableMilestone      = "zzz_development_manager_milestone"
	tableTask           = "zzz_development_manager_task"
	tableTaskDependency = "zzz_development_manager_task_dependency"
	tableTaskLog        = "zzz_development_manager_task_log"

	milestoneColumns = `id, project_id, version, name, COALESCE(goal,''), status, start_date, target_date,
		released_at, owner, deleted, deleted_at, created_by, updated_by, created_at, updated_at`
	taskColumns = `id, project_id, milestone_id, iteration_id, requirement_id, parent_id, code, title,
		COALESCE(description,''), type, status, priority, estimate_hours, actual_hours, owner,
		start_date, due_date, started_at, done_at, order_no, deleted, deleted_at, created_by, updated_by,
		created_at, updated_at`
)

// PMRepository 里程碑/任务 PostgreSQL 存储。
type PMRepository struct {
	db *sql.DB
}

func NewPMRepository(db *sql.DB) *PMRepository {
	return &PMRepository{db: db}
}

func (r *PMRepository) query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return r.db.QueryContext(ctx, dbutil.Rebind(query), args...)
}

func (r *PMRepository) queryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return r.db.QueryRowContext(ctx, dbutil.Rebind(query), args...)
}

func (r *PMRepository) exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return r.db.ExecContext(ctx, dbutil.Rebind(query), args...)
}

func scanMilestone(row rowScanner) (*Milestone, error) {
	var m Milestone
	var goal string
	var startDate, targetDate, releasedAt, deletedAt sql.NullTime
	if err := row.Scan(&m.ID, &m.ProjectID, &m.Version, &m.Name, &goal, &m.Status,
		&startDate, &targetDate, &releasedAt, &m.Owner, &m.Deleted, &deletedAt,
		&m.CreatedBy, &m.UpdatedBy, &m.CreatedAt, &m.UpdatedAt); err != nil {
		return nil, err
	}
	m.Goal = goal
	m.StartDate = nullTime(startDate)
	m.TargetDate = nullTime(targetDate)
	m.ReleasedAt = nullTime(releasedAt)
	m.DeletedAt = nullTime(deletedAt)
	return &m, nil
}

func scanTask(row rowScanner) (*Task, error) {
	var t Task
	var description string
	var milestoneID, iterationID, requirementID, parentID sql.NullInt64
	var startDate, dueDate, startedAt, doneAt, deletedAt sql.NullTime
	if err := row.Scan(&t.ID, &t.ProjectID, &milestoneID, &iterationID, &requirementID, &parentID,
		&t.Code, &t.Title, &description, &t.Type, &t.Status, &t.Priority,
		&t.EstimateHours, &t.ActualHours, &t.Owner, &startDate, &dueDate, &startedAt, &doneAt,
		&t.OrderNo, &t.Deleted, &deletedAt, &t.CreatedBy, &t.UpdatedBy, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return nil, err
	}
	t.Description = description
	t.MilestoneID = nullInt64(milestoneID)
	t.IterationID = nullInt64(iterationID)
	t.RequirementID = nullInt64(requirementID)
	t.ParentID = nullInt64(parentID)
	t.StartDate = nullTime(startDate)
	t.DueDate = nullTime(dueDate)
	t.StartedAt = nullTime(startedAt)
	t.DoneAt = nullTime(doneAt)
	t.DeletedAt = nullTime(deletedAt)
	return &t, nil
}

func nullTime(v sql.NullTime) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time
	return &t
}

func nullInt64(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	n := v.Int64
	return &n
}

// ProjectExists 判断项目是否存在且未删除。
func (r *PMRepository) ProjectExists(ctx context.Context, projectID int64) (bool, error) {
	var exists bool
	err := r.queryRow(ctx, `SELECT EXISTS(SELECT 1 FROM zzz_development_manager_project WHERE id = ? AND deleted = false)`, projectID).Scan(&exists)
	return exists, err
}

func (r *PMRepository) milestoneInProject(ctx context.Context, milestoneID, projectID int64) (bool, error) {
	var exists bool
	err := r.queryRow(ctx, `SELECT EXISTS(SELECT 1 FROM `+tableMilestone+` WHERE id = ? AND project_id = ? AND deleted = false)`, milestoneID, projectID).Scan(&exists)
	return exists, err
}

func (r *PMRepository) requirementInProject(ctx context.Context, requirementID, projectID int64) (bool, error) {
	var exists bool
	err := r.queryRow(ctx, `SELECT EXISTS(SELECT 1 FROM zzz_development_manager_requirement WHERE id = ? AND project_id = ? AND deleted = false)`, requirementID, projectID).Scan(&exists)
	return exists, err
}

func (r *PMRepository) taskInProject(ctx context.Context, taskID, projectID int64) (bool, error) {
	var exists bool
	err := r.queryRow(ctx, `SELECT EXISTS(SELECT 1 FROM `+tableTask+` WHERE id = ? AND project_id = ? AND deleted = false)`, taskID, projectID).Scan(&exists)
	return exists, err
}

// ---------- 里程碑 ----------

func (r *PMRepository) ListMilestones(ctx context.Context, projectID int64, req *ListMilestonesReq) ([]Milestone, bool, int, error) {
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
		conditions = append(conditions, "(version ILIKE ? OR name ILIKE ? OR COALESCE(goal,'') ILIKE ?)")
		args = append(args, like, like, like)
	}
	where := strings.Join(conditions, " AND ")

	var total int
	if err := r.queryRow(ctx, "SELECT COUNT(*) FROM "+tableMilestone+" WHERE "+where, args...).Scan(&total); err != nil {
		return nil, false, 0, err
	}
	query := "SELECT " + milestoneColumns + " FROM " + tableMilestone + " WHERE " + where +
		" ORDER BY id DESC LIMIT ? OFFSET ?"
	rows, err := r.query(ctx, query, append(args, req.PageSize+1, req.Offset())...)
	if err != nil {
		return nil, false, 0, err
	}
	defer rows.Close()
	var items []Milestone
	for rows.Next() {
		item, err := scanMilestone(rows)
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

func (r *PMRepository) GetMilestone(ctx context.Context, id int64) (*Milestone, error) {
	item, err := scanMilestone(r.queryRow(ctx, "SELECT "+milestoneColumns+" FROM "+tableMilestone+" WHERE id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, NewDevelopmentManagerError(ErrMilestoneNotFound)
	}
	return item, err
}

func (r *PMRepository) CreateMilestone(ctx context.Context, m *Milestone) (*Milestone, error) {
	var id int64
	err := r.queryRow(ctx, `INSERT INTO `+tableMilestone+`
		(project_id, version, name, goal, status, start_date, target_date, released_at, owner, created_by, updated_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		m.ProjectID, m.Version, m.Name, m.Goal, m.Status, m.StartDate, m.TargetDate, m.ReleasedAt, m.Owner, m.CreatedBy, m.UpdatedBy).Scan(&id)
	if err != nil {
		return nil, err
	}
	return r.GetMilestone(ctx, id)
}

func (r *PMRepository) UpdateMilestone(ctx context.Context, id int64, updates map[string]any) error {
	if len(updates) == 0 {
		return nil
	}
	allowed := map[string]bool{"version": true, "name": true, "goal": true, "status": true, "start_date": true, "target_date": true, "released_at": true, "owner": true, "updated_by": true}
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
	sets = append(sets, "updated_at = now()")
	args = append(args, id)
	res, err := r.exec(ctx, "UPDATE "+tableMilestone+" SET "+strings.Join(sets, ", ")+" WHERE id = ? AND deleted = false", args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return NewDevelopmentManagerError(ErrMilestoneNotFound)
	}
	return nil
}

func (r *PMRepository) DeleteMilestone(ctx context.Context, id int64) error {
	res, err := r.exec(ctx, `UPDATE `+tableMilestone+` SET deleted = true, deleted_at = now(), updated_at = now() WHERE id = ? AND deleted = false`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return NewDevelopmentManagerError(ErrMilestoneNotFound)
	}
	return nil
}

func (r *PMRepository) MilestoneProgress(ctx context.Context, id int64) (*MilestoneProgress, error) {
	p := &MilestoneProgress{MilestoneID: id}
	err := r.queryRow(ctx, `SELECT COUNT(*),
		COUNT(*) FILTER (WHERE status = 'todo'),
		COUNT(*) FILTER (WHERE status = 'doing'),
		COUNT(*) FILTER (WHERE status = 'blocked'),
		COUNT(*) FILTER (WHERE status = 'review'),
		COUNT(*) FILTER (WHERE status = 'done'),
		COUNT(*) FILTER (WHERE status = 'cancelled'),
		COALESCE(SUM(estimate_hours),0), COALESCE(SUM(actual_hours),0)
		FROM `+tableTask+` WHERE milestone_id = ? AND deleted = false`, id).Scan(
		&p.Total, &p.Todo, &p.Doing, &p.Blocked, &p.Review, &p.Done, &p.Cancelled, &p.EstimateHours, &p.ActualHours)
	if err != nil {
		return nil, err
	}
	if p.Total > 0 {
		p.Percent = float64(p.Done) / float64(p.Total) * 100
	}
	return p, nil
}

// ---------- 任务 ----------

func (r *PMRepository) ListTasks(ctx context.Context, projectID int64, req *ListTasksReq) ([]Task, bool, int, error) {
	req.Normalize()
	conditions := []string{"project_id = ?"}
	args := []any{projectID}
	if req.Deleted == nil || !*req.Deleted {
		conditions = append(conditions, "deleted = false")
	} else {
		conditions = append(conditions, "deleted = true")
	}
	if req.MilestoneID != nil {
		conditions = append(conditions, "milestone_id = ?")
		args = append(args, *req.MilestoneID)
	}
	if req.IterationID != nil {
		conditions = append(conditions, "iteration_id = ?")
		args = append(args, *req.IterationID)
	}
	if req.RequirementID != nil {
		conditions = append(conditions, "requirement_id = ?")
		args = append(args, *req.RequirementID)
	}
	if req.ParentID != nil {
		conditions = append(conditions, "parent_id = ?")
		args = append(args, *req.ParentID)
	}
	if req.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, req.Status)
	}
	if req.Type != "" {
		conditions = append(conditions, "type = ?")
		args = append(args, req.Type)
	}
	if req.Priority != "" {
		conditions = append(conditions, "priority = ?")
		args = append(args, req.Priority)
	}
	if req.Owner != "" {
		conditions = append(conditions, "owner = ?")
		args = append(args, req.Owner)
	}
	if req.Keyword != "" {
		like := "%" + req.Keyword + "%"
		conditions = append(conditions, "(code ILIKE ? OR title ILIKE ? OR COALESCE(description,'') ILIKE ?)")
		args = append(args, like, like, like)
	}
	where := strings.Join(conditions, " AND ")

	var total int
	if err := r.queryRow(ctx, "SELECT COUNT(*) FROM "+tableTask+" WHERE "+where, args...).Scan(&total); err != nil {
		return nil, false, 0, err
	}
	query := "SELECT " + taskColumns + " FROM " + tableTask + " WHERE " + where +
		" ORDER BY order_no ASC, id ASC LIMIT ? OFFSET ?"
	rows, err := r.query(ctx, query, append(args, req.PageSize+1, req.Offset())...)
	if err != nil {
		return nil, false, 0, err
	}
	defer rows.Close()
	var items []Task
	for rows.Next() {
		item, err := scanTask(rows)
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

func (r *PMRepository) GetTask(ctx context.Context, id int64) (*Task, error) {
	item, err := scanTask(r.queryRow(ctx, "SELECT "+taskColumns+" FROM "+tableTask+" WHERE id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, NewDevelopmentManagerError(ErrTaskNotFound)
	}
	return item, err
}

func (r *PMRepository) CreateTask(ctx context.Context, t *Task) (*Task, error) {
	var id int64
	err := r.queryRow(ctx, `INSERT INTO `+tableTask+`
		(project_id, milestone_id, iteration_id, requirement_id, parent_id, code, title, description,
		 type, status, priority, estimate_hours, actual_hours, owner, start_date, due_date, order_no,
		 created_by, updated_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		t.ProjectID, t.MilestoneID, t.IterationID, t.RequirementID, t.ParentID, t.Code, t.Title, t.Description,
		t.Type, t.Status, t.Priority, t.EstimateHours, t.ActualHours, t.Owner, t.StartDate, t.DueDate, t.OrderNo,
		t.CreatedBy, t.UpdatedBy).Scan(&id)
	if err != nil {
		return nil, err
	}
	return r.GetTask(ctx, id)
}

func (r *PMRepository) UpdateTask(ctx context.Context, id int64, updates map[string]any) error {
	if len(updates) == 0 {
		return nil
	}
	allowed := map[string]bool{
		"milestone_id": true, "iteration_id": true, "requirement_id": true, "parent_id": true,
		"code": true, "title": true, "description": true, "type": true, "priority": true,
		"estimate_hours": true, "actual_hours": true, "owner": true,
		"start_date": true, "due_date": true, "order_no": true, "updated_by": true,
	}
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
	sets = append(sets, "updated_at = now()")
	args = append(args, id)
	res, err := r.exec(ctx, "UPDATE "+tableTask+" SET "+strings.Join(sets, ", ")+" WHERE id = ? AND deleted = false", args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return NewDevelopmentManagerError(ErrTaskNotFound)
	}
	return nil
}

func (r *PMRepository) DeleteTask(ctx context.Context, id int64) error {
	res, err := r.exec(ctx, `UPDATE `+tableTask+` SET deleted = true, deleted_at = now(), updated_at = now() WHERE id = ? AND deleted = false`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return NewDevelopmentManagerError(ErrTaskNotFound)
	}
	return nil
}

// ChangeTaskStatusTx 状态流转 + 工时 + 日志，事务内完成。
func (r *PMRepository) ChangeTaskStatusTx(ctx context.Context, id int64, from, to string, hours float64, note, operator string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var current string
	if err := tx.QueryRowContext(ctx, dbutil.Rebind(`SELECT status FROM `+tableTask+` WHERE id = ? AND deleted = false FOR UPDATE`), id).Scan(&current); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return NewDevelopmentManagerError(ErrTaskNotFound)
		}
		return err
	}
	if current != from {
		return NewDevelopmentManagerError(ErrInvalidTaskTransition).WithDetail("任务状态已变化，请刷新后重试")
	}

	sets := []string{"status = ?", "updated_at = now()"}
	args := []any{to}
	if to == TaskStatusDoing {
		sets = append(sets, "started_at = COALESCE(started_at, now())")
	}
	if to == TaskStatusDone {
		sets = append(sets, "done_at = now()")
	}
	if to == TaskStatusTodo || to == TaskStatusDoing {
		sets = append(sets, "done_at = NULL")
	}
	if hours > 0 {
		sets = append(sets, "actual_hours = actual_hours + ?")
		args = append(args, hours)
	}
	args = append(args, id)
	if _, err := tx.ExecContext(ctx, dbutil.Rebind("UPDATE "+tableTask+" SET "+strings.Join(sets, ", ")+" WHERE id = ?"), args...); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, dbutil.Rebind(`INSERT INTO `+tableTaskLog+`
		(task_id, from_status, to_status, hours, note, operator) VALUES (?, ?, ?, ?, ?, ?)`),
		id, from, to, hours, note, operator); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *PMRepository) ListTaskLogs(ctx context.Context, taskID int64, limit int) ([]TaskLog, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.query(ctx, `SELECT id, task_id, from_status, to_status, hours, COALESCE(note,''), operator, created_at
		FROM `+tableTaskLog+` WHERE task_id = ? ORDER BY id DESC LIMIT ?`, taskID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []TaskLog
	for rows.Next() {
		var l TaskLog
		if err := rows.Scan(&l.ID, &l.TaskID, &l.FromStatus, &l.ToStatus, &l.Hours, &l.Note, &l.Operator, &l.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, l)
	}
	return items, rows.Err()
}

func (r *PMRepository) AddTaskLog(ctx context.Context, taskID int64, req *AddTaskLogReq, operator string) (*TaskLog, error) {
	hours := 0.0
	if req.Hours != nil {
		hours = *req.Hours
	}
	var id int64
	err := r.queryRow(ctx, `INSERT INTO `+tableTaskLog+`
		(task_id, from_status, to_status, hours, note, operator) VALUES (?, ?, ?, ?, ?, ?) RETURNING id`,
		taskID, req.FromStatus, req.ToStatus, hours, req.Note, operator).Scan(&id)
	if err != nil {
		return nil, err
	}
	var l TaskLog
	if err := r.queryRow(ctx, `SELECT id, task_id, from_status, to_status, hours, COALESCE(note,''), operator, created_at
		FROM `+tableTaskLog+` WHERE id = ?`, id).Scan(&l.ID, &l.TaskID, &l.FromStatus, &l.ToStatus, &l.Hours, &l.Note, &l.Operator, &l.CreatedAt); err != nil {
		return nil, err
	}
	return &l, nil
}

// ---------- 依赖 ----------

func (r *PMRepository) ListDependencies(ctx context.Context, taskID int64) ([]TaskDependency, error) {
	rows, err := r.query(ctx, `SELECT id, task_id, depends_on_task_id, dep_type, lag_days, created_at
		FROM `+tableTaskDependency+` WHERE task_id = ? ORDER BY id ASC`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []TaskDependency
	for rows.Next() {
		var d TaskDependency
		if err := rows.Scan(&d.ID, &d.TaskID, &d.DependsOnTaskID, &d.DepType, &d.LagDays, &d.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, d)
	}
	return items, rows.Err()
}

func (r *PMRepository) AddDependency(ctx context.Context, taskID int64, req *AddTaskDependencyReq) (*TaskDependency, error) {
	var id int64
	err := r.queryRow(ctx, `INSERT INTO `+tableTaskDependency+`
		(task_id, depends_on_task_id, dep_type, lag_days) VALUES (?, ?, ?, ?) RETURNING id`,
		taskID, req.DependsOnTaskID, req.DepType, req.LagDays).Scan(&id)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return nil, NewDevelopmentManagerError(ErrTaskDependencyCycle).WithDetail("依赖已存在")
		}
		return nil, err
	}
	var d TaskDependency
	if err := r.queryRow(ctx, `SELECT id, task_id, depends_on_task_id, dep_type, lag_days, created_at
		FROM `+tableTaskDependency+` WHERE id = ?`, id).Scan(&d.ID, &d.TaskID, &d.DependsOnTaskID, &d.DepType, &d.LagDays, &d.CreatedAt); err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *PMRepository) RemoveDependency(ctx context.Context, taskID, depID int64) error {
	res, err := r.exec(ctx, `DELETE FROM `+tableTaskDependency+` WHERE id = ? AND task_id = ?`, depID, taskID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return NewDevelopmentManagerError(ErrTaskDependencyNotFound)
	}
	return nil
}

// WouldCreateCycle 检查新增 taskID -> dependsOnID 是否形成环。
func (r *PMRepository) WouldCreateCycle(ctx context.Context, taskID, dependsOnID int64) (bool, error) {
	var cycle bool
	err := r.queryRow(ctx, `
		WITH RECURSIVE chain(id) AS (
			SELECT ?::bigint
			UNION
			SELECT d.depends_on_task_id
			FROM `+tableTaskDependency+` d
			JOIN chain c ON d.task_id = c.id
		)
		SELECT EXISTS(SELECT 1 FROM chain WHERE id = ?)`, dependsOnID, taskID).Scan(&cycle)
	return cycle, err
}

// NextTaskCode 生成默认任务编号（TASK-0001）。
func (r *PMRepository) NextTaskCode(ctx context.Context) (string, error) {
	var n int64
	if err := r.queryRow(ctx, `SELECT COALESCE(MAX(id),0) + 1 FROM `+tableTask+` WHERE deleted = false`).Scan(&n); err != nil {
		return "", err
	}
	return fmt.Sprintf("TASK-%04d", n), nil
}
