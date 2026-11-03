package goodser

import "testing"

func TestBundleEffectiveSubRules(t *testing.T) {
	bundle := &Bundle{
		MinSubQuantity: 1,
		MinSubAmount:   20,
		Pool: &BundlePool{
			MinItemCount: 3,
			MinAmount:    50,
		},
	}
	if got := bundle.EffectiveMinItemCount(); got != 3 {
		t.Fatalf("EffectiveMinItemCount = %d，期望 3", got)
	}
	if got := bundle.EffectiveMinSubAmount(); got != 50 {
		t.Fatalf("EffectiveMinSubAmount = %.2f，期望 50", got)
	}

	bundle.MinSubQuantity = 5
	bundle.MinSubAmount = 80
	if got := bundle.EffectiveMinItemCount(); got != 5 {
		t.Fatalf("EffectiveMinItemCount = %d，期望 5", got)
	}
	if got := bundle.EffectiveMinSubAmount(); got != 80 {
		t.Fatalf("EffectiveMinSubAmount = %.2f，期望 80", got)
	}

	legacy := &Bundle{MinSubQuantity: 2, MinSubAmount: 10}
	if got := legacy.EffectiveMinItemCount(); got != 2 {
		t.Fatalf("legacy EffectiveMinItemCount = %d，期望 2", got)
	}
	if got := legacy.EffectiveMinSubAmount(); got != 10 {
		t.Fatalf("legacy EffectiveMinSubAmount = %.2f，期望 10", got)
	}
}

func TestBundleAvailability(t *testing.T) {
	main := BundleMainProduct{ProductID: "main-1", Available: 5, ExpectedPrice: 40}
	sub := func(id string, available int, price float64) BundleSubProduct {
		return BundleSubProduct{ProductID: id, Available: available, ExpectedPrice: price, DefaultQuantity: 1}
	}

	cases := []struct {
		name   string
		bundle *Bundle
		want   int
	}{
		{
			name:   "无主商品",
			bundle: &Bundle{Status: BundleStatusActive},
			want:   0,
		},
		{
			name: "主商品库存不足",
			bundle: &Bundle{
				Status:          BundleStatusActive,
				MinMainQuantity: 2,
				MainProducts:    []BundleMainProduct{{ProductID: "main-1", Available: 1, ExpectedPrice: 40}},
			},
			want: 0,
		},
		{
			name: "无捆物规则",
			bundle: &Bundle{
				Status:          BundleStatusActive,
				MinMainQuantity: 1,
				MainProducts:    []BundleMainProduct{main},
			},
			want: 5,
		},
		{
			name: "池数量不足",
			bundle: &Bundle{
				Status:          BundleStatusActive,
				MinMainQuantity: 1,
				MainProducts:    []BundleMainProduct{main},
				SubProducts:     []BundleSubProduct{sub("sub-1", 2, 30)},
				Pool:            &BundlePool{Status: BundlePoolStatusActive, MinItemCount: 3},
			},
			want: 0,
		},
		{
			name: "池金额不足",
			bundle: &Bundle{
				Status:          BundleStatusActive,
				MinMainQuantity: 1,
				MainProducts:    []BundleMainProduct{main},
				SubProducts:     []BundleSubProduct{sub("sub-1", 1, 30)},
				Pool:            &BundlePool{Status: BundlePoolStatusActive, MinItemCount: 1, MinAmount: 50},
			},
			want: 0,
		},
		{
			name: "池规则满足",
			bundle: &Bundle{
				Status:          BundleStatusActive,
				MinMainQuantity: 1,
				MainProducts:    []BundleMainProduct{main},
				SubProducts:     []BundleSubProduct{sub("sub-1", 2, 30)},
				Pool:            &BundlePool{Status: BundlePoolStatusActive, MinItemCount: 2, MinAmount: 50},
			},
			want: 5,
		},
		{
			name: "池停用",
			bundle: &Bundle{
				Status:          BundleStatusActive,
				MinMainQuantity: 1,
				MainProducts:    []BundleMainProduct{main},
				SubProducts:     []BundleSubProduct{sub("sub-1", 2, 30)},
				Pool:            &BundlePool{Status: BundlePoolStatusInactive, MinItemCount: 1},
			},
			want: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := bundleAvailability(tc.bundle); got != tc.want {
				t.Fatalf("bundleAvailability = %d，期望 %d", got, tc.want)
			}
		})
	}
}

func TestBundlePoolSubProducts(t *testing.T) {
	pool := &BundlePool{Items: []BundlePoolItem{{
		ProductID:       "p1",
		ProductName:     "商品一",
		ProductCode:     "A-1",
		DefaultQuantity: 2,
		ExpectedPrice:   12,
		Available:       3,
		ReservedQty:     1,
		TotalQuantity:   4,
	}}}
	subs := bundlePoolSubProducts(pool)
	if len(subs) != 1 {
		t.Fatalf("成员数量 = %d，期望 1", len(subs))
	}
	item := subs[0]
	if item.ProductID != "p1" || item.DefaultQuantity != 2 || item.TotalQuantity != 4 {
		t.Fatalf("转换结果不正确: %+v", item)
	}
}
