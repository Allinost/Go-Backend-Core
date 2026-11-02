package goodser

import (
	"encoding/json"
	"testing"
)

func TestBundleMainFilterMatches(t *testing.T) {
	seriesID := "series-1"
	setTypeID := "set-1"
	productTypeID := "type-1"
	product := &Product{
		Name:          "限定拍立得",
		Code:          "A-01-0001-0001-N",
		StatusCode:    "N",
		SeriesID:      &seriesID,
		SetTypeID:     &setTypeID,
		ProductTypeID: &productTypeID,
	}

	cases := []struct {
		name     string
		filter   BundleMainFilter
		chars    []string
		expected bool
	}{
		{"空条件", BundleMainFilter{}, nil, true},
		{"系列匹配", BundleMainFilter{SeriesID: "series-1"}, nil, true},
		{"系列不匹配", BundleMainFilter{SeriesID: "series-2"}, nil, false},
		{"状态匹配", BundleMainFilter{StatusCode: "N"}, nil, true},
		{"状态不匹配", BundleMainFilter{StatusCode: "S"}, nil, false},
		{"角色匹配", BundleMainFilter{CharacterID: "char-1"}, []string{"char-1", "char-2"}, true},
		{"角色不匹配", BundleMainFilter{CharacterID: "char-3"}, []string{"char-1"}, false},
		{"关键词名称", BundleMainFilter{Keyword: "拍立得"}, nil, true},
		{"关键词编码", BundleMainFilter{Keyword: "a-01"}, nil, true},
		{"关键词不匹配", BundleMainFilter{Keyword: "手办"}, nil, false},
		{"组合条件", BundleMainFilter{SeriesID: "series-1", SetTypeID: "set-1", ProductTypeID: "type-1"}, nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := bundleMainFilterMatches(c.filter, product, c.chars); got != c.expected {
				t.Fatalf("bundleMainFilterMatches = %v，期望 %v", got, c.expected)
			}
		})
	}
}

func TestBundleOrderSelectionNormalize(t *testing.T) {
	selection := &BundleOrderSelection{
		BundleID:      " bundle-1 ",
		MainProductID: " product-1 ",
	}
	selection.Normalize()
	if selection.BundleID != "bundle-1" || selection.MainProductID != "product-1" {
		t.Fatalf("去空白失败: %+v", selection)
	}
	if selection.MainQuantity != 1 || selection.Quantity != 1 {
		t.Fatalf("默认数量 = %d/%d，期望 1/1", selection.MainQuantity, selection.Quantity)
	}

	selection = &BundleOrderSelection{MainQuantity: 3, Quantity: 2}
	selection.Normalize()
	if selection.MainQuantity != 3 || selection.Quantity != 2 {
		t.Fatalf("不应覆盖显式数量: %+v", selection)
	}
}

func TestRoundMoney(t *testing.T) {
	cases := []struct {
		input float64
		want  float64
	}{
		{0, 0},
		{1.234, 1.23},
		{1.235, 1.24},
		{19.999, 20},
	}
	for _, c := range cases {
		if got := roundMoney(c.input); got != c.want {
			t.Fatalf("roundMoney(%v) = %v，期望 %v", c.input, got, c.want)
		}
	}
}

func TestValidateBundleEnums(t *testing.T) {
	for _, status := range []string{"", BundleStatusActive, BundleStatusInactive} {
		if err := validateBundleStatus(status); err != nil {
			t.Fatalf("状态 %q 应合法: %v", status, err)
		}
	}
	if err := validateBundleStatus("deleted"); err == nil {
		t.Fatal("非法状态应返回错误")
	}
	for _, mode := range []string{BundlePriceModeSum, BundlePriceModeFixed, BundlePriceModeMainOnly} {
		if err := validateBundlePriceMode(mode); err != nil {
			t.Fatalf("价格模式 %q 应合法: %v", mode, err)
		}
	}
	if err := validateBundlePriceMode("discount"); err == nil {
		t.Fatal("非法价格模式应返回错误")
	}
}

func TestBundleSnapshotJSON(t *testing.T) {
	if data := bundleSnapshotJSON(nil); data != nil {
		t.Fatalf("nil 快照 = %s，期望 nil", data)
	}
	data := bundleSnapshotJSON(&BundleOrderSnapshot{BundleID: "bundle-1", TotalPrice: 12.5})
	var decoded BundleOrderSnapshot
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("快照 JSON 无法解析: %v", err)
	}
	if decoded.BundleID != "bundle-1" || decoded.TotalPrice != 12.5 {
		t.Fatalf("快照内容异常: %+v", decoded)
	}
}
