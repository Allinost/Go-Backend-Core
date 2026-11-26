package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Allinost/go-backend-core/internal/testutil"
)

func TestGoodserTableSpecs(t *testing.T) {
	const want = 19
	seen := make(map[string]bool)
	var got int
	for _, spec := range tableSpecs {
		if !strings.HasPrefix(spec.Source, "zzz_goodser_") {
			continue
		}
		got++
		if spec.Source != spec.Target {
			t.Errorf("Goodser 源/目标表名不一致: %s -> %s", spec.Source, spec.Target)
		}
		if seen[spec.Target] {
			t.Errorf("Goodser 表重复: %s", spec.Target)
		}
		seen[spec.Target] = true
	}
	if got != want {
		t.Fatalf("Goodser 表数量 = %d, 期望 %d", got, want)
	}
}

func TestGoodserTargetsExistInMigration(t *testing.T) {
	path := filepath.Join("..", "..", "migrations", "postgres", "008_goodser.sql")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取迁移失败: %v", err)
	}
	sql := string(data)
	for _, spec := range tableSpecs {
		if !strings.HasPrefix(spec.Target, "zzz_goodser_") {
			continue
		}
		if !strings.Contains(sql, "CREATE TABLE IF NOT EXISTS "+spec.Target) {
			t.Errorf("迁移 008 缺少表: %s", spec.Target)
		}
	}
}

func TestSourceWallTime(t *testing.T) {
	old := sourceLocation
	sourceLocation = time.FixedZone("CST", 8*3600)
	defer func() { sourceLocation = old }()

	base := time.Date(2026, 11, 26, 2, 0, 0, 0, time.UTC)

	cases := []struct {
		name string
		in   any
		want time.Time
		ok   bool
	}{
		{"bytes", []byte("2026-11-26 10:00:00"), base, true},
		{"string", "2026-11-26 10:00:00", base, true},
		{"fraction", "2026-11-26 10:00:00.500", base.Add(500 * time.Millisecond), true},
		{"time", time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC), base, true},
		{"zero-date", "0000-00-00 00:00:00", time.Time{}, false},
		{"empty", "", time.Time{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := sourceWallTime(tc.in)
			if ok != tc.ok {
				t.Fatalf("ok = %v, 期望 %v", ok, tc.ok)
			}
			if !ok {
				return
			}
			if !got.Equal(tc.want) {
				t.Fatalf("got %s, 期望 %s", got.UTC(), tc.want)
			}
		})
	}
}

func TestNormalizeValueTimestamptz(t *testing.T) {
	old := sourceLocation
	sourceLocation = time.FixedZone("CST", 8*3600)
	defer func() { sourceLocation = old }()

	got, ok := normalizeValue([]byte("2026-11-26 10:00:00"), "timestamptz").(time.Time)
	if !ok {
		t.Fatal("timestamptz 未转换为 time.Time")
	}
	want := time.Date(2026, 11, 26, 2, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %s, 期望 %s", got.UTC(), want)
	}
}

func TestNormalizeMigratedImageURL(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want any
	}{
		{
			name: "presigned",
			in:   []byte("https://fs.example.com/goodser/images/a.png?X-Amz-Signature=abc&X-Amz-Expires=604800"),
			want: "https://fs.example.com/goodser/images/a.png",
		},
		{
			name: "fragment",
			in:   "https://fs.example.com/goodser/images/a.png#frag",
			want: "https://fs.example.com/goodser/images/a.png",
		},
		{
			name: "other-host",
			in:   "https://cdn.example.com/a.png?x=1",
			want: "https://cdn.example.com/a.png?x=1",
		},
		{
			name: "nil",
			in:   nil,
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeMigratedImageURL(tc.in)
			if got != tc.want {
				t.Fatalf("got %#v, 期望 %#v", got, tc.want)
			}
		})
	}
}

// TestLoadGoodserRows 用合成 dump 验证 loadData 的 Goodser 处理：
// 时区换算、图片 URL 去签名、jsonb、identity 显式 ID 与序列重置。
func TestLoadGoodserRows(t *testing.T) {
	db := testutil.OpenPG(t)
	require.NoError(t, testutil.ApplyMigrations(t, db))

	dump := map[string][]map[string]any{
		"zzz_goodser_inventories": {
			{
				"id":         "inv-1",
				"name":       "主目录",
				"sort_order": int64(1),
				"created_at": []byte("2026-11-26 10:00:00"),
			},
		},
		"zzz_goodser_products": {
			{
				"id":                "p-1",
				"inventory_id":      "inv-1",
				"code":              "A-01-0001-0000-C",
				"main_zone":         "A",
				"sub_zone":          "01",
				"seq_number":        int64(1),
				"quantity":          int64(2),
				"reserved_quantity": int64(0),
				"status_code":       "C",
				"name":              "徽章",
				"image_url":         []byte("https://fs.example.com/goodser/images/p1.png?X-Amz-Signature=x"),
				"images":            []byte(`["https://fs.example.com/goodser/images/p1.png"]`),
				"tags":              []byte(`["限量"]`),
				"created_at":        []byte("2026-11-26 10:00:00"),
				"updated_at":        []byte("2026-11-26 10:00:00"),
			},
		},
		"zzz_goodser_stock_movements": {
			{
				"id":             int64(1),
				"inventory_id":   "inv-1",
				"product_id":     "p-1",
				"quantity_delta": int64(0),
				"reserved_delta": int64(-1),
				"quantity_after": int64(2),
				"reserved_after": int64(0),
				"reason":         "outbound_confirm",
				"created_at":     []byte("2026-11-26 10:00:00"),
			},
		},
	}

	require.NoError(t, loadData(context.Background(), db, dump))

	// MySQL 10:00 (+08) → timestamptz 02:00Z
	var createdAt time.Time
	require.NoError(t, db.QueryRow(`SELECT created_at FROM zzz_goodser_products WHERE id = 'p-1'`).Scan(&createdAt))
	require.Equal(t, time.Date(2026, 11, 26, 2, 0, 0, 0, time.UTC), createdAt.UTC())

	// 预签名参数已去掉
	var imageURL string
	require.NoError(t, db.QueryRow(`SELECT image_url FROM zzz_goodser_products WHERE id = 'p-1'`).Scan(&imageURL))
	require.Equal(t, "https://fs.example.com/goodser/images/p1.png", imageURL)

	// identity 显式插入后序列已重置，下一次自增不会冲突
	var lastValue int64
	require.NoError(t, db.QueryRow(`SELECT last_value FROM zzz_goodser_stock_movements_id_seq`).Scan(&lastValue))
	require.GreaterOrEqual(t, lastValue, int64(2))

	// 业务校验器应通过
	require.NoError(t, verifyGoodser(context.Background(), db))
}
