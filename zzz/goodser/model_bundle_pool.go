package goodser

import "time"

// BundlePoolStatus 捆物池状态
const (
	BundlePoolStatusActive   = "active"
	BundlePoolStatusInactive = "inactive"
)

// BundlePoolItem 捆物池成员，附带商品库存信息用于展示
type BundlePoolItem struct {
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

// BundlePool 捆物池，按库存目录隔离，可被多个捆复用
type BundlePool struct {
	ID           string           `json:"_id"`
	InventoryID  string           `json:"inventory_id"`
	Code         string           `json:"code"`
	Name         string           `json:"name"`
	Description  *string          `json:"description,omitempty"`
	Status       string           `json:"status"`
	MinItemCount int              `json:"min_item_count"`
	MinAmount    float64          `json:"min_amount"`
	Items        []BundlePoolItem `json:"items"`
	CreatedAt    time.Time        `json:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at"`
}

// BundlePoolItemReq 捆物池成员配置项
type BundlePoolItemReq struct {
	ProductID       string `json:"product_id"`
	DefaultQuantity int    `json:"default_quantity"`
}

// CreateBundlePoolReq 创建捆物池
type CreateBundlePoolReq struct {
	InventoryID  string              `json:"inventory_id"`
	Code         string              `json:"code"`
	Name         string              `json:"name"`
	Description  *string             `json:"description"`
	Status       string              `json:"status"`
	MinItemCount int                 `json:"min_item_count"`
	MinAmount    float64             `json:"min_amount"`
	Items        []BundlePoolItemReq `json:"items"`
}

// UpdateBundlePoolReq 更新捆物池，未传字段保持不变
type UpdateBundlePoolReq struct {
	ID           string               `json:"id"`
	Code         *string              `json:"code"`
	Name         *string              `json:"name"`
	Description  *string              `json:"description"`
	Status       *string              `json:"status"`
	MinItemCount *int                 `json:"min_item_count"`
	MinAmount    *float64             `json:"min_amount"`
	Items        *[]BundlePoolItemReq `json:"items"`
}

// EffectiveMinItemCount 合并池与捆的最少捆物数量规则
func (b *Bundle) EffectiveMinItemCount() int {
	value := b.MinSubQuantity
	if b.Pool != nil && b.Pool.MinItemCount > value {
		value = b.Pool.MinItemCount
	}
	if value < 0 {
		return 0
	}
	return value
}

// EffectiveMinSubAmount 合并池与捆的最低捆物金额规则
func (b *Bundle) EffectiveMinSubAmount() float64 {
	value := b.MinSubAmount
	if b.Pool != nil && b.Pool.MinAmount > value {
		value = b.Pool.MinAmount
	}
	if value < 0 {
		return 0
	}
	return value
}
