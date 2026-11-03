package goodser

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	tableBundles            = "zzz_goodser_bundles"
	tableBundleMainProducts = "zzz_goodser_bundle_main_products"
	tableBundleSubProducts  = "zzz_goodser_bundle_sub_products"
)

// ListBundles 查询捆模板基础信息
func (r *Repository) ListBundles(ctx context.Context, inventoryID, status string) ([]Bundle, error) {
	query := `SELECT id, inventory_id, code, name, description, status, main_filter, pool_id,
		min_main_quantity, min_main_amount, min_sub_quantity, max_sub_quantity, min_sub_amount,
		price_mode, fixed_price, created_at, updated_at
		FROM ` + tableBundles + ` WHERE inventory_id = ?`
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
	items := make([]Bundle, 0)
	for rows.Next() {
		item, err := scanBundle(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// GetBundle 查询捆模板基础信息
func (r *Repository) GetBundle(ctx context.Context, id string) (*Bundle, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, inventory_id, code, name, description, status, main_filter, pool_id,
		min_main_quantity, min_main_amount, min_sub_quantity, max_sub_quantity, min_sub_amount,
		price_mode, fixed_price, created_at, updated_at
		FROM `+tableBundles+` WHERE id = ?`, id)
	item, err := scanBundle(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, NewGoodserError(ErrBundleNotFound)
		}
		return nil, err
	}
	return &item, nil
}

func (r *Repository) getBundleForUpdateTx(ctx context.Context, tx *sql.Tx, id string) (*Bundle, error) {
	row := tx.QueryRowContext(ctx, `SELECT id, inventory_id, code, name, description, status, main_filter, pool_id,
		min_main_quantity, min_main_amount, min_sub_quantity, max_sub_quantity, min_sub_amount,
		price_mode, fixed_price, created_at, updated_at
		FROM `+tableBundles+` WHERE id = ? FOR UPDATE`, id)
	item, err := scanBundle(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, NewGoodserError(ErrBundleNotFound)
		}
		return nil, err
	}
	return &item, nil
}

func scanBundle(scanner interface{ Scan(...any) error }) (Bundle, error) {
	var (
		item        Bundle
		description sql.NullString
		filterRaw   sql.NullString
		poolID      sql.NullString
		status      string
	)
	if err := scanner.Scan(&item.ID, &item.InventoryID, &item.Code, &item.Name, &description, &status, &filterRaw, &poolID,
		&item.MinMainQuantity, &item.MinMainAmount, &item.MinSubQuantity, &item.MaxSubQuantity, &item.MinSubAmount,
		&item.PriceMode, &item.FixedPrice, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return Bundle{}, err
	}
	if poolID.Valid && strings.TrimSpace(poolID.String) != "" {
		item.PoolID = &poolID.String
	}
	if description.Valid {
		item.Description = &description.String
	}
	item.Status = status
	if filterRaw.Valid && strings.TrimSpace(filterRaw.String) != "" {
		if err := json.Unmarshal([]byte(filterRaw.String), &item.MainFilter); err != nil {
			return Bundle{}, err
		}
	}
	item.MainFilter.Normalize()
	return item, nil
}

// LoadBundleMainProducts 查询显式主商品清单，不做筛选
func (r *Repository) LoadBundleMainProducts(ctx context.Context, bundleID string) ([]BundleMainProduct, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT p.id, p.name, p.code, p.image_url, p.expected_price,
		p.quantity, p.reserved_quantity, p.series_id, p.set_type_id, p.product_type_id
		FROM `+tableBundleMainProducts+` bm
		JOIN `+tableProducts+` p ON p.id = bm.product_id
		WHERE bm.bundle_id = ?
		ORDER BY bm.sort_order, bm.id`, bundleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanBundleMainProducts(rows)
}

// LoadBundleSubProducts 查询捆配置的副区商品，可按关键词和库存过滤
func (r *Repository) LoadBundleSubProducts(ctx context.Context, bundleID, keyword string, onlyAvailable bool) ([]BundleSubProduct, error) {
	query := `SELECT sp.product_id, p.name, p.code, p.image_url, sp.default_quantity,
		p.expected_price, p.quantity, p.reserved_quantity
		FROM ` + tableBundleSubProducts + ` sp
		JOIN ` + tableProducts + ` p ON p.id = sp.product_id
		WHERE sp.bundle_id = ?`
	args := []any{bundleID}
	if kw := strings.TrimSpace(keyword); kw != "" {
		query += " AND (p.name LIKE ? OR p.code LIKE ?)"
		like := "%" + kw + "%"
		args = append(args, like, like)
	}
	if onlyAvailable {
		query += " AND p.quantity > 0"
	}
	query += " ORDER BY sp.sort_order, sp.id"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]BundleSubProduct, 0)
	for rows.Next() {
		var item BundleSubProduct
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

// ListBundleMainCandidates 按主商品筛选条件查询候选主商品
func (r *Repository) ListBundleMainCandidates(ctx context.Context, inventoryID string, filter BundleMainFilter, keyword string, limit int) ([]BundleMainProduct, error) {
	query := `SELECT p.id, p.name, p.code, p.image_url, p.expected_price,
		p.quantity, p.reserved_quantity, p.series_id, p.set_type_id, p.product_type_id
		FROM ` + tableProducts + ` p WHERE p.inventory_id = ?`
	args := []any{inventoryID}
	if filter.SeriesID != "" {
		query += " AND p.series_id = ?"
		args = append(args, filter.SeriesID)
	}
	if filter.SetTypeID != "" {
		query += " AND p.set_type_id = ?"
		args = append(args, filter.SetTypeID)
	}
	if filter.ProductTypeID != "" {
		query += " AND p.product_type_id = ?"
		args = append(args, filter.ProductTypeID)
	}
	if filter.StatusCode != "" {
		query += " AND p.status_code = ?"
		args = append(args, filter.StatusCode)
	}
	if filter.CharacterID != "" {
		query += " AND EXISTS (SELECT 1 FROM " + tableProductCharacters + ` pc
			WHERE pc.product_id = p.id AND pc.character_id = ?)`
		args = append(args, filter.CharacterID)
	}
	kw := strings.TrimSpace(filter.Keyword)
	if kw == "" {
		kw = strings.TrimSpace(keyword)
	}
	if kw != "" {
		query += " AND (p.name LIKE ? OR p.code LIKE ?)"
		like := "%" + kw + "%"
		args = append(args, like, like)
	}
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	query += " ORDER BY p.created_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanBundleMainProducts(rows)
}

// ResolveBundleMainProducts 返回捆最终允许的主商品。
// 显式清单与筛选条件同时存在时按 AND 处理，关键词可覆盖筛选条件中的关键词。
func (r *Repository) ResolveBundleMainProducts(ctx context.Context, bundle *Bundle, keyword string, limit int) ([]BundleMainProduct, error) {
	explicit, err := r.LoadBundleMainProducts(ctx, bundle.ID)
	if err != nil {
		return nil, err
	}
	filter := bundle.MainFilter
	if strings.TrimSpace(keyword) != "" {
		filter.Keyword = strings.TrimSpace(keyword)
	}

	if len(explicit) == 0 {
		if filter.IsZero() {
			return []BundleMainProduct{}, nil
		}
		return r.ListBundleMainCandidates(ctx, bundle.InventoryID, filter, "", limit)
	}

	if filter.IsZero() {
		return explicit, nil
	}
	products := make([]Product, 0, len(explicit))
	byID := make(map[string]*BundleMainProduct, len(explicit))
	for i := range explicit {
		byID[explicit[i].ProductID] = &explicit[i]
		products = append(products, Product{
			ID:            explicit[i].ProductID,
			Name:          explicit[i].ProductName,
			Code:          explicit[i].ProductCode,
			SeriesID:      explicit[i].SeriesID,
			SetTypeID:     explicit[i].SetTypeID,
			ProductTypeID: explicit[i].ProductTypeID,
		})
	}
	ids := make([]string, 0, len(products))
	for _, product := range products {
		ids = append(ids, product.ID)
	}
	characterMap, err := r.ListProductCharacterIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]BundleMainProduct, 0, len(explicit))
	for _, product := range products {
		if !bundleMainFilterMatches(filter, &product, characterMap[product.ID]) {
			continue
		}
		if item := byID[product.ID]; item != nil {
			item.CharacterIDs = characterMap[product.ID]
			result = append(result, *item)
		}
	}
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func scanBundleMainProducts(rows *sql.Rows) ([]BundleMainProduct, error) {
	items := make([]BundleMainProduct, 0)
	for rows.Next() {
		var item BundleMainProduct
		if err := rows.Scan(&item.ProductID, &item.ProductName, &item.ProductCode, &item.ImageURL,
			&item.ExpectedPrice, &item.Available, &item.ReservedQty,
			&item.SeriesID, &item.SetTypeID, &item.ProductTypeID); err != nil {
			return nil, err
		}
		item.TotalQuantity = item.Available + item.ReservedQty
		item.ImageURL = normalizeImageURL(item.ImageURL)
		items = append(items, item)
	}
	return items, rows.Err()
}

// CreateBundle 创建捆模板及主/副商品配置
func (r *Repository) CreateBundle(ctx context.Context, req *CreateBundleReq) (*Bundle, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	now := time.Now()
	status := req.Status
	if status == "" {
		status = BundleStatusActive
	}
	priceMode := req.PriceMode
	if priceMode == "" {
		priceMode = BundlePriceModeSum
	}
	filterJSON := marshalBundleFilter(req.MainFilter)
	bundle := &Bundle{
		ID:              uuid.New().String(),
		InventoryID:     req.InventoryID,
		Code:            strings.TrimSpace(req.Code),
		Name:            strings.TrimSpace(req.Name),
		Description:     req.Description,
		Status:          status,
		PoolID:          normalizeOptionalID(req.PoolID),
		MinMainQuantity: req.MinMainQuantity,
		MinMainAmount:   req.MinMainAmount,
		MinSubQuantity:  req.MinSubQuantity,
		MaxSubQuantity:  req.MaxSubQuantity,
		MinSubAmount:    req.MinSubAmount,
		PriceMode:       priceMode,
		FixedPrice:      req.FixedPrice,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if req.MainFilter != nil {
		bundle.MainFilter = *req.MainFilter
		bundle.MainFilter.Normalize()
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO `+tableBundles+`
		(id, inventory_id, code, name, description, status, main_filter, pool_id,
		 min_main_quantity, min_main_amount, min_sub_quantity, max_sub_quantity, min_sub_amount,
		 price_mode, fixed_price, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		bundle.ID, bundle.InventoryID, bundle.Code, bundle.Name, bundle.Description, bundle.Status, filterJSON, bundle.PoolID,
		bundle.MinMainQuantity, bundle.MinMainAmount, bundle.MinSubQuantity, bundle.MaxSubQuantity, bundle.MinSubAmount,
		bundle.PriceMode, bundle.FixedPrice, bundle.CreatedAt, bundle.UpdatedAt); err != nil {
		return nil, err
	}
	if err := replaceBundleMainProductsTx(ctx, tx, bundle.ID, req.MainProductIDs); err != nil {
		return nil, err
	}
	if err := replaceBundleSubProductsTx(ctx, tx, bundle.ID, req.SubProducts); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetBundle(ctx, bundle.ID)
}

// UpdateBundle 更新捆模板及主/副商品配置
func (r *Repository) UpdateBundle(ctx context.Context, req *UpdateBundleReq) (*Bundle, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	bundle, err := r.getBundleForUpdateTx(ctx, tx, req.ID)
	if err != nil {
		return nil, err
	}
	if req.Code != nil {
		bundle.Code = strings.TrimSpace(*req.Code)
	}
	if req.Name != nil {
		bundle.Name = strings.TrimSpace(*req.Name)
	}
	if req.Description != nil {
		bundle.Description = req.Description
	}
	if req.Status != nil {
		bundle.Status = *req.Status
	}
	if req.MainFilter != nil {
		bundle.MainFilter = *req.MainFilter
		bundle.MainFilter.Normalize()
	}
	if req.MinMainQuantity != nil {
		bundle.MinMainQuantity = *req.MinMainQuantity
	}
	if req.MinMainAmount != nil {
		bundle.MinMainAmount = *req.MinMainAmount
	}
	if req.MinSubQuantity != nil {
		bundle.MinSubQuantity = *req.MinSubQuantity
	}
	if req.MaxSubQuantity != nil {
		bundle.MaxSubQuantity = *req.MaxSubQuantity
	}
	if req.MinSubAmount != nil {
		bundle.MinSubAmount = *req.MinSubAmount
	}
	if req.PriceMode != nil {
		bundle.PriceMode = *req.PriceMode
	}
	if req.FixedPrice != nil {
		bundle.FixedPrice = *req.FixedPrice
	}
	if req.PoolID != nil {
		bundle.PoolID = normalizeOptionalID(req.PoolID)
	}
	bundle.UpdatedAt = time.Now()

	filterJSON := marshalBundleFilter(&bundle.MainFilter)
	if _, err := tx.ExecContext(ctx, `UPDATE `+tableBundles+`
		SET code = ?, name = ?, description = ?, status = ?, main_filter = ?, pool_id = ?,
		    min_main_quantity = ?, min_main_amount = ?, min_sub_quantity = ?, max_sub_quantity = ?, min_sub_amount = ?,
		    price_mode = ?, fixed_price = ?, updated_at = ?
		WHERE id = ?`,
		bundle.Code, bundle.Name, bundle.Description, bundle.Status, filterJSON, bundle.PoolID,
		bundle.MinMainQuantity, bundle.MinMainAmount, bundle.MinSubQuantity, bundle.MaxSubQuantity, bundle.MinSubAmount,
		bundle.PriceMode, bundle.FixedPrice, bundle.UpdatedAt, bundle.ID); err != nil {
		return nil, err
	}
	if req.MainProductIDs != nil {
		if err := replaceBundleMainProductsTx(ctx, tx, bundle.ID, *req.MainProductIDs); err != nil {
			return nil, err
		}
	}
	if req.SubProducts != nil {
		if err := replaceBundleSubProductsTx(ctx, tx, bundle.ID, *req.SubProducts); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetBundle(ctx, bundle.ID)
}

// DeleteBundle 删除捆模板及其配置
func (r *Repository) DeleteBundle(ctx context.Context, id string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var comboRefs int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+tableComboItems+` WHERE item_type = 'bundle' AND bundle_id = ?`, id).Scan(&comboRefs); err != nil {
		return err
	}
	if comboRefs > 0 {
		return NewGoodserError(ErrBundleInUse)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM `+tableBundleMainProducts+` WHERE bundle_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM `+tableBundleSubProducts+` WHERE bundle_id = ?`, id); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM `+tableBundles+` WHERE id = ?`, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return NewGoodserError(ErrBundleNotFound)
	}
	return tx.Commit()
}

// CountProductBundleRefs 统计商品被捆主商品、副区商品或捆物池引用的次数
func (r *Repository) CountProductBundleRefs(ctx context.Context, productID string) (int, error) {
	var mainCount, subCount, poolCount int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+tableBundleMainProducts+` WHERE product_id = ?`, productID).Scan(&mainCount); err != nil {
		return 0, err
	}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+tableBundleSubProducts+` WHERE product_id = ?`, productID).Scan(&subCount); err != nil {
		return 0, err
	}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+tableBundlePoolItems+` WHERE product_id = ?`, productID).Scan(&poolCount); err != nil {
		return 0, err
	}
	return mainCount + subCount + poolCount, nil
}

// CountBundlesByProduct 供删除商品时给出更准确的提示
func (r *Repository) CountBundlesByProduct(ctx context.Context, productID string) (int, error) {
	return r.CountProductBundleRefs(ctx, productID)
}

func replaceBundleMainProductsTx(ctx context.Context, tx *sql.Tx, bundleID string, productIDs []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM `+tableBundleMainProducts+` WHERE bundle_id = ?`, bundleID); err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(productIDs))
	for i, productID := range productIDs {
		productID = strings.TrimSpace(productID)
		if productID == "" {
			continue
		}
		if _, exists := seen[productID]; exists {
			continue
		}
		seen[productID] = struct{}{}
		if _, err := tx.ExecContext(ctx, `INSERT INTO `+tableBundleMainProducts+` (bundle_id, product_id, sort_order)
			VALUES (?, ?, ?)`, bundleID, productID, i); err != nil {
			return err
		}
	}
	return nil
}

func replaceBundleSubProductsTx(ctx context.Context, tx *sql.Tx, bundleID string, items []BundleSubItemReq) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM `+tableBundleSubProducts+` WHERE bundle_id = ?`, bundleID); err != nil {
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
		if _, err := tx.ExecContext(ctx, `INSERT INTO `+tableBundleSubProducts+` (bundle_id, product_id, default_quantity, sort_order)
			VALUES (?, ?, ?, ?)`, bundleID, productID, qty, i); err != nil {
			return err
		}
	}
	return nil
}

func marshalBundleFilter(filter *BundleMainFilter) any {
	if filter == nil {
		return nil
	}
	normalized := *filter
	normalized.Normalize()
	if normalized.IsZero() {
		return nil
	}
	data, err := json.Marshal(normalized)
	if err != nil {
		return nil
	}
	return data
}

// bundleMainFilterMatches 在 Go 层校验主商品是否满足筛选条件
func bundleMainFilterMatches(filter BundleMainFilter, product *Product, characterIDs []string) bool {
	if product == nil {
		return false
	}
	if filter.SeriesID != "" && (product.SeriesID == nil || *product.SeriesID != filter.SeriesID) {
		return false
	}
	if filter.SetTypeID != "" && (product.SetTypeID == nil || *product.SetTypeID != filter.SetTypeID) {
		return false
	}
	if filter.ProductTypeID != "" && (product.ProductTypeID == nil || *product.ProductTypeID != filter.ProductTypeID) {
		return false
	}
	if filter.StatusCode != "" && product.StatusCode != filter.StatusCode {
		return false
	}
	if filter.CharacterID != "" {
		found := false
		for _, id := range characterIDs {
			if id == filter.CharacterID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if filter.Keyword != "" {
		kw := strings.ToLower(filter.Keyword)
		if !strings.Contains(strings.ToLower(product.Name), kw) && !strings.Contains(strings.ToLower(product.Code), kw) {
			return false
		}
	}
	return true
}
