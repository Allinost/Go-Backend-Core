package goodser

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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
		 FROM `+tableInventories+` ORDER BY sort_order`)
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
		SortOrder: 0,
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
	item.UpdatedAt = time.Now()
	_, err = r.db.ExecContext(ctx,
		`UPDATE `+tableInventories+` SET name = ?, updated_at = ? WHERE id = ?`,
		item.Name, item.UpdatedAt, item.ID)
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
		        quantity, reserved_quantity, status_code, name,
		        original_price, market_price, expected_price,
		        remark, storage_location, image_url, image_list, tags,
		        created_at, updated_at
		 FROM `+tableProducts+` WHERE inventory_id = ? ORDER BY seq_number`, inventoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProducts(rows)
}

func (r *Repository) SearchProducts(ctx context.Context, inventoryID, keyword string) ([]Product, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, inventory_id, code, main_zone, sub_zone, seq_number,
		        quantity, reserved_quantity, status_code, name,
		        original_price, market_price, expected_price,
		        remark, storage_location, image_url, image_list, tags,
		        created_at, updated_at
		 FROM `+tableProducts+` WHERE inventory_id = ? AND name LIKE ? ORDER BY seq_number`,
		inventoryID, "%"+keyword+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProducts(rows)
}

func (r *Repository) GetProduct(ctx context.Context, id string) (*Product, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, inventory_id, code, main_zone, sub_zone, seq_number,
		        quantity, reserved_quantity, status_code, name,
		        original_price, market_price, expected_price,
		        remark, storage_location, image_url, image_list, tags,
		        created_at, updated_at
		 FROM `+tableProducts+` WHERE id = ?`, id)
	return scanProduct(row)
}

func (r *Repository) FindProductByCode(ctx context.Context, inventoryID, code string) (*Product, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, inventory_id, code, main_zone, sub_zone, seq_number,
		        quantity, reserved_quantity, status_code, name,
		        original_price, market_price, expected_price,
		        remark, storage_location, image_url, image_list, tags,
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
	return scanner.Scan(
		&p.ID, &p.InventoryID, &p.Code, &p.MainZone, &p.SubZone, &p.SeqNumber,
		&p.Quantity, &p.ReservedQuantity, &p.StatusCode, &p.Name,
		&p.OriginalPrice, &p.MarketPrice, &p.ExpectedPrice,
		&p.Remark, &p.StorageLocation, &p.ImageURL, &p.ImageList, &p.Tags,
		&p.CreatedAt, &p.UpdatedAt,
	)
}

// --- Seq Counters & Recycled ---

func (r *Repository) PopRecycledSeq(ctx context.Context, tx *sql.Tx, inventoryID, mainZone, subZone string) (int, error) {
	var seq int
	err := tx.QueryRowContext(ctx,
		`SELECT seq_number FROM `+tableRecycledSeq+
			` WHERE inventory_id = ? AND main_zone = ? AND sub_zone = ?
			 ORDER BY seq_number LIMIT 1 FOR UPDATE`,
		inventoryID, mainZone, subZone).Scan(&seq)
	if err != nil {
		return 0, err
	}
	_, err = tx.ExecContext(ctx,
		`DELETE FROM `+tableRecycledSeq+` WHERE inventory_id = ? AND main_zone = ? AND sub_zone = ? AND seq_number = ?`,
		inventoryID, mainZone, subZone, seq)
	return seq, err
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

// --- Outbound Orders ---

func (r *Repository) ListOutboundOrders(ctx context.Context, inventoryID string) ([]OutboundOrder, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, inventory_id, order_no, type, status, order_info, remark, items,
		        source_reserve_id, created_at, updated_at, confirmed_at, cancelled_at
		 FROM `+tableOutboundOrders+` WHERE inventory_id = ? ORDER BY created_at DESC`, inventoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanOrders(rows)
}

func scanOrders(rows *sql.Rows) ([]OutboundOrder, error) {
	var items []OutboundOrder
	for rows.Next() {
		var item OutboundOrder
		if err := rows.Scan(
			&item.ID, &item.InventoryID, &item.OrderNo, &item.Type, &item.Status,
			&item.OrderInfo, &item.Remark, &item.Items,
			&item.SourceReserveID,
			&item.CreatedAt, &item.UpdatedAt, &item.ConfirmedAt, &item.CancelledAt,
		); err != nil {
			return nil, err
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

// --- Product Mutations ---

func (r *Repository) UpdateProduct(ctx context.Context, req *UpdateProductReq) (*Product, error) {
	product, err := r.GetProduct(ctx, req.ID)
	if err != nil {
		return nil, err
	}
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
	if req.OriginalPrice != nil {
		product.OriginalPrice = *req.OriginalPrice
	}
	if req.MarketPrice != nil {
		product.MarketPrice = *req.MarketPrice
	}
	if req.ExpectedPrice != nil {
		product.ExpectedPrice = *req.ExpectedPrice
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
	if req.Tags != nil {
		data, _ := json.Marshal(*req.Tags)
		product.Tags = data
	}

	product.UpdatedAt = time.Now()
	_, err = r.db.ExecContext(ctx,
		`UPDATE `+tableProducts+` SET
		 code=?, main_zone=?, sub_zone=?, seq_number=?,
		 quantity=?, reserved_quantity=?, status_code=?, name=?,
		 original_price=?, market_price=?, expected_price=?,
		 remark=?, storage_location=?, image_url=?, tags=?, updated_at=?
		 WHERE id=?`,
		product.Code, product.MainZone, product.SubZone, product.SeqNumber,
		product.Quantity, product.ReservedQuantity, product.StatusCode, product.Name,
		product.OriginalPrice, product.MarketPrice, product.ExpectedPrice,
		product.Remark, product.StorageLocation, product.ImageURL, product.Tags,
		product.UpdatedAt, product.ID)
	if err != nil {
		return nil, err
	}
	return product, nil
}

func (r *Repository) DeleteProduct(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM `+tableProducts+` WHERE id = ?`, id)
	return err
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

	p := &Product{
		ID:               uuid.New().String(),
		InventoryID:      req.InventoryID,
		Code:             fmt.Sprintf("%s-%s-%04d-%04d-%s", req.MainZone, req.SubZone, req.SeqNumber, qty, req.StatusCode),
		MainZone:         req.MainZone,
		SubZone:          req.SubZone,
		SeqNumber:        req.SeqNumber,
		Quantity:         qty,
		ReservedQuantity: 0,
		StatusCode:       req.StatusCode,
		Name:             req.Name,
		OriginalPrice:    req.OriginalPrice,
		MarketPrice:      req.MarketPrice,
		ExpectedPrice:    req.ExpectedPrice,
		Remark:           req.Remark,
		StorageLocation:  req.StorageLocation,
		ImageURL:         req.ImageURL,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if req.Tags != nil {
		data, _ := json.Marshal(*req.Tags)
		p.Tags = data
	}

	_, err := tx.ExecContext(ctx,
		`INSERT INTO `+tableProducts+`
		 (id, inventory_id, code, main_zone, sub_zone, seq_number, quantity, reserved_quantity,
		  status_code, name, original_price, market_price, expected_price,
		  remark, storage_location, image_url, tags, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.InventoryID, p.Code, p.MainZone, p.SubZone, p.SeqNumber,
		p.Quantity, p.ReservedQuantity, p.StatusCode, p.Name,
		p.OriginalPrice, p.MarketPrice, p.ExpectedPrice,
		p.Remark, p.StorageLocation, p.ImageURL, p.Tags,
		p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (r *Repository) AddProductQuantityTx(ctx context.Context, tx *sql.Tx, productID string, quantity int) error {
	_, err := tx.ExecContext(ctx,
		`UPDATE `+tableProducts+` SET quantity = quantity + ?, updated_at = ? WHERE id = ?`,
		quantity, time.Now(), productID)
	return err
}

func (r *Repository) UpdateProductReservedTx(ctx context.Context, tx *sql.Tx, productID string, deltaQty, deltaReserved int) error {
	_, err := tx.ExecContext(ctx,
		`UPDATE `+tableProducts+` SET quantity = quantity + ?, reserved_quantity = reserved_quantity + ?, updated_at = ? WHERE id = ?`,
		deltaQty, deltaReserved, time.Now(), productID)
	return err
}

func (r *Repository) GetProductForUpdateTx(ctx context.Context, tx *sql.Tx, productID string) (*Product, error) {
	row := tx.QueryRowContext(ctx,
		`SELECT id, inventory_id, code, main_zone, sub_zone, seq_number,
		        quantity, reserved_quantity, status_code, name,
		        original_price, market_price, expected_price,
		        remark, storage_location, image_url, image_list, tags,
		        created_at, updated_at
		 FROM `+tableProducts+` WHERE id = ? FOR UPDATE`, productID)
	return scanProduct(row)
}

func (r *Repository) CreateOutboundOrderTx(ctx context.Context, tx *sql.Tx, req *CreateOutboundReq) (*OutboundOrder, error) {
	now := time.Now()
	itemsJSON, _ := json.Marshal(req.Items)
	orderType := "outbound"
	if req.Type != nil {
		orderType = *req.Type
	}
	status := "pending"
	if req.Status != nil {
		status = *req.Status
	}

	order := &OutboundOrder{
		ID:              uuid.New().String(),
		InventoryID:     req.InventoryID,
		OrderNo:         req.OrderNo,
		Type:            orderType,
		Status:          status,
		OrderInfo:       req.OrderInfo,
		Remark:          req.Remark,
		Items:           itemsJSON,
		SourceReserveID: req.SourceReserveID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	_, err := tx.ExecContext(ctx,
		`INSERT INTO `+tableOutboundOrders+`
		 (id, inventory_id, order_no, type, status, order_info, remark, items,
		  source_reserve_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		order.ID, order.InventoryID, order.OrderNo, order.Type, order.Status,
		order.OrderInfo, order.Remark, order.Items,
		order.SourceReserveID,
		order.CreatedAt, order.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return order, nil
}

func (r *Repository) GetOutboundOrderForUpdateTx(ctx context.Context, tx *sql.Tx, orderID string) (*OutboundOrder, error) {
	row := tx.QueryRowContext(ctx,
		`SELECT id, inventory_id, order_no, type, status, order_info, remark, items,
		        source_reserve_id, created_at, updated_at, confirmed_at, cancelled_at
		 FROM `+tableOutboundOrders+` WHERE id = ? FOR UPDATE`, orderID)
	var item OutboundOrder
	if err := row.Scan(
		&item.ID, &item.InventoryID, &item.OrderNo, &item.Type, &item.Status,
		&item.OrderInfo, &item.Remark, &item.Items,
		&item.SourceReserveID,
		&item.CreatedAt, &item.UpdatedAt, &item.ConfirmedAt, &item.CancelledAt,
	); err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) UpdateOutboundOrderStatusTx(ctx context.Context, tx *sql.Tx, orderID, status string, confirmedAt, cancelledAt *time.Time) error {
	_, err := tx.ExecContext(ctx,
		`UPDATE `+tableOutboundOrders+` SET status = ?, confirmed_at = ?, cancelled_at = ?, updated_at = ? WHERE id = ?`,
		status, confirmedAt, cancelledAt, time.Now(), orderID)
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
