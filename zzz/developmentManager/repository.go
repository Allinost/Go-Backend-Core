package developmentmanager

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const (
	tableProject  = "zzz_development_manager_project"
	tableRelation = "zzz_development_manager_project_relation"
	tableAsset    = "zzz_development_manager_project_asset"

	projectColumns = "id, project_key, name, type, description, repo_url, repo_path, default_branch, " +
		"tech_stack, status, deleted, deleted_at, created_by, updated_by, created_at, updated_at"

	relationColumns = "id, from_project_id, to_project_id, relation_type, note, deleted, deleted_at, " +
		"created_by, created_at, updated_at"

	assetColumns = "id, project_id, category, title, uri, format, version, size, checksum, note, " +
		"deleted, deleted_at, created_at, updated_at"
)

// Repository 数据访问层
type Repository struct {
	db *sql.DB
}

// NewRepository 创建 Repository
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanProject(row rowScanner) (*Project, error) {
	var p Project
	var techStack sql.NullString
	var deletedAt sql.NullTime
	err := row.Scan(&p.ID, &p.ProjectKey, &p.Name, &p.Type, &p.Description, &p.RepoURL, &p.RepoPath,
		&p.DefaultBranch, &techStack, &p.Status, &p.Deleted, &deletedAt, &p.CreatedBy, &p.UpdatedBy,
		&p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if techStack.Valid && techStack.String != "" {
		_ = json.Unmarshal([]byte(techStack.String), &p.TechStack)
	}
	if deletedAt.Valid {
		t := deletedAt.Time
		p.DeletedAt = &t
	}
	return &p, nil
}

func scanRelation(row rowScanner) (*ProjectRelation, error) {
	var r ProjectRelation
	var deletedAt sql.NullTime
	var fromKey, fromName, toKey, toName sql.NullString
	err := row.Scan(&r.ID, &r.FromProjectID, &r.ToProjectID, &r.RelationType, &r.Note, &r.Deleted,
		&deletedAt, &r.CreatedBy, &r.CreatedAt, &r.UpdatedAt, &fromKey, &fromName, &toKey, &toName)
	if err != nil {
		return nil, err
	}
	r.FromProjectKey = fromKey.String
	r.FromProjectName = fromName.String
	r.ToProjectKey = toKey.String
	r.ToProjectName = toName.String
	return &r, nil
}

func scanAsset(row rowScanner) (*ProjectAsset, error) {
	var a ProjectAsset
	var deletedAt sql.NullTime
	err := row.Scan(&a.ID, &a.ProjectID, &a.Category, &a.Title, &a.URI, &a.Format, &a.Version,
		&a.Size, &a.Checksum, &a.Note, &a.Deleted, &deletedAt, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// ---------- 项目 ----------

// ListProjects 分页查询项目（默认仅未删除，deleted 参数可切换）
func (r *Repository) ListProjects(ctx context.Context, req *ListProjectsReq) ([]Project, bool, int, error) {
	req.Normalize()

	var conditions []string
	var args []any
	conditions = append(conditions, "1=1")

	if req.Deleted == nil || !*req.Deleted {
		conditions = append(conditions, "deleted = 0")
	} else {
		conditions = append(conditions, "deleted = 1")
	}
	if req.Type != "" {
		conditions = append(conditions, "type = ?")
		args = append(args, req.Type)
	}
	if req.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, req.Status)
	}
	if req.Keyword != "" {
		conditions = append(conditions, "(name LIKE ? OR project_key LIKE ? OR description LIKE ?)")
		like := "%" + req.Keyword + "%"
		args = append(args, like, like, like)
	}
	where := strings.Join(conditions, " AND ")

	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+tableProject+" WHERE "+where, args...).Scan(&total); err != nil {
		return nil, false, 0, err
	}

	limit := req.PageSize + 1
	query := "SELECT " + projectColumns + " FROM " + tableProject + " WHERE " + where +
		" ORDER BY id DESC LIMIT ? OFFSET ?"
	rows, err := r.db.QueryContext(ctx, query, append(args, limit, req.Offset())...)
	if err != nil {
		return nil, false, 0, err
	}
	defer rows.Close()

	var items []Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, false, 0, err
		}
		items = append(items, *p)
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

// GetProject 按 ID 查询项目（含已删除，由 deleted 字段区分）
func (r *Repository) GetProject(ctx context.Context, id int64) (*Project, error) {
	p, err := scanProject(r.db.QueryRowContext(ctx,
		"SELECT "+projectColumns+" FROM "+tableProject+" WHERE id = ?", id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, NewDevelopmentManagerError(ErrProjectNotFound)
		}
		return nil, err
	}
	return p, nil
}

// GetProjectByKey 按唯一键查询项目（含已删除）
func (r *Repository) GetProjectByKey(ctx context.Context, key string) (*Project, error) {
	p, err := scanProject(r.db.QueryRowContext(ctx,
		"SELECT "+projectColumns+" FROM "+tableProject+" WHERE project_key = ?", key))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, NewDevelopmentManagerError(ErrProjectNotFound)
		}
		return nil, err
	}
	return p, nil
}

// CreateProject 新建项目
func (r *Repository) CreateProject(ctx context.Context, p *Project) (*Project, error) {
	techStack := "null"
	if len(p.TechStack) > 0 {
		b, _ := json.Marshal(p.TechStack)
		techStack = string(b)
	}
	status := p.Status
	if status == "" {
		status = ProjectStatusActive
	}
	res, err := r.db.ExecContext(ctx,
		"INSERT INTO "+tableProject+
			" (project_key, name, type, description, repo_url, repo_path, default_branch, tech_stack, status, created_by, updated_by)"+
			" VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		p.ProjectKey, p.Name, p.Type, p.Description, p.RepoURL, p.RepoPath, p.DefaultBranch,
		techStack, status, p.CreatedBy, p.UpdatedBy)
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate entry") {
			return nil, NewDevelopmentManagerError(ErrDuplicateProjectKey)
		}
		return nil, err
	}
	id, _ := res.LastInsertId()
	return r.GetProject(ctx, id)
}

// UpdateProject 全量更新项目
func (r *Repository) UpdateProject(ctx context.Context, p *Project) (*Project, error) {
	techStack := "null"
	if len(p.TechStack) > 0 {
		b, _ := json.Marshal(p.TechStack)
		techStack = string(b)
	}
	res, err := r.db.ExecContext(ctx,
		"UPDATE "+tableProject+
			" SET project_key=?, name=?, type=?, description=?, repo_url=?, repo_path=?, default_branch=?, "+
			"tech_stack=?, status=?, updated_by=? WHERE id=? AND deleted=0",
		p.ProjectKey, p.Name, p.Type, p.Description, p.RepoURL, p.RepoPath, p.DefaultBranch,
		techStack, p.Status, p.UpdatedBy, p.ID)
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate entry") {
			return nil, NewDevelopmentManagerError(ErrDuplicateProjectKey)
		}
		return nil, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		// 区分不存在与已删除
		if _, err := r.GetProject(ctx, p.ID); err != nil {
			return nil, err
		}
		return nil, NewDevelopmentManagerError(ErrProjectNotFound)
	}
	return r.GetProject(ctx, p.ID)
}

// ListDependents 查询引用了指定项目的启用中关联（被依赖方视角，用于删除前二次确认）
func (r *Repository) ListDependents(ctx context.Context, projectID int64) ([]ProjectRelation, error) {
	query := "SELECT r.id, r.from_project_id, r.to_project_id, r.relation_type, r.note, r.deleted, " +
		"r.deleted_at, r.created_by, r.created_at, r.updated_at, " +
		"fp.project_key, fp.name, tp.project_key, tp.name " +
		"FROM " + tableRelation + " r " +
		"LEFT JOIN " + tableProject + " fp ON fp.id = r.from_project_id " +
		"LEFT JOIN " + tableProject + " tp ON tp.id = r.to_project_id " +
		"WHERE r.to_project_id = ? AND r.deleted = 0 ORDER BY r.id DESC"
	rows, err := r.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []ProjectRelation
	for rows.Next() {
		rel, err := scanRelation(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *rel)
	}
	return items, rows.Err()
}

// DeleteProjectTx 在事务中软删除项目并级联标记关联与资产
func (r *Repository) DeleteProjectTx(ctx context.Context, projectID int64, now time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		"UPDATE "+tableProject+" SET deleted=1, deleted_at=? WHERE id=? AND deleted=0", now, projectID); err != nil {
		return err
	}
	if err := r.CascadeDeleteRelations(ctx, tx, projectID, now); err != nil {
		return err
	}
	if err := r.CascadeDeleteAssets(ctx, tx, projectID, now); err != nil {
		return err
	}
	return tx.Commit()
}

// CascadeDeleteRelations 删除项目时级联标记其发出与收到的关联为已删除
func (r *Repository) CascadeDeleteRelations(ctx context.Context, exec execer, projectID int64, now time.Time) error {
	_, err := exec.ExecContext(ctx,
		"UPDATE "+tableRelation+" SET deleted=1, deleted_at=? WHERE deleted=0 AND (from_project_id=? OR to_project_id=?)",
		now, projectID, projectID)
	return err
}

// CascadeDeleteAssets 删除项目时级联标记其资产索引为已删除
func (r *Repository) CascadeDeleteAssets(ctx context.Context, exec execer, projectID int64, now time.Time) error {
	_, err := exec.ExecContext(ctx,
		"UPDATE "+tableAsset+" SET deleted=1, deleted_at=? WHERE deleted=0 AND project_id=?",
		now, projectID)
	return err
}

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// ---------- 关联 ----------

// ListRelations 查询项目关联（含两端项目 key/名称，from 与 to 任一端命中即返回）
func (r *Repository) ListRelations(ctx context.Context, projectID int64, req *ListRelationsReq) ([]ProjectRelation, bool, int, error) {
	req.Normalize()

	var conditions []string
	var args []any
	conditions = append(conditions, "(from_project_id = ? OR to_project_id = ?)")
	args = append(args, projectID, projectID)

	if req.Deleted == nil || !*req.Deleted {
		conditions = append(conditions, "r.deleted = 0")
	} else {
		conditions = append(conditions, "r.deleted = 1")
	}
	if req.Type != "" {
		conditions = append(conditions, "r.relation_type = ?")
		args = append(args, req.Type)
	}
	where := strings.Join(conditions, " AND ")

	var total int
	countQuery := "SELECT COUNT(*) FROM " + tableRelation + " r WHERE " + where
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, false, 0, err
	}

	base := "FROM " + tableRelation + " r " +
		"LEFT JOIN " + tableProject + " fp ON fp.id = r.from_project_id " +
		"LEFT JOIN " + tableProject + " tp ON tp.id = r.to_project_id " +
		"WHERE " + where
	selectCols := "SELECT r.id, r.from_project_id, r.to_project_id, r.relation_type, r.note, r.deleted, " +
		"r.deleted_at, r.created_by, r.created_at, r.updated_at, fp.project_key, fp.name, tp.project_key, tp.name "

	query := selectCols + base + " ORDER BY r.id DESC LIMIT ? OFFSET ?"
	rows, err := r.db.QueryContext(ctx, query, append(args, req.PageSize+1, req.Offset())...)
	if err != nil {
		return nil, false, 0, err
	}
	defer rows.Close()

	var items []ProjectRelation
	for rows.Next() {
		rel, err := scanRelation(rows)
		if err != nil {
			return nil, false, 0, err
		}
		items = append(items, *rel)
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

// GetRelation 查询单条关联
func (r *Repository) GetRelation(ctx context.Context, id int64) (*ProjectRelation, error) {
	query := "SELECT r.id, r.from_project_id, r.to_project_id, r.relation_type, r.note, r.deleted, " +
		"r.deleted_at, r.created_by, r.created_at, r.updated_at, " +
		"fp.project_key, fp.name, tp.project_key, tp.name " +
		"FROM " + tableRelation + " r " +
		"LEFT JOIN " + tableProject + " fp ON fp.id = r.from_project_id " +
		"LEFT JOIN " + tableProject + " tp ON tp.id = r.to_project_id " +
		"WHERE r.id = ?"
	rel, err := scanRelation(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, NewDevelopmentManagerError(ErrRelationNotFound)
		}
		return nil, err
	}
	return rel, nil
}

// RelationExists 判断同两端同类型的未删除关联是否已存在
func (r *Repository) RelationExists(ctx context.Context, fromID, toID int64, relType string) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM "+tableRelation+
			" WHERE from_project_id=? AND to_project_id=? AND relation_type=? AND deleted=0",
		fromID, toID, relType).Scan(&n)
	return n > 0, err
}

// CreateRelation 新建关联
func (r *Repository) CreateRelation(ctx context.Context, rel *ProjectRelation) (*ProjectRelation, error) {
	res, err := r.db.ExecContext(ctx,
		"INSERT INTO "+tableRelation+" (from_project_id, to_project_id, relation_type, note, created_by) VALUES (?, ?, ?, ?, ?)",
		rel.FromProjectID, rel.ToProjectID, rel.RelationType, rel.Note, rel.CreatedBy)
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate entry") {
			return nil, NewDevelopmentManagerError(ErrDuplicateProjectKey)
		}
		return nil, err
	}
	id, _ := res.LastInsertId()
	return r.GetRelation(ctx, id)
}

// SoftDeleteRelation 软删除关联
func (r *Repository) SoftDeleteRelation(ctx context.Context, id int64, now time.Time) error {
	res, err := r.db.ExecContext(ctx,
		"UPDATE "+tableRelation+" SET deleted=1, deleted_at=? WHERE id=? AND deleted=0", now, id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		if _, err := r.GetRelation(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// ---------- 资产 ----------

// ListAssets 分页查询项目资产索引
func (r *Repository) ListAssets(ctx context.Context, projectID int64, req *ListAssetsReq) ([]ProjectAsset, bool, int, error) {
	req.Normalize()

	var conditions []string
	var args []any
	conditions = append(conditions, "project_id = ?")
	args = append(args, projectID)
	if req.Deleted == nil || !*req.Deleted {
		conditions = append(conditions, "deleted = 0")
	} else {
		conditions = append(conditions, "deleted = 1")
	}
	if req.Category != "" {
		conditions = append(conditions, "category = ?")
		args = append(args, req.Category)
	}
	where := strings.Join(conditions, " AND ")

	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+tableAsset+" WHERE "+where, args...).Scan(&total); err != nil {
		return nil, false, 0, err
	}

	query := "SELECT " + assetColumns + " FROM " + tableAsset + " WHERE " + where +
		" ORDER BY id DESC LIMIT ? OFFSET ?"
	rows, err := r.db.QueryContext(ctx, query, append(args, req.PageSize+1, req.Offset())...)
	if err != nil {
		return nil, false, 0, err
	}
	defer rows.Close()

	var items []ProjectAsset
	for rows.Next() {
		a, err := scanAsset(rows)
		if err != nil {
			return nil, false, 0, err
		}
		items = append(items, *a)
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

// GetAsset 查询单条资产索引
func (r *Repository) GetAsset(ctx context.Context, id int64) (*ProjectAsset, error) {
	a, err := scanAsset(r.db.QueryRowContext(ctx,
		"SELECT "+assetColumns+" FROM "+tableAsset+" WHERE id = ?", id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, NewDevelopmentManagerError(ErrAssetNotFound)
		}
		return nil, err
	}
	return a, nil
}

// AssetExists 判断同项目同 URI 的未删除资产是否已存在（seed 幂等用）
func (r *Repository) AssetExists(ctx context.Context, projectID int64, uri string) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM "+tableAsset+" WHERE project_id=? AND uri=? AND deleted=0",
		projectID, uri).Scan(&n)
	return n > 0, err
}

// CreateAsset 新建资产索引
func (r *Repository) CreateAsset(ctx context.Context, a *ProjectAsset) (*ProjectAsset, error) {
	res, err := r.db.ExecContext(ctx,
		"INSERT INTO "+tableAsset+
			" (project_id, category, title, uri, format, version, size, checksum, note) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		a.ProjectID, a.Category, a.Title, a.URI, a.Format, a.Version, a.Size, a.Checksum, a.Note)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return r.GetAsset(ctx, id)
}

// SoftDeleteAsset 软删除资产索引
func (r *Repository) SoftDeleteAsset(ctx context.Context, id int64, now time.Time) error {
	res, err := r.db.ExecContext(ctx,
		"UPDATE "+tableAsset+" SET deleted=1, deleted_at=? WHERE id=? AND deleted=0", now, id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		if _, err := r.GetAsset(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// ---------- 统计 ----------

// Stats 汇总仪表盘统计
func (r *Repository) Stats(ctx context.Context) (*StatsResp, error) {
	resp := &StatsResp{
		ProjectsByType:   map[string]int{},
		ProjectsByStatus: map[string]int{},
	}

	rows, err := r.db.QueryContext(ctx,
		"SELECT type, status, COUNT(*) FROM "+tableProject+" WHERE deleted=0 GROUP BY type, status")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var t, s string
		var n int
		if err := rows.Scan(&t, &s, &n); err != nil {
			return nil, err
		}
		resp.ProjectsByType[t] += n
		resp.ProjectsByStatus[s] += n
		resp.ProjectsTotal += n
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM "+tableProject+" WHERE deleted=1").Scan(&resp.ProjectsDeleted); err != nil {
		return nil, err
	}
	if err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM "+tableRelation+" WHERE deleted=0").Scan(&resp.RelationsActive); err != nil {
		return nil, err
	}
	if err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM "+tableAsset+" WHERE deleted=0").Scan(&resp.AssetsTotal); err != nil {
		return nil, err
	}
	return resp, nil
}
