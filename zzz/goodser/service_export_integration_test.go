package goodser

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestExportJobsAndPreview 覆盖导出任务 CRUD、直接商品计价与失败状态回写。
func TestExportJobsAndPreview(t *testing.T) {
	ctx := context.Background()
	repo := newPGRepository(t)
	svc := NewService(repo, nil)
	svc.SetExportOptions(ExportOptions{MaxItems: 10, MaxWidth: 2048, DefaultColumns: 4})

	inv, err := repo.CreateInventory(ctx, &CreateInventoryReq{Name: "导出目录"})
	require.NoError(t, err)

	qty := 3
	tags := []string{"导出"}
	tx, err := repo.BeginTx(ctx)
	require.NoError(t, err)
	product, err := repo.CreateProductTx(ctx, tx, &CreateProductReq{
		InventoryID:   inv.ID,
		Code:          "E-01-0001-0000-C",
		MainZone:      "E",
		SubZone:       "01",
		SeqNumber:     1,
		Quantity:      &qty,
		StatusCode:    "C",
		Name:          "导出徽章",
		ExpectedPrice: 12.5,
		CostPrice:     5,
		Tags:          &tags,
	})
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	req := &ExportRequest{
		Items:      []ExportItemRequest{{Type: ExportTypeProduct, ID: product.ID, Quantity: 2}},
		Background: ExportBackgroundBlack,
		Format:     ExportFormatPNG,
	}
	preview, err := svc.PreviewExport(ctx, inv.ID, req)
	require.NoError(t, err)
	require.Len(t, preview.Lines, 1)
	require.Equal(t, 12.5, preview.Lines[0].UnitPrice)
	require.Equal(t, 25.0, preview.Lines[0].Amount)
	require.Equal(t, 2, preview.TotalQuantity)
	require.Equal(t, 25.0, preview.TotalAmount)

	// 库存目录不匹配
	other := &ExportRequest{Items: []ExportItemRequest{{Type: ExportTypeProduct, ID: product.ID, Quantity: 1}}}
	_, err = svc.PreviewExport(ctx, "other-inventory", other)
	require.Error(t, err)

	// 任务 CRUD
	job := &ExportJob{
		ID:          uuid.NewString(),
		InventoryID: inv.ID,
		Status:      ExportStatusPending,
		Request:     json.RawMessage(`{"items":[]}`),
		CreatedAt:   time.Now().UTC(),
	}
	require.NoError(t, repo.CreateExportJob(ctx, job))

	got, err := repo.GetExportJob(ctx, job.ID)
	require.NoError(t, err)
	require.Equal(t, ExportStatusPending, got.Status)

	list, err := repo.ListExportJobs(ctx, inv.ID, 10)
	require.NoError(t, err)
	require.Len(t, list, 1)

	// 没有存储时，同步导出应把任务标记为 failed，而不是返回 500
	created, err := svc.CreateExport(ctx, inv.ID, req, 0)
	require.NoError(t, err)
	require.Equal(t, ExportStatusFailed, created.Status)
	require.NotNil(t, created.Error)
	require.Nil(t, created.ResultURL)

	require.NoError(t, svc.DeleteExport(ctx, job.ID))
	_, err = repo.GetExportJob(ctx, job.ID)
	require.Error(t, err)
}
