package goodser

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

const tableExportJobs = "zzz_goodser_export_jobs"
const tableExportTemplates = "zzz_goodser_export_templates"

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

// --- 导出模板 ---

// CreateExportTemplate 新建模板。
func (r *Repository) CreateExportTemplate(ctx context.Context, tpl *ExportTemplate) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO `+tableExportTemplates+`
		 (id, inventory_id, name, config, is_default, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		tpl.ID, tpl.InventoryID, tpl.Name, string(tpl.Config), tpl.IsDefault, tpl.CreatedAt, tpl.UpdatedAt)
	return err
}

// UpdateExportTemplate 更新模板。
func (r *Repository) UpdateExportTemplate(ctx context.Context, tpl *ExportTemplate) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE `+tableExportTemplates+`
		 SET inventory_id = ?, name = ?, config = ?, is_default = ?, updated_at = ?
		 WHERE id = ?`,
		tpl.InventoryID, tpl.Name, string(tpl.Config), tpl.IsDefault, tpl.UpdatedAt, tpl.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// GetExportTemplate 读取单个模板。
func (r *Repository) GetExportTemplate(ctx context.Context, id string) (*ExportTemplate, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, inventory_id, name, config, is_default, created_at, updated_at
		 FROM `+tableExportTemplates+` WHERE id = ?`, id)
	return scanExportTemplate(row)
}

// ListExportTemplates 查询模板；inventoryID 为空时返回全部。
func (r *Repository) ListExportTemplates(ctx context.Context, inventoryID string) ([]ExportTemplate, error) {
	query := `SELECT id, inventory_id, name, config, is_default, created_at, updated_at
	          FROM ` + tableExportTemplates
	args := make([]any, 0, 1)
	if inventoryID != "" {
		query += ` WHERE inventory_id = ? OR inventory_id IS NULL`
		args = append(args, inventoryID)
	}
	query += ` ORDER BY is_default DESC, updated_at DESC, id DESC`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]ExportTemplate, 0)
	for rows.Next() {
		tpl, err := scanExportTemplate(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *tpl)
	}
	return items, rows.Err()
}

// DeleteExportTemplate 删除模板。
func (r *Repository) DeleteExportTemplate(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM `+tableExportTemplates+` WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// ClearDefaultExportTemplates 清除同一库存目录（或全局）下的默认标记。
func (r *Repository) ClearDefaultExportTemplates(ctx context.Context, inventoryID string) error {
	if inventoryID == "" {
		_, err := r.db.ExecContext(ctx,
			`UPDATE `+tableExportTemplates+` SET is_default = false, updated_at = now()
			 WHERE is_default = true AND inventory_id IS NULL`)
		return err
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE `+tableExportTemplates+` SET is_default = false, updated_at = now()
		 WHERE is_default = true AND inventory_id = ?`, inventoryID)
	return err
}

func scanExportTemplate(row exportRowScanner) (*ExportTemplate, error) {
	var (
		tpl    ExportTemplate
		inv    sql.NullString
		config []byte
	)
	if err := row.Scan(&tpl.ID, &inv, &tpl.Name, &config, &tpl.IsDefault, &tpl.CreatedAt, &tpl.UpdatedAt); err != nil {
		return nil, err
	}
	if inv.Valid {
		tpl.InventoryID = &inv.String
	}
	tpl.Config = json.RawMessage(config)
	return &tpl, nil
}
