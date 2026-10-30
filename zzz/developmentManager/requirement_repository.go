package developmentmanager

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

const (
	tableRequirement        = "zzz_development_manager_requirement"
	tableRequirementVersion = "zzz_development_manager_requirement_version"

	requirementColumns = "r.id, r.project_id, r.code, r.title, r.content, r.type, r.priority, r.status, " +
		"r.source, r.owner, r.deleted, r.deleted_at, r.created_by, r.updated_by, r.created_at, r.updated_at, " +
		"(SELECT COALESCE(MAX(v.version_no), 0) FROM zzz_development_manager_requirement_version v WHERE v.requirement_id = r.id)"

	requirementVersionColumns = "id, requirement_id, version_no, title, content, status, change_note, operator, created_at"
)

func scanRequirement(row rowScanner) (*Requirement, error) {
	var r Requirement
	var content sql.NullString
	var deletedAt sql.NullTime
	if err := row.Scan(&r.ID, &r.ProjectID, &r.Code, &r.Title, &content, &r.Type, &r.Priority, &r.Status,
		&r.Source, &r.Owner, &r.Deleted, &deletedAt, &r.CreatedBy, &r.UpdatedBy, &r.CreatedAt,
		&r.UpdatedAt, &r.LatestVersion); err != nil {
		return nil, err
	}
	r.Content = content.String
	if deletedAt.Valid {
		t := deletedAt.Time
		r.DeletedAt = &t
	}
	return &r, nil
}

func scanRequirementVersion(row rowScanner) (*RequirementVersion, error) {
	var v RequirementVersion
	var content sql.NullString
	if err := row.Scan(&v.ID, &v.RequirementID, &v.VersionNo, &v.Title, &content, &v.Status,
		&v.ChangeNote, &v.Operator, &v.CreatedAt); err != nil {
		return nil, err
	}
	v.Content = content.String
	return &v, nil
}

// ListRequirements 分页查询项目需求
func (r *Repository) ListRequirements(ctx context.Context, projectID int64, req *ListRequirementsReq) ([]Requirement, bool, int, error) {
	req.Normalize()

	conditions := []string{"r.project_id = ?"}
	args := []any{projectID}
	if req.Deleted == nil || !*req.Deleted {
		conditions = append(conditions, "r.deleted = 0")
	} else {
		conditions = append(conditions, "r.deleted = 1")
	}
	if req.Status != "" {
		conditions = append(conditions, "r.status = ?")
		args = append(args, req.Status)
	}
	if req.Type != "" {
		conditions = append(conditions, "r.type = ?")
		args = append(args, req.Type)
	}
	if req.Priority != "" {
		conditions = append(conditions, "r.priority = ?")
		args = append(args, req.Priority)
	}
	if req.Keyword != "" {
		conditions = append(conditions, "(r.title LIKE ? OR r.code LIKE ? OR r.content LIKE ?)")
		like := "%" + req.Keyword + "%"
		args = append(args, like, like, like)
	}
	where := strings.Join(conditions, " AND ")

	var total int
	if err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM "+tableRequirement+" r WHERE "+where, args...).Scan(&total); err != nil {
		return nil, false, 0, err
	}

	query := "SELECT " + requirementColumns + " FROM " + tableRequirement + " r WHERE " + where +
		" ORDER BY r.id DESC LIMIT ? OFFSET ?"
	rows, err := r.db.QueryContext(ctx, query, append(args, req.PageSize+1, req.Offset())...)
	if err != nil {
		return nil, false, 0, err
	}
	defer rows.Close()

	var items []Requirement
	for rows.Next() {
		item, err := scanRequirement(rows)
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

// GetRequirement 按 ID 查询需求（含已删除）
func (r *Repository) GetRequirement(ctx context.Context, id int64) (*Requirement, error) {
	item, err := scanRequirement(r.db.QueryRowContext(ctx,
		"SELECT "+requirementColumns+" FROM "+tableRequirement+" r WHERE r.id = ?", id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, NewDevelopmentManagerError(ErrRequirementNotFound)
		}
		return nil, err
	}
	return item, nil
}

// NextRequirementNo 项目内下一个需求序号（解析 RQM-xxxx 的数字部分）
func (r *Repository) NextRequirementNo(ctx context.Context, projectID int64) (int, error) {
	var maxNo int
	err := r.db.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(CAST(SUBSTRING(code, 5) AS UNSIGNED)), 0) FROM "+tableRequirement+
			" WHERE project_id = ?", projectID).Scan(&maxNo)
	if err != nil {
		return 0, err
	}
	return maxNo + 1, nil
}

// txQueryExecer 事务内执行与查询
type txQueryExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// nextVersionNo 事务内计算下一版本号
func nextVersionNo(ctx context.Context, exec txQueryExecer, requirementID int64) (int, error) {
	var maxNo sql.NullInt64
	if err := exec.QueryRowContext(ctx,
		"SELECT MAX(version_no) FROM "+tableRequirementVersion+" WHERE requirement_id = ?",
		requirementID).Scan(&maxNo); err != nil {
		return 0, err
	}
	return int(maxNo.Int64) + 1, nil
}

func insertRequirementVersion(ctx context.Context, exec txQueryExecer, req *Requirement, versionNo int,
	status, changeNote, operator string) error {
	_, err := exec.ExecContext(ctx,
		"INSERT INTO "+tableRequirementVersion+
			" (requirement_id, version_no, title, content, status, change_note, operator) VALUES (?, ?, ?, ?, ?, ?, ?)",
		req.ID, versionNo, req.Title, req.Content, status, changeNote, operator)
	return err
}

// CreateRequirementTx 事务内创建需求并写入初始版本（version_no=1）
func (r *Repository) CreateRequirementTx(ctx context.Context, req *Requirement, changeNote string) (*Requirement, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx,
		"INSERT INTO "+tableRequirement+
			" (project_id, code, title, content, type, priority, status, source, owner, created_by, updated_by)"+
			" VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		req.ProjectID, req.Code, req.Title, req.Content, req.Type, req.Priority, req.Status, req.Source,
		req.Owner, req.CreatedBy, req.UpdatedBy)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	req.ID = id
	if err := insertRequirementVersion(ctx, tx, req, 1, req.Status, changeNote, req.CreatedBy); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetRequirement(ctx, id)
}

// UpdateRequirementTx 事务内更新需求并追加版本快照
func (r *Repository) UpdateRequirementTx(ctx context.Context, req *Requirement, status, changeNote, operator string) (*Requirement, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		"UPDATE "+tableRequirement+" SET title=?, content=?, type=?, priority=?, status=?, source=?, "+
			"owner=?, updated_by=? WHERE id=? AND deleted=0",
		req.Title, req.Content, req.Type, req.Priority, status, req.Source, req.Owner, operator, req.ID); err != nil {
		return nil, err
	}
	versionNo, err := nextVersionNo(ctx, tx, req.ID)
	if err != nil {
		return nil, err
	}
	if err := insertRequirementVersion(ctx, tx, req, versionNo, status, changeNote, operator); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetRequirement(ctx, req.ID)
}

// SoftDeleteRequirementTx 事务内软删除需求并写入"已删除"快照
func (r *Repository) SoftDeleteRequirementTx(ctx context.Context, req *Requirement, operator string, now time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		"UPDATE "+tableRequirement+" SET deleted=1, deleted_at=?, updated_by=? WHERE id=? AND deleted=0",
		now, operator, req.ID); err != nil {
		return err
	}
	versionNo, err := nextVersionNo(ctx, tx, req.ID)
	if err != nil {
		return err
	}
	if err := insertRequirementVersion(ctx, tx, req, versionNo, VersionStatusDeleted, "需求已删除", operator); err != nil {
		return err
	}
	return tx.Commit()
}

// ListRequirementVersions 查询需求版本历史（最新在前）
func (r *Repository) ListRequirementVersions(ctx context.Context, requirementID int64) ([]RequirementVersion, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT "+requirementVersionColumns+" FROM "+tableRequirementVersion+
			" WHERE requirement_id = ? ORDER BY version_no DESC", requirementID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []RequirementVersion
	for rows.Next() {
		v, err := scanRequirementVersion(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *v)
	}
	return items, rows.Err()
}
