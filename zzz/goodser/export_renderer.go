package goodser

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"math"
	"strings"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// exportRenderItem 渲染用的单行数据，价格已在服务层算好。
type exportRenderItem struct {
	Name      string
	Image     image.Image
	Quantity  int
	UnitPrice float64
	Amount    float64
	PriceMode string
}

// ExportRenderOptions 渲染参数。
type ExportRenderOptions struct {
	Width        int
	Columns      int
	Background   string
	TextColor    string
	Format       string
	Title        string
	ShowName     bool
	ShowPrice    bool
	ShowQuantity bool
	PriceDisplay string
	FontPath     string
}

type exportPalette struct {
	background      color.RGBA
	text            color.RGBA
	subText         color.RGBA
	price           color.RGBA
	title           color.RGBA
	badgeBg         color.RGBA
	badgeText       color.RGBA
	placeholder     color.RGBA
	placeholderText color.RGBA
	outline         color.RGBA
	useOutline      bool
}

func exportPaletteFor(background, textColor string) exportPalette {
	p := exportPalette{
		background:      color.RGBA{R: 255, G: 255, B: 255, A: 255},
		text:            color.RGBA{R: 28, G: 28, B: 32, A: 255},
		subText:         color.RGBA{R: 95, G: 95, B: 105, A: 255},
		price:           color.RGBA{R: 24, G: 24, B: 28, A: 255},
		title:           color.RGBA{R: 20, G: 20, B: 24, A: 255},
		badgeBg:         color.RGBA{R: 22, G: 22, B: 26, A: 255},
		badgeText:       color.RGBA{R: 250, G: 250, B: 252, A: 255},
		placeholder:     color.RGBA{R: 232, G: 232, B: 238, A: 255},
		placeholderText: color.RGBA{R: 150, G: 150, B: 160, A: 255},
		outline:         color.RGBA{R: 255, G: 255, B: 255, A: 235},
	}
	switch background {
	case ExportBackgroundBlack:
		p.background = color.RGBA{R: 18, G: 18, B: 20, A: 255}
		p.text = color.RGBA{R: 238, G: 238, B: 242, A: 255}
		p.subText = color.RGBA{R: 170, G: 170, B: 182, A: 255}
		p.price = color.RGBA{R: 255, G: 255, B: 255, A: 255}
		p.title = color.RGBA{R: 246, G: 246, B: 250, A: 255}
		p.badgeBg = color.RGBA{R: 246, G: 246, B: 250, A: 255}
		p.badgeText = color.RGBA{R: 18, G: 18, B: 22, A: 255}
		p.placeholder = color.RGBA{R: 42, G: 42, B: 48, A: 255}
		p.placeholderText = color.RGBA{R: 120, G: 120, B: 132, A: 255}
		p.outline = color.RGBA{R: 0, G: 0, B: 0, A: 235}
	case ExportBackgroundTransparent:
		p.useOutline = true
		p.outline = color.RGBA{R: 255, G: 255, B: 255, A: 235}
		p.badgeBg = color.RGBA{R: 255, G: 255, B: 255, A: 225}
		p.badgeText = color.RGBA{R: 20, G: 20, B: 24, A: 255}
	}
	switch textColor {
	case ExportTextColorDark:
		p.text = color.RGBA{R: 28, G: 28, B: 32, A: 255}
		p.subText = color.RGBA{R: 95, G: 95, B: 105, A: 255}
		p.price = color.RGBA{R: 24, G: 24, B: 28, A: 255}
		p.title = color.RGBA{R: 20, G: 20, B: 24, A: 255}
		if background == ExportBackgroundBlack {
			p.useOutline = true
			p.outline = color.RGBA{R: 0, G: 0, B: 0, A: 235}
		}
	case ExportTextColorLight:
		p.text = color.RGBA{R: 240, G: 240, B: 245, A: 255}
		p.subText = color.RGBA{R: 200, G: 200, B: 210, A: 255}
		p.price = color.RGBA{R: 255, G: 255, B: 255, A: 255}
		p.title = color.RGBA{R: 250, G: 250, B: 255, A: 255}
		if background != ExportBackgroundBlack {
			p.useOutline = true
			p.outline = color.RGBA{R: 255, G: 255, B: 255, A: 235}
		}
	}
	return p
}

// renderExportImage 把商品行渲染成 PNG/JPEG。
func renderExportImage(items []exportRenderItem, opts ExportRenderOptions) ([]byte, string, error) {
	if len(items) == 0 {
		return nil, "", fmt.Errorf("导出内容为空")
	}
	if opts.Width < 1024 {
		opts.Width = 2048
	}
	if opts.Columns < 3 || opts.Columns > 5 {
		opts.Columns = 4
	}

	width := opts.Width
	scale := float64(width) / 2048.0
	margin := int(math.Round(64 * scale))
	gap := int(math.Round(32 * scale))
	cols := opts.Columns
	cellW := (width - 2*margin - (cols-1)*gap) / cols
	if cellW < 120 {
		cellW = 120
	}

	titleFace := exportFace(opts.FontPath, 42*scale)
	nameFace := exportFace(opts.FontPath, 24*scale)
	priceFace := exportFace(opts.FontPath, 24*scale)
	badgeFace := exportFace(opts.FontPath, 22*scale)
	placeholderFace := exportFace(opts.FontPath, 24*scale)

	titleH := 0
	if opts.Title != "" {
		titleH = titleFace.Metrics().Height.Ceil() + int(math.Round(24*scale))
	}
	nameLineH := nameFace.Metrics().Height.Ceil() + int(math.Round(4*scale))
	nameBlockH := 0
	if opts.ShowName {
		nameBlockH = nameLineH*2 + int(math.Round(10*scale))
	}
	priceLineH := priceFace.Metrics().Height.Ceil() + int(math.Round(2*scale))
	priceBlockH := 0
	if opts.ShowPrice {
		priceBlockH = priceLineH*2 + int(math.Round(6*scale))
	}

	cellH := cellW + int(math.Round(14*scale)) + nameBlockH + priceBlockH
	rows := (len(items) + cols - 1) / cols
	totalH := margin*2 + titleH + rows*cellH
	if rows > 1 {
		totalH += (rows - 1) * gap
	}

	canvas := image.NewRGBA(image.Rect(0, 0, width, totalH))
	pal := exportPaletteFor(opts.Background, opts.TextColor)
	if opts.Background != ExportBackgroundTransparent {
		draw.Draw(canvas, canvas.Bounds(), image.NewUniform(pal.background), image.Point{}, draw.Src)
	}

	if opts.Title != "" {
		baseline := margin + titleFace.Metrics().Ascent.Ceil()
		drawCenteredExportText(canvas, titleFace, opts.Title, width/2, baseline, pal.title, pal.outline, pal.useOutline)
	}

	contentTop := margin + titleH
	for i, item := range items {
		row := i / cols
		col := i % cols
		x := margin + col*(cellW+gap)
		y := contentTop + row*(cellH+gap)
		drawExportCell(drawTarget{
			dst: canvas, x: x, y: y, width: cellW, item: item, opts: opts, pal: pal,
			nameFace: nameFace, priceFace: priceFace, badgeFace: badgeFace,
			placeholderFace: placeholderFace, scale: scale,
		})
	}

	var buf bytes.Buffer
	switch opts.Format {
	case ExportFormatJPEG:
		if opts.Background == ExportBackgroundTransparent {
			flat := image.NewRGBA(canvas.Bounds())
			draw.Draw(flat, flat.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
			draw.Draw(flat, flat.Bounds(), canvas, image.Point{}, draw.Over)
			canvas = flat
		}
		if err := jpeg.Encode(&buf, canvas, &jpeg.Options{Quality: 90}); err != nil {
			return nil, "", err
		}
		return buf.Bytes(), "image/jpeg", nil
	default:
		if err := png.Encode(&buf, canvas); err != nil {
			return nil, "", err
		}
		return buf.Bytes(), "image/png", nil
	}
}

type drawTarget struct {
	dst             *image.RGBA
	x, y, width     int
	item            exportRenderItem
	opts            ExportRenderOptions
	pal             exportPalette
	nameFace        font.Face
	priceFace       font.Face
	badgeFace       font.Face
	placeholderFace font.Face
	scale           float64
}

func drawExportCell(t drawTarget) {
	cellRect := image.Rect(t.x, t.y, t.x+t.width, t.y+t.width)
	radius := int(math.Round(14 * t.scale))

	if t.item.Image != nil {
		scaled := scaleExportImageContain(t.item.Image, t.width, t.width)
		if scaled != nil {
			ox := t.x + (t.width-scaled.Bounds().Dx())/2
			oy := t.y + (t.width-scaled.Bounds().Dy())/2
			target := image.Rect(ox, oy, ox+scaled.Bounds().Dx(), oy+scaled.Bounds().Dy())
			mask := exportRoundedMask(target.Dx(), target.Dy(), radius)
			draw.DrawMask(t.dst, target, scaled, scaled.Bounds().Min, mask, image.Point{}, draw.Over)
		}
	} else {
		drawExportRoundRect(t.dst, cellRect, radius, t.pal.placeholder)
		baseline := t.y + t.width/2 + t.placeholderFace.Metrics().Ascent.Ceil()/2
		drawCenteredExportText(t.dst, t.placeholderFace, "无图", t.x+t.width/2, baseline, t.pal.placeholderText, t.pal.outline, false)
	}

	if t.opts.ShowQuantity && t.item.Quantity > 0 {
		label := fmt.Sprintf("×%d", t.item.Quantity)
		padX := int(math.Round(12 * t.scale))
		padY := int(math.Round(6 * t.scale))
		bw := font.MeasureString(t.badgeFace, label).Ceil() + padX*2
		bh := t.badgeFace.Metrics().Height.Ceil() + padY*2
		bx := t.x + t.width - bw - int(math.Round(10*t.scale))
		by := t.y + t.width - bh - int(math.Round(10*t.scale))
		drawExportRoundRect(t.dst, image.Rect(bx, by, bx+bw, by+bh), int(math.Round(10*t.scale)), t.pal.badgeBg)
		drawExportText(t.dst, t.badgeFace, label, bx+padX, by+padY+t.badgeFace.Metrics().Ascent.Ceil(), t.pal.badgeText, t.pal.outline, false)
	}

	textY := t.y + t.width + int(math.Round(14*t.scale))
	if t.opts.ShowName {
		lines := wrapExportLines(t.nameFace, t.item.Name, t.width, 2)
		baseline := textY + t.nameFace.Metrics().Ascent.Ceil()
		for _, line := range lines {
			drawExportText(t.dst, t.nameFace, line, t.x, baseline, t.pal.text, t.pal.outline, t.pal.useOutline)
			baseline += t.nameFace.Metrics().Height.Ceil() + int(math.Round(4*t.scale))
		}
		textY += t.nameFace.Metrics().Height.Ceil()*2 + int(math.Round(10*t.scale))
	}
	if t.opts.ShowPrice {
		main, sub := exportPriceText(t.item, t.opts.PriceDisplay)
		baseline := textY + t.priceFace.Metrics().Ascent.Ceil()
		drawExportText(t.dst, t.priceFace, main, t.x, baseline, t.pal.price, t.pal.outline, t.pal.useOutline)
		if sub != "" {
			baseline += t.priceFace.Metrics().Height.Ceil() + int(math.Round(2*t.scale))
			drawExportText(t.dst, t.priceFace, sub, t.x, baseline, t.pal.subText, t.pal.outline, t.pal.useOutline)
		}
	}
}

// exportPriceText 生成价格主文案与副文案。
func exportPriceText(item exportRenderItem, display string) (string, string) {
	if item.UnitPrice <= 0 && item.Amount <= 0 {
		return "—", ""
	}
	qty := item.Quantity
	if qty < 1 {
		qty = 1
	}
	unit := fmt.Sprintf("¥%.2f", item.UnitPrice)
	amount := fmt.Sprintf("¥%.2f", item.Amount)
	if qty <= 1 {
		if item.UnitPrice > 0 {
			return unit, ""
		}
		return amount, ""
	}
	switch display {
	case ExportPriceUnit:
		return fmt.Sprintf("%s ×%d", unit, qty), ""
	case ExportPriceBoth:
		return amount, fmt.Sprintf("%s ×%d", unit, qty)
	default:
		return amount, fmt.Sprintf("%s ×%d", unit, qty)
	}
}

func drawExportText(dst draw.Image, face font.Face, text string, x, baseline int, col, outline color.RGBA, useOutline bool) {
	if text == "" {
		return
	}
	d := font.Drawer{Dst: dst, Face: face, Dot: fixed.P(x, baseline)}
	if useOutline {
		d.Src = image.NewUniform(outline)
		for _, off := range []image.Point{{-1, 0}, {1, 0}, {0, -1}, {0, 1}, {-1, -1}, {1, 1}, {-1, 1}, {1, -1}} {
			d.Dot = fixed.P(x+off.X, baseline+off.Y)
			d.DrawString(text)
		}
	}
	d.Src = image.NewUniform(col)
	d.Dot = fixed.P(x, baseline)
	d.DrawString(text)
}

func drawCenteredExportText(dst draw.Image, face font.Face, text string, centerX, baseline int, col, outline color.RGBA, useOutline bool) {
	w := font.MeasureString(face, text).Ceil()
	drawExportText(dst, face, text, centerX-w/2, baseline, col, outline, useOutline)
}

// wrapExportLines 按宽度换行，最多 maxLines 行，超出加省略号。
func wrapExportLines(face font.Face, text string, maxWidth, maxLines int) []string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) == 0 || maxLines <= 0 {
		return nil
	}
	var lines []string
	var current []rune
	truncated := false
	for _, r := range runes {
		current = append(current, r)
		if font.MeasureString(face, string(current)).Ceil() <= maxWidth {
			continue
		}
		if len(current) == 1 {
			// 单字超宽，直接换行
			continue
		}
		current = current[:len(current)-1]
		lines = append(lines, string(current))
		current = []rune{r}
		if len(lines) == maxLines {
			truncated = true
			current = nil
			break
		}
	}
	if len(current) > 0 {
		if len(lines) < maxLines {
			lines = append(lines, string(current))
		} else {
			truncated = true
		}
	}
	if truncated && len(lines) > 0 {
		last := []rune(lines[len(lines)-1])
		for len(last) > 0 && font.MeasureString(face, string(last)+"…").Ceil() > maxWidth {
			last = last[:len(last)-1]
		}
		lines[len(lines)-1] = string(last) + "…"
	}
	return lines
}

func scaleExportImageContain(src image.Image, maxW, maxH int) image.Image {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	if sw <= 0 || sh <= 0 {
		return nil
	}
	ratio := math.Min(float64(maxW)/float64(sw), float64(maxH)/float64(sh))
	nw := int(math.Round(float64(sw) * ratio))
	nh := int(math.Round(float64(sh) * ratio))
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	return dst
}

func drawExportRoundRect(dst draw.Image, rect image.Rectangle, radius int, col color.Color) {
	if rect.Empty() {
		return
	}
	mask := exportRoundedMask(rect.Dx(), rect.Dy(), radius)
	draw.DrawMask(dst, rect, image.NewUniform(col), image.Point{}, mask, image.Point{}, draw.Over)
}

// exportRoundedMask 生成圆角矩形蒙版。
func exportRoundedMask(w, h, radius int) *image.Alpha {
	if w <= 0 || h <= 0 {
		return image.NewAlpha(image.Rect(0, 0, 0, 0))
	}
	if radius < 0 {
		radius = 0
	}
	if radius*2 > w {
		radius = w / 2
	}
	if radius*2 > h {
		radius = h / 2
	}
	mask := image.NewAlpha(image.Rect(0, 0, w, h))
	if radius == 0 {
		for y := 0; y < h; y++ {
			row := mask.Pix[y*mask.Stride : y*mask.Stride+w]
			for x := range row {
				row[x] = 255
			}
		}
		return mask
	}
	r2 := radius * radius
	for y := 0; y < h; y++ {
		row := mask.Pix[y*mask.Stride : y*mask.Stride+w]
		for x := 0; x < w; x++ {
			dx, dy := 0, 0
			switch {
			case x < radius && y < radius:
				dx, dy = radius-x, radius-y
			case x >= w-radius && y < radius:
				dx, dy = x-(w-radius-1), radius-y
			case x < radius && y >= h-radius:
				dx, dy = radius-x, y-(h-radius-1)
			case x >= w-radius && y >= h-radius:
				dx, dy = x-(w-radius-1), y-(h-radius-1)
			}
			if dx != 0 || dy != 0 {
				if dx*dx+dy*dy > r2 {
					continue
				}
			}
			row[x] = 255
		}
	}
	return mask
}
