package goodsercatalog

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/Allinost/go-backend-core/internal/services/storage"
	"github.com/google/uuid"
)

type Service struct {
	repo      *Repository
	store     storage.Storage
	visionURL string
	client    *http.Client
}

func NewService(repo *Repository, store storage.Storage) *Service {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("GOODSER_VISION_URL")), "/")
	if base == "" {
		base = "http://127.0.0.1:8000"
	}
	return &Service{
		repo:      repo,
		store:     store,
		visionURL: base,
		client:    &http.Client{Timeout: 60 * time.Second},
	}
}

func (s *Service) ListCatalogs(ctx context.Context, req *ListCatalogsReq) ([]Catalog, error) {
	return s.repo.ListCatalogs(ctx, req)
}

func (s *Service) CreateCatalog(ctx context.Context, req *CreateCatalogReq, owner int64) (*Catalog, error) {
	return s.repo.CreateCatalog(ctx, req, owner)
}

func (s *Service) GetCatalog(ctx context.Context, id string) (*CatalogDetail, error) {
	return s.repo.GetDetail(ctx, id)
}

func (s *Service) UpdateCatalog(ctx context.Context, id string, req *UpdateCatalogReq) (*Catalog, error) {
	return s.repo.UpdateCatalog(ctx, id, req)
}

func (s *Service) DeleteCatalog(ctx context.Context, id string) error {
	return s.repo.DeleteCatalog(ctx, id)
}

func (s *Service) AddImage(ctx context.Context, catalogID string, req *AddImageReq) (*CatalogImage, error) {
	return s.repo.AddImage(ctx, catalogID, req)
}

func (s *Service) UploadImage(ctx context.Context, catalogID string, filename, contentType string, reader io.Reader) (*CatalogImage, error) {
	if s.store == nil {
		return nil, fmt.Errorf("图片存储未启用（RustFS/S3）")
	}
	ext := strings.ToLower(path.Ext(filename))
	if ext == "" {
		ext = ".jpg"
	}
	key := fmt.Sprintf("goodser/catalogs/%s%s", uuid.NewString(), ext)
	if _, err := s.store.Upload(ctx, key, reader, storage.WithContentType(contentType)); err != nil {
		return nil, err
	}
	url, err := s.store.SignedURL(ctx, key, 7*24*time.Hour)
	if err != nil {
		return nil, err
	}
	if idx := strings.IndexByte(url, '?'); idx >= 0 {
		url = url[:idx]
	}
	return s.repo.AddImage(ctx, catalogID, &AddImageReq{ImageURL: url})
}

func (s *Service) CreateItem(ctx context.Context, catalogID string, req *CreateItemReq) (*CatalogItem, error) {
	return s.repo.CreateItem(ctx, catalogID, req)
}

func (s *Service) UpdateItem(ctx context.Context, id string, req *UpdateItemReq) (*CatalogItem, error) {
	return s.repo.UpdateItem(ctx, id, req)
}

func (s *Service) DeleteItem(ctx context.Context, id string) error {
	return s.repo.DeleteItem(ctx, id)
}

type visionSegmentResponse struct {
	Boxes []json.RawMessage `json:"boxes"`
	Crops []string          `json:"crops"`
}

func (s *Service) SegmentImage(ctx context.Context, imageID string) (*SegmentResult, error) {
	image, err := s.repo.GetImage(ctx, imageID)
	if err != nil {
		return nil, err
	}
	data, err := s.download(ctx, image.ImageURL)
	if err != nil {
		_ = s.repo.SetImageStatus(ctx, imageID, "failed", err.Error())
		return nil, err
	}
	_ = s.repo.SetImageStatus(ctx, imageID, "processing", "")
	result, err := s.callSegment(ctx, data)
	if err != nil {
		_ = s.repo.SetImageStatus(ctx, imageID, "failed", err.Error())
		return nil, err
	}
	items := make([]CatalogItem, 0, len(result.Crops))
	for idx, crop := range result.Crops {
		cropURL := crop
		if s.store != nil && strings.HasPrefix(crop, "data:image/") {
			uploaded, err := s.uploadDataURL(ctx, crop)
			if err == nil && uploaded != "" {
				cropURL = uploaded
			}
		}
		box := json.RawMessage(`null`)
		if idx < len(result.Boxes) {
			box = result.Boxes[idx]
		}
		item, err := s.repo.CreateItem(ctx, image.CatalogID, &CreateItemReq{
			ImageID:   image.ID,
			Title:     fmt.Sprintf("柄图 %d", idx+1),
			ImageURL:  cropURL,
			ThumbURL:  cropURL,
			Box:       box,
			SortOrder: idx,
		})
		if err != nil {
			continue
		}
		items = append(items, *item)
	}
	_ = s.repo.SetImageStatus(ctx, imageID, "done", "")
	return &SegmentResult{Boxes: result.Boxes, Items: items}, nil
}

func (s *Service) download(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("下载图片失败: HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 20*1024*1024))
}

func (s *Service) callSegment(ctx context.Context, image []byte) (*visionSegmentResponse, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("image", "catalog.jpg")
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(image); err != nil {
		return nil, err
	}
	_ = writer.Close()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.visionURL+"/v1/segment", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("视觉服务不可用: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("视觉服务返回 %d: %s", resp.StatusCode, string(payload))
	}
	var out visionSegmentResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *Service) uploadDataURL(ctx context.Context, dataURL string) (string, error) {
	idx := strings.Index(dataURL, ",")
	if idx < 0 {
		return "", fmt.Errorf("无效的 data URL")
	}
	meta := dataURL[:idx]
	ext := ".jpg"
	if strings.Contains(meta, "image/png") {
		ext = ".png"
	}
	raw, err := base64.StdEncoding.DecodeString(dataURL[idx+1:])
	if err != nil {
		return "", err
	}
	key := fmt.Sprintf("goodser/catalogs/items/%s%s", uuid.NewString(), ext)
	if _, err := s.store.Upload(ctx, key, bytes.NewReader(raw), storage.WithContentType("image/"+strings.TrimPrefix(ext, "."))); err != nil {
		return "", err
	}
	url, err := s.store.SignedURL(ctx, key, 7*24*time.Hour)
	if err != nil {
		return "", err
	}
	if i := strings.IndexByte(url, '?'); i >= 0 {
		url = url[:i]
	}
	return url, nil
}
