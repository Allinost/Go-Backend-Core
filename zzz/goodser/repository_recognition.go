package goodser

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

const (
	tableRecognitionJobs   = "zzz_goodser_recognition_jobs"
	tableRecognitionItems  = "zzz_goodser_recognition_items"
	tableProductEmbeddings = "zzz_goodser_product_embeddings"
)

// --- 识别任务 ---

func (r *Repository) CreateRecognitionJob(ctx context.Context, job *RecognitionJob) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO `+tableRecognitionJobs+`
		 (id, inventory_id, image_url, mode, status, result, error, created_by, created_at, finished_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.ID, job.InventoryID, job.ImageURL, job.Mode, job.Status,
		rawJSONValue(job.Result), job.Error, job.CreatedBy, job.CreatedAt, job.FinishedAt)
	return err
}

func (r *Repository) UpdateRecognitionJob(ctx context.Context, id, status string, result json.RawMessage, errMsg *string, finishedAt *time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE `+tableRecognitionJobs+`
		 SET status = ?, result = ?, error = ?, finished_at = ?
		 WHERE id = ?`,
		status, rawJSONValue(result), errMsg, finishedAt, id)
	return err
}

func (r *Repository) GetRecognitionJob(ctx context.Context, id string) (*RecognitionJob, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, inventory_id, image_url, mode, status, result, error, created_by, created_at, finished_at
		 FROM `+tableRecognitionJobs+` WHERE id = ?`, id)
	return scanRecognitionJob(row)
}

func (r *Repository) ListRecognitionJobs(ctx context.Context, inventoryID string, limit int) ([]RecognitionJob, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	query := `SELECT id, inventory_id, image_url, mode, status, result, error, created_by, created_at, finished_at
	          FROM ` + tableRecognitionJobs
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
	items := make([]RecognitionJob, 0, limit)
	for rows.Next() {
		job, err := scanRecognitionJob(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *job)
	}
	return items, rows.Err()
}

// --- 识别项 ---

func (r *Repository) CreateRecognitionItem(ctx context.Context, item *RecognitionItem) error {
	return r.db.QueryRowContext(ctx,
		`INSERT INTO `+tableRecognitionItems+`
		 (job_id, box, crop_url, product_id, catalog_item_id, quantity, confidence, matched_by, alternatives, confirmed, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 RETURNING id`,
		item.JobID, rawJSONValue(item.Box), item.CropURL, item.ProductID, item.CatalogItemID,
		item.Quantity, item.Confidence, item.MatchedBy, rawJSONValue(item.Alternatives),
		item.Confirmed, item.CreatedAt).Scan(&item.ID)
}

func (r *Repository) ListRecognitionItems(ctx context.Context, jobID string) ([]RecognitionItem, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, job_id, box, crop_url, product_id, catalog_item_id, quantity, confidence,
		        matched_by, alternatives, confirmed, created_at
		 FROM `+tableRecognitionItems+` WHERE job_id = ? ORDER BY id`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]RecognitionItem, 0)
	for rows.Next() {
		item, err := scanRecognitionItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (r *Repository) UpdateRecognitionItem(ctx context.Context, jobID string, itemID int64, productID *string, quantity *int, confirmed *bool) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE `+tableRecognitionItems+`
		 SET product_id = COALESCE(?, product_id),
		     quantity = COALESCE(?, quantity),
		     confirmed = COALESCE(?, confirmed)
		 WHERE id = ? AND job_id = ?`,
		productID, quantity, confirmed, itemID, jobID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// --- 向量 ---

func (r *Repository) DeleteProductEmbeddings(ctx context.Context, inventoryID string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM `+tableProductEmbeddings+` WHERE inventory_id = ? AND source_type = ?`,
		inventoryID, EmbeddingSourceProduct)
	return err
}

func (r *Repository) UpsertProductEmbedding(ctx context.Context, e *ProductEmbedding) error {
	embedding, err := json.Marshal(e.Vector)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx,
		`INSERT INTO `+tableProductEmbeddings+`
		 (inventory_id, source_type, source_id, image_url, model, dim, embedding, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, now(), now())
		 ON CONFLICT (source_type, source_id, image_url, model)
		 DO UPDATE SET inventory_id = EXCLUDED.inventory_id, dim = EXCLUDED.dim,
		               embedding = EXCLUDED.embedding, updated_at = now()`,
		e.InventoryID, e.SourceType, e.SourceID, e.ImageURL, e.Model, e.Dim, string(embedding))
	return err
}

func (r *Repository) ListProductEmbeddings(ctx context.Context, inventoryID string) ([]ProductEmbedding, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, inventory_id, source_type, source_id, image_url, model, dim, embedding
		 FROM `+tableProductEmbeddings+` WHERE inventory_id = ?`, inventoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ProductEmbedding, 0)
	for rows.Next() {
		var (
			e         ProductEmbedding
			embedding []byte
		)
		if err := rows.Scan(&e.ID, &e.InventoryID, &e.SourceType, &e.SourceID, &e.ImageURL, &e.Model, &e.Dim, &embedding); err != nil {
			return nil, err
		}
		if len(embedding) > 0 {
			if err := json.Unmarshal(embedding, &e.Vector); err != nil {
				continue
			}
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *Repository) CountProductEmbeddings(ctx context.Context, inventoryID string) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM `+tableProductEmbeddings+` WHERE inventory_id = ? AND source_type = ?`,
		inventoryID, EmbeddingSourceProduct).Scan(&n)
	return n, err
}

// --- scan helpers ---

type recognitionRowScanner interface {
	Scan(dest ...any) error
}

func scanRecognitionJob(row recognitionRowScanner) (*RecognitionJob, error) {
	var (
		job        RecognitionJob
		result     []byte
		errMsg     sql.NullString
		createdBy  sql.NullInt64
		finishedAt sql.NullTime
	)
	if err := row.Scan(&job.ID, &job.InventoryID, &job.ImageURL, &job.Mode, &job.Status,
		&result, &errMsg, &createdBy, &job.CreatedAt, &finishedAt); err != nil {
		return nil, err
	}
	job.Result = json.RawMessage(result)
	if errMsg.Valid {
		job.Error = &errMsg.String
	}
	if createdBy.Valid {
		job.CreatedBy = &createdBy.Int64
	}
	if finishedAt.Valid {
		job.FinishedAt = &finishedAt.Time
	}
	return &job, nil
}

func scanRecognitionItem(row recognitionRowScanner) (*RecognitionItem, error) {
	var (
		item          RecognitionItem
		box           []byte
		cropURL       sql.NullString
		productID     sql.NullString
		catalogItemID sql.NullString
		matchedBy     sql.NullString
		alternatives  []byte
	)
	if err := row.Scan(&item.ID, &item.JobID, &box, &cropURL, &productID, &catalogItemID,
		&item.Quantity, &item.Confidence, &matchedBy, &alternatives, &item.Confirmed, &item.CreatedAt); err != nil {
		return nil, err
	}
	item.Box = json.RawMessage(box)
	item.Alternatives = json.RawMessage(alternatives)
	if cropURL.Valid {
		item.CropURL = cropURL.String
	}
	if productID.Valid {
		item.ProductID = &productID.String
	}
	if catalogItemID.Valid {
		item.CatalogItemID = &catalogItemID.String
	}
	if matchedBy.Valid {
		item.MatchedBy = matchedBy.String
	}
	return &item, nil
}

func rawJSONValue(data json.RawMessage) any {
	if len(data) == 0 {
		return nil
	}
	return string(data)
}
