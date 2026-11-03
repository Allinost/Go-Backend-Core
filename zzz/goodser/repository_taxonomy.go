package goodser

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
)

// --- Dictionaries ---

func (r *Repository) ListDictionaries(ctx context.Context, kind string, enabled *bool) ([]Dictionary, error) {
	query := `SELECT id, kind, code, name, parent_id, sort_order, enabled, extra, created_at, updated_at
		FROM ` + tableDictionaries
	var conditions []string
	var args []any
	if kind != "" {
		conditions = append(conditions, "kind = ?")
		args = append(args, kind)
	}
	if enabled != nil {
		conditions = append(conditions, "enabled = ?")
		args = append(args, *enabled)
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY kind, sort_order, name"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Dictionary, 0)
	for rows.Next() {
		item, err := scanDictionary(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) GetDictionary(ctx context.Context, id string) (*Dictionary, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, kind, code, name, parent_id, sort_order, enabled, extra, created_at, updated_at
		FROM `+tableDictionaries+` WHERE id = ?`, id)
	item, err := scanDictionary(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, NewGoodserError(ErrDictionaryNotFound)
		}
		return nil, err
	}
	return &item, nil
}

func (r *Repository) CreateDictionary(ctx context.Context, req *CreateDictionaryReq) (*Dictionary, error) {
	now := time.Now()
	code := strings.TrimSpace(req.Code)
	if code == "" {
		code = strings.ReplaceAll(uuid.New().String(), "-", "")[:12]
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	item := &Dictionary{
		ID:        uuid.New().String(),
		Kind:      req.Kind,
		Code:      code,
		Name:      strings.TrimSpace(req.Name),
		ParentID:  normalizeOptionalID(req.ParentID),
		SortOrder: req.SortOrder,
		Enabled:   enabled,
		Extra:     req.Extra,
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO `+tableDictionaries+`
		(id, kind, code, name, parent_id, sort_order, enabled, extra, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.ID, item.Kind, item.Code, item.Name, item.ParentID, item.SortOrder,
		item.Enabled, item.Extra, item.CreatedAt, item.UpdatedAt)
	if err != nil {
		if isDuplicateKey(err) {
			return nil, NewGoodserError(ErrDuplicateCode)
		}
		return nil, err
	}
	return item, nil
}

func (r *Repository) UpdateDictionary(ctx context.Context, req *UpdateDictionaryReq) (*Dictionary, error) {
	item, err := r.GetDictionary(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	if req.Code != nil {
		item.Code = strings.TrimSpace(*req.Code)
	}
	if req.Name != nil {
		item.Name = strings.TrimSpace(*req.Name)
	}
	if req.ParentID != nil {
		item.ParentID = normalizeOptionalID(req.ParentID)
	}
	if req.SortOrder != nil {
		item.SortOrder = *req.SortOrder
	}
	if req.Enabled != nil {
		item.Enabled = *req.Enabled
	}
	if len(req.Extra) > 0 {
		item.Extra = req.Extra
	}
	item.UpdatedAt = time.Now()
	_, err = r.db.ExecContext(ctx, `UPDATE `+tableDictionaries+`
		SET code = ?, name = ?, parent_id = ?, sort_order = ?, enabled = ?, extra = ?, updated_at = ?
		WHERE id = ?`,
		item.Code, item.Name, item.ParentID, item.SortOrder, item.Enabled, item.Extra, item.UpdatedAt, item.ID)
	if err != nil {
		if isDuplicateKey(err) {
			return nil, NewGoodserError(ErrDuplicateCode)
		}
		return nil, err
	}
	return item, nil
}

func (r *Repository) DeleteDictionary(ctx context.Context, id string) error {
	item, err := r.GetDictionary(ctx, id)
	if err != nil {
		return err
	}
	refs, err := r.CountDictionaryRefs(ctx, item)
	if err != nil {
		return err
	}
	if refs > 0 {
		return NewGoodserError(ErrDictionaryInUse)
	}
	_, err = r.db.ExecContext(ctx, `DELETE FROM `+tableDictionaries+` WHERE id = ?`, id)
	return err
}

// CountDictionaryRefs 统计字典项被商品、角色关联、子字典或捆筛选条件引用的次数
func (r *Repository) CountDictionaryRefs(ctx context.Context, item *Dictionary) (int, error) {
	var count int
	switch DictionaryKind(item.Kind) {
	case DictionaryKindSeries:
		if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+tableProducts+` WHERE series_id = ?`, item.ID).Scan(&count); err != nil {
			return 0, err
		}
		var children int
		if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+tableDictionaries+` WHERE parent_id = ?`, item.ID).Scan(&children); err != nil {
			return 0, err
		}
		bundleRefs, err := r.countBundleFilterRefs(ctx, "series_id", item.ID)
		if err != nil {
			return 0, err
		}
		return count + children + bundleRefs, nil
	case DictionaryKindSetType:
		if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+tableProducts+` WHERE set_type_id = ?`, item.ID).Scan(&count); err != nil {
			return 0, err
		}
		bundleRefs, err := r.countBundleFilterRefs(ctx, "set_type_id", item.ID)
		if err != nil {
			return 0, err
		}
		return count + bundleRefs, nil
	case DictionaryKindProductType:
		if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+tableProducts+` WHERE product_type_id = ?`, item.ID).Scan(&count); err != nil {
			return 0, err
		}
		bundleRefs, err := r.countBundleFilterRefs(ctx, "product_type_id", item.ID)
		if err != nil {
			return 0, err
		}
		return count + bundleRefs, nil
	case DictionaryKindCharacter:
		if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+tableProductCharacters+` WHERE character_id = ?`, item.ID).Scan(&count); err != nil {
			return 0, err
		}
		var children int
		if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+tableDictionaries+` WHERE parent_id = ?`, item.ID).Scan(&children); err != nil {
			return 0, err
		}
		bundleRefs, err := r.countBundleFilterRefs(ctx, "character_id", item.ID)
		if err != nil {
			return 0, err
		}
		return count + children + bundleRefs, nil
	default:
		return 0, nil
	}
}

// countBundleFilterRefs 统计捆模板的主商品筛选条件对某个字典项的引用次数
func (r *Repository) countBundleFilterRefs(ctx context.Context, filterKey, id string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM `+tableBundles+` WHERE JSON_UNQUOTE(JSON_EXTRACT(main_filter, ?)) = ?`,
		"$."+filterKey, id).Scan(&count)
	return count, err
}

// GetDictionaryNames 批量读取字典名称，返回 id -> name 映射
func (r *Repository) GetDictionaryNames(ctx context.Context, ids []string) (map[string]string, error) {
	result := make(map[string]string, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	placeholders := make([]string, 0, len(ids))
	args := make([]any, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}
	if len(placeholders) == 0 {
		return result, nil
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, name FROM `+tableDictionaries+`
		WHERE id IN (`+strings.Join(placeholders, ",")+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		result[id] = name
	}
	return result, rows.Err()
}

// FindDictionaryByName 按类型与名称查找启用中的字典项，供客户按名称筛选
func (r *Repository) FindDictionaryByName(ctx context.Context, kind, name string) (*Dictionary, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, kind, code, name, parent_id, sort_order, enabled, extra, created_at, updated_at
		FROM `+tableDictionaries+` WHERE kind = ? AND name = ? AND enabled = 1 LIMIT 1`, kind, strings.TrimSpace(name))
	item, err := scanDictionary(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, NewGoodserError(ErrDictionaryNotFound)
		}
		return nil, err
	}
	return &item, nil
}

func scanDictionary(scanner interface{ Scan(...any) error }) (Dictionary, error) {
	var (
		item    Dictionary
		extra   sql.NullString
		enabled bool
	)
	if err := scanner.Scan(&item.ID, &item.Kind, &item.Code, &item.Name, &item.ParentID,
		&item.SortOrder, &enabled, &extra, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return Dictionary{}, err
	}
	item.Enabled = enabled
	if extra.Valid {
		item.Extra = json.RawMessage(extra.String)
	}
	return item, nil
}

// --- Product characters ---

func (r *Repository) ListProductCharacterIDs(ctx context.Context, productIDs []string) (map[string][]string, error) {
	result := make(map[string][]string, len(productIDs))
	if len(productIDs) == 0 {
		return result, nil
	}
	placeholders := make([]string, len(productIDs))
	args := make([]any, len(productIDs))
	for i, id := range productIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	rows, err := r.db.QueryContext(ctx, `SELECT product_id, character_id FROM `+tableProductCharacters+
		` WHERE product_id IN (`+strings.Join(placeholders, ",")+`) ORDER BY character_id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var productID, characterID string
		if err := rows.Scan(&productID, &characterID); err != nil {
			return nil, err
		}
		result[productID] = append(result[productID], characterID)
	}
	return result, rows.Err()
}

func (r *Repository) ReplaceProductCharactersTx(ctx context.Context, tx *sql.Tx, productID string, characterIDs []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM `+tableProductCharacters+` WHERE product_id = ?`, productID); err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(characterIDs))
	for _, characterID := range characterIDs {
		characterID = strings.TrimSpace(characterID)
		if characterID == "" {
			continue
		}
		if _, exists := seen[characterID]; exists {
			continue
		}
		seen[characterID] = struct{}{}
		if _, err := tx.ExecContext(ctx, `INSERT INTO `+tableProductCharacters+` (product_id, character_id) VALUES (?, ?)`,
			productID, characterID); err != nil {
			return err
		}
	}
	return nil
}

// --- Series view ---

func (r *Repository) ListSeriesSummary(ctx context.Context, inventoryID string) ([]SeriesSummary, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT d.id, d.name, COUNT(p.id),
		       COALESCE(SUM(p.quantity + p.reserved_quantity), 0),
		       COALESCE(SUM(p.quantity), 0)
		FROM `+tableDictionaries+` d
		LEFT JOIN `+tableProducts+` p ON p.series_id = d.id AND p.inventory_id = ?
		WHERE d.kind = ? AND d.enabled = 1
		GROUP BY d.id, d.name, d.sort_order
		ORDER BY d.sort_order, d.name`, inventoryID, string(DictionaryKindSeries))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]SeriesSummary, 0)
	for rows.Next() {
		var item SeriesSummary
		if err := rows.Scan(&item.SeriesID, &item.SeriesName, &item.ProductCount, &item.TotalQuantity, &item.Available); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// --- Combos ---

func (r *Repository) ListCombos(ctx context.Context, inventoryID, status string) ([]Combo, error) {
	query := `SELECT c.id, c.inventory_id, c.code, c.name, c.description, c.image_url, c.price, c.status,
		c.created_at, c.updated_at,
		ci.item_type, ci.product_id, p.name, p.code, p.image_url, ci.quantity, p.quantity,
		ci.bundle_id, b.name
		FROM ` + tableCombos + ` c
		LEFT JOIN ` + tableComboItems + ` ci ON ci.combo_id = c.id
		LEFT JOIN ` + tableProducts + ` p ON p.id = ci.product_id
		LEFT JOIN ` + tableBundles + ` b ON b.id = ci.bundle_id
		WHERE c.inventory_id = ?`
	args := []any{inventoryID}
	if status != "" {
		query += " AND c.status = ?"
		args = append(args, status)
	}
	query += " ORDER BY c.created_at DESC, ci.sort_order, ci.id"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanComboRows(rows)
}

func (r *Repository) GetCombo(ctx context.Context, id string) (*Combo, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, inventory_id, code, name, description, image_url, price, status,
		created_at, updated_at
		FROM `+tableCombos+` WHERE id = ?`, id)
	var combo Combo
	if err := row.Scan(&combo.ID, &combo.InventoryID, &combo.Code, &combo.Name, &combo.Description,
		&combo.ImageURL, &combo.Price, &combo.Status, &combo.CreatedAt, &combo.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, NewGoodserError(ErrComboNotFound)
		}
		return nil, err
	}
	items, err := r.loadComboItems(ctx, id)
	if err != nil {
		return nil, err
	}
	combo.Items = items
	return &combo, nil
}

func (r *Repository) loadComboItems(ctx context.Context, comboID string) ([]ComboItem, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT ci.item_type, ci.product_id, p.name, p.code, p.image_url,
		ci.quantity, p.quantity, ci.bundle_id, b.name
		FROM `+tableComboItems+` ci
		LEFT JOIN `+tableProducts+` p ON p.id = ci.product_id
		LEFT JOIN `+tableBundles+` b ON b.id = ci.bundle_id
		WHERE ci.combo_id = ?
		ORDER BY ci.sort_order, ci.id`, comboID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]ComboItem, 0)
	for rows.Next() {
		var (
			item         ComboItem
			itemType     sql.NullString
			productID    sql.NullString
			productName  sql.NullString
			productCode  sql.NullString
			productImage sql.NullString
			available    sql.NullInt64
			bundleID     sql.NullString
			bundleName   sql.NullString
		)
		if err := rows.Scan(&itemType, &productID, &productName, &productCode, &productImage,
			&item.Quantity, &available, &bundleID, &bundleName); err != nil {
			return nil, err
		}
		item.ItemType = itemType.String
		if item.ItemType == "" {
			item.ItemType = ComboItemTypeProduct
		}
		item.ProductID = productID.String
		item.ProductName = productName.String
		item.ProductCode = productCode.String
		if productImage.Valid {
			value := productImage.String
			item.ImageURL = &value
		}
		item.Available = int(available.Int64)
		item.BundleID = bundleID.String
		item.BundleName = bundleName.String
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) CreateCombo(ctx context.Context, req *CreateComboReq) (*Combo, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	now := time.Now()
	status := req.Status
	if status == "" {
		status = ComboStatusActive
	}
	combo := &Combo{
		ID:          uuid.New().String(),
		InventoryID: req.InventoryID,
		Code:        req.Code,
		Name:        req.Name,
		Description: req.Description,
		ImageURL:    req.ImageURL,
		Price:       req.Price,
		Status:      status,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO `+tableCombos+`
		(id, inventory_id, code, name, description, image_url, price, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		combo.ID, combo.InventoryID, combo.Code, combo.Name, combo.Description,
		combo.ImageURL, combo.Price, combo.Status, combo.CreatedAt, combo.UpdatedAt); err != nil {
		return nil, err
	}
	if err := replaceComboItemsTx(ctx, tx, combo.ID, req.Items); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetCombo(ctx, combo.ID)
}

func (r *Repository) UpdateCombo(ctx context.Context, req *UpdateComboReq) (*Combo, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	combo, err := r.getComboForUpdateTx(ctx, tx, req.ID)
	if err != nil {
		return nil, err
	}
	if req.Code != nil {
		combo.Code = *req.Code
	}
	if req.Name != nil {
		combo.Name = *req.Name
	}
	if req.Description != nil {
		combo.Description = req.Description
	}
	if req.ImageURL != nil {
		combo.ImageURL = req.ImageURL
	}
	if req.Price != nil {
		combo.Price = *req.Price
	}
	if req.Status != nil {
		combo.Status = *req.Status
	}
	combo.UpdatedAt = time.Now()
	if _, err := tx.ExecContext(ctx, `UPDATE `+tableCombos+`
		SET code = ?, name = ?, description = ?, image_url = ?, price = ?, status = ?, updated_at = ?
		WHERE id = ?`,
		combo.Code, combo.Name, combo.Description, combo.ImageURL, combo.Price, combo.Status,
		combo.UpdatedAt, combo.ID); err != nil {
		return nil, err
	}
	if req.Items != nil {
		if err := replaceComboItemsTx(ctx, tx, combo.ID, *req.Items); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetCombo(ctx, combo.ID)
}

func (r *Repository) DeleteCombo(ctx context.Context, id string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM `+tableComboItems+` WHERE combo_id = ?`, id); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM `+tableCombos+` WHERE id = ?`, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return NewGoodserError(ErrComboNotFound)
	}
	return tx.Commit()
}

func (r *Repository) getComboForUpdateTx(ctx context.Context, tx *sql.Tx, id string) (*Combo, error) {
	row := tx.QueryRowContext(ctx, `SELECT id, inventory_id, code, name, description, image_url, price, status,
		created_at, updated_at FROM `+tableCombos+` WHERE id = ? FOR UPDATE`, id)
	var combo Combo
	if err := row.Scan(&combo.ID, &combo.InventoryID, &combo.Code, &combo.Name, &combo.Description,
		&combo.ImageURL, &combo.Price, &combo.Status, &combo.CreatedAt, &combo.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, NewGoodserError(ErrComboNotFound)
		}
		return nil, err
	}
	return &combo, nil
}

func replaceComboItemsTx(ctx context.Context, tx *sql.Tx, comboID string, items []ComboItemReq) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM `+tableComboItems+` WHERE combo_id = ?`, comboID); err != nil {
		return err
	}
	for i, item := range items {
		itemType := strings.TrimSpace(item.ItemType)
		if itemType == "" {
			itemType = ComboItemTypeProduct
		}
		var productID, bundleID any
		if itemType == ComboItemTypeBundle {
			bundleID = strings.TrimSpace(item.BundleID)
			productID = nil
		} else {
			productID = strings.TrimSpace(item.ProductID)
			bundleID = nil
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO `+tableComboItems+`
			(combo_id, item_type, product_id, bundle_id, quantity, sort_order)
			VALUES (?, ?, ?, ?, ?, ?)`, comboID, itemType, productID, bundleID, item.Quantity, i); err != nil {
			return err
		}
	}
	return nil
}

func scanComboRows(rows *sql.Rows) ([]Combo, error) {
	byID := make(map[string]*Combo)
	order := make([]string, 0)
	for rows.Next() {
		var (
			combo                               Combo
			itemType                            sql.NullString
			productID, productName, productCode sql.NullString
			productImage                        sql.NullString
			itemQty, available                  sql.NullInt64
			bundleID, bundleName                sql.NullString
		)
		if err := rows.Scan(&combo.ID, &combo.InventoryID, &combo.Code, &combo.Name, &combo.Description,
			&combo.ImageURL, &combo.Price, &combo.Status, &combo.CreatedAt, &combo.UpdatedAt,
			&itemType, &productID, &productName, &productCode, &productImage, &itemQty, &available,
			&bundleID, &bundleName); err != nil {
			return nil, err
		}
		existing, ok := byID[combo.ID]
		if !ok {
			clone := combo
			clone.Items = make([]ComboItem, 0)
			byID[combo.ID] = &clone
			order = append(order, combo.ID)
			existing = &clone
		}
		if !itemType.Valid && !productID.Valid && !bundleID.Valid {
			continue
		}
		item := ComboItem{
			ItemType:  itemType.String,
			ProductID: productID.String,
			BundleID:  bundleID.String,
			Quantity:  int(itemQty.Int64),
			Available: int(available.Int64),
		}
		if item.ItemType == "" {
			item.ItemType = ComboItemTypeProduct
		}
		if item.ItemType == ComboItemTypeBundle {
			item.BundleName = bundleName.String
			item.ProductName = bundleName.String
			item.ProductCode = ""
		} else {
			item.ProductName = productName.String
			item.ProductCode = productCode.String
			if productImage.Valid {
				value := productImage.String
				item.ImageURL = &value
			}
		}
		existing.Items = append(existing.Items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result := make([]Combo, 0, len(order))
	for _, id := range order {
		result = append(result, *byID[id])
	}
	return result, nil
}

func isDuplicateKey(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "duplicate") || strings.Contains(message, "1062")
}

// normalizeOptionalID 把空字符串指针转为 nil，避免写入无意义的空外键
func normalizeOptionalID(id *string) *string {
	if id == nil {
		return nil
	}
	value := strings.TrimSpace(*id)
	if value == "" {
		return nil
	}
	return &value
}
