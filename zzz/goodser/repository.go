package goodser

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	tableInventories    = "zzz_goodser_inventories"
	tableProducts       = "zzz_goodser_products"
	tableRecycledSeq    = "zzz_goodser_recycled_seq_numbers"
	tableSeqCounters    = "zzz_goodser_seq_counters"
	tableOutboundOrders = "zzz_goodser_outbound_orders"
	tableInboundLogs    = "zzz_goodser_inbound_logs"
	tableStatusCodes    = "zzz_goodser_status_codes"
	tableTags           = "zzz_goodser_tags"
	tableStockMovements = "zzz_goodser_stock_movements"
	tableReturns        = "zzz_goodser_returns"

	tableDictionaries      = "zzz_goodser_dictionaries"
	tableProductCharacters = "zzz_goodser_product_characters"
	tableCombos            = "zzz_goodser_combos"
	tableComboItems        = "zzz_goodser_combo_items"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// --- Inventories ---

func (r *Repository) ListInventories(ctx context.Context) ([]Inventory, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, sort_order, created_at, updated_at
		 FROM `+tableInventories+` ORDER BY sort_order, created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Inventory
	for rows.Next() {
		var item Inventory
		if err := rows.Scan(&item.ID, &item.Name, &item.SortOrder, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) GetInventory(ctx context.Context, id string) (*Inventory, error) {
	var item Inventory
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, sort_order, created_at, updated_at
		 FROM `+tableInventories+` WHERE id = ?`, id).
		Scan(&item.ID, &item.Name, &item.SortOrder, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) CreateInventory(ctx context.Context, req *CreateInventoryReq) (*Inventory, error) {
	now := time.Now()
	item := &Inventory{
		ID:        uuid.New().String(),
		Name:      req.Name,
		SortOrder: req.SortOrder,
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO `+tableInventories+` (id, name, sort_order, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)`,
		item.ID, item.Name, item.SortOrder, item.CreatedAt, item.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (r *Repository) UpdateInventory(ctx context.Context, req *UpdateInventoryReq) (*Inventory, error) {
	item, err := r.GetInventory(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	if req.Name != nil {
		item.Name = *req.Name
	}
	if req.SortOrder != nil {
		item.SortOrder = *req.SortOrder
	}
	item.UpdatedAt = time.Now()
	_, err = r.db.ExecContext(ctx,
		`UPDATE `+tableInventories+` SET name = ?, sort_order = ?, updated_at = ? WHERE id = ?`,
		item.Name, item.SortOrder, item.UpdatedAt, item.ID)
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (r *Repository) DeleteInventory(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM `+tableInventories+` WHERE id = ?`, id)
	return err
}

// --- Products ---

func (r *Repository) ListProducts(ctx context.Context, inventoryID string) ([]Product, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, inventory_id, code, main_zone, sub_zone, seq_number,
		        quantity, reserved_quantity, status_code, name, series_id, set_type_id, product_type_id,
		        original_price, market_price, expected_price, cost_price,
		        remark, storage_location, image_url, images, tags,
		        created_at, updated_at
		 FROM `+tableProducts+` WHERE inventory_id = ? ORDER BY seq_number`, inventoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProducts(rows)
}

func (r *Repository) ListProductsPaginated(ctx context.Context, inventoryID string, limit, offset int) ([]Product, bool, int, error) {
	var total int
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM `+tableProducts+` WHERE inventory_id = ?`, inventoryID).Scan(&total); err != nil {
		return nil, false, 0, err
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, inventory_id, code, main_zone, sub_zone, seq_number,
		        quantity, reserved_quantity, status_code, name, series_id, set_type_id, product_type_id,
		        original_price, market_price, expected_price, cost_price,
		        remark, storage_location, image_url, images, tags,
		        created_at, updated_at
		 FROM `+tableProducts+` WHERE inventory_id = ? ORDER BY seq_number LIMIT ? OFFSET ?`,
		inventoryID, limit+1, offset)
	if err != nil {
		return nil, false, 0, err
	}
	defer rows.Close()
	items, err := scanProducts(rows)
	if err != nil {
		return nil, false, 0, err
	}
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	return items, hasMore, total, nil
}

func (r *Repository) SearchProductsPaginated(ctx context.Context, req *QueryProductsReq) ([]Product, bool, int, error) {
	var conditions []string
	var args []interface{}

	conditions = append(conditions, "inventory_id = ?")
	args = append(args, req.InventoryID)

	if req.Keyword != "" {
		conditions = append(conditions, "(name LIKE ? OR code LIKE ? OR remark LIKE ?)")
		kw := "%" + req.Keyword + "%"
		args = append(args, kw, kw, kw)
	}
	if req.MainZone != "" {
		conditions = append(conditions, "main_zone = ?")
		args = append(args, req.MainZone)
	}
	if req.StatusCode != "" {
		conditions = append(conditions, "status_code = ?")
		args = append(args, req.StatusCode)
	}
	if req.TagID != "" {
		conditions = append(conditions, "JSON_CONTAINS(tags, ?)")
		tagJSON, _ := json.Marshal(req.TagID)
		args = append(args, string(tagJSON))
	}
	if req.SeriesID != "" {
		conditions = append(conditions, "series_id = ?")
		args = append(args, req.SeriesID)
	}
	if req.SetTypeID != "" {
		conditions = append(conditions, "set_type_id = ?")
		args = append(args, req.SetTypeID)
	}
	if req.ProductTypeID != "" {
		conditions = append(conditions, "product_type_id = ?")
		args = append(args, req.ProductTypeID)
	}
	if req.CharacterID != "" {
		conditions = append(conditions, "EXISTS (SELECT 1 FROM "+tableProductCharacters+" pc WHERE pc.product_id = "+tableProducts+".id AND pc.character_id = ?)")
		args = append(args, req.CharacterID)
	}
	if req.OnlyAvailable {
		conditions = append(conditions, "quantity > 0")
	}

	whereClause := strings.Join(conditions, " AND ")

	// Total count
	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", tableProducts, whereClause)
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, false, 0, err
	}

	// Sort — whitelist to prevent SQL injection
	sortColumns := map[string]string{
		"code":           "code",
		"name":           "name",
		"quantity":       "quantity",
		"expected_price": "expected_price",
	}
	orderBy := "seq_number ASC"
	if col, ok := sortColumns[req.SortBy]; ok {
		orderBy = col + " " + strings.ToUpper(req.SortOrder)
	}

	// Data query with limit+1 for has_more detection
	limit := req.PageSize
	offset := (req.Page - 1) * req.PageSize

	selectCols := `id, inventory_id, code, main_zone, sub_zone, seq_number,
		       quantity, reserved_quantity, status_code, name, series_id, set_type_id, product_type_id,
		       original_price, market_price, expected_price, cost_price,
		       remark, storage_location, image_url, images, tags,
		       created_at, updated_at`

	dataQuery := fmt.Sprintf("SELECT %s FROM %s WHERE %s ORDER BY %s LIMIT ? OFFSET ?",
		selectCols, tableProducts, whereClause, orderBy)

	dataArgs := append(args, limit+1, offset)
	rows, err := r.db.QueryContext(ctx, dataQuery, dataArgs...)
	if err != nil {
		return nil, false, 0, err
	}
	defer rows.Close()

	items, err := scanProducts(rows)
	if err != nil {
		return nil, false, 0, err
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}

	return items, hasMore, total, nil
}

func (r *Repository) GetProduct(ctx context.Context, id string) (*Product, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, inventory_id, code, main_zone, sub_zone, seq_number,
		        quantity, reserved_quantity, status_code, name, series_id, set_type_id, product_type_id,
		        original_price, market_price, expected_price, cost_price,
		        remark, storage_location, image_url, images, tags,
		        created_at, updated_at
		 FROM `+tableProducts+` WHERE id = ?`, id)
	return scanProduct(row)
}

func (r *Repository) FindProductByCode(ctx context.Context, inventoryID, code string) (*Product, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, inventory_id, code, main_zone, sub_zone, seq_number,
		        quantity, reserved_quantity, status_code, name, series_id, set_type_id, product_type_id,
		        original_price, market_price, expected_price, cost_price,
		        remark, storage_location, image_url, images, tags,
		        created_at, updated_at
		 FROM `+tableProducts+` WHERE inventory_id = ? AND code = ?`, inventoryID, code)
	return scanProduct(row)
}

func scanProducts(rows *sql.Rows) ([]Product, error) {
	var items []Product
	for rows.Next() {
		var item Product
		if err := scanProductRow(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanProduct(row *sql.Row) (*Product, error) {
	var item Product
	if err := scanProductRow(row, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

func scanProductRow(scanner interface {
	Scan(dest ...interface{}) error
}, p *Product) error {
	var images, tags sql.NullString
	if err := scanner.Scan(
		&p.ID, &p.InventoryID, &p.Code, &p.MainZone, &p.SubZone, &p.SeqNumber,
		&p.Quantity, &p.ReservedQuantity, &p.StatusCode, &p.Name,
		&p.SeriesID, &p.SetTypeID, &p.ProductTypeID,
		&p.OriginalPrice, &p.MarketPrice, &p.ExpectedPrice, &p.CostPrice,
		&p.Remark, &p.StorageLocation, &p.ImageURL, &images, &tags,
		&p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		return err
	}
	if images.Valid {
		p.Images = json.RawMessage(images.String)
	}
	if tags.Valid {
		p.Tags = json.RawMessage(tags.String)
	}
	p.ImageURL = normalizeImageURL(p.ImageURL)
	p.recalcTotal()
	return nil
}

// --- Seq Counters & Recycled ---

func (r *Repository) PeekRecycledSeq(ctx context.Context, tx *sql.Tx, inventoryID, mainZone, subZone string) (int, error) {
	var seq int
	err := tx.QueryRowContext(ctx,
		`SELECT seq_number FROM `+tableRecycledSeq+
			` WHERE inventory_id = ? AND main_zone = ? AND sub_zone = ?
			 ORDER BY seq_number LIMIT 1 FOR UPDATE`,
		inventoryID, mainZone, subZone).Scan(&seq)
	return seq, err
}

// ConsumeRecycledSeq 仅在序号真正被使用后调用，避免入库校验失败导致序号泄漏
func (r *Repository) ConsumeRecycledSeq(ctx context.Context, tx *sql.Tx, inventoryID, mainZone, subZone string, seq int) error {
	_, err := tx.ExecContext(ctx,
		`DELETE FROM `+tableRecycledSeq+` WHERE inventory_id = ? AND main_zone = ? AND sub_zone = ? AND seq_number = ?`,
		inventoryID, mainZone, subZone, seq)
	return err
}

func (r *Repository) IncrementSeqCounter(ctx context.Context, tx *sql.Tx, inventoryID, mainZone, subZone string) (int, error) {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO `+tableSeqCounters+` (id, inventory_id, main_zone, sub_zone, current_max)
		 VALUES (?, ?, ?, ?, 1)
		 ON DUPLICATE KEY UPDATE current_max = current_max + 1`,
		uuid.New().String(), inventoryID, mainZone, subZone)
	if err != nil {
		return 0, err
	}
	var seq int
	err = tx.QueryRowContext(ctx,
		`SELECT current_max FROM `+tableSeqCounters+
			` WHERE inventory_id = ? AND main_zone = ? AND sub_zone = ? FOR UPDATE`,
		inventoryID, mainZone, subZone).Scan(&seq)
	return seq, err
}

// EnsureSeqCounter 确保计数器不低于指定序号，用于 inboundSingle/inboundBatch 创建新商品后同步计数器
func (r *Repository) EnsureSeqCounter(ctx context.Context, tx *sql.Tx, inventoryID, mainZone, subZone string, seqNumber int) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO `+tableSeqCounters+` (id, inventory_id, main_zone, sub_zone, current_max)
		 VALUES (?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE current_max = GREATEST(current_max, VALUES(current_max))`,
		uuid.New().String(), inventoryID, mainZone, subZone, seqNumber)
	return err
}

// --- Outbound Orders ---

func (r *Repository) ListOutboundOrders(ctx context.Context, inventoryID string) ([]OutboundOrder, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, inventory_id, order_no, type, status, order_info, remark, items,
		        source_reserve_id, created_at, updated_at, confirmed_at, cancelled_at,
		        converted_at, returned_at, combo_id, combo_quantity,
		        bundle_id, bundle_main_product_id, bundle_quantity, bundle_snapshot
		 FROM `+tableOutboundOrders+` WHERE inventory_id = ? ORDER BY created_at DESC`, inventoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanOrders(rows)
}

func (r *Repository) ListOutboundOrdersPaginated(ctx context.Context, inventoryID, keyword string, limit, offset int) ([]OutboundOrder, bool, int, error) {
	where := `inventory_id = ?`
	args := []any{inventoryID}
	if keyword != "" {
		like := "%" + keyword + "%"
		where += ` AND (order_no LIKE ? OR order_info LIKE ? OR remark LIKE ?)`
		args = append(args, like, like, like)
	}
	var total int
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM `+tableOutboundOrders+` WHERE `+where, args...).Scan(&total); err != nil {
		return nil, false, 0, err
	}
	queryArgs := append(append([]any{}, args...), limit+1, offset)
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, inventory_id, order_no, type, status, order_info, remark, items,
		        source_reserve_id, created_at, updated_at, confirmed_at, cancelled_at,
		        converted_at, returned_at, combo_id, combo_quantity,
		        bundle_id, bundle_main_product_id, bundle_quantity, bundle_snapshot
		 FROM `+tableOutboundOrders+` WHERE `+where+` ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		queryArgs...)
	if err != nil {
		return nil, false, 0, err
	}
	defer rows.Close()
	items, err := scanOrders(rows)
	if err != nil {
		return nil, false, 0, err
	}
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	return items, hasMore, total, nil
}

func scanOrders(rows *sql.Rows) ([]OutboundOrder, error) {
	var items []OutboundOrder
	for rows.Next() {
		var item OutboundOrder
		var snapshot sql.NullString
		if err := rows.Scan(
			&item.ID, &item.InventoryID, &item.OrderNo, &item.Type, &item.Status,
			&item.OrderInfo, &item.Remark, &item.Items,
			&item.SourceReserveID,
			&item.CreatedAt, &item.UpdatedAt, &item.ConfirmedAt, &item.CancelledAt,
			&item.ConvertedAt, &item.ReturnedAt, &item.ComboID, &item.ComboQuantity,
			&item.BundleID, &item.BundleMainProductID, &item.BundleQuantity, &snapshot,
		); err != nil {
			return nil, err
		}
		if snapshot.Valid {
			item.BundleSnapshot = json.RawMessage(snapshot.String)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// --- Inbound Logs ---

func (r *Repository) ListInboundLogs(ctx context.Context, inventoryID string) ([]InboundLog, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, inventory_id, order_no, type, remark, items, created_at
		 FROM `+tableInboundLogs+` WHERE inventory_id = ? ORDER BY created_at DESC`, inventoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanInboundLogs(rows)
}

func (r *Repository) ListInboundLogsPaginated(ctx context.Context, inventoryID string, limit, offset int) ([]InboundLog, bool, int, error) {
	var total int
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM `+tableInboundLogs+` WHERE inventory_id = ?`, inventoryID).Scan(&total); err != nil {
		return nil, false, 0, err
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, inventory_id, order_no, type, remark, items, created_at
		 FROM `+tableInboundLogs+` WHERE inventory_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		inventoryID, limit+1, offset)
	if err != nil {
		return nil, false, 0, err
	}
	defer rows.Close()
	items, err := scanInboundLogs(rows)
	if err != nil {
		return nil, false, 0, err
	}
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	return items, hasMore, total, nil
}

func scanInboundLogs(rows *sql.Rows) ([]InboundLog, error) {
	var items []InboundLog
	for rows.Next() {
		var item InboundLog
		if err := rows.Scan(&item.ID, &item.InventoryID, &item.OrderNo, &item.Type,
			&item.Remark, &item.Items, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) GetInboundLog(ctx context.Context, id string) (*InboundLog, error) {
	var item InboundLog
	err := r.db.QueryRowContext(ctx,
		`SELECT id, inventory_id, order_no, type, remark, items, created_at
		 FROM `+tableInboundLogs+` WHERE id = ?`, id).
		Scan(&item.ID, &item.InventoryID, &item.OrderNo, &item.Type,
			&item.Remark, &item.Items, &item.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) CreateInboundLog(ctx context.Context, req *CreateInboundLogReq) (*InboundLog, error) {
	now := time.Now()
	itemsJSON, _ := json.Marshal(req.Items)
	item := &InboundLog{
		ID:          uuid.New().String(),
		InventoryID: req.InventoryID,
		OrderNo:     req.OrderNo,
		Type:        req.Type,
		Remark:      req.Remark,
		Items:       itemsJSON,
		CreatedAt:   now,
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO `+tableInboundLogs+` (id, inventory_id, order_no, type, remark, items, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		item.ID, item.InventoryID, item.OrderNo, item.Type, item.Remark, item.Items, item.CreatedAt)
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (r *Repository) UpdateInboundLog(ctx context.Context, req *UpdateInboundLogReq) (*InboundLog, error) {
	log, err := r.GetInboundLog(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	if req.OrderNo != nil {
		log.OrderNo = req.OrderNo
	}
	if req.Type != nil {
		log.Type = *req.Type
	}
	if req.Remark != nil {
		log.Remark = req.Remark
	}
	if req.Items != nil {
		itemsJSON, _ := json.Marshal(req.Items)
		log.Items = itemsJSON
	}
	_, err = r.db.ExecContext(ctx,
		`UPDATE `+tableInboundLogs+` SET order_no = ?, type = ?, remark = ?, items = ? WHERE id = ?`,
		log.OrderNo, log.Type, log.Remark, log.Items, log.ID)
	if err != nil {
		return nil, err
	}
	return log, nil
}

func (r *Repository) DeleteInboundLog(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM `+tableInboundLogs+` WHERE id = ?`, id)
	return err
}

// --- Product Mutations ---

// UpdateProductTx 更新商品基础信息。
// 库存数量变更由调用方写入 stock_movements，避免手工改数无法追溯。
func (r *Repository) UpdateProductTx(ctx context.Context, tx *sql.Tx, req *UpdateProductReq) (*Product, error) {
	product, err := r.GetProductForUpdateTx(ctx, tx, req.ID)
	if err != nil {
		return nil, err
	}
	applyProductUpdate(product, req)
	if product.Quantity < 0 || product.ReservedQuantity < 0 {
		return nil, NewGoodserError(ErrInvalidQuantity)
	}

	product.UpdatedAt = time.Now()
	_, err = tx.ExecContext(ctx,
		`UPDATE `+tableProducts+` SET
		 code=?, main_zone=?, sub_zone=?, seq_number=?,
		 quantity=?, reserved_quantity=?, status_code=?, name=?,
		 series_id=?, set_type_id=?, product_type_id=?,
		 original_price=?, market_price=?, expected_price=?, cost_price=?,
		 remark=?, storage_location=?, image_url=?, images=?, tags=?, updated_at=?
		 WHERE id=?`,
		product.Code, product.MainZone, product.SubZone, product.SeqNumber,
		product.Quantity, product.ReservedQuantity, product.StatusCode, product.Name,
		product.SeriesID, product.SetTypeID, product.ProductTypeID,
		product.OriginalPrice, product.MarketPrice, product.ExpectedPrice, product.CostPrice,
		product.Remark, product.StorageLocation, product.ImageURL, product.Images, product.Tags,
		product.UpdatedAt, product.ID)
	if err != nil {
		return nil, err
	}
	product.recalcTotal()
	return product, nil
}

func applyProductUpdate(product *Product, req *UpdateProductReq) {
	if req.Code != nil {
		product.Code = *req.Code
	}
	if req.MainZone != nil {
		product.MainZone = *req.MainZone
	}
	if req.SubZone != nil {
		product.SubZone = *req.SubZone
	}
	if req.SeqNumber != nil {
		product.SeqNumber = *req.SeqNumber
	}
	if req.Quantity != nil {
		product.Quantity = *req.Quantity
	}
	if req.StatusCode != nil {
		product.StatusCode = *req.StatusCode
	}
	if req.Name != nil {
		product.Name = *req.Name
	}
	if req.SeriesID != nil {
		product.SeriesID = normalizeOptionalID(req.SeriesID)
	}
	if req.SetTypeID != nil {
		product.SetTypeID = normalizeOptionalID(req.SetTypeID)
	}
	if req.ProductTypeID != nil {
		product.ProductTypeID = normalizeOptionalID(req.ProductTypeID)
	}
	if req.OriginalPrice != nil {
		product.OriginalPrice = *req.OriginalPrice
	}
	if req.MarketPrice != nil {
		product.MarketPrice = *req.MarketPrice
	}
	if req.ExpectedPrice != nil {
		product.ExpectedPrice = *req.ExpectedPrice
	}
	if req.CostPrice != nil {
		product.CostPrice = *req.CostPrice
	}
	if req.Remark != nil {
		product.Remark = *req.Remark
	}
	if req.StorageLocation != nil {
		product.StorageLocation = *req.StorageLocation
	}
	if req.ImageURL != nil {
		product.ImageURL = req.ImageURL
	}
	if req.Images != nil {
		data, _ := json.Marshal(*req.Images)
		product.Images = data
	}
	if req.Tags != nil {
		data, _ := json.Marshal(*req.Tags)
		product.Tags = data
	}
}

func (r *Repository) DeleteProduct(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM `+tableProducts+` WHERE id = ?`, id)
	return err
}

func (r *Repository) DeleteProductTx(ctx context.Context, tx *sql.Tx, id string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM `+tableProductCharacters+` WHERE product_id = ?`, id); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `DELETE FROM `+tableProducts+` WHERE id = ?`, id)
	return err
}

// CountProductComboRefs 统计商品被组合包引用的次数
func (r *Repository) CountProductComboRefs(ctx context.Context, productID string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+tableComboItems+` WHERE product_id = ?`, productID).Scan(&count)
	return count, err
}

// --- Tags ---

func (r *Repository) ListTags(ctx context.Context) ([]Tag, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, color, created_at FROM `+tableTags+` ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Tag
	for rows.Next() {
		var item Tag
		if err := rows.Scan(&item.ID, &item.Name, &item.Color, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) GetTag(ctx context.Context, id string) (*Tag, error) {
	var item Tag
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, color, created_at FROM `+tableTags+` WHERE id = ?`, id).
		Scan(&item.ID, &item.Name, &item.Color, &item.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) UpdateTag(ctx context.Context, req *UpdateTagReq) (*Tag, error) {
	tag, err := r.GetTag(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	if req.Name != nil {
		tag.Name = *req.Name
	}
	if req.Color != nil {
		tag.Color = *req.Color
	}
	_, err = r.db.ExecContext(ctx,
		`UPDATE `+tableTags+` SET name = ?, color = ? WHERE id = ?`,
		tag.Name, tag.Color, tag.ID)
	if err != nil {
		return nil, err
	}
	return tag, nil
}

func (r *Repository) DeleteTag(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM `+tableTags+` WHERE id = ?`, id)
	return err
}

func (r *Repository) CreateTag(ctx context.Context, req *CreateTagReq) (*Tag, error) {
	now := time.Now()
	color := "#1890ff"
	if req.Color != nil {
		color = *req.Color
	}
	item := &Tag{
		ID:        uuid.New().String(),
		Name:      req.Name,
		Color:     color,
		CreatedAt: now,
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO `+tableTags+` (id, name, color, created_at) VALUES (?, ?, ?, ?)`,
		item.ID, item.Name, item.Color, item.CreatedAt)
	if err != nil {
		return nil, err
	}
	return item, nil
}

// --- Status Codes ---

func (r *Repository) GetStatusCode(ctx context.Context, id string) (*StatusCode, error) {
	var item StatusCode
	err := r.db.QueryRowContext(ctx,
		`SELECT id, code, label, is_system, created_at FROM `+tableStatusCodes+` WHERE id = ?`, id).
		Scan(&item.ID, &item.Code, &item.Label, &item.IsSystem, &item.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) CreateStatusCode(ctx context.Context, req *AddStatusCodeReq) (*StatusCode, error) {
	now := time.Now()
	item := &StatusCode{
		ID:        uuid.New().String(),
		Code:      req.Code,
		Label:     req.Label,
		IsSystem:  false,
		CreatedAt: now,
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO `+tableStatusCodes+` (id, code, label, is_system, created_at) VALUES (?, ?, ?, ?, ?)`,
		item.ID, item.Code, item.Label, item.IsSystem, item.CreatedAt)
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (r *Repository) UpdateStatusCode(ctx context.Context, req *UpdateStatusCodeReq) (*StatusCode, error) {
	sc, err := r.GetStatusCode(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	if req.Label != "" {
		sc.Label = req.Label
	}
	_, err = r.db.ExecContext(ctx,
		`UPDATE `+tableStatusCodes+` SET label = ? WHERE id = ?`,
		sc.Label, sc.ID)
	if err != nil {
		return nil, err
	}
	return sc, nil
}

func (r *Repository) DeleteStatusCode(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM `+tableStatusCodes+` WHERE id = ?`, id)
	return err
}

func (r *Repository) CountProductsByStatusCode(ctx context.Context, code string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM `+tableProducts+` WHERE status_code = ?`, code).Scan(&count)
	return count, err
}

func (r *Repository) ListStatusCodes(ctx context.Context) ([]StatusCode, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, code, label, is_system, created_at FROM `+tableStatusCodes+` ORDER BY code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []StatusCode
	for rows.Next() {
		var item StatusCode
		if err := rows.Scan(&item.ID, &item.Code, &item.Label, &item.IsSystem, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// --- Transaction ---

func (r *Repository) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return r.db.BeginTx(ctx, nil)
}

// --- Transaction helpers ---

func (r *Repository) CreateProductTx(ctx context.Context, tx *sql.Tx, req *CreateProductReq) (*Product, error) {
	now := time.Now()
	qty := 0
	if req.Quantity != nil {
		qty = *req.Quantity
	}

	code := req.Code
	if code == "" {
		code = fmt.Sprintf("%s-%s-%04d-%04d-%s", req.MainZone, req.SubZone, req.SeqNumber, 0, req.StatusCode)
	}

	p := &Product{
		ID:               uuid.New().String(),
		InventoryID:      req.InventoryID,
		Code:             code,
		MainZone:         req.MainZone,
		SubZone:          req.SubZone,
		SeqNumber:        req.SeqNumber,
		Quantity:         qty,
		ReservedQuantity: 0,
		StatusCode:       req.StatusCode,
		Name:             req.Name,
		SeriesID:         normalizeOptionalID(req.SeriesID),
		SetTypeID:        normalizeOptionalID(req.SetTypeID),
		ProductTypeID:    normalizeOptionalID(req.ProductTypeID),
		OriginalPrice:    req.OriginalPrice,
		MarketPrice:      req.MarketPrice,
		ExpectedPrice:    req.ExpectedPrice,
		Remark:           req.Remark,
		StorageLocation:  req.StorageLocation,
		ImageURL:         req.ImageURL,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if req.Images != nil {
		data, _ := json.Marshal(*req.Images)
		p.Images = data
	}
	if req.Tags != nil {
		data, _ := json.Marshal(*req.Tags)
		p.Tags = data
	}

	_, err := tx.ExecContext(ctx,
		`INSERT INTO `+tableProducts+`
		 (id, inventory_id, code, main_zone, sub_zone, seq_number, quantity, reserved_quantity,
		  status_code, name, series_id, set_type_id, product_type_id,
		  original_price, market_price, expected_price, cost_price,
		  remark, storage_location, image_url, images, tags, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.InventoryID, p.Code, p.MainZone, p.SubZone, p.SeqNumber,
		p.Quantity, p.ReservedQuantity, p.StatusCode, p.Name,
		p.SeriesID, p.SetTypeID, p.ProductTypeID,
		p.OriginalPrice, p.MarketPrice, p.ExpectedPrice, p.CostPrice,
		p.Remark, p.StorageLocation, p.ImageURL, p.Images, p.Tags,
		p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	p.recalcTotal()
	return p, nil
}

func (r *Repository) AddProductQuantityTx(ctx context.Context, tx *sql.Tx, productID string, quantity int) error {
	_, err := tx.ExecContext(ctx,
		`UPDATE `+tableProducts+` SET quantity = quantity + ?, updated_at = ? WHERE id = ?`,
		quantity, time.Now(), productID)
	return err
}

func (r *Repository) UpdateProductReservedTx(ctx context.Context, tx *sql.Tx, productID string, deltaQty, deltaReserved int) error {
	res, err := tx.ExecContext(ctx,
		`UPDATE `+tableProducts+`
		 SET quantity = quantity + ?, reserved_quantity = reserved_quantity + ?, updated_at = ?
		 WHERE id = ? AND quantity + ? >= 0 AND reserved_quantity + ? >= 0`,
		deltaQty, deltaReserved, time.Now(), productID, deltaQty, deltaReserved)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return NewGoodserError(ErrInsufficientStock)
	}
	return nil
}

// ApplyStockDeltaTx 原子调整可用库存与锁定库存，并返回调整后的商品。
// 条件更新保证任何一方都不会变成负数，并发下不会超卖。
func (r *Repository) ApplyStockDeltaTx(ctx context.Context, tx *sql.Tx, productID string, deltaQty, deltaReserved int) (*Product, error) {
	res, err := tx.ExecContext(ctx,
		`UPDATE `+tableProducts+`
		 SET quantity = quantity + ?, reserved_quantity = reserved_quantity + ?, updated_at = ?
		 WHERE id = ? AND quantity + ? >= 0 AND reserved_quantity + ? >= 0`,
		deltaQty, deltaReserved, time.Now(), productID, deltaQty, deltaReserved)
	if err != nil {
		return nil, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, NewGoodserError(ErrInsufficientStock)
	}
	return r.GetProductForUpdateTx(ctx, tx, productID)
}

// AddStockMovementTx 写入库存流水
func (r *Repository) AddStockMovementTx(ctx context.Context, tx *sql.Tx, m *StockMovement) error {
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now()
	}
	_, err := tx.ExecContext(ctx,
		`INSERT INTO `+tableStockMovements+`
		 (inventory_id, product_id, quantity_delta, reserved_delta, quantity_after, reserved_after,
		  reason, ref_type, ref_id, actor, remark, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.InventoryID, m.ProductID, m.QuantityDelta, m.ReservedDelta,
		m.QuantityAfter, m.ReservedAfter, m.Reason, m.RefType, m.RefID,
		m.Actor, m.Remark, m.CreatedAt)
	return err
}

func (r *Repository) GetProductForUpdateTx(ctx context.Context, tx *sql.Tx, productID string) (*Product, error) {
	row := tx.QueryRowContext(ctx,
		`SELECT id, inventory_id, code, main_zone, sub_zone, seq_number,
		        quantity, reserved_quantity, status_code, name, series_id, set_type_id, product_type_id,
		        original_price, market_price, expected_price, cost_price,
		        remark, storage_location, image_url, images, tags,
		        created_at, updated_at
		 FROM `+tableProducts+` WHERE id = ? FOR UPDATE`, productID)
	return scanProduct(row)
}

func (r *Repository) CreateOutboundOrderTx(ctx context.Context, tx *sql.Tx, req *CreateOutboundReq) (*OutboundOrder, error) {
	now := time.Now()
	itemsJSON, _ := json.Marshal(req.Items)
	orderType := "outbound"
	if req.Type != nil && *req.Type != "" {
		orderType = *req.Type
	}

	order := &OutboundOrder{
		ID:                  uuid.New().String(),
		InventoryID:         req.InventoryID,
		OrderNo:             req.OrderNo,
		Type:                orderType,
		Status:              string(OrderStatusPending),
		OrderInfo:           req.OrderInfo,
		Remark:              req.Remark,
		Items:               itemsJSON,
		SourceReserveID:     req.SourceReserveID,
		ComboID:             req.ComboID,
		ComboQuantity:       req.ComboQuantity,
		BundleID:            req.BundleID,
		BundleMainProductID: req.BundleMainProductID,
		BundleQuantity:      req.BundleQuantity,
		BundleSnapshot:      req.BundleSnapshot,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	_, err := tx.ExecContext(ctx,
		`INSERT INTO `+tableOutboundOrders+`
		 (id, inventory_id, order_no, type, status, order_info, remark, items,
		  source_reserve_id, combo_id, combo_quantity,
		  bundle_id, bundle_main_product_id, bundle_quantity, bundle_snapshot,
		  created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		order.ID, order.InventoryID, order.OrderNo, order.Type, order.Status,
		order.OrderInfo, order.Remark, order.Items,
		order.SourceReserveID, order.ComboID, order.ComboQuantity,
		order.BundleID, order.BundleMainProductID, order.BundleQuantity, order.BundleSnapshot,
		order.CreatedAt, order.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return order, nil
}

func (r *Repository) GetOutboundOrder(ctx context.Context, orderID string) (*OutboundOrder, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, inventory_id, order_no, type, status, order_info, remark, items,
		        source_reserve_id, created_at, updated_at, confirmed_at, cancelled_at,
		        converted_at, returned_at, combo_id, combo_quantity,
		        bundle_id, bundle_main_product_id, bundle_quantity, bundle_snapshot
		 FROM `+tableOutboundOrders+` WHERE id = ?`, orderID)
	return scanOrderRow(row)
}

func (r *Repository) GetOutboundOrderForUpdateTx(ctx context.Context, tx *sql.Tx, orderID string) (*OutboundOrder, error) {
	row := tx.QueryRowContext(ctx,
		`SELECT id, inventory_id, order_no, type, status, order_info, remark, items,
		        source_reserve_id, created_at, updated_at, confirmed_at, cancelled_at,
		        converted_at, returned_at, combo_id, combo_quantity,
		        bundle_id, bundle_main_product_id, bundle_quantity, bundle_snapshot
		 FROM `+tableOutboundOrders+` WHERE id = ? FOR UPDATE`, orderID)
	return scanOrderRow(row)
}

// FindChildOutboundTx 查询由某张预留单转出的出库单，用于转单幂等
func (r *Repository) FindChildOutboundTx(ctx context.Context, tx *sql.Tx, reserveID string) (*OutboundOrder, error) {
	row := tx.QueryRowContext(ctx,
		`SELECT id, inventory_id, order_no, type, status, order_info, remark, items,
		        source_reserve_id, created_at, updated_at, confirmed_at, cancelled_at,
		        converted_at, returned_at, combo_id, combo_quantity,
		        bundle_id, bundle_main_product_id, bundle_quantity, bundle_snapshot
		 FROM `+tableOutboundOrders+` WHERE source_reserve_id = ? ORDER BY created_at LIMIT 1`, reserveID)
	return scanOrderRow(row)
}

func scanOrderRow(row *sql.Row) (*OutboundOrder, error) {
	var item OutboundOrder
	var snapshot sql.NullString
	if err := row.Scan(
		&item.ID, &item.InventoryID, &item.OrderNo, &item.Type, &item.Status,
		&item.OrderInfo, &item.Remark, &item.Items,
		&item.SourceReserveID,
		&item.CreatedAt, &item.UpdatedAt, &item.ConfirmedAt, &item.CancelledAt,
		&item.ConvertedAt, &item.ReturnedAt, &item.ComboID, &item.ComboQuantity,
		&item.BundleID, &item.BundleMainProductID, &item.BundleQuantity, &snapshot,
	); err != nil {
		return nil, err
	}
	if snapshot.Valid {
		item.BundleSnapshot = json.RawMessage(snapshot.String)
	}
	return &item, nil
}

func (r *Repository) UpdateOutboundOrderStatusTx(ctx context.Context, tx *sql.Tx, orderID, status string, confirmedAt, cancelledAt, convertedAt, returnedAt *time.Time) error {
	_, err := tx.ExecContext(ctx,
		`UPDATE `+tableOutboundOrders+`
		 SET status = ?, confirmed_at = ?, cancelled_at = ?, converted_at = ?, returned_at = ?, updated_at = ?
		 WHERE id = ?`,
		status, confirmedAt, cancelledAt, convertedAt, returnedAt, time.Now(), orderID)
	return err
}

func (r *Repository) CreateInboundLogItemTx(ctx context.Context, tx *sql.Tx, inventoryID, logType string, items json.RawMessage) (*InboundLog, error) {
	now := time.Now()
	item := &InboundLog{
		ID:          uuid.New().String(),
		InventoryID: inventoryID,
		Type:        logType,
		Items:       items,
		CreatedAt:   now,
	}
	_, err := tx.ExecContext(ctx,
		`INSERT INTO `+tableInboundLogs+` (id, inventory_id, type, items, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		item.ID, item.InventoryID, item.Type, item.Items, item.CreatedAt)
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (r *Repository) AddRecycledSeqTx(ctx context.Context, tx *sql.Tx, inventoryID, mainZone, subZone string, seqNumber int) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO `+tableRecycledSeq+` (id, inventory_id, main_zone, sub_zone, seq_number, recycled_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), inventoryID, mainZone, subZone, seqNumber, time.Now())
	return err
}

// --- Returns ---

func (r *Repository) CreateReturnTx(ctx context.Context, tx *sql.Tx, ret *ReturnOrder) error {
	now := time.Now()
	if ret.CreatedAt.IsZero() {
		ret.CreatedAt = now
	}
	ret.UpdatedAt = now
	_, err := tx.ExecContext(ctx,
		`INSERT INTO `+tableReturns+`
		 (id, inventory_id, outbound_order_id, order_no, status, reason, remark, items,
		  created_at, updated_at, confirmed_at, cancelled_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ret.ID, ret.InventoryID, ret.OutboundOrderID, ret.OrderNo, ret.Status, ret.Reason,
		ret.Remark, ret.Items, ret.CreatedAt, ret.UpdatedAt, ret.ConfirmedAt, ret.CancelledAt)
	return err
}

func (r *Repository) GetReturn(ctx context.Context, id string) (*ReturnOrder, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, inventory_id, outbound_order_id, order_no, status, reason, remark, items,
		        created_at, updated_at, confirmed_at, cancelled_at
		 FROM `+tableReturns+` WHERE id = ?`, id)
	return scanReturnRow(row)
}

func (r *Repository) GetReturnForUpdateTx(ctx context.Context, tx *sql.Tx, id string) (*ReturnOrder, error) {
	row := tx.QueryRowContext(ctx,
		`SELECT id, inventory_id, outbound_order_id, order_no, status, reason, remark, items,
		        created_at, updated_at, confirmed_at, cancelled_at
		 FROM `+tableReturns+` WHERE id = ? FOR UPDATE`, id)
	return scanReturnRow(row)
}

func (r *Repository) ListReturns(ctx context.Context, inventoryID, outboundOrderID string) ([]ReturnOrder, error) {
	query := `SELECT id, inventory_id, outbound_order_id, order_no, status, reason, remark, items,
	                 created_at, updated_at, confirmed_at, cancelled_at
	          FROM ` + tableReturns + ` WHERE inventory_id = ?`
	args := []interface{}{inventoryID}
	if outboundOrderID != "" {
		query += ` AND outbound_order_id = ?`
		args = append(args, outboundOrderID)
	}
	query += ` ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []ReturnOrder
	for rows.Next() {
		var item ReturnOrder
		if err := scanReturnFields(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) UpdateReturnStatusTx(ctx context.Context, tx *sql.Tx, id, status string, confirmedAt, cancelledAt *time.Time) error {
	_, err := tx.ExecContext(ctx,
		`UPDATE `+tableReturns+`
		 SET status = ?, confirmed_at = ?, cancelled_at = ?, updated_at = ?
		 WHERE id = ?`,
		status, confirmedAt, cancelledAt, time.Now(), id)
	return err
}

// SumReturnedQtyTx 统计某出库单在指定状态退货单中的已退数量，按商品聚合。
// 创建退货时把 pending 也计入，防止并发提交超过可退数量。
func (r *Repository) SumReturnedQtyTx(ctx context.Context, tx *sql.Tx, outboundOrderID string, statuses []string) (map[string]int, error) {
	result := make(map[string]int)
	if len(statuses) == 0 {
		return result, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(statuses)), ",")
	args := []interface{}{outboundOrderID}
	for _, s := range statuses {
		args = append(args, s)
	}
	rows, err := tx.QueryContext(ctx,
		`SELECT items FROM `+tableReturns+`
		 WHERE outbound_order_id = ? AND status IN (`+placeholders+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var raw json.RawMessage
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var items []ReturnItem
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, err
		}
		for _, item := range items {
			result[item.ProductID] += item.Quantity
		}
	}
	return result, rows.Err()
}

func scanReturnRow(row *sql.Row) (*ReturnOrder, error) {
	var item ReturnOrder
	if err := scanReturnFields(row, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

func scanReturnFields(scanner interface {
	Scan(dest ...interface{}) error
}, item *ReturnOrder) error {
	if err := scanner.Scan(
		&item.ID, &item.InventoryID, &item.OutboundOrderID, &item.OrderNo,
		&item.Status, &item.Reason, &item.Remark, &item.Items,
		&item.CreatedAt, &item.UpdatedAt, &item.ConfirmedAt, &item.CancelledAt,
	); err != nil {
		return err
	}
	return nil
}
