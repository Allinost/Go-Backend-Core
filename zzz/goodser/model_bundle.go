package goodser

import (
	"encoding/json"
	"strings"
	"time"
)

// BundleStatus 捆模板状态
const (
	BundleStatusActive   = "active"
	BundleStatusInactive = "inactive"
)

// 捆价格模式
const (
	BundlePriceModeSum      = "sum"       // 主商品与副区商品价格合计
	BundlePriceModeFixed    = "fixed"     // 整捆固定价
	BundlePriceModeMainOnly = "main_only" // 只收主商品价格，副区商品作为搭配
)

// BundleMainFilter 主商品筛选条件，所有非空字段按 AND 匹配
type BundleMainFilter struct {
	SeriesID      string `json:"series_id,omitempty"`
	SetTypeID     string `json:"set_type_id,omitempty"`
	ProductTypeID string `json:"product_type_id,omitempty"`
	CharacterID   string `json:"character_id,omitempty"`
	StatusCode    string `json:"status_code,omitempty"`
	Keyword       string `json:"keyword,omitempty"`
}

// IsZero 判断筛选条件是否为空
func (f BundleMainFilter) IsZero() bool {
	return strings.TrimSpace(f.SeriesID) == "" &&
		strings.TrimSpace(f.SetTypeID) == "" &&
		strings.TrimSpace(f.ProductTypeID) == "" &&
		strings.TrimSpace(f.CharacterID) == "" &&
		strings.TrimSpace(f.StatusCode) == "" &&
		strings.TrimSpace(f.Keyword) == ""
}

// Normalize 清理筛选条件中的空白
func (f *BundleMainFilter) Normalize() {
	if f == nil {
		return
	}
	f.SeriesID = strings.TrimSpace(f.SeriesID)
	f.SetTypeID = strings.TrimSpace(f.SetTypeID)
	f.ProductTypeID = strings.TrimSpace(f.ProductTypeID)
	f.CharacterID = strings.TrimSpace(f.CharacterID)
	f.StatusCode = strings.TrimSpace(f.StatusCode)
	f.Keyword = strings.TrimSpace(f.Keyword)
}

// BundleMainProduct 捆允许的主商品
type BundleMainProduct struct {
	ProductID     string   `json:"product_id"`
	ProductName   string   `json:"product_name"`
	ProductCode   string   `json:"product_code"`
	ImageURL      *string  `json:"image_url,omitempty"`
	ExpectedPrice float64  `json:"expected_price"`
	Available     int      `json:"available"`
	ReservedQty   int      `json:"reserved_quantity"`
	TotalQuantity int      `json:"total_quantity"`
	SeriesID      *string  `json:"series_id,omitempty"`
	SetTypeID     *string  `json:"set_type_id,omitempty"`
	ProductTypeID *string  `json:"product_type_id,omitempty"`
	CharacterIDs  []string `json:"character_ids,omitempty"`
}

// BundleSubProduct 捆允许搭配的副区商品
type BundleSubProduct struct {
	ProductID       string  `json:"product_id"`
	ProductName     string  `json:"product_name"`
	ProductCode     string  `json:"product_code"`
	ImageURL        *string `json:"image_url,omitempty"`
	DefaultQuantity int     `json:"default_quantity"`
	ExpectedPrice   float64 `json:"expected_price"`
	Available       int     `json:"available"`
	ReservedQty     int     `json:"reserved_quantity"`
	TotalQuantity   int     `json:"total_quantity"`
}

// Bundle 捆模板，不持有独立库存
type Bundle struct {
	ID              string              `json:"_id"`
	InventoryID     string              `json:"inventory_id"`
	Code            string              `json:"code"`
	Name            string              `json:"name"`
	Description     *string             `json:"description"`
	Status          string              `json:"status"`
	MainFilter      BundleMainFilter    `json:"main_filter"`
	MainProductIDs  []string            `json:"main_product_ids"`
	MainProducts    []BundleMainProduct `json:"main_products"`
	SubProducts     []BundleSubProduct  `json:"sub_products"`
	PoolID          *string             `json:"pool_id,omitempty"`
	Pool            *BundlePool         `json:"pool,omitempty"`
	Availability    int                 `json:"availability"`
	MinMainQuantity int                 `json:"min_main_quantity"`
	MinMainAmount   float64             `json:"min_main_amount"`
	MinSubQuantity  int                 `json:"min_sub_quantity"`
	MaxSubQuantity  int                 `json:"max_sub_quantity"`
	MinSubAmount    float64             `json:"min_sub_amount"`
	PriceMode       string              `json:"price_mode"`
	FixedPrice      float64             `json:"fixed_price"`
	CreatedAt       time.Time           `json:"created_at"`
	UpdatedAt       time.Time           `json:"updated_at"`
}

// BundleSubItemReq 副区商品配置项
type BundleSubItemReq struct {
	ProductID       string `json:"product_id"`
	DefaultQuantity int    `json:"default_quantity"`
}

// CreateBundleReq 创建捆模板
type CreateBundleReq struct {
	InventoryID     string             `json:"inventory_id"`
	Code            string             `json:"code"`
	Name            string             `json:"name"`
	Description     *string            `json:"description"`
	Status          string             `json:"status"`
	MainFilter      *BundleMainFilter  `json:"main_filter"`
	MainProductIDs  []string           `json:"main_product_ids"`
	SubProducts     []BundleSubItemReq `json:"sub_products"`
	PoolID          *string            `json:"pool_id"`
	MinMainQuantity int                `json:"min_main_quantity"`
	MinMainAmount   float64            `json:"min_main_amount"`
	MinSubQuantity  int                `json:"min_sub_quantity"`
	MaxSubQuantity  int                `json:"max_sub_quantity"`
	MinSubAmount    float64            `json:"min_sub_amount"`
	PriceMode       string             `json:"price_mode"`
	FixedPrice      float64            `json:"fixed_price"`
}

// UpdateBundleReq 更新捆模板，未传字段保持不变
type UpdateBundleReq struct {
	ID              string              `json:"id"`
	Code            *string             `json:"code"`
	Name            *string             `json:"name"`
	Description     *string             `json:"description"`
	Status          *string             `json:"status"`
	MainFilter      *BundleMainFilter   `json:"main_filter"`
	MainProductIDs  *[]string           `json:"main_product_ids"`
	SubProducts     *[]BundleSubItemReq `json:"sub_products"`
	PoolID          *string             `json:"pool_id"`
	MinMainQuantity *int                `json:"min_main_quantity"`
	MinMainAmount   *float64            `json:"min_main_amount"`
	MinSubQuantity  *int                `json:"min_sub_quantity"`
	MaxSubQuantity  *int                `json:"max_sub_quantity"`
	MinSubAmount    *float64            `json:"min_sub_amount"`
	PriceMode       *string             `json:"price_mode"`
	FixedPrice      *float64            `json:"fixed_price"`
}

// BundleOrderSubItem 下单时选择的副区商品，数量按单捆计
type BundleOrderSubItem struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

// BundleOrderSelection 下单时选择的捆内容，数量均按单捆计
type BundleOrderSelection struct {
	BundleID      string               `json:"bundle_id"`
	MainProductID string               `json:"main_product_id"`
	MainQuantity  int                  `json:"main_quantity"`
	Quantity      int                  `json:"quantity"`
	SubItems      []BundleOrderSubItem `json:"sub_items"`
}

// Normalize 清理并补默认值
func (s *BundleOrderSelection) Normalize() {
	if s == nil {
		return
	}
	s.BundleID = strings.TrimSpace(s.BundleID)
	s.MainProductID = strings.TrimSpace(s.MainProductID)
	if s.MainQuantity <= 0 {
		s.MainQuantity = 1
	}
	if s.Quantity <= 0 {
		s.Quantity = 1
	}
}

// BundlePreviewReq 捆下单预览
type BundlePreviewReq struct {
	MainProductID string               `json:"main_product_id"`
	MainQuantity  int                  `json:"main_quantity"`
	Quantity      int                  `json:"quantity"`
	SubItems      []BundleOrderSubItem `json:"sub_items"`
}

// BundleSelectedSubItem 已选副区商品及计价信息
type BundleSelectedSubItem struct {
	ProductID   string  `json:"product_id"`
	ProductName string  `json:"product_name"`
	ProductCode string  `json:"product_code"`
	Quantity    int     `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	Amount      float64 `json:"amount"`
	Available   int     `json:"available"`
	ImageURL    *string `json:"image_url,omitempty"`
}

// BundlePreviewResp 捆下单预览结果
type BundlePreviewResp struct {
	BundleID     string                  `json:"bundle_id"`
	BundleCode   string                  `json:"bundle_code"`
	BundleName   string                  `json:"bundle_name"`
	InventoryID  string                  `json:"inventory_id"`
	PriceMode    string                  `json:"price_mode"`
	Quantity     int                     `json:"quantity"`
	MainProduct  BundleMainProduct       `json:"main_product"`
	MainQuantity int                     `json:"main_quantity"`
	MainAmount   float64                 `json:"main_amount"`
	SubItems     []BundleSelectedSubItem `json:"sub_items"`
	SubQuantity  int                     `json:"sub_quantity"`
	SubAmount    float64                 `json:"sub_amount"`
	UnitPrice    float64                 `json:"unit_price"`
	TotalPrice   float64                 `json:"total_price"`
	Items        []OrderItem             `json:"items"`
	Snapshot     BundleOrderSnapshot     `json:"snapshot"`
}

// BundleOrderSnapshot 写入出库单的捆快照
type BundleOrderSnapshot struct {
	BundleID        string                  `json:"bundle_id"`
	BundleCode      string                  `json:"bundle_code"`
	BundleName      string                  `json:"bundle_name"`
	PoolID          string                  `json:"pool_id,omitempty"`
	PoolName        string                  `json:"pool_name,omitempty"`
	MainProductID   string                  `json:"main_product_id"`
	MainProductName string                  `json:"main_product_name"`
	MainQuantity    int                     `json:"main_quantity"`
	Quantity        int                     `json:"quantity"`
	SubItems        []BundleSelectedSubItem `json:"sub_items"`
	PriceMode       string                  `json:"price_mode"`
	MinMainQuantity int                     `json:"min_main_quantity"`
	MinMainAmount   float64                 `json:"min_main_amount"`
	MinSubQuantity  int                     `json:"min_sub_quantity"`
	MaxSubQuantity  int                     `json:"max_sub_quantity"`
	MinSubAmount    float64                 `json:"min_sub_amount"`
	UnitPrice       float64                 `json:"unit_price"`
	TotalPrice      float64                 `json:"total_price"`
}

// CustomerBundleView 客户可见的捆模板
type CustomerBundleView struct {
	ID              string              `json:"bundle_id"`
	InventoryID     string              `json:"inventory_id"`
	Name            string              `json:"name"`
	Code            string              `json:"code"`
	Description     *string             `json:"description"`
	MainFilter      BundleMainFilter    `json:"main_filter"`
	MainProducts    []BundleMainProduct `json:"main_products"`
	SubProducts     []BundleSubProduct  `json:"sub_products"`
	Pool            *BundlePool         `json:"pool,omitempty"`
	Availability    int                 `json:"availability"`
	MinMainQuantity int                 `json:"min_main_quantity"`
	MinMainAmount   float64             `json:"min_main_amount"`
	MinSubQuantity  int                 `json:"min_sub_quantity"`
	MaxSubQuantity  int                 `json:"max_sub_quantity"`
	MinSubAmount    float64             `json:"min_sub_amount"`
	PriceMode       string              `json:"price_mode"`
	FixedPrice      float64             `json:"fixed_price"`
}

// bundleSnapshotJSON 序列化快照，失败时返回空 JSON 对象
func bundleSnapshotJSON(snapshot *BundleOrderSnapshot) json.RawMessage {
	if snapshot == nil {
		return nil
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return data
}
