package goodser

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	_ "golang.org/x/image/webp"

	"github.com/Allinost/go-backend-core/internal/pkg/logger"
	"github.com/Allinost/go-backend-core/internal/services/storage"
	"github.com/google/uuid"
)

const (
	exportImageMaxBytes = 20 << 20
	exportImageWorkers  = 8
	exportFetchTimeout  = 8 * time.Second
	exportSyncThreshold = 30
	exportURLLifetime   = 7 * 24 * time.Hour
)

// PreviewExport 计算价格与布局摘要，不渲染图片。
func (s *Service) PreviewExport(ctx context.Context, inventoryID string, req *ExportRequest) (*ExportPreview, error) {
	if req == nil {
		return nil, NewGoodserError(ErrExportInvalid, "请求为空")
	}
	opts := s.exportOptions()
	req.Normalize(opts.DefaultColumns, opts.MaxWidth)
	if len(req.Items) == 0 {
		return nil, NewGoodserError(ErrExportItemsEmpty)
	}
	if len(req.Items) > opts.MaxItems {
		return nil, NewGoodserError(ErrExportTooMany, fmt.Sprintf("最多 %d 项", opts.MaxItems))
	}
	lines, err := s.buildExportLines(ctx, inventoryID, req)
	if err != nil {
		return nil, err
	}
	totalAmount := 0.0
	totalQuantity := 0
	for _, line := range lines {
		totalAmount += line.Amount
		totalQuantity += line.Quantity
	}
	return &ExportPreview{
		Lines:         lines,
		TotalAmount:   roundMoney(totalAmount),
		TotalQuantity: totalQuantity,
		Columns:       req.Columns,
		Width:         req.Width,
		Background:    req.Background,
		Format:        req.Format,
	}, nil
}

// CreateExport 创建导出任务；不超过 30 项时同步渲染，否则后台任务。
func (s *Service) CreateExport(ctx context.Context, inventoryID string, req *ExportRequest, createdBy int64) (*ExportJob, error) {
	preview, err := s.PreviewExport(ctx, inventoryID, req)
	if err != nil {
		return nil, err
	}
	opts := s.exportOptions()
	requestJSON, err := json.Marshal(req)
	if err != nil {
		return nil, NewGoodserError(ErrExportInvalid, "请求序列化失败")
	}
	now := time.Now().UTC()
	expiresAt := now.Add(exportURLLifetime)
	job := &ExportJob{
		ID:          uuid.NewString(),
		InventoryID: inventoryID,
		Status:      ExportStatusPending,
		Request:     requestJSON,
		CreatedAt:   now,
		ExpiresAt:   &expiresAt,
	}
	if createdBy > 0 {
		job.CreatedBy = &createdBy
	}
	if err := s.repo.CreateExportJob(ctx, job); err != nil {
		return nil, err
	}

	if len(preview.Lines) <= exportSyncThreshold {
		s.renderExport(ctx, job.ID, preview, *req, opts)
		return s.GetExport(ctx, job.ID)
	}
	go func() {
		runCtx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
		defer cancel()
		s.renderExport(runCtx, job.ID, preview, *req, opts)
	}()
	return job, nil
}

// GetExport 查询导出任务。
func (s *Service) GetExport(ctx context.Context, id string) (*ExportJob, error) {
	job, err := s.repo.GetExportJob(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, NewGoodserError(ErrExportNotFound)
	}
	if err != nil {
		return nil, err
	}
	return job, nil
}

// ListExports 查询导出历史。
func (s *Service) ListExports(ctx context.Context, inventoryID string, limit int) ([]ExportJob, error) {
	return s.repo.ListExportJobs(ctx, inventoryID, limit)
}

// DeleteExport 删除导出记录，并尽力删除对象存储中的文件。
func (s *Service) DeleteExport(ctx context.Context, id string) error {
	job, err := s.GetExport(ctx, id)
	if err != nil {
		return err
	}
	if job.ResultURL != nil && s.store != nil {
		if key := exportObjectKey(*job.ResultURL); key != "" {
			if derr := s.store.Delete(ctx, key); derr != nil {
				logger.Warn().Err(derr).Str("key", key).Msg("goodser: 删除导出文件失败")
			}
		}
	}
	if err := s.repo.DeleteExportJob(ctx, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return NewGoodserError(ErrExportNotFound)
		}
		return err
	}
	return nil
}

// buildExportLines 逐项计算名称、图片、单价与金额。
func (s *Service) buildExportLines(ctx context.Context, inventoryID string, req *ExportRequest) ([]ExportLine, error) {
	lines := make([]ExportLine, 0, len(req.Items))
	for _, item := range req.Items {
		if item.ID == "" {
			return nil, NewGoodserError(ErrExportInvalid, "缺少商品 ID")
		}
		switch item.Type {
		case ExportTypeProduct:
			product, err := s.GetProduct(ctx, item.ID)
			if err != nil {
				return nil, err
			}
			if product.InventoryID != inventoryID {
				return nil, NewGoodserError(ErrProductInventory)
			}
			line := ExportLine{
				Type:      ExportTypeProduct,
				ID:        product.ID,
				Name:      product.Name,
				Quantity:  item.Quantity,
				UnitPrice: productSalePrice(product),
				PriceMode: "direct",
				ImageURL:  productImageURL(product),
			}
			line.Amount = roundMoney(line.UnitPrice * float64(line.Quantity))
			lines = append(lines, line)

		case ExportTypeCombo:
			combo, err := s.GetCombo(ctx, item.ID)
			if err != nil {
				return nil, err
			}
			if combo.InventoryID != inventoryID {
				return nil, NewGoodserError(ErrProductInventory, "组合包不属于当前库存目录")
			}
			line := ExportLine{
				Type:      ExportTypeCombo,
				ID:        combo.ID,
				Name:      combo.Name,
				Quantity:  item.Quantity,
				UnitPrice: combo.Price,
				PriceMode: "combo",
				ImageURL:  derefString(combo.ImageURL),
			}
			line.Amount = roundMoney(line.UnitPrice * float64(line.Quantity))
			for _, comp := range combo.Items {
				name := comp.ProductName
				if name == "" {
					name = comp.BundleName
				}
				line.Components = append(line.Components, ExportComponent{
					Name:     name,
					Quantity: comp.Quantity,
				})
			}
			lines = append(lines, line)

		case ExportTypeBundle:
			previewReq := &BundlePreviewReq{Quantity: item.Quantity}
			if item.BundleSelection != nil {
				previewReq.MainProductID = strings.TrimSpace(item.BundleSelection.MainProductID)
				previewReq.MainQuantity = item.BundleSelection.MainQuantity
				previewReq.SubItems = item.BundleSelection.SubItems
			}
			resp, err := s.PreviewBundle(ctx, item.ID, previewReq)
			if err != nil {
				return nil, err
			}
			if resp.InventoryID != inventoryID {
				return nil, NewGoodserError(ErrProductInventory, "捆不属于当前库存目录")
			}
			qty := resp.Quantity
			if qty <= 0 {
				qty = item.Quantity
			}
			line := ExportLine{
				Type:      ExportTypeBundle,
				ID:        resp.BundleID,
				Name:      resp.BundleName,
				Quantity:  qty,
				UnitPrice: resp.UnitPrice,
				PriceMode: resp.PriceMode,
				ImageURL:  derefString(resp.MainProduct.ImageURL),
			}
			line.Amount = resp.TotalPrice
			if line.Amount <= 0 {
				line.Amount = roundMoney(line.UnitPrice * float64(line.Quantity))
			}
			if resp.MainProduct.ProductName != "" {
				line.Components = append(line.Components, ExportComponent{
					Name:     resp.MainProduct.ProductName,
					Quantity: resp.MainQuantity,
					Amount:   resp.MainAmount,
				})
			}
			for _, sub := range resp.SubItems {
				line.Components = append(line.Components, ExportComponent{
					Name:     sub.ProductName,
					Quantity: sub.Quantity,
					Amount:   sub.Amount,
				})
			}
			lines = append(lines, line)

		default:
			return nil, NewGoodserError(ErrExportInvalid, "不支持的类型: "+item.Type)
		}
	}
	return lines, nil
}

// renderExport 抓图、渲染、上传并回写任务状态。
func (s *Service) renderExport(ctx context.Context, jobID string, preview *ExportPreview, req ExportRequest, opts ExportOptions) {
	fail := func(err error) {
		msg := err.Error()
		now := time.Now().UTC()
		if uerr := s.repo.UpdateExportJob(context.Background(), jobID, ExportStatusFailed, nil, &msg, &now); uerr != nil {
			logger.Error().Err(uerr).Str("job", jobID).Msg("goodser: 更新导出失败状态失败")
		}
		logger.Warn().Err(err).Str("job", jobID).Msg("goodser: 导出失败")
	}

	if err := s.repo.UpdateExportJob(ctx, jobID, ExportStatusProcessing, nil, nil, nil); err != nil {
		logger.Warn().Err(err).Str("job", jobID).Msg("goodser: 更新导出处理中状态失败")
	}

	urls := make([]string, 0, len(preview.Lines))
	for _, line := range preview.Lines {
		urls = append(urls, line.ImageURL)
	}
	images := fetchExportImages(ctx, urls)

	items := make([]exportRenderItem, 0, len(preview.Lines))
	for _, line := range preview.Lines {
		items = append(items, exportRenderItem{
			Name:      line.Name,
			Image:     images[line.ImageURL],
			Quantity:  line.Quantity,
			UnitPrice: line.UnitPrice,
			Amount:    line.Amount,
			PriceMode: line.PriceMode,
		})
	}

	data, contentType, err := renderExportImage(items, ExportRenderOptions{
		Width:        req.Width,
		Columns:      req.Columns,
		Background:   req.Background,
		TextColor:    req.TextColor,
		Format:       req.Format,
		Title:        req.Title,
		ShowName:     req.ShowName != nil && *req.ShowName,
		ShowPrice:    req.ShowPrice != nil && *req.ShowPrice,
		ShowQuantity: req.ShowQuantity != nil && *req.ShowQuantity,
		PriceDisplay: req.PriceDisplay,
		FontPath:     opts.FontPath,
	})
	if err != nil {
		fail(NewGoodserError(ErrExportRender, err.Error()))
		return
	}
	if s.store == nil {
		fail(NewGoodserError(ErrExportStorage, "图片存储未配置"))
		return
	}

	ext := ".png"
	if contentType == "image/jpeg" {
		ext = ".jpg"
	}
	key := "goodser/exports/" + jobID + ext
	if _, err := s.store.Upload(ctx, key, bytes.NewReader(data), storage.WithContentType(contentType)); err != nil {
		fail(NewGoodserError(ErrExportStorage, err.Error()))
		return
	}
	signedURL, err := s.store.SignedURL(ctx, key, exportURLLifetime)
	if err != nil {
		fail(NewGoodserError(ErrExportStorage, err.Error()))
		return
	}
	resultURL := stripQuery(signedURL)
	now := time.Now().UTC()
	if err := s.repo.UpdateExportJob(ctx, jobID, ExportStatusSucceeded, &resultURL, nil, &now); err != nil {
		logger.Error().Err(err).Str("job", jobID).Msg("goodser: 更新导出成功状态失败")
	}
}

// fetchExportImages 并发下载图片，失败的行保持 nil，渲染时使用占位图。
func fetchExportImages(ctx context.Context, urls []string) map[string]image.Image {
	out := make(map[string]image.Image)
	uniq := make(map[string]bool, len(urls))
	client := &http.Client{Timeout: exportFetchTimeout}
	sem := make(chan struct{}, exportImageWorkers)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, raw := range urls {
		raw = strings.TrimSpace(raw)
		if raw == "" || uniq[raw] {
			continue
		}
		uniq[raw] = true
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			img := downloadExportImage(ctx, client, url)
			if img == nil {
				return
			}
			mu.Lock()
			out[url] = img
			mu.Unlock()
		}(raw)
	}
	wg.Wait()
	return out
}

func downloadExportImage(ctx context.Context, client *http.Client, rawURL string) image.Image {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil
	}
	resp, err := client.Do(req)
	if err != nil {
		logger.Warn().Err(err).Str("url", rawURL).Msg("goodser: 下载导出图片失败")
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logger.Warn().Int("status", resp.StatusCode).Str("url", rawURL).Msg("goodser: 下载导出图片失败")
		return nil
	}
	img, _, err := image.Decode(io.LimitReader(resp.Body, exportImageMaxBytes))
	if err != nil {
		logger.Warn().Err(err).Str("url", rawURL).Msg("goodser: 解析导出图片失败")
		return nil
	}
	return img
}

func productImageURL(product *Product) string {
	if product == nil {
		return ""
	}
	if product.ImageURL != nil && strings.TrimSpace(*product.ImageURL) != "" {
		return strings.TrimSpace(*product.ImageURL)
	}
	if len(product.Images) > 0 {
		var urls []string
		if err := json.Unmarshal(product.Images, &urls); err == nil && len(urls) > 0 {
			return strings.TrimSpace(urls[0])
		}
	}
	return ""
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(*v)
}

func stripQuery(raw string) string {
	if idx := strings.IndexAny(raw, "?#"); idx >= 0 {
		return raw[:idx]
	}
	return raw
}

// exportObjectKey 从公开 URL 中还原对象 key；无法识别时返回空。
func exportObjectKey(rawURL string) string {
	const marker = "/goodser/exports/"
	idx := strings.Index(rawURL, marker)
	if idx < 0 {
		return ""
	}
	key := rawURL[idx+1:]
	if cut := strings.IndexAny(key, "?#"); cut >= 0 {
		key = key[:cut]
	}
	return key
}
