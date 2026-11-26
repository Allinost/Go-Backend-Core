package goodser

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Allinost/go-backend-core/internal/testutil"
)

// newPGRepository 在临时 schema 上应用全部 PG 迁移，返回 PG 版 Repository。
// 未设置 TEST_DATABASE_URL 时 testutil.OpenPG 会跳过测试。
func newPGRepository(t *testing.T) *Repository {
	t.Helper()
	db := testutil.OpenPG(t)
	require.NoError(t, testutil.ApplyMigrations(t, db))
	return NewRepository(db)
}

func TestRepositoryInventoryCRUD(t *testing.T) {
	ctx := context.Background()
	repo := newPGRepository(t)

	inv, err := repo.CreateInventory(ctx, &CreateInventoryReq{Name: "主目录", SortOrder: 1})
	require.NoError(t, err)
	require.NotEmpty(t, inv.ID)

	list, err := repo.ListInventories(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)

	got, err := repo.GetInventory(ctx, inv.ID)
	require.NoError(t, err)
	require.Equal(t, "主目录", got.Name)

	name := "改名"
	updated, err := repo.UpdateInventory(ctx, &UpdateInventoryReq{ID: inv.ID, Name: &name})
	require.NoError(t, err)
	require.Equal(t, "改名", updated.Name)

	require.NoError(t, repo.DeleteInventory(ctx, inv.ID))
	list, err = repo.ListInventories(ctx)
	require.NoError(t, err)
	require.Empty(t, list)
}

// TestRepositorySeqCounter 覆盖 ON CONFLICT 的计数器自增与取大值语义。
func TestRepositorySeqCounter(t *testing.T) {
	ctx := context.Background()
	repo := newPGRepository(t)

	tx, err := repo.BeginTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	seq, err := repo.IncrementSeqCounter(ctx, tx, "inv-1", "A", "01")
	require.NoError(t, err)
	require.Equal(t, 1, seq)

	seq, err = repo.IncrementSeqCounter(ctx, tx, "inv-1", "A", "01")
	require.NoError(t, err)
	require.Equal(t, 2, seq)

	require.NoError(t, repo.EnsureSeqCounter(ctx, tx, "inv-1", "A", "01", 10))
	seq, err = repo.IncrementSeqCounter(ctx, tx, "inv-1", "A", "01")
	require.NoError(t, err)
	require.Equal(t, 11, seq)

	require.NoError(t, tx.Commit())
}

// TestRepositoryProductSearchAndStats 覆盖 jsonb 标签包含、ILIKE 关键字、成本价与库存统计。
func TestRepositoryProductSearchAndStats(t *testing.T) {
	ctx := context.Background()
	repo := newPGRepository(t)

	inv, err := repo.CreateInventory(ctx, &CreateInventoryReq{Name: "目录"})
	require.NoError(t, err)

	qty := 2
	tags := []string{"限量", "徽章"}
	tx, err := repo.BeginTx(ctx)
	require.NoError(t, err)
	product, err := repo.CreateProductTx(ctx, tx, &CreateProductReq{
		InventoryID:   inv.ID,
		Code:          "A-01-0001-0000-C",
		MainZone:      "A",
		SubZone:       "01",
		SeqNumber:     1,
		Quantity:      &qty,
		StatusCode:    "C",
		Name:          "测试徽章",
		CostPrice:     5,
		ExpectedPrice: 12.5,
		Tags:          &tags,
	})
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	// jsonb 标签包含查询
	req := &QueryProductsReq{InventoryID: inv.ID, TagID: "限量"}
	req.Normalize()
	items, hasMore, total, err := repo.SearchProductsPaginated(ctx, req)
	require.NoError(t, err)
	require.False(t, hasMore)
	require.Equal(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, product.ID, items[0].ID)

	// 不存在的标签
	missing := &QueryProductsReq{InventoryID: inv.ID, TagID: "不存在"}
	missing.Normalize()
	items, _, total, err = repo.SearchProductsPaginated(ctx, missing)
	require.NoError(t, err)
	require.Equal(t, 0, total)
	require.Empty(t, items)

	// 关键字大小写不敏感（ILIKE）
	keyword := &QueryProductsReq{InventoryID: inv.ID, Keyword: "a-01"}
	keyword.Normalize()
	items, _, total, err = repo.SearchProductsPaginated(ctx, keyword)
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, items, 1)

	// 成本价在创建时写入
	got, err := repo.GetProduct(ctx, product.ID)
	require.NoError(t, err)
	require.InDelta(t, 5, got.CostPrice, 1e-9)

	// 库存统计
	stats, err := repo.StatsStock(ctx, inv.ID)
	require.NoError(t, err)
	require.Equal(t, 1, stats.TotalProducts)
	require.Equal(t, 2, stats.Available)
	require.Equal(t, 2, stats.Total)
}

// TestRepositoryStatsTrend 覆盖 timestamptz 按上海时区按天聚合，避免 UTC 跨日。
func TestRepositoryStatsTrend(t *testing.T) {
	ctx := context.Background()
	repo := newPGRepository(t)

	inv, err := repo.CreateInventory(ctx, &CreateInventoryReq{Name: "目录"})
	require.NoError(t, err)

	cst := time.FixedZone("CST", 8*3600)
	tx, err := repo.BeginTx(ctx)
	require.NoError(t, err)
	outbound := &StockMovement{
		InventoryID:   inv.ID,
		ProductID:     "p-1",
		ReservedDelta: -1,
		Reason:        StockReasonOutboundConfirm,
		CreatedAt:     time.Date(2026, 11, 26, 1, 0, 0, 0, cst), // UTC 2026-11-25 17:00
	}
	restock := &StockMovement{
		InventoryID:   inv.ID,
		ProductID:     "p-1",
		QuantityDelta: 1,
		Reason:        StockReasonReturnRestock,
		CreatedAt:     time.Date(2026, 11, 26, 10, 0, 0, 0, cst),
	}
	require.NoError(t, repo.AddStockMovementTx(ctx, tx, outbound))
	require.NoError(t, repo.AddStockMovementTx(ctx, tx, restock))
	require.NoError(t, tx.Commit())

	points, err := repo.StatsTrend(ctx, inv.ID, time.Date(2026, 11, 26, 0, 0, 0, 0, cst))
	require.NoError(t, err)
	require.Len(t, points, 1)
	require.Equal(t, "2026-11-26", points[0].Date)
	require.Equal(t, 1, points[0].OutboundQuantity)
	require.Equal(t, 1, points[0].ReturnQuantity)

	// HAVING 使用聚合表达式（PG 不支持 HAVING 引用 SELECT 别名）
	since := time.Date(2026, 11, 26, 0, 0, 0, 0, cst)
	profitRows, err := repo.StatsProfit(ctx, inv.ID, since)
	require.NoError(t, err)
	require.Len(t, profitRows, 1)
	require.Equal(t, 1, profitRows[0].OutboundQty)

	top, err := repo.StatsTopProducts(ctx, inv.ID, since, 5)
	require.NoError(t, err)
	require.Len(t, top, 1)
	require.Equal(t, 1, top[0].OutboundQuantity)
}
