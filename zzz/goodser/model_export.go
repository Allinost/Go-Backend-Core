package goodser

import (
	"encoding/json"
	"strings"
	"time"
)

// 平铺图导出的固定枚举值。
const (
	ExportTypeProduct = "product"
	ExportTypeBundle  = "bundle"
	ExportTypeCombo   = "combo"

	ExportBackgroundBlack       = "black"
	ExportBackgroundWhite       = "white"
	ExportBackgroundTransparent = "transparent"

	ExportTextColorAuto  = "auto"
	ExportTextColorDark  = "dark"
	ExportTextColorLight = "light"

	ExportFormatPNG  = "png"
	ExportFormatJPEG = "jpeg"

	ExportPriceUnit  = "unit"
	ExportPriceTotal = "total"
	ExportPriceBoth  = "both"

	ExportStatusPending    = "pending"
	ExportStatusProcessing = "processing"
	ExportStatusSucceeded  = "succeeded"
	ExportStatusFailed     = "failed"
)

// ExportBundleSelection 导出捆时选择的主品与捆品，与下单预览一致。
type ExportBundleSelection struct {
	MainProductID string               `json:"main_product_id"`
	MainQuantity  int                  `json:"main_quantity"`
	SubItems      []BundleOrderSubItem `json:"sub_items"`
}

// ExportItemRequest 平铺图导出单项。
type ExportItemRequest struct {
	Type            string                 `json:"type"`
	ID              string                 `json:"id"`
	Quantity        int                    `json:"quantity"`
	BundleSelection *ExportBundleSelection `json:"bundle_selection,omitempty"`
}

// ExportRequest 平铺图导出请求。
type ExportRequest struct {
	Items        []ExportItemRequest `json:"items"`
	Columns      int                 `json:"columns"`
	Width        int                 `json:"width"`
	Background   string              `json:"background"`
	TextColor    string              `json:"text_color"`
	Format       string              `json:"format"`
	ShowName     *bool               `json:"show_name"`
	ShowPrice    *bool               `json:"show_price"`
	ShowQuantity *bool               `json:"show_quantity"`
	PriceDisplay string              `json:"price_display"`
	Title        string              `json:"title"`
}

// Normalize 填充默认值并约束取值范围。
func (r *ExportRequest) Normalize(defaultColumns, maxWidth int) {
	if defaultColumns < 3 || defaultColumns > 5 {
		defaultColumns = 4
	}
	if r.Columns < 3 || r.Columns > 5 {
		r.Columns = defaultColumns
	}
	if maxWidth <= 0 {
		maxWidth = 3072
	}
	if r.Width < 1024 {
		r.Width = 2048
	}
	if r.Width > maxWidth {
		r.Width = maxWidth
	}

	switch strings.ToLower(strings.TrimSpace(r.Background)) {
	case ExportBackgroundBlack, ExportBackgroundWhite, ExportBackgroundTransparent:
		r.Background = strings.ToLower(strings.TrimSpace(r.Background))
	default:
		r.Background = ExportBackgroundWhite
	}
	switch strings.ToLower(strings.TrimSpace(r.TextColor)) {
	case ExportTextColorDark, ExportTextColorLight:
		r.TextColor = strings.ToLower(strings.TrimSpace(r.TextColor))
	default:
		r.TextColor = ExportTextColorAuto
	}
	switch strings.ToLower(strings.TrimSpace(r.Format)) {
	case ExportFormatJPEG, "jpg":
		r.Format = ExportFormatJPEG
	default:
		r.Format = ExportFormatPNG
	}
	switch strings.ToLower(strings.TrimSpace(r.PriceDisplay)) {
	case ExportPriceUnit, ExportPriceBoth:
		r.PriceDisplay = strings.ToLower(strings.TrimSpace(r.PriceDisplay))
	default:
		r.PriceDisplay = ExportPriceTotal
	}
	if r.ShowName == nil {
		r.ShowName = boolPtr(true)
	}
	if r.ShowPrice == nil {
		r.ShowPrice = boolPtr(true)
	}
	if r.ShowQuantity == nil {
		r.ShowQuantity = boolPtr(true)
	}
	r.Title = strings.TrimSpace(r.Title)

	for i := range r.Items {
		item := &r.Items[i]
		item.Type = strings.ToLower(strings.TrimSpace(item.Type))
		if item.Type == "" {
			item.Type = ExportTypeProduct
		}
		item.ID = strings.TrimSpace(item.ID)
		if item.Quantity <= 0 {
			item.Quantity = 1
		}
	}
}

func boolPtr(v bool) *bool { return &v }

// ExportComponent 捆/组合包的组件明细。
type ExportComponent struct {
	Name     string  `json:"name"`
	Quantity int     `json:"quantity"`
	Amount   float64 `json:"amount"`
}

// ExportLine 导出预览中的一行，价格由服务端计算，前端只展示。
type ExportLine struct {
	Type       string            `json:"type"`
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Quantity   int               `json:"quantity"`
	UnitPrice  float64           `json:"unit_price"`
	Amount     float64           `json:"amount"`
	PriceMode  string            `json:"price_mode"`
	ImageURL   string            `json:"image_url,omitempty"`
	Components []ExportComponent `json:"components,omitempty"`
}

// ExportPreview 导出预览结果。
type ExportPreview struct {
	Lines         []ExportLine `json:"lines"`
	TotalAmount   float64      `json:"total_amount"`
	TotalQuantity int          `json:"total_quantity"`
	Columns       int          `json:"columns"`
	Width         int          `json:"width"`
	Background    string       `json:"background"`
	Format        string       `json:"format"`
}

// ExportJob 导出任务。
type ExportJob struct {
	ID          string          `json:"id"`
	InventoryID string          `json:"inventory_id"`
	Status      string          `json:"status"`
	Request     json.RawMessage `json:"request"`
	ResultURL   *string         `json:"result_url,omitempty"`
	Error       *string         `json:"error,omitempty"`
	CreatedBy   *int64          `json:"created_by,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	FinishedAt  *time.Time      `json:"finished_at,omitempty"`
	ExpiresAt   *time.Time      `json:"expires_at,omitempty"`
}

// ExportOptions 导出运行参数，由配置注入。
type ExportOptions struct {
	MaxItems       int
	MaxWidth       int
	DefaultColumns int
	FontPath       string
	TempDir        string
	Timeout        time.Duration
}

// ExportTemplate 导出模板：只保存样式配置，不含商品明细。
type ExportTemplate struct {
	ID          string          `json:"id"`
	InventoryID *string         `json:"inventory_id,omitempty"`
	Name        string          `json:"name"`
	Config      json.RawMessage `json:"config"`
	IsDefault   bool            `json:"is_default"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// ExportTemplateRequest 新建/更新导出模板。
type ExportTemplateRequest struct {
	InventoryID string          `json:"inventory_id"`
	Name        string          `json:"name"`
	Config      json.RawMessage `json:"config"`
	IsDefault   bool            `json:"is_default"`
}

// normalize 约束导出参数，配置缺省时使用安全默认值。
func (o ExportOptions) normalize() ExportOptions {
	if o.MaxItems <= 0 || o.MaxItems > 500 {
		o.MaxItems = 200
	}
	if o.MaxWidth < 1024 {
		o.MaxWidth = 3072
	}
	if o.DefaultColumns < 3 || o.DefaultColumns > 5 {
		o.DefaultColumns = 4
	}
	if o.Timeout <= 0 {
		o.Timeout = 120 * time.Second
	}
	return o
}
