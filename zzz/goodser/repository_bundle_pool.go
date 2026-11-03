package goodser

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	tableBundlePools     = "zzz_goodser_bundle_pools"
	tableBundlePoolItems = "zzz_goodser_bundle_pool_items"
)

// ListBundlePools 查询捆物池
func (r *Repository) ListBundlePools(ctx context.Context, inventoryID, status string) ([]BundlePool, error) {
	query := `SELECT id, inventory_id, code, name, description, status, min_item_count, min_amount,
		created_at, updated_at FROM ` + tableBundlePools + ` WHERE inventory_id = ?`
	args := []any{inventoryID}
	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}
	query += " ORDER BY created_at DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]BundlePool, 0)
	for rows.Next() {
		item, err := scanBundlePool(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range items {
		poolItems, err := r.loadBundlePoolItems(ctx, items[i].ID)
		if err != nil {
			return nil, err
		}
		items[i].Items = poolItems
	}
	return items, nil
}

// GetBundlePool 查询捆物池详情
func (r *Repository) GetBundlePool(ctx context.Context, id string) (*BundlePool, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, inventory_id, code, name, description, status, min_item_count, min_amount,
		created_at, updated_at FROM `+tableBundlePools+` WHERE id = ?`, id)
	item, err := scanBundlePool(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, NewGoodserError(ErrBundlePoolNotFound)
		}
		return nil, err
	}
	items, err := r.loadBundlePoolItems(ctx, id)
	if err != nil {
		return nil, err
	}
	item.Items = items
	return &item, nil
}

func scanBundlePool(scanner interface{ Scan(...any) error }) (BundlePool, error) {
	var (
		item        BundlePool
		description sql.NullString
	)
	if err := scanner.Scan(&item.ID, &item.InventoryID, &item.Code, &item.Name, &description, &item.Status,
		&item.MinItemCount, &item.MinAmount, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return BundlePool{}, err
	}
	if description.Valid {
		item.Description = &description.String
	}
	item.Items = make([]BundlePoolItem, 0)
	return item, nil
}

func (r *Repository) loadBundlePoolItems(ctx context.Context, poolID string) ([]BundlePoolItem, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT pi.product_id, p.name, p.code, p.image_url, pi.default_quantity,
		p.expected_price, p.quantity, p.reserved_quantity
		FROM `+tableBundlePoolItems+` pi
		JOIN `+tableProducts+` p ON p.id = pi.product_id
		WHERE pi.pool_id = ?
		ORDER BY pi.sort_order, pi.id`, poolID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]BundlePoolItem, 0)
	for rows.Next() {
		var item BundlePoolItem
		if err := rows.Scan(&item.ProductID, &item.ProductName, &item.ProductCode, &item.ImageURL,
			&item.DefaultQuantity, &item.ExpectedPrice, &item.Available, &item.ReservedQty); err != nil {
			return nil, err
		}
		item.TotalQuantity = item.Available + item.ReservedQty
		item.ImageURL = normalizeImageURL(item.ImageURL)
		items = append(items, item)
	}
	return items, rows.Err()
}

// CreateBundlePool 创建捆物池及成员
func (r *Repository) CreateBundlePool(ctx context.Context, req *CreateBundlePoolReq) (*BundlePool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	now := time.Now()
	status := req.Status
	if status == "" {
		status = BundlePoolStatusActive
	}
	pool := &BundlePool{
		ID:           uuid.New().String(),
		InventoryID:  req.InventoryID,
		Code:         strings.TrimSpace(req.Code),
		Name:         strings.TrimSpace(req.Name),
		Description:  req.Description,
		Status:       status,
		MinItemCount: req.MinItemCount,
		MinAmount:    req.MinAmount,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO `+tableBundlePools+`
		(id, inventory_id, code, name, description, status, min_item_count, min_amount, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		pool.ID, pool.InventoryID, pool.Code, pool.Name, pool.Description, pool.Status,
		pool.MinItemCount, pool.MinAmount, pool.CreatedAt, pool.UpdatedAt); err != nil {
		return nil, err
	}
	if err := replaceBundlePoolItemsTx(ctx, tx, pool.ID, req.Items); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetBundlePool(ctx, pool.ID)
}

// UpdateBundlePool 更新捆物池及成员
func (r *Repository) UpdateBundlePool(ctx context.Context, req *UpdateBundlePoolReq) (*BundlePool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	pool, err := getBundlePoolForUpdateTx(ctx, tx, req.ID)
	if err != nil {
		return nil, err
	}
	if req.Code != nil {
		pool.Code = strings.TrimSpace(*req.Code)
	}
	if req.Name != nil {
		pool.Name = strings.TrimSpace(*req.Name)
	}
	if req.Description != nil {
		pool.Description = req.Description
	}
	if req.Status != nil {
		pool.Status = *req.Status
	}
	if req.MinItemCount != nil {
		pool.MinItemCount = *req.MinItemCount
	}
	if req.MinAmount != nil {
		pool.MinAmount = *req.MinAmount
	}
	pool.UpdatedAt = time.Now()

	if _, err := tx.ExecContext(ctx, `UPDATE `+tableBundlePools+`
		SET code = ?, name = ?, description = ?, status = ?, min_item_count = ?, min_amount = ?, updated_at = ?
		WHERE id = ?`,
		pool.Code, pool.Name, pool.Description, pool.Status, pool.MinItemCount, pool.MinAmount,
		pool.UpdatedAt, pool.ID); err != nil {
		return nil, err
	}
	if req.Items != nil {
		if err := replaceBundlePoolItemsTx(ctx, tx, pool.ID, *req.Items); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetBundlePool(ctx, pool.ID)
}

// DeleteBundlePool 删除捆物池
func (r *Repository) DeleteBundlePool(ctx context.Context, id string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var refs int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+tableBundles+` WHERE pool_id = ?`, id).Scan(&refs); err != nil {
		return err
	}
	if refs > 0 {
		return NewGoodserError(ErrBundlePoolInUse)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM `+tableBundlePoolItems+` WHERE pool_id = ?`, id); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM `+tableBundlePools+` WHERE id = ?`, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return NewGoodserError(ErrBundlePoolNotFound)
	}
	return tx.Commit()
}

func getBundlePoolForUpdateTx(ctx context.Context, tx *sql.Tx, id string) (*BundlePool, error) {
	row := tx.QueryRowContext(ctx, `SELECT id, inventory_id, code, name, description, status, min_item_count, min_amount,
		created_at, updated_at FROM `+tableBundlePools+` WHERE id = ? FOR UPDATE`, id)
	item, err := scanBundlePool(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, NewGoodserError(ErrBundlePoolNotFound)
		}
		return nil, err
	}
	return &item, nil
}

func replaceBundlePoolItemsTx(ctx context.Context, tx *sql.Tx, poolID string, items []BundlePoolItemReq) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM `+tableBundlePoolItems+` WHERE pool_id = ?`, poolID); err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(items))
	for i, item := range items {
		productID := strings.TrimSpace(item.ProductID)
		if productID == "" {
			continue
		}
		if _, exists := seen[productID]; exists {
			continue
		}
		seen[productID] = struct{}{}
		qty := item.DefaultQuantity
		if qty <= 0 {
			qty = 1
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO `+tableBundlePoolItems+` (pool_id, product_id, default_quantity, sort_order)
			VALUES (?, ?, ?, ?)`, poolID, productID, qty, i); err != nil {
			return err
		}
	}
	return nil
}
