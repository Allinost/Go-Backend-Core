package goodser

import (
	"encoding/json"
	"time"
)

// OrderType 出库单类型
type OrderType string

const (
	OrderTypeOutbound OrderType = "outbound"
	OrderTypeReserve  OrderType = "reserve"
)

// OrderStatus 出库单状态
type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusReserved  OrderStatus = "reserved" // 仅用于兼容历史数据，新数据不再写入
	OrderStatusConverted OrderStatus = "converted"
	OrderStatusConfirmed OrderStatus = "confirmed"
	OrderStatusCancelled OrderStatus = "cancelled"

	OrderStatusPartiallyReturned OrderStatus = "partially_returned"
	OrderStatusReturned          OrderStatus = "returned"
)

// InboundType 入库类型
type InboundType string

const (
	InboundTypeSingle InboundType = "single"
	InboundTypeBatch  InboundType = "batch"
	InboundTypeSearch InboundType = "search"
)

// Inventory 库存目录
type Inventory struct {
	ID        string    `json:"_id"`
	Name      string    `json:"name"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Product 商品
// D1 库存语义：Quantity 为可用库存，ReservedQuantity 为已锁定库存，
// TotalQuantity 为物理总量（= Quantity + ReservedQuantity）。
type Product struct {
	ID               string          `json:"_id"`
	InventoryID      string          `json:"inventory_id"`
	Code             string          `json:"code"`
	MainZone         string          `json:"main_zone"`
	SubZone          string          `json:"sub_zone"`
	SeqNumber        int             `json:"seq_number"`
	Quantity         int             `json:"quantity"`
	ReservedQuantity int             `json:"reserved_quantity"`
	TotalQuantity    int             `json:"total_quantity"`
	StatusCode       string          `json:"status_code"`
	Name             string          `json:"name"`
	SeriesID         *string         `json:"series_id"`
	SetTypeID        *string         `json:"set_type_id"`
	ProductTypeID    *string         `json:"product_type_id"`
	CharacterIDs     []string        `json:"character_ids,omitempty"`
	OriginalPrice    float64         `json:"original_price,omitempty"`
	MarketPrice      float64         `json:"market_price,omitempty"`
	ExpectedPrice    float64         `json:"expected_price,omitempty"`
	CostPrice        float64         `json:"cost_price,omitempty"`
	Remark           string          `json:"remark,omitempty"`
	StorageLocation  string          `json:"storage_location,omitempty"`
	ImageURL         *string         `json:"image_url"`
	Images           json.RawMessage `json:"images,omitempty"`
	Tags             json.RawMessage `json:"tags,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

// recalcTotal 重新计算物理总量，所有修改 Quantity/ReservedQuantity 后必须调用
func (p *Product) recalcTotal() {
	p.TotalQuantity = p.Quantity + p.ReservedQuantity
}

// OrderItem 出库单/入库单中的商品项
type OrderItem struct {
	ProductID   string  `json:"product_id"`
	ProductName string  `json:"product_name"`
	ProductCode string  `json:"product_code"`
	Quantity    int     `json:"quantity"`
	ImageURL    *string `json:"image_url,omitempty"`
}

// OutboundOrder 出库单/预留单
type OutboundOrder struct {
	ID                  string          `json:"_id"`
	InventoryID         string          `json:"inventory_id"`
	OrderNo             string          `json:"order_no"`
	Type                string          `json:"type"`
	Status              string          `json:"status"`
	OrderInfo           *string         `json:"order_info"`
	Remark              *string         `json:"remark"`
	Items               json.RawMessage `json:"items"`
	SourceReserveID     *string         `json:"source_reserve_id"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
	ConfirmedAt         *time.Time      `json:"confirmed_at"`
	CancelledAt         *time.Time      `json:"cancelled_at"`
	ConvertedAt         *time.Time      `json:"converted_at"`
	ReturnedAt          *time.Time      `json:"returned_at"`
	ComboID             *string         `json:"combo_id,omitempty"`
	ComboQuantity       int             `json:"combo_quantity,omitempty"`
	BundleID            *string         `json:"bundle_id,omitempty"`
	BundleMainProductID *string         `json:"bundle_main_product_id,omitempty"`
	BundleQuantity      int             `json:"bundle_quantity,omitempty"`
	BundleSnapshot      json.RawMessage `json:"bundle_snapshot,omitempty"`
}

// ReturnStatus 退货单状态
type ReturnStatus string

const (
	ReturnStatusPending   ReturnStatus = "pending"
	ReturnStatusConfirmed ReturnStatus = "confirmed"
	ReturnStatusCancelled ReturnStatus = "cancelled"
)

// ReturnReason 退货原因
type ReturnReason string

const (
	ReturnReasonRefundOnly     ReturnReason = "refund_only"     // 仅退款，不退货
	ReturnReasonReturnRefund   ReturnReason = "return_refund"   // 退货退款
	ReturnReasonShippingDamage ReturnReason = "shipping_damage" // 运损
	ReturnReasonOther          ReturnReason = "other"
)

// ReturnItem 退货明细
type ReturnItem struct {
	ProductID   string  `json:"product_id"`
	ProductName string  `json:"product_name"`
	ProductCode string  `json:"product_code"`
	Quantity    int     `json:"quantity"`
	Restock     *bool   `json:"restock"`
	ImageURL    *string `json:"image_url,omitempty"`
}

// ReturnOrder 退货单
type ReturnOrder struct {
	ID              string          `json:"_id"`
	InventoryID     string          `json:"inventory_id"`
	OutboundOrderID string          `json:"outbound_order_id"`
	OrderNo         string          `json:"order_no"`
	Status          string          `json:"status"`
	Reason          string          `json:"reason"`
	Remark          *string         `json:"remark"`
	Items           json.RawMessage `json:"items"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	ConfirmedAt     *time.Time      `json:"confirmed_at"`
	CancelledAt     *time.Time      `json:"cancelled_at"`
}

// StockMovement 库存流水
type StockMovement struct {
	ID            uint64    `json:"id"`
	InventoryID   string    `json:"inventory_id"`
	ProductID     string    `json:"product_id"`
	QuantityDelta int       `json:"quantity_delta"`
	ReservedDelta int       `json:"reserved_delta"`
	QuantityAfter int       `json:"quantity_after"`
	ReservedAfter int       `json:"reserved_after"`
	Reason        string    `json:"reason"`
	RefType       string    `json:"ref_type"`
	RefID         string    `json:"ref_id"`
	Actor         string    `json:"actor"`
	Remark        string    `json:"remark"`
	CreatedAt     time.Time `json:"created_at"`
}

// 库存流水原因
const (
	StockReasonInboundSingle   = "inbound_single"
	StockReasonInboundBatch    = "inbound_batch"
	StockReasonInboundImport   = "inbound_import"
	StockReasonInboundManual   = "inbound_manual"
	StockReasonProductDelete   = "product_delete"
	StockReasonManualAdjust    = "manual_adjust"
	StockReasonReserveCreated  = "reserve_created"
	StockReasonOutboundCreated = "outbound_created"
	StockReasonReserveConvert  = "reserve_convert"
	StockReasonReserveCancel   = "reserve_cancel"
	StockReasonOutboundConfirm = "outbound_confirm"
	StockReasonOutboundCancel  = "outbound_cancel"
	StockReasonReturnRestock   = "return_restock"
	StockReasonReturnNoRestock = "return_no_restock"
)

// InboundLog 入库日志
type InboundLog struct {
	ID          string          `json:"_id"`
	InventoryID string          `json:"inventory_id"`
	OrderNo     *string         `json:"order_no"`
	Type        string          `json:"type"`
	Remark      *string         `json:"remark"`
	Items       json.RawMessage `json:"items"`
	CreatedAt   time.Time       `json:"created_at"`
}

// Tag 标签
type Tag struct {
	ID        string    `json:"_id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	CreatedAt time.Time `json:"created_at"`
}

// StatusCode 状态编码
type StatusCode struct {
	ID        string    `json:"_id"`
	Code      string    `json:"code"`
	Label     string    `json:"label"`
	IsSystem  bool      `json:"is_system"`
	CreatedAt time.Time `json:"created_at"`
}

// DictionaryKind 受控维度类型
type DictionaryKind string

const (
	DictionaryKindSeries      DictionaryKind = "series"
	DictionaryKindCharacter   DictionaryKind = "character"
	DictionaryKindSetType     DictionaryKind = "set_type"
	DictionaryKindProductType DictionaryKind = "product_type"
)

// ValidDictionaryKind 判断维度类型是否合法
func ValidDictionaryKind(kind string) bool {
	switch DictionaryKind(kind) {
	case DictionaryKindSeries, DictionaryKindCharacter, DictionaryKindSetType, DictionaryKindProductType:
		return true
	}
	return false
}

// Dictionary 受控维度字典项
type Dictionary struct {
	ID        string          `json:"_id"`
	Kind      string          `json:"kind"`
	Code      string          `json:"code"`
	Name      string          `json:"name"`
	ParentID  *string         `json:"parent_id,omitempty"`
	SortOrder int             `json:"sort_order"`
	Enabled   bool            `json:"enabled"`
	Extra     json.RawMessage `json:"extra,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// ComboStatus 组合包状态
const (
	ComboStatusActive   = "active"
	ComboStatusInactive = "inactive"
)

// ComboItem 组合包组件
type ComboItem struct {
	ProductID   string  `json:"product_id"`
	ProductName string  `json:"product_name"`
	ProductCode string  `json:"product_code"`
	Quantity    int     `json:"quantity"`
	Available   int     `json:"available"`
	ImageURL    *string `json:"image_url,omitempty"`
}

// Combo 组合包（BOM），不持有独立库存
type Combo struct {
	ID           string      `json:"_id"`
	InventoryID  string      `json:"inventory_id"`
	Code         string      `json:"code"`
	Name         string      `json:"name"`
	Description  *string     `json:"description"`
	ImageURL     *string     `json:"image_url"`
	Price        float64     `json:"price"`
	Status       string      `json:"status"`
	Items        []ComboItem `json:"items"`
	Availability int         `json:"availability"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
}

// SeriesSummary 系列聚合视图
type SeriesSummary struct {
	SeriesID      string `json:"series_id"`
	SeriesName    string `json:"series_name"`
	ProductCount  int    `json:"product_count"`
	TotalQuantity int    `json:"total_quantity"`
	Available     int    `json:"available"`
}

// CustomerProductView 客户可见的商品视图
type CustomerProductView struct {
	ID               string   `json:"product_id"`
	InventoryID      string   `json:"inventory_id"`
	Name             string   `json:"name"`
	Code             string   `json:"code"`
	StatusCode       string   `json:"status_code"`
	Quantity         int      `json:"quantity"`
	ReservedQuantity int      `json:"reserved_quantity"`
	TotalQuantity    int      `json:"total_quantity"`
	ExpectedPrice    float64  `json:"expected_price"`
	ImageURL         *string  `json:"image_url"`
	Series           string   `json:"series,omitempty"`
	Characters       []string `json:"characters,omitempty"`
	SetType          string   `json:"set_type,omitempty"`
	ProductType      string   `json:"product_type,omitempty"`
	Available        bool     `json:"available"`
}

// CustomerComboView 客户可见的组合包视图
type CustomerComboView struct {
	ID           string      `json:"combo_id"`
	InventoryID  string      `json:"inventory_id"`
	Name         string      `json:"name"`
	Code         string      `json:"code"`
	Description  *string     `json:"description"`
	ImageURL     *string     `json:"image_url"`
	Price        float64     `json:"price"`
	Availability int         `json:"availability"`
	Components   []ComboItem `json:"components"`
}

// CustomerStockQueryResp 客户库存查询结果
type CustomerStockQueryResp struct {
	Items    []CustomerProductView `json:"items"`
	Combos   []CustomerComboView   `json:"combos"`
	Bundles  []CustomerBundleView  `json:"bundles"`
	Total    int                   `json:"total"`
	HasMore  bool                  `json:"has_more"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"page_size"`
}

// --- Request DTOs ---

type CreateInventoryReq struct {
	Name string `json:"name"`
}

type UpdateInventoryReq struct {
	ID   string  `json:"id"`
	Name *string `json:"name"`
}

type DeleteInventoryReq struct {
	ID string `json:"id"`
}

type CreateProductReq struct {
	InventoryID     string    `json:"inventory_id"`
	Code            string    `json:"code"`
	MainZone        string    `json:"main_zone"`
	SubZone         string    `json:"sub_zone"`
	SeqNumber       int       `json:"seq_number"`
	Quantity        *int      `json:"quantity"`
	StatusCode      string    `json:"status_code"`
	Name            string    `json:"name"`
	OriginalPrice   float64   `json:"original_price,omitempty"`
	MarketPrice     float64   `json:"market_price,omitempty"`
	ExpectedPrice   float64   `json:"expected_price,omitempty"`
	CostPrice       float64   `json:"cost_price,omitempty"`
	Remark          string    `json:"remark,omitempty"`
	StorageLocation string    `json:"storage_location,omitempty"`
	ImageURL        *string   `json:"image_url"`
	Images          *[]string `json:"images"`
	Tags            *[]string `json:"tags"`
	SeriesID        *string   `json:"series_id"`
	SetTypeID       *string   `json:"set_type_id"`
	ProductTypeID   *string   `json:"product_type_id"`
	CharacterIDs    *[]string `json:"character_ids"`
}

type UpdateProductReq struct {
	ID              string    `json:"id"`
	InventoryID     *string   `json:"inventory_id"`
	Code            *string   `json:"code"`
	MainZone        *string   `json:"main_zone"`
	SubZone         *string   `json:"sub_zone"`
	SeqNumber       *int      `json:"seq_number"`
	Quantity        *int      `json:"quantity"`
	StatusCode      *string   `json:"status_code"`
	Name            *string   `json:"name"`
	OriginalPrice   *float64  `json:"original_price"`
	MarketPrice     *float64  `json:"market_price"`
	ExpectedPrice   *float64  `json:"expected_price"`
	CostPrice       *float64  `json:"cost_price"`
	Remark          *string   `json:"remark"`
	StorageLocation *string   `json:"storage_location"`
	ImageURL        *string   `json:"image_url"`
	Images          *[]string `json:"images"`
	Tags            *[]string `json:"tags"`
	SeriesID        *string   `json:"series_id"`
	SetTypeID       *string   `json:"set_type_id"`
	ProductTypeID   *string   `json:"product_type_id"`
	CharacterIDs    *[]string `json:"character_ids"`
}

type AllocateSeqReq struct {
	InventoryID string `json:"inventory_id"`
	MainZone    string `json:"main_zone"`
	SubZone     string `json:"sub_zone"`
}

type AllocateSeqResp struct {
	SeqNumber int `json:"seq_number"`
}

type InboundSingleReq struct {
	InventoryID     string    `json:"inventory_id"`
	OrderNo         *string   `json:"order_no"`
	Code            string    `json:"code"`
	MainZone        string    `json:"main_zone"`
	SubZone         string    `json:"sub_zone"`
	SeqNumber       int       `json:"seq_number"`
	Quantity        *int      `json:"quantity"`
	StatusCode      string    `json:"status_code"`
	Name            string    `json:"name"`
	OriginalPrice   float64   `json:"original_price,omitempty"`
	MarketPrice     float64   `json:"market_price,omitempty"`
	ExpectedPrice   float64   `json:"expected_price,omitempty"`
	CostPrice       float64   `json:"cost_price,omitempty"`
	Remark          string    `json:"remark,omitempty"`
	StorageLocation string    `json:"storage_location,omitempty"`
	ImageURL        *string   `json:"image_url"`
	Images          *[]string `json:"images"`
	Tags            *[]string `json:"tags"`
	SeriesID        *string   `json:"series_id"`
	SetTypeID       *string   `json:"set_type_id"`
	ProductTypeID   *string   `json:"product_type_id"`
	CharacterIDs    *[]string `json:"character_ids"`
}

type InboundBatchReq struct {
	InventoryID string                `json:"inventory_id"`
	OrderNo     *string               `json:"order_no"`
	Remark      *string               `json:"remark"`
	Items       []InboundBatchReqItem `json:"items"`
}

type InboundBatchReqItem struct {
	Code            string    `json:"code"`
	MainZone        string    `json:"main_zone"`
	SubZone         string    `json:"sub_zone"`
	SeqNumber       int       `json:"seq_number"`
	Quantity        *int      `json:"quantity"`
	StatusCode      string    `json:"status_code"`
	Name            string    `json:"name"`
	OriginalPrice   float64   `json:"original_price,omitempty"`
	MarketPrice     float64   `json:"market_price,omitempty"`
	ExpectedPrice   float64   `json:"expected_price,omitempty"`
	CostPrice       float64   `json:"cost_price,omitempty"`
	Remark          string    `json:"remark,omitempty"`
	StorageLocation string    `json:"storage_location,omitempty"`
	ImageURL        *string   `json:"image_url"`
	Images          *[]string `json:"images"`
	Tags            *[]string `json:"tags"`
	SeriesID        *string   `json:"series_id"`
	SetTypeID       *string   `json:"set_type_id"`
	ProductTypeID   *string   `json:"product_type_id"`
	CharacterIDs    *[]string `json:"character_ids"`
}

type InboundSearchImportReq struct {
	InventoryID string                `json:"inventory_id"`
	OrderNo     *string               `json:"order_no"`
	Remark      *string               `json:"remark"`
	Items       []SearchImportReqItem `json:"items"`
}

type SearchImportReqItem struct {
	ProductID   string  `json:"product_id"`
	ProductName string  `json:"product_name"`
	ProductCode string  `json:"product_code"`
	Quantity    int     `json:"quantity"`
	ImageURL    *string `json:"image_url"`
}

type CreateOutboundReq struct {
	InventoryID string `json:"inventory_id"`
	OrderNo     string `json:"order_no"`
	// Type 仅用于兼容旧客户端：reserve 表示预留单，其余一律按 outbound 处理。
	// 单据状态由服务端推导，客户端不能传 status。
	Type            *string     `json:"type"`
	OrderInfo       *string     `json:"order_info"`
	Remark          *string     `json:"remark"`
	Items           []OrderItem `json:"items"`
	SourceReserveID *string     `json:"source_reserve_id"`
	ComboID         *string     `json:"combo_id,omitempty"`
	ComboQuantity   int         `json:"combo_quantity,omitempty"`
	// Bundle 传值时由服务端校验并展开主商品与副区商品，Items 应留空
	Bundle *BundleOrderSelection `json:"bundle,omitempty"`
	// 以下字段仅服务端内部使用，用于把捆信息写入出库单
	BundleID            *string         `json:"-"`
	BundleMainProductID *string         `json:"-"`
	BundleQuantity      int             `json:"-"`
	BundleSnapshot      json.RawMessage `json:"-"`
}

type ConfirmOutboundReq struct {
	ID string `json:"id"`
}

type CancelOutboundReq struct {
	ID string `json:"id"`
}

type CancelReserveReq struct {
	ID string `json:"id"`
}

type ReserveToOutboundReq struct {
	ID          string `json:"id"`
	InventoryID string `json:"inventory_id"`
	OrderNo     string `json:"order_no"`
	// Items 保留字段仅为兼容旧客户端，转单时以原预留单明细为准
	Items     []OrderItem `json:"items"`
	OrderInfo *string     `json:"order_info"`
	Remark    *string     `json:"remark"`
}

// CreateReturnReq 创建退货单（可多选同一出库单内的商品）
type CreateReturnReq struct {
	OutboundOrderID string       `json:"outbound_order_id"`
	Reason          string       `json:"reason"`
	Remark          *string      `json:"remark"`
	Items           []ReturnItem `json:"items"`
}

// ConfirmReturnReq 确认退货单
type ConfirmReturnReq struct {
	ID string `json:"id"`
}

// CancelReturnReq 取消退货单
type CancelReturnReq struct {
	ID string `json:"id"`
}

// QueryReturnsReq 退货单查询
type QueryReturnsReq struct {
	InventoryID     string `json:"inventory_id"`
	OutboundOrderID string `json:"outbound_order_id"`
}

type CreateInboundLogReq struct {
	InventoryID string      `json:"inventory_id"`
	OrderNo     *string     `json:"order_no"`
	Type        string      `json:"type"`
	Remark      *string     `json:"remark"`
	Items       []OrderItem `json:"items"`
}

type UpdateInboundLogReq struct {
	ID      string      `json:"id"`
	OrderNo *string     `json:"order_no"`
	Type    *string     `json:"type"`
	Remark  *string     `json:"remark"`
	Items   []OrderItem `json:"items"`
}

type CreateTagReq struct {
	Name  string  `json:"name"`
	Color *string `json:"color"`
}

type UpdateTagReq struct {
	ID    string  `json:"id"`
	Name  *string `json:"name"`
	Color *string `json:"color"`
}

type AddStatusCodeReq struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

type UpdateStatusCodeReq struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// InboundBatchResp 批量入库响应
type InboundBatchResp struct {
	Items []Product `json:"items"`
	Count int       `json:"count"`
}

// InboundSearchImportResp 搜索导入入库响应
type InboundSearchImportResp struct {
	Items []Product `json:"items"`
	Count int       `json:"count"`
}

// SyncAllResp 全量同步响应
type SyncAllResp struct {
	Inventories    []Inventory                `json:"inventories"`
	Products       map[string][]Product       `json:"products"`
	OutboundOrders map[string][]OutboundOrder `json:"outbound_orders"`
	InboundLogs    map[string][]InboundLog    `json:"inbound_logs"`
	Tags           []Tag                      `json:"tags"`
	StatusCodes    []StatusCode               `json:"status_codes"`
}

type PaginatedReq struct {
	InventoryID string `json:"inventory_id"`
	Page        int    `json:"page"`
	PageSize    int    `json:"page_size"`
}

type QueryProductsReq struct {
	InventoryID   string `json:"inventory_id"`
	Keyword       string `json:"keyword"`
	MainZone      string `json:"main_zone"`
	StatusCode    string `json:"status_code"`
	TagID         string `json:"tag_id"`
	SeriesID      string `json:"series_id"`
	CharacterID   string `json:"character_id"`
	SetTypeID     string `json:"set_type_id"`
	ProductTypeID string `json:"product_type_id"`
	OnlyAvailable bool   `json:"only_available"`
	SortBy        string `json:"sort_by"`
	SortOrder     string `json:"sort_order"`
	Page          int    `json:"page"`
	PageSize      int    `json:"page_size"`
}

func (r *QueryProductsReq) Normalize() {
	if r.Page < 1 {
		r.Page = 1
	}
	if r.PageSize < 1 || r.PageSize > 100 {
		r.PageSize = 20
	}
	if r.SortOrder != "asc" && r.SortOrder != "desc" {
		r.SortOrder = "asc"
	}
}

func (r *PaginatedReq) Normalize() {
	if r.Page < 1 {
		r.Page = 1
	}
	if r.PageSize < 1 || r.PageSize > 100 {
		r.PageSize = 20
	}
}

func (r *PaginatedReq) Offset() int {
	return (r.Page - 1) * r.PageSize
}

type PaginatedResp[T any] struct {
	Items   []T  `json:"items"`
	HasMore bool `json:"has_more"`
	Total   int  `json:"total"`
}

type InventoryStats struct {
	TotalProducts int     `json:"total_products"`
	TotalQuantity int     `json:"total_quantity"`
	TotalValue    float64 `json:"total_value"`
	CategoryCount int     `json:"category_count"`
}

// --- P1 dictionaries / combos / customer DTOs ---

type ListDictionariesReq struct {
	Kind    string `json:"kind"`
	Enabled *bool  `json:"enabled"`
}

type CreateDictionaryReq struct {
	Kind      string          `json:"kind"`
	Code      string          `json:"code"`
	Name      string          `json:"name"`
	ParentID  *string         `json:"parent_id"`
	SortOrder int             `json:"sort_order"`
	Enabled   *bool           `json:"enabled"`
	Extra     json.RawMessage `json:"extra"`
}

type UpdateDictionaryReq struct {
	ID        string          `json:"id"`
	Code      *string         `json:"code"`
	Name      *string         `json:"name"`
	ParentID  *string         `json:"parent_id"`
	SortOrder *int            `json:"sort_order"`
	Enabled   *bool           `json:"enabled"`
	Extra     json.RawMessage `json:"extra"`
}

type ComboItemReq struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

type CreateComboReq struct {
	InventoryID string         `json:"inventory_id"`
	Code        string         `json:"code"`
	Name        string         `json:"name"`
	Description *string        `json:"description"`
	ImageURL    *string        `json:"image_url"`
	Price       float64        `json:"price"`
	Status      string         `json:"status"`
	Items       []ComboItemReq `json:"items"`
}

type UpdateComboReq struct {
	ID          string          `json:"id"`
	Code        *string         `json:"code"`
	Name        *string         `json:"name"`
	Description *string         `json:"description"`
	ImageURL    *string         `json:"image_url"`
	Price       *float64        `json:"price"`
	Status      *string         `json:"status"`
	Items       *[]ComboItemReq `json:"items"`
}

// CustomerStockQueryReq 客户只读库存查询
type CustomerStockQueryReq struct {
	InventoryID       string `json:"inventory_id"`
	Keyword           string `json:"keyword"`
	SeriesID          string `json:"series_id"`
	SeriesName        string `json:"series_name"`
	CharacterID       string `json:"character_id"`
	CharacterName     string `json:"character_name"`
	SetTypeID         string `json:"set_type_id"`
	ProductTypeID     string `json:"product_type_id"`
	ProductType       string `json:"product_type"`
	StatusCode        string `json:"status_code"`
	IncludeOutOfStock bool   `json:"include_out_of_stock"`
	Page              int    `json:"page"`
	PageSize          int    `json:"page_size"`
}

func (r *CustomerStockQueryReq) Normalize() {
	if r.Page < 1 {
		r.Page = 1
	}
	if r.PageSize < 1 || r.PageSize > 50 {
		r.PageSize = 20
	}
}

// CustomerOrderItem 客户订单请求明细
type CustomerOrderItem struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

// CustomerOrderRequestReq 客户提交订单请求，创建 pending 预留并锁定库存
type CustomerOrderRequestReq struct {
	InventoryID   string                `json:"inventory_id"`
	CustomerName  string                `json:"customer_name"`
	Contact       string                `json:"contact"`
	Items         []CustomerOrderItem   `json:"items"`
	ComboID       string                `json:"combo_id"`
	ComboQuantity int                   `json:"combo_quantity"`
	Bundle        *BundleOrderSelection `json:"bundle,omitempty"`
	OrderInfo     *string               `json:"order_info"`
	Remark        *string               `json:"remark"`
}

// CustomerOrderRequestResp 客户订单请求结果
type CustomerOrderRequestResp struct {
	OrderID    string  `json:"order_id"`
	OrderNo    string  `json:"order_no"`
	Status     string  `json:"status"`
	TotalQty   int     `json:"total_quantity"`
	TotalPrice float64 `json:"total_price,omitempty"`
	BundleID   string  `json:"bundle_id,omitempty"`
	NotifyOK   bool    `json:"notify_ok"`
	NotifyErr  string  `json:"notify_error,omitempty"`
}
