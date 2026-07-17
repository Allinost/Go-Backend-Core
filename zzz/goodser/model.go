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
	OrderStatusReserved  OrderStatus = "reserved"
	OrderStatusConfirmed OrderStatus = "confirmed"
	OrderStatusCancelled OrderStatus = "cancelled"
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
type Product struct {
	ID               string          `json:"_id"`
	InventoryID      string          `json:"inventory_id"`
	Code             string          `json:"code"`
	MainZone         string          `json:"main_zone"`
	SubZone          string          `json:"sub_zone"`
	SeqNumber        int             `json:"seq_number"`
	Quantity         int             `json:"quantity"`
	ReservedQuantity int             `json:"reserved_quantity"`
	StatusCode       string          `json:"status_code"`
	Name             string          `json:"name"`
	OriginalPrice    float64         `json:"original_price,omitempty"`
	MarketPrice      float64         `json:"market_price,omitempty"`
	ExpectedPrice    float64         `json:"expected_price,omitempty"`
	Remark           string          `json:"remark,omitempty"`
	StorageLocation  string          `json:"storage_location,omitempty"`
	ImageURL         *string         `json:"image_url"`
	Images           json.RawMessage `json:"images,omitempty"`
	Tags             json.RawMessage `json:"tags,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

// OrderItem 出库单/入库单中的商品项
type OrderItem struct {
	ProductID   string  `json:"product_id"`
	ProductName string  `json:"product_name"`
	ProductCode string  `json:"product_code"`
	Quantity    int     `json:"quantity"`
	ImageURL    *string `json:"image_url,omitempty"`
}

// OutboundOrder 出库单
type OutboundOrder struct {
	ID              string          `json:"_id"`
	InventoryID     string          `json:"inventory_id"`
	OrderNo         string          `json:"order_no"`
	Type            string          `json:"type"`
	Status          string          `json:"status"`
	OrderInfo       *string         `json:"order_info"`
	Remark          *string         `json:"remark"`
	Items           json.RawMessage `json:"items"`
	SourceReserveID *string         `json:"source_reserve_id"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	ConfirmedAt     *time.Time      `json:"confirmed_at"`
	CancelledAt     *time.Time      `json:"cancelled_at"`
}

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
	Remark          string    `json:"remark,omitempty"`
	StorageLocation string    `json:"storage_location,omitempty"`
	ImageURL        *string   `json:"image_url"`
	Images          *[]string `json:"images"`
	Tags            *[]string `json:"tags"`
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
	Remark          *string   `json:"remark"`
	StorageLocation *string   `json:"storage_location"`
	ImageURL        *string   `json:"image_url"`
	Images          *[]string `json:"images"`
	Tags            *[]string `json:"tags"`
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
	Remark          string    `json:"remark,omitempty"`
	StorageLocation string    `json:"storage_location,omitempty"`
	ImageURL        *string   `json:"image_url"`
	Images          *[]string `json:"images"`
	Tags            *[]string `json:"tags"`
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
	Remark          string    `json:"remark,omitempty"`
	StorageLocation string    `json:"storage_location,omitempty"`
	ImageURL        *string   `json:"image_url"`
	Images          *[]string `json:"images"`
	Tags            *[]string `json:"tags"`
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
	InventoryID     string      `json:"inventory_id"`
	OrderNo         string      `json:"order_no"`
	Type            *string     `json:"type"`
	Status          *string     `json:"status"`
	OrderInfo       *string     `json:"order_info"`
	Remark          *string     `json:"remark"`
	Items           []OrderItem `json:"items"`
	SourceReserveID *string     `json:"source_reserve_id"`
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
	ID          string      `json:"id"`
	InventoryID string      `json:"inventory_id"`
	OrderNo     string      `json:"order_no"`
	Items       []OrderItem `json:"items"`
	OrderInfo   *string     `json:"order_info"`
	Remark      *string     `json:"remark"`
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
	InventoryID string `json:"inventory_id"`
	Keyword     string `json:"keyword"`
	MainZone    string `json:"main_zone"`
	StatusCode  string `json:"status_code"`
	TagID       string `json:"tag_id"`
	SortBy      string `json:"sort_by"`
	SortOrder   string `json:"sort_order"`
	Page        int    `json:"page"`
	PageSize    int    `json:"page_size"`
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
