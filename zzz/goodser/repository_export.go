package goodser

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

const tableExportJobs = "zzz_goodser_export_jobs"

// CreateExportJob 写入导出任务。
func (r *Repository) CreateExportJob(ctx context.Context, job *ExportJob) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO `+tableExportJobs+`
		 (id, inventory_id, status, request, result_url, error, created_by, created_at, finished_at, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.ID, job.InventoryID, job.Status, string(job.Request),
		job.ResultURL, job.Error, job.CreatedBy, job.CreatedAt, job.FinishedAt, job.ExpiresAt)
	return err
}

// UpdateExportJob 更新任务状态与结果。
func (r *Repository) UpdateExportJob(ctx context.Context, id, status string, resultURL, errMsg *string, finishedAt *time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE `+tableExportJobs+`
		 SET status = ?, result_url = ?, error = ?, finished_at = ?
		 WHERE id = ?`,
		status, resultURL, errMsg, finishedAt, id)
	return err
}

// GetExportJob 读取单个导出任务。
func (r *Repository) GetExportJob(ctx context.Context, id string) (*ExportJob, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, inventory_id, status, request, result_url, error, created_by,
		        created_at, finished_at, expires_at
		 FROM `+tableExportJobs+` WHERE id = ?`, id)
	return scanExportJob(row)
}

// ListExportJobs 查询导出历史；inventoryID 为空时返回全部。
func (r *Repository) ListExportJobs(ctx context.Context, inventoryID string, limit int) ([]ExportJob, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	query := `SELECT id, inventory_id, status, request, result_url, error, created_by,
	                 created_at, finished_at, expires_at
	          FROM ` + tableExportJobs
	args := make([]any, 0, 2)
	if inventoryID != "" {
		query += ` WHERE inventory_id = ?`
		args = append(args, inventoryID)
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT ?`
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]ExportJob, 0, limit)
	for rows.Next() {
		job, err := scanExportJob(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *job)
	}
	return items, rows.Err()
}

// DeleteExportJob 删除导出任务记录。
func (r *Repository) DeleteExportJob(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM `+tableExportJobs+` WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

type exportRowScanner interface {
	Scan(dest ...any) error
}

func scanExportJob(row exportRowScanner) (*ExportJob, error) {
	var (
		job        ExportJob
		request    []byte
		resultURL  sql.NullString
		errMsg     sql.NullString
		createdBy  sql.NullInt64
		finishedAt sql.NullTime
		expiresAt  sql.NullTime
	)
	if err := row.Scan(
		&job.ID, &job.InventoryID, &job.Status, &request,
		&resultURL, &errMsg, &createdBy, &job.CreatedAt, &finishedAt, &expiresAt,
	); err != nil {
		return nil, err
	}
	job.Request = json.RawMessage(request)
	if resultURL.Valid {
		job.ResultURL = &resultURL.String
	}
	if errMsg.Valid {
		job.Error = &errMsg.String
	}
	if createdBy.Valid {
		job.CreatedBy = &createdBy.Int64
	}
	if finishedAt.Valid {
		job.FinishedAt = &finishedAt.Time
	}
	if expiresAt.Valid {
		job.ExpiresAt = &expiresAt.Time
	}
	return &job, nil
}
