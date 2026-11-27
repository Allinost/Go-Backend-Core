package goodser

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeVision struct {
	analyze *VisionAnalyzeResponse
	embed   *VisionEmbedResponse
}

func (f *fakeVision) Analyze(ctx context.Context, imageBytes []byte) (*VisionAnalyzeResponse, error) {
	return f.analyze, nil
}

func (f *fakeVision) Embed(ctx context.Context, imageBytes []byte) (*VisionEmbedResponse, error) {
	return f.embed, nil
}

func TestCosineSimilarity(t *testing.T) {
	require.InDelta(t, 1, cosineSimilarity([]float64{1, 0}, []float64{2, 0}), 1e-9)
	require.InDelta(t, 0, cosineSimilarity([]float64{1, 0}, []float64{0, 1}), 1e-9)
	require.Equal(t, float64(0), cosineSimilarity([]float64{1}, []float64{1, 2}))
	require.Equal(t, float64(0), cosineSimilarity(nil, nil))
}

// TestRecognitionCreateAndApply 覆盖：/v1/analyze 结果 → 向量 Top-K → 聚合 → 确认 → apply。
func TestRecognitionCreateAndApply(t *testing.T) {
	ctx := context.Background()
	repo := newPGRepository(t)
	svc := NewService(repo, nil)

	inv, err := repo.CreateInventory(ctx, &CreateInventoryReq{Name: "识别目录"})
	require.NoError(t, err)

	qty := 1
	tx, err := repo.BeginTx(ctx)
	require.NoError(t, err)
	p1, err := repo.CreateProductTx(ctx, tx, &CreateProductReq{
		InventoryID: inv.ID, Code: "R-01-0001-0000-A", MainZone: "R", SubZone: "01",
		SeqNumber: 1, Quantity: &qty, StatusCode: "A", Name: "识别商品一", ExpectedPrice: 10,
	})
	require.NoError(t, err)
	p2, err := repo.CreateProductTx(ctx, tx, &CreateProductReq{
		InventoryID: inv.ID, Code: "R-01-0002-0000-A", MainZone: "R", SubZone: "01",
		SeqNumber: 2, Quantity: &qty, StatusCode: "A", Name: "识别商品二", ExpectedPrice: 20,
	})
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	require.NoError(t, repo.UpsertProductEmbedding(ctx, &ProductEmbedding{
		InventoryID: inv.ID, SourceType: EmbeddingSourceProduct, SourceID: p1.ID,
		ImageURL: "https://example.com/p1.jpg", Model: "fake", Dim: 3, Vector: []float64{1, 0, 0},
	}))
	require.NoError(t, repo.UpsertProductEmbedding(ctx, &ProductEmbedding{
		InventoryID: inv.ID, SourceType: EmbeddingSourceProduct, SourceID: p2.ID,
		ImageURL: "https://example.com/p2.jpg", Model: "fake", Dim: 3, Vector: []float64{0, 1, 0},
	}))

	svc.SetVisionClient(&fakeVision{analyze: &VisionAnalyzeResponse{
		Model:     map[string]any{"embedder": "fake", "dim": 3},
		ElapsedMS: 12,
		Items: []VisionAnalyzeItem{{
			Box:    VisionBox{X: 10, Y: 10, W: 100, H: 100, Score: 0.98},
			Vector: []float64{0.9, 0.1, 0},
		}},
	}})

	job, err := svc.CreateRecognition(ctx, inv.ID, []byte("fake-image"), "https://example.com/scan.jpg", "multi", 0)
	require.NoError(t, err)
	require.Equal(t, RecognitionStatusSucceeded, job.Status)
	require.Len(t, job.Items, 1)
	require.NotNil(t, job.Items[0].ProductID)
	require.Equal(t, p1.ID, *job.Items[0].ProductID)
	require.Len(t, job.Groups, 1)
	require.Equal(t, p1.ID, job.Groups[0].ProductID)
	require.Equal(t, 1, job.Groups[0].Quantity)
	require.Equal(t, "识别商品一", job.Groups[0].Name)

	// 确认识别项后 apply
	confirmed := true
	updated, err := svc.ConfirmRecognitionItem(ctx, job.ID, job.Items[0].ID, nil, nil, &confirmed)
	require.NoError(t, err)
	require.True(t, updated.Items[0].Confirmed)

	applied, err := svc.ApplyRecognition(ctx, job.ID, &RecognitionApplyRequest{})
	require.NoError(t, err)
	require.Equal(t, RecognitionStatusApplied, applied.Status)
	require.NotEmpty(t, applied.Groups)

	// 未确认时不应允许 apply
	job2, err := svc.CreateRecognition(ctx, inv.ID, []byte("fake-image"), "https://example.com/scan2.jpg", "multi", 0)
	require.NoError(t, err)
	_, err = svc.ApplyRecognition(ctx, job2.ID, &RecognitionApplyRequest{})
	require.Error(t, err)
}

// TestRebuildEmbeddings 覆盖图片下载 + /v1/embed + jsonb 向量写入。
func TestRebuildEmbeddings(t *testing.T) {
	ctx := context.Background()
	repo := newPGRepository(t)
	svc := NewService(repo, nil)

	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(buf.Bytes())
	}))
	defer server.Close()

	inv, err := repo.CreateInventory(ctx, &CreateInventoryReq{Name: "向量目录"})
	require.NoError(t, err)

	imageURL := server.URL + "/p1.png"
	qty := 1
	tx, err := repo.BeginTx(ctx)
	require.NoError(t, err)
	product, err := repo.CreateProductTx(ctx, tx, &CreateProductReq{
		InventoryID: inv.ID, Code: "V-01-0001-0000-A", MainZone: "V", SubZone: "01",
		SeqNumber: 1, Quantity: &qty, StatusCode: "A", Name: "向量商品", ImageURL: &imageURL,
	})
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	svc.SetVisionClient(&fakeVision{embed: &VisionEmbedResponse{
		Model: "fake-clip", Dim: 3, Vector: []float64{0.2, 0.3, 0.5},
	}})

	result, err := svc.RebuildEmbeddings(ctx, inv.ID)
	require.NoError(t, err)
	require.Equal(t, 1, result.Total)
	require.Equal(t, 1, result.Indexed)
	require.Equal(t, 0, result.Failed)
	require.Equal(t, "fake-clip", result.Model)

	count, err := repo.CountProductEmbeddings(ctx, inv.ID)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	rows, err := repo.ListProductEmbeddings(ctx, inv.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, product.ID, rows[0].SourceID)
	require.Len(t, rows[0].Vector, 3)

	// 索引统计
	stats, err := svc.EmbeddingStats(ctx, inv.ID)
	require.NoError(t, err)
	require.Equal(t, 1, stats["embeddings"])
	require.Equal(t, 1, stats["products_with_image"])
}

// TestRecognitionVisionFailure 视觉不可用时应落库失败任务。
func TestRecognitionVisionFailure(t *testing.T) {
	ctx := context.Background()
	repo := newPGRepository(t)
	svc := NewService(repo, nil)
	inv, err := repo.CreateInventory(ctx, &CreateInventoryReq{Name: "失败目录"})
	require.NoError(t, err)

	svc.SetVisionClient(&failingVision{})
	_, err = svc.CreateRecognition(ctx, inv.ID, []byte("x"), "https://example.com/a.jpg", "multi", 0)
	require.Error(t, err)

	jobs, err := repo.ListRecognitionJobs(ctx, inv.ID, 10)
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.Equal(t, RecognitionStatusFailed, jobs[0].Status)
	require.NotNil(t, jobs[0].Error)
}

type failingVision struct{}

func (f *failingVision) Analyze(ctx context.Context, image []byte) (*VisionAnalyzeResponse, error) {
	return nil, errors.New("vision down")
}
func (f *failingVision) Embed(ctx context.Context, image []byte) (*VisionEmbedResponse, error) {
	return nil, errors.New("vision down")
}
