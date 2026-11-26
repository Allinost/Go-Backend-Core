package goodser

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/stretchr/testify/require"
)

func testExportImage(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 120, A: 255})
		}
	}
	return img
}

func TestRenderExportImage(t *testing.T) {
	items := []exportRenderItem{
		{Name: "测试徽章 Test Badge", Image: testExportImage(120, 80), Quantity: 2, UnitPrice: 12.5, Amount: 25, PriceMode: "direct"},
		{Name: "无图商品", Quantity: 1, PriceMode: "direct"},
	}
	base := ExportRenderOptions{
		Width:        1024,
		Columns:      4,
		Background:   ExportBackgroundBlack,
		TextColor:    ExportTextColorAuto,
		Format:       ExportFormatPNG,
		Title:        "2026-11-26 发货清单",
		ShowName:     true,
		ShowPrice:    true,
		ShowQuantity: true,
		PriceDisplay: ExportPriceTotal,
	}

	data, contentType, err := renderExportImage(items, base)
	require.NoError(t, err)
	require.Equal(t, "image/png", contentType)

	img, err := png.Decode(bytes.NewReader(data))
	require.NoError(t, err)
	require.Greater(t, img.Bounds().Dx(), 0)
	require.Greater(t, img.Bounds().Dy(), 0)

	// 黑底：左上角应是背景色
	r, g, b, a := img.At(0, 0).RGBA()
	require.Equal(t, uint32(18), r>>8)
	require.Equal(t, uint32(18), g>>8)
	require.Equal(t, uint32(20), b>>8)
	require.Equal(t, uint32(255), a>>8)

	// 透明背景
	transparent := base
	transparent.Background = ExportBackgroundTransparent
	data, _, err = renderExportImage(items, transparent)
	require.NoError(t, err)
	img, err = png.Decode(bytes.NewReader(data))
	require.NoError(t, err)
	_, _, _, a = img.At(0, 0).RGBA()
	require.Equal(t, uint32(0), a>>8)

	// JPEG 不支持透明，应压到白底
	jpegOpts := transparent
	jpegOpts.Format = ExportFormatJPEG
	data, contentType, err = renderExportImage(items, jpegOpts)
	require.NoError(t, err)
	require.Equal(t, "image/jpeg", contentType)
	require.NotEmpty(t, data)
}

func TestRenderExportImageEmpty(t *testing.T) {
	_, _, err := renderExportImage(nil, ExportRenderOptions{Width: 1024, Columns: 4})
	require.Error(t, err)
}

func TestExportPriceText(t *testing.T) {
	item := exportRenderItem{Quantity: 3, UnitPrice: 12.5, Amount: 37.5}

	main, sub := exportPriceText(item, ExportPriceTotal)
	require.Equal(t, "¥37.50", main)
	require.Equal(t, "¥12.50 ×3", sub)

	main, sub = exportPriceText(item, ExportPriceUnit)
	require.Equal(t, "¥12.50 ×3", main)
	require.Empty(t, sub)

	main, sub = exportPriceText(item, ExportPriceBoth)
	require.Equal(t, "¥37.50", main)
	require.Equal(t, "¥12.50 ×3", sub)

	main, _ = exportPriceText(exportRenderItem{Quantity: 1, UnitPrice: 9, Amount: 9}, ExportPriceTotal)
	require.Equal(t, "¥9.00", main)

	main, _ = exportPriceText(exportRenderItem{Quantity: 1}, ExportPriceTotal)
	require.Equal(t, "—", main)
}

func TestExportRequestNormalize(t *testing.T) {
	req := &ExportRequest{
		Items: []ExportItemRequest{{ID: "p1"}},
	}
	req.Normalize(4, 3072)
	require.Equal(t, 4, req.Columns)
	require.Equal(t, 2048, req.Width)
	require.Equal(t, ExportBackgroundWhite, req.Background)
	require.Equal(t, ExportFormatPNG, req.Format)
	require.Equal(t, ExportPriceTotal, req.PriceDisplay)
	require.NotNil(t, req.ShowName)
	require.True(t, *req.ShowName)
	require.True(t, *req.ShowPrice)
	require.True(t, *req.ShowQuantity)
	require.Equal(t, ExportTypeProduct, req.Items[0].Type)
	require.Equal(t, 1, req.Items[0].Quantity)

	// 显式关闭显示项
	f := false
	req = &ExportRequest{ShowName: &f, ShowPrice: &f, ShowQuantity: &f, Columns: 5, Width: 4096}
	req.Normalize(3, 3072)
	require.Equal(t, 5, req.Columns)
	require.Equal(t, 3072, req.Width)
	require.False(t, *req.ShowName)

	// 非法取值回退
	req = &ExportRequest{Columns: 9, Width: 100, Background: "pink", Format: "bmp", PriceDisplay: "x"}
	req.Normalize(4, 3072)
	require.Equal(t, 4, req.Columns)
	require.Equal(t, 2048, req.Width)
	require.Equal(t, ExportBackgroundWhite, req.Background)
	require.Equal(t, ExportFormatPNG, req.Format)
	require.Equal(t, ExportPriceTotal, req.PriceDisplay)
}

func TestExportObjectKey(t *testing.T) {
	require.Equal(t, "goodser/exports/x.png",
		exportObjectKey("https://fs.example.com/goodser/exports/x.png?X-Amz-Signature=1"))
	require.Equal(t, "goodser/exports/y.jpg",
		exportObjectKey("https://fs.example.com/goodser/exports/y.jpg#frag"))
	require.Equal(t, "", exportObjectKey("https://fs.example.com/other/x.png"))
}

func TestExportURIPaths(t *testing.T) {
	require.Equal(t, "https://fs.example.com/a.png", stripQuery("https://fs.example.com/a.png?x=1"))
	require.Equal(t, "https://fs.example.com/a.png", stripQuery("https://fs.example.com/a.png"))
}
