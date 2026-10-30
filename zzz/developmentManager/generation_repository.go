package developmentmanager

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

const (
	tableGeneration = "zzz_development_manager_generation"

	generationColumns = "id, requirement_id, project_id, kind, engine, engine_used, auto_commit, committed, " +
		"branch, commit_hash, status, input_snapshot, output_ref, files, error, operator, started_at, " +
		"finished_at, created_at, updated_at"
)

func scanGeneration(row rowScanner) (*Generation, error) {
	var g Generation
	var snapshot, files []byte
	var startedAt, finishedAt sql.NullTime
	if err := row.Scan(&g.ID, &g.RequirementID, &g.ProjectID, &g.Kind, &g.Engine, &g.EngineUsed,
		&g.AutoCommit, &g.Committed, &g.Branch, &g.CommitHash, &g.Status, &snapshot, &g.OutputRef,
		&files, &g.Error, &g.Operator, &startedAt, &finishedAt, &g.CreatedAt, &g.UpdatedAt); err != nil {
		return nil, err
	}
	if len(snapshot) > 0 {
		g.InputSnapshot = json.RawMessage(snapshot)
	}
	if len(files) > 0 {
		_ = json.Unmarshal(files, &g.Files)
	}
	if startedAt.Valid {
		t := startedAt.Time
		g.StartedAt = &t
	}
	if finishedAt.Valid {
		t := finishedAt.Time
		g.FinishedAt = &t
	}
	return &g, nil
}

// CreateGeneration 创建生成任务（pending）
func (r *Repository) CreateGeneration(ctx context.Context, g *Generation) (*Generation, error) {
	var snapshot any
	if len(g.InputSnapshot) > 0 {
		snapshot = string(g.InputSnapshot)
	}
	res, err := r.db.ExecContext(ctx,
		"INSERT INTO "+tableGeneration+
			" (requirement_id, project_id, kind, engine, auto_commit, status, input_snapshot, operator)"+
			" VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		g.RequirementID, g.ProjectID, g.Kind, g.Engine, g.AutoCommit, g.Status, snapshot, g.Operator)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return r.GetGeneration(ctx, id)
}

// GetGeneration 按 ID 查询生成任务
func (r *Repository) GetGeneration(ctx context.Context, id int64) (*Generation, error) {
	g, err := scanGeneration(r.db.QueryRowContext(ctx,
		"SELECT "+generationColumns+" FROM "+tableGeneration+" WHERE id = ?", id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, NewDevelopmentManagerError(ErrGenerationNotFound)
		}
		return nil, err
	}
	return g, nil
}

// ListGenerationsByRequirement 查询需求下的生成任务（最新在前）
func (r *Repository) ListGenerationsByRequirement(ctx context.Context, requirementID int64) ([]Generation, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT "+generationColumns+" FROM "+tableGeneration+
			" WHERE requirement_id = ? ORDER BY id DESC", requirementID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Generation
	for rows.Next() {
		g, err := scanGeneration(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *g)
	}
	return items, rows.Err()
}

// UpdateGeneration 更新任务执行状态与产物
func (r *Repository) UpdateGeneration(ctx context.Context, g *Generation) error {
	var files any
	if len(g.Files) > 0 {
		b, _ := json.Marshal(g.Files)
		files = string(b)
	}
	_, err := r.db.ExecContext(ctx,
		"UPDATE "+tableGeneration+" SET engine_used=?, committed=?, branch=?, commit_hash=?, status=?, "+
			"output_ref=?, files=?, error=?, started_at=?, finished_at=? WHERE id=?",
		g.EngineUsed, g.Committed, g.Branch, g.CommitHash, g.Status, g.OutputRef, files, g.Error,
		g.StartedAt, g.FinishedAt, g.ID)
	return err
}
