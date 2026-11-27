package goodser

import (
	"encoding/json"
	"time"
)

// 识别状态
const (
	RecognitionStatusPending    = "pending"
	RecognitionStatusProcessing = "processing"
	RecognitionStatusSucceeded  = "succeeded"
	RecognitionStatusFailed     = "failed"
	RecognitionStatusApplied    = "applied"
)

// 识别模式
const (
	RecognitionModeMulti      = "multi"
	RecognitionModeSimilarity = "similarity"
)

// 向量来源
const (
	EmbeddingSourceProduct     = "product"
	EmbeddingSourceCatalogItem = "catalog_item"
)

// RecognitionCandidate 单个检测框的相似商品候选。
type RecognitionCandidate struct {
	ProductID  string  `json:"product_id,omitempty"`
	SourceType string  `json:"source_type"`
	SourceID   string  `json:"source_id"`
	Name       string  `json:"name,omitempty"`
	Code       string  `json:"code,omitempty"`
	ImageURL   string  `json:"image_url,omitempty"`
	Score      float64 `json:"score"`
}

// RecognitionItem 识别项（一个检测框一条）。
type RecognitionItem struct {
	ID            int64           `json:"id"`
	JobID         string          `json:"job_id"`
	Box           json.RawMessage `json:"box,omitempty"`
	CropURL       string          `json:"crop_url,omitempty"`
	ProductID     *string         `json:"product_id,omitempty"`
	CatalogItemID *string         `json:"catalog_item_id,omitempty"`
	Quantity      int             `json:"quantity"`
	Confidence    float64         `json:"confidence"`
	MatchedBy     string          `json:"matched_by,omitempty"`
	Alternatives  json.RawMessage `json:"alternatives,omitempty"`
	Confirmed     bool            `json:"confirmed"`
	CreatedAt     time.Time       `json:"created_at"`
}

// RecognitionGroup 按商品聚合后的识别结果。
type RecognitionGroup struct {
	ProductID string  `json:"product_id"`
	Name      string  `json:"name,omitempty"`
	Code      string  `json:"code,omitempty"`
	ImageURL  string  `json:"image_url,omitempty"`
	Quantity  int     `json:"quantity"`
	Score     float64 `json:"score"`
	ItemIDs   []int64 `json:"item_ids"`
}

// RecognitionJob 识别任务。
type RecognitionJob struct {
	ID          string             `json:"id"`
	InventoryID string             `json:"inventory_id"`
	ImageURL    string             `json:"image_url"`
	Mode        string             `json:"mode"`
	Status      string             `json:"status"`
	Result      json.RawMessage    `json:"result,omitempty"`
	Error       *string            `json:"error,omitempty"`
	CreatedBy   *int64             `json:"created_by,omitempty"`
	CreatedAt   time.Time          `json:"created_at"`
	FinishedAt  *time.Time         `json:"finished_at,omitempty"`
	Items       []RecognitionItem  `json:"items,omitempty"`
	Groups      []RecognitionGroup `json:"groups,omitempty"`
}

// RecognitionApplyItem 复核时修改单个识别项。
type RecognitionApplyItem struct {
	ItemID    int64   `json:"item_id"`
	ProductID *string `json:"product_id,omitempty"`
	Quantity  *int    `json:"quantity,omitempty"`
	Confirmed *bool   `json:"confirmed,omitempty"`
}

// RecognitionApplyRequest 提交复核结果。
type RecognitionApplyRequest struct {
	Items []RecognitionApplyItem `json:"items"`
}

// ProductEmbedding 图片向量（当前以 jsonb 存储，pgvector 就绪后可迁移为 vector 列）。
type ProductEmbedding struct {
	ID          int64
	InventoryID string
	SourceType  string
	SourceID    string
	ImageURL    string
	Model       string
	Dim         int
	Vector      []float64
}

// EmbeddingRebuildResult 向量重建结果。
type EmbeddingRebuildResult struct {
	Total   int    `json:"total"`
	Indexed int    `json:"indexed"`
	Failed  int    `json:"failed"`
	Model   string `json:"model,omitempty"`
	Dim     int    `json:"dim,omitempty"`
}
