package goodser

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Allinost/go-backend-core/internal/services/storage"
	"github.com/google/uuid"
)

const (
	recognitionTopK       = 5
	recognitionImageLimit = 20 << 20
	recognitionCacheTTL   = 5 * time.Minute
	recognitionWorkers    = 4
)

type embeddingCacheEntry struct {
	rows    []ProductEmbedding
	expires time.Time
}

var recognitionEmbeddingCache sync.Map // inventoryID -> embeddingCacheEntry

// CreateRecognition 创建识别任务：调用 /v1/analyze，逐框查相似商品并聚合。
func (s *Service) CreateRecognition(ctx context.Context, inventoryID string, image []byte, imageURL, mode string, createdBy int64) (*RecognitionJob, error) {
	if _, err := s.GetInventory(ctx, inventoryID); err != nil {
		return nil, err
	}
	mode = normalizeRecognitionMode(mode)
	if len(image) == 0 {
		if strings.TrimSpace(imageURL) == "" {
			return nil, NewGoodserError(ErrRecognitionImage, "缺少图片或 image_url")
		}
		data, err := downloadImageBytes(ctx, imageURL)
		if err != nil {
			return nil, NewGoodserError(ErrRecognitionImage, err.Error())
		}
		image = data
	}

	job := &RecognitionJob{
		ID:          uuid.NewString(),
		InventoryID: inventoryID,
		ImageURL:    strings.TrimSpace(imageURL),
		Mode:        mode,
		Status:      RecognitionStatusProcessing,
		CreatedAt:   time.Now().UTC(),
	}
	if createdBy > 0 {
		job.CreatedBy = &createdBy
	}
	// 上传原图，便于事后复核；失败不阻塞识别。
	if s.store != nil && len(image) > 0 {
		contentType := http.DetectContentType(image)
		ext := ".jpg"
		if strings.Contains(contentType, "png") {
			ext = ".png"
		} else if strings.Contains(contentType, "webp") {
			ext = ".webp"
		}
		key := "goodser/recognitions/" + job.ID + ext
		if _, err := s.store.Upload(ctx, key, bytes.NewReader(image), storage.WithContentType(contentType)); err == nil {
			if signed, err := s.store.SignedURL(ctx, key, exportURLLifetime); err == nil {
				job.ImageURL = stripQuery(signed)
			}
		}
	}
	if err := s.repo.CreateRecognitionJob(ctx, job); err != nil {
		return nil, err
	}

	analysis, err := s.vision.Analyze(ctx, image)
	if err != nil {
		s.failRecognition(ctx, job.ID, err)
		return nil, NewGoodserError(ErrRecognitionVision, err.Error())
	}

	items := make([]RecognitionItem, 0, len(analysis.Items))
	for _, detected := range analysis.Items {
		box, _ := json.Marshal(detected.Box)
		item := RecognitionItem{
			JobID:     job.ID,
			Box:       box,
			CropURL:   detected.CropURL,
			Quantity:  1,
			CreatedAt: time.Now().UTC(),
		}
		if len(detected.Vector) > 0 {
			candidates, err := s.searchSimilarProducts(ctx, inventoryID, detected.Vector, recognitionTopK)
			if err == nil && len(candidates) > 0 {
				productID := candidates[0].ProductID
				item.ProductID = &productID
				item.Confidence = candidates[0].Score
				item.MatchedBy = "image"
				item.Alternatives, _ = json.Marshal(candidates)
			}
		}
		if err := s.repo.CreateRecognitionItem(ctx, &item); err != nil {
			s.failRecognition(ctx, job.ID, err)
			return nil, err
		}
		items = append(items, item)
	}

	groups := s.buildRecognitionGroups(ctx, items)
	result, _ := json.Marshal(map[string]any{
		"model":      analysis.Model,
		"elapsed_ms": analysis.ElapsedMS,
		"groups":     groups,
	})
	now := time.Now().UTC()
	if err := s.repo.UpdateRecognitionJob(ctx, job.ID, RecognitionStatusSucceeded, result, nil, &now); err != nil {
		return nil, err
	}
	return s.GetRecognition(ctx, job.ID)
}

// GetRecognition 读取识别任务及识别项/聚合结果。
func (s *Service) GetRecognition(ctx context.Context, id string) (*RecognitionJob, error) {
	job, err := s.repo.GetRecognitionJob(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, NewGoodserError(ErrRecognitionNotFound)
	}
	if err != nil {
		return nil, err
	}
	items, err := s.repo.ListRecognitionItems(ctx, id)
	if err != nil {
		return nil, err
	}
	job.Items = items
	job.Groups = s.buildRecognitionGroups(ctx, items)
	return job, nil
}

// ListRecognitionJobs 识别历史。
func (s *Service) ListRecognitionJobs(ctx context.Context, inventoryID string, limit int) ([]RecognitionJob, error) {
	return s.repo.ListRecognitionJobs(ctx, inventoryID, limit)
}

// ConfirmRecognitionItem 人工确认/修改单个识别项。
func (s *Service) ConfirmRecognitionItem(ctx context.Context, jobID string, itemID int64, productID *string, quantity *int, confirmed *bool) (*RecognitionJob, error) {
	if _, err := s.GetRecognition(ctx, jobID); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateRecognitionItem(ctx, jobID, itemID, productID, quantity, confirmed); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, NewGoodserError(ErrRecognitionItemNotFound)
		}
		return nil, err
	}
	return s.GetRecognition(ctx, jobID)
}

// ApplyRecognition 提交复核结果并标记任务为 applied。
// 首期不直接改库存：返回聚合后的入库草稿，由客户端走现有入库接口或后续实现批量入库。
func (s *Service) ApplyRecognition(ctx context.Context, jobID string, req *RecognitionApplyRequest) (*RecognitionJob, error) {
	job, err := s.GetRecognition(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if req != nil {
		for _, item := range req.Items {
			if err := s.repo.UpdateRecognitionItem(ctx, jobID, item.ItemID, item.ProductID, item.Quantity, item.Confirmed); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return nil, NewGoodserError(ErrRecognitionItemNotFound)
				}
				return nil, err
			}
		}
		job, err = s.GetRecognition(ctx, jobID)
		if err != nil {
			return nil, err
		}
	}

	draft := make([]RecognitionGroup, 0)
	for _, group := range job.Groups {
		confirmed := false
		for _, item := range job.Items {
			if item.ProductID == nil || *item.ProductID != group.ProductID {
				continue
			}
			if item.Confirmed {
				confirmed = true
				break
			}
		}
		if confirmed {
			draft = append(draft, group)
		}
	}
	if len(draft) == 0 {
		return nil, NewGoodserError(ErrRecognitionNoConfirmed, "请先确认至少一个识别项")
	}

	result, _ := json.Marshal(map[string]any{
		"groups":     draft,
		"applied_at": time.Now().UTC(),
	})
	now := time.Now().UTC()
	if err := s.repo.UpdateRecognitionJob(ctx, job.ID, RecognitionStatusApplied, result, nil, &now); err != nil {
		return nil, err
	}
	return s.GetRecognition(ctx, jobID)
}

// RebuildEmbeddings 重建某库存目录下商品图片向量。
func (s *Service) RebuildEmbeddings(ctx context.Context, inventoryID string) (*EmbeddingRebuildResult, error) {
	if _, err := s.GetInventory(ctx, inventoryID); err != nil {
		return nil, err
	}
	products, err := s.repo.ListProducts(ctx, inventoryID)
	if err != nil {
		return nil, err
	}
	type target struct {
		product  *Product
		imageURL string
	}
	targets := make([]target, 0, len(products))
	for i := range products {
		url := productImageURL(&products[i])
		if url == "" {
			continue
		}
		targets = append(targets, target{product: &products[i], imageURL: url})
	}
	if err := s.repo.DeleteProductEmbeddings(ctx, inventoryID); err != nil {
		return nil, err
	}

	result := &EmbeddingRebuildResult{Total: len(targets)}
	var (
		mu              sync.Mutex
		wg              sync.WaitGroup
		limit           = make(chan struct{}, recognitionWorkers)
		model           string
		dim             int
		indexed, failed int
	)
	for _, t := range targets {
		wg.Add(1)
		go func(t target) {
			defer wg.Done()
			limit <- struct{}{}
			defer func() { <-limit }()
			if err := ctx.Err(); err != nil {
				mu.Lock()
				failed++
				mu.Unlock()
				return
			}
			image, err := downloadImageBytes(ctx, t.imageURL)
			if err != nil {
				mu.Lock()
				failed++
				mu.Unlock()
				return
			}
			embedding, err := s.vision.Embed(ctx, image)
			if err != nil {
				mu.Lock()
				failed++
				mu.Unlock()
				return
			}
			entry := &ProductEmbedding{
				InventoryID: inventoryID,
				SourceType:  EmbeddingSourceProduct,
				SourceID:    t.product.ID,
				ImageURL:    t.imageURL,
				Model:       embedding.Model,
				Dim:         embedding.Dim,
				Vector:      embedding.Vector,
			}
			if err := s.repo.UpsertProductEmbedding(ctx, entry); err != nil {
				mu.Lock()
				failed++
				mu.Unlock()
				return
			}
			mu.Lock()
			indexed++
			model = embedding.Model
			dim = embedding.Dim
			mu.Unlock()
		}(t)
	}
	wg.Wait()
	recognitionEmbeddingCache.Delete(inventoryID)
	result.Indexed = indexed
	result.Failed = failed
	result.Model = model
	result.Dim = dim
	return result, nil
}

// EmbeddingStats 返回当前索引数量。
func (s *Service) EmbeddingStats(ctx context.Context, inventoryID string) (map[string]any, error) {
	total, err := s.repo.CountProductEmbeddings(ctx, inventoryID)
	if err != nil {
		return nil, err
	}
	products, _ := s.repo.ListProducts(ctx, inventoryID)
	withImage := 0
	for i := range products {
		if productImageURL(&products[i]) != "" {
			withImage++
		}
	}
	return map[string]any{
		"products":            len(products),
		"products_with_image": withImage,
		"embeddings":          total,
	}, nil
}

// searchSimilarProducts 用 Go 侧余弦相似度做 Top-K。
// 当前向量存 jsonb（pgvector 未启用）；数据量增大后应切换到 pgvector HNSW。
func (s *Service) searchSimilarProducts(ctx context.Context, inventoryID string, vector []float64, topK int) ([]RecognitionCandidate, error) {
	rows, err := s.loadEmbeddings(ctx, inventoryID, false)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 || len(vector) == 0 {
		return []RecognitionCandidate{}, nil
	}
	type scored struct {
		row   ProductEmbedding
		score float64
	}
	scores := make([]scored, 0, len(rows))
	for _, row := range rows {
		if len(row.Vector) != len(vector) {
			continue
		}
		scores = append(scores, scored{row: row, score: cosineSimilarity(vector, row.Vector)})
	}
	sort.Slice(scores, func(i, j int) bool { return scores[i].score > scores[j].score })

	seen := make(map[string]bool)
	out := make([]RecognitionCandidate, 0, topK)
	for _, item := range scores {
		key := item.row.SourceType + ":" + item.row.SourceID
		if seen[key] {
			continue
		}
		seen[key] = true
		candidate := RecognitionCandidate{
			SourceType: item.row.SourceType,
			SourceID:   item.row.SourceID,
			ImageURL:   item.row.ImageURL,
			Score:      clamp01(item.score),
		}
		if item.row.SourceType == EmbeddingSourceProduct {
			candidate.ProductID = item.row.SourceID
			if product, err := s.GetProduct(ctx, item.row.SourceID); err == nil {
				candidate.Name = product.Name
				candidate.Code = product.Code
			}
		}
		out = append(out, candidate)
		if len(out) >= topK {
			break
		}
	}
	return out, nil
}

func (s *Service) loadEmbeddings(ctx context.Context, inventoryID string, force bool) ([]ProductEmbedding, error) {
	if !force {
		if cached, ok := recognitionEmbeddingCache.Load(inventoryID); ok {
			if entry, ok := cached.(embeddingCacheEntry); ok && time.Now().Before(entry.expires) {
				return entry.rows, nil
			}
		}
	}
	rows, err := s.repo.ListProductEmbeddings(ctx, inventoryID)
	if err != nil {
		return nil, err
	}
	recognitionEmbeddingCache.Store(inventoryID, embeddingCacheEntry{rows: rows, expires: time.Now().Add(recognitionCacheTTL)})
	return rows, nil
}

func (s *Service) buildRecognitionGroups(ctx context.Context, items []RecognitionItem) []RecognitionGroup {
	type groupAcc struct {
		group RecognitionGroup
	}
	accs := make(map[string]*groupAcc)
	order := make([]string, 0)
	for _, item := range items {
		if item.ProductID == nil || *item.ProductID == "" {
			continue
		}
		productID := *item.ProductID
		acc, ok := accs[productID]
		if !ok {
			group := RecognitionGroup{ProductID: productID}
			if name, code, image := candidateInfo(item.Alternatives, productID); name != "" || code != "" || image != "" {
				group.Name, group.Code, group.ImageURL = name, code, image
			} else if product, err := s.GetProduct(ctx, productID); err == nil {
				group.Name = product.Name
				group.Code = product.Code
				group.ImageURL = productImageURL(product)
			}
			acc = &groupAcc{group: group}
			accs[productID] = acc
			order = append(order, productID)
		}
		qty := item.Quantity
		if qty <= 0 {
			qty = 1
		}
		acc.group.Quantity += qty
		if item.Confidence > acc.group.Score {
			acc.group.Score = item.Confidence
		}
		acc.group.ItemIDs = append(acc.group.ItemIDs, item.ID)
	}
	out := make([]RecognitionGroup, 0, len(order))
	for _, productID := range order {
		out = append(out, accs[productID].group)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Quantity == out[j].Quantity {
			return out[i].Score > out[j].Score
		}
		return out[i].Quantity > out[j].Quantity
	})
	return out
}

func (s *Service) failRecognition(ctx context.Context, jobID string, err error) {
	msg := err.Error()
	if len(msg) > 500 {
		msg = msg[:500]
	}
	now := time.Now().UTC()
	_ = s.repo.UpdateRecognitionJob(context.Background(), jobID, RecognitionStatusFailed, nil, &msg, &now)
}

func candidateInfo(raw json.RawMessage, productID string) (string, string, string) {
	if len(raw) == 0 {
		return "", "", ""
	}
	var candidates []RecognitionCandidate
	if err := json.Unmarshal(raw, &candidates); err != nil {
		return "", "", ""
	}
	for _, candidate := range candidates {
		if candidate.ProductID == productID {
			return candidate.Name, candidate.Code, candidate.ImageURL
		}
	}
	if len(candidates) > 0 {
		return candidates[0].Name, candidates[0].Code, candidates[0].ImageURL
	}
	return "", "", ""
}

func normalizeRecognitionMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case RecognitionModeSimilarity:
		return RecognitionModeSimilarity
	default:
		return RecognitionModeMulti
	}
}

func downloadImageBytes(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(rawURL), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "go-backend-core/goodser")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, recognitionImageLimit))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("图片为空")
	}
	return data, nil
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
