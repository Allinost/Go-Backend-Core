package goodsercatalog

import (
	"encoding/json"
	"time"
)

type Catalog struct {
	ID          string    `json:"_id"`
	InventoryID string    `json:"inventory_id,omitempty"`
	Title       string    `json:"title"`
	Remark      string    `json:"remark,omitempty"`
	CoverURL    string    `json:"cover_url,omitempty"`
	Status      string    `json:"status"`
	ImageCount  int       `json:"image_count"`
	ItemCount   int       `json:"item_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CatalogImage struct {
	ID         string    `json:"_id"`
	CatalogID  string    `json:"catalog_id"`
	ImageURL   string    `json:"image_url"`
	Width      int       `json:"width,omitempty"`
	Height     int       `json:"height,omitempty"`
	CropStatus string    `json:"crop_status"`
	CropError  string    `json:"crop_error,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type CatalogItem struct {
	ID        string          `json:"_id"`
	CatalogID string          `json:"catalog_id"`
	ImageID   string          `json:"image_id,omitempty"`
	ProductID string          `json:"product_id,omitempty"`
	Title     string          `json:"title,omitempty"`
	Remark    string          `json:"remark,omitempty"`
	ImageURL  string          `json:"image_url"`
	ThumbURL  string          `json:"thumb_url,omitempty"`
	Box       json.RawMessage `json:"box,omitempty"`
	SortOrder int             `json:"sort_order"`
	Status    string          `json:"status"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type CatalogDetail struct {
	Catalog
	Images []CatalogImage `json:"images"`
	Items  []CatalogItem  `json:"items"`
}

type CreateCatalogReq struct {
	InventoryID string `json:"inventory_id"`
	Title       string `json:"title" binding:"required"`
	Remark      string `json:"remark"`
}

type UpdateCatalogReq struct {
	Title  *string `json:"title"`
	Remark *string `json:"remark"`
	Status *string `json:"status"`
}

type AddImageReq struct {
	ImageURL string `json:"image_url"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
}

type CreateItemReq struct {
	ImageID   string          `json:"image_id"`
	ProductID string          `json:"product_id"`
	Title     string          `json:"title"`
	Remark    string          `json:"remark"`
	ImageURL  string          `json:"image_url" binding:"required"`
	ThumbURL  string          `json:"thumb_url"`
	Box       json.RawMessage `json:"box"`
	SortOrder int             `json:"sort_order"`
}

type UpdateItemReq struct {
	ProductID *string         `json:"product_id"`
	Title     *string         `json:"title"`
	Remark    *string         `json:"remark"`
	ImageURL  *string         `json:"image_url"`
	ThumbURL  *string         `json:"thumb_url"`
	Box       json.RawMessage `json:"box"`
	SortOrder *int            `json:"sort_order"`
	Status    *string         `json:"status"`
}

type ListCatalogsReq struct {
	InventoryID string `form:"inventory_id"`
	Keyword     string `form:"keyword"`
}

type SegmentResult struct {
	Boxes []json.RawMessage `json:"boxes"`
	Items []CatalogItem     `json:"items"`
}
