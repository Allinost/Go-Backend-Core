package goodsercatalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Allinost/go-backend-core/internal/pkg/dbutil"
	"github.com/google/uuid"
)

const (
	tableCatalog      = "zzz_goodser_catalogs"
	tableCatalogImage = "zzz_goodser_catalog_images"
	tableCatalogItem  = "zzz_goodser_catalog_items"
)

var ErrNotFound = errors.New("goodser catalog: not found")

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

func (r *Repository) query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return r.db.QueryContext(ctx, dbutil.Rebind(query), args...)
}

func (r *Repository) queryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return r.db.QueryRowContext(ctx, dbutil.Rebind(query), args...)
}

func (r *Repository) exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return r.db.ExecContext(ctx, dbutil.Rebind(query), args...)
}

func (r *Repository) ListCatalogs(ctx context.Context, req *ListCatalogsReq) ([]Catalog, error) {
	conditions := []string{"1=1"}
	args := []any{}
	if strings.TrimSpace(req.InventoryID) != "" {
		conditions = append(conditions, "COALESCE(c.inventory_id, '') = ?")
		args = append(args, strings.TrimSpace(req.InventoryID))
	}
	if keyword := strings.TrimSpace(req.Keyword); keyword != "" {
		conditions = append(conditions, "(c.title ILIKE ? OR COALESCE(c.remark, '') ILIKE ?)")
		like := "%" + keyword + "%"
		args = append(args, like, like)
	}
	query := `SELECT c.id, COALESCE(c.inventory_id, ''), c.title, COALESCE(c.remark, ''),
		COALESCE(c.cover_url, ''), c.status, c.created_at, c.updated_at,
		(SELECT COUNT(*) FROM ` + tableCatalogImage + ` i WHERE i.catalog_id = c.id),
		(SELECT COUNT(*) FROM ` + tableCatalogItem + ` it WHERE it.catalog_id = c.id)
		FROM ` + tableCatalog + ` c WHERE ` + strings.Join(conditions, " AND ") + `
		ORDER BY c.updated_at DESC, c.id DESC`
	rows, err := r.query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Catalog
	for rows.Next() {
		item, err := scanCatalog(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	if out == nil {
		out = []Catalog{}
	}
	return out, rows.Err()
}

func (r *Repository) CreateCatalog(ctx context.Context, req *CreateCatalogReq, owner int64) (*Catalog, error) {
	id := uuid.NewString()
	var inventory any
	if strings.TrimSpace(req.InventoryID) != "" {
		inventory = strings.TrimSpace(req.InventoryID)
	}
	_, err := r.exec(ctx,
		`INSERT INTO `+tableCatalog+` (id, inventory_id, title, remark, status, owner_user_id)
		 VALUES (?, ?, ?, ?, 'active', NULLIF(?, 0))`,
		id, inventory, strings.TrimSpace(req.Title), strings.TrimSpace(req.Remark), owner)
	if err != nil {
		return nil, err
	}
	return r.GetCatalog(ctx, id)
}

func (r *Repository) GetCatalog(ctx context.Context, id string) (*Catalog, error) {
	query := `SELECT c.id, COALESCE(c.inventory_id, ''), c.title, COALESCE(c.remark, ''),
		COALESCE(c.cover_url, ''), c.status, c.created_at, c.updated_at,
		(SELECT COUNT(*) FROM ` + tableCatalogImage + ` i WHERE i.catalog_id = c.id),
		(SELECT COUNT(*) FROM ` + tableCatalogItem + ` it WHERE it.catalog_id = c.id)
		FROM ` + tableCatalog + ` c WHERE c.id = ?`
	item, err := scanCatalog(r.queryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return item, nil
}

func (r *Repository) GetDetail(ctx context.Context, id string) (*CatalogDetail, error) {
	catalog, err := r.GetCatalog(ctx, id)
	if err != nil {
		return nil, err
	}
	images, err := r.ListImages(ctx, id)
	if err != nil {
		return nil, err
	}
	items, err := r.ListItems(ctx, id)
	if err != nil {
		return nil, err
	}
	return &CatalogDetail{Catalog: *catalog, Images: images, Items: items}, nil
}

func (r *Repository) UpdateCatalog(ctx context.Context, id string, req *UpdateCatalogReq) (*Catalog, error) {
	existing, err := r.GetCatalog(ctx, id)
	if err != nil {
		return nil, err
	}
	title := existing.Title
	remark := existing.Remark
	status := existing.Status
	if req.Title != nil {
		title = strings.TrimSpace(*req.Title)
	}
	if req.Remark != nil {
		remark = strings.TrimSpace(*req.Remark)
	}
	if req.Status != nil && strings.TrimSpace(*req.Status) != "" {
		status = strings.TrimSpace(*req.Status)
	}
	if title == "" {
		return nil, fmt.Errorf("标题不能为空")
	}
	_, err = r.exec(ctx,
		`UPDATE `+tableCatalog+` SET title = ?, remark = ?, status = ?, updated_at = now() WHERE id = ?`,
		title, remark, status, id)
	if err != nil {
		return nil, err
	}
	return r.GetCatalog(ctx, id)
}

func (r *Repository) DeleteCatalog(ctx context.Context, id string) error {
	res, err := r.exec(ctx, `DELETE FROM `+tableCatalog+` WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) ListImages(ctx context.Context, catalogID string) ([]CatalogImage, error) {
	rows, err := r.query(ctx,
		`SELECT id, catalog_id, image_url, COALESCE(width, 0), COALESCE(height, 0),
		 crop_status, COALESCE(crop_error, ''), created_at
		 FROM `+tableCatalogImage+` WHERE catalog_id = ? ORDER BY created_at ASC, id ASC`, catalogID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CatalogImage
	for rows.Next() {
		item, err := scanImage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	if out == nil {
		out = []CatalogImage{}
	}
	return out, rows.Err()
}

func (r *Repository) AddImage(ctx context.Context, catalogID string, req *AddImageReq) (*CatalogImage, error) {
	if _, err := r.GetCatalog(ctx, catalogID); err != nil {
		return nil, err
	}
	id := uuid.NewString()
	_, err := r.exec(ctx,
		`INSERT INTO `+tableCatalogImage+` (id, catalog_id, image_url, width, height, crop_status)
		 VALUES (?, ?, ?, ?, ?, 'pending')`,
		id, catalogID, strings.TrimSpace(req.ImageURL), req.Width, req.Height)
	if err != nil {
		return nil, err
	}
	_, _ = r.exec(ctx,
		`UPDATE `+tableCatalog+` SET cover_url = COALESCE(NULLIF(cover_url, ''), ?), updated_at = now() WHERE id = ?`,
		strings.TrimSpace(req.ImageURL), catalogID)
	return r.GetImage(ctx, id)
}

func (r *Repository) GetImage(ctx context.Context, id string) (*CatalogImage, error) {
	item, err := scanImage(r.queryRow(ctx,
		`SELECT id, catalog_id, image_url, COALESCE(width, 0), COALESCE(height, 0),
		 crop_status, COALESCE(crop_error, ''), created_at
		 FROM `+tableCatalogImage+` WHERE id = ?`, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return item, nil
}

func (r *Repository) SetImageStatus(ctx context.Context, id, status, errText string) error {
	_, err := r.exec(ctx,
		`UPDATE `+tableCatalogImage+` SET crop_status = ?, crop_error = ?, updated_at = now() WHERE id = ?`,
		status, errText, id)
	return err
}

func (r *Repository) ListItems(ctx context.Context, catalogID string) ([]CatalogItem, error) {
	rows, err := r.query(ctx,
		`SELECT id, catalog_id, COALESCE(image_id, ''), COALESCE(product_id, ''),
		 COALESCE(title, ''), COALESCE(remark, ''), image_url, COALESCE(thumb_url, ''),
		 COALESCE(box, 'null'::jsonb), COALESCE(sort_order, 0), status, created_at, updated_at
		 FROM `+tableCatalogItem+` WHERE catalog_id = ? ORDER BY sort_order ASC, created_at ASC, id ASC`, catalogID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CatalogItem
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	if out == nil {
		out = []CatalogItem{}
	}
	return out, rows.Err()
}

func (r *Repository) CreateItem(ctx context.Context, catalogID string, req *CreateItemReq) (*CatalogItem, error) {
	if _, err := r.GetCatalog(ctx, catalogID); err != nil {
		return nil, err
	}
	id := uuid.NewString()
	box := req.Box
	if len(box) == 0 {
		box = json.RawMessage(`null`)
	}
	_, err := r.exec(ctx,
		`INSERT INTO `+tableCatalogItem+`
		 (id, catalog_id, image_id, product_id, title, remark, image_url, thumb_url, box, sort_order, status)
		 VALUES (?, ?, NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?, NULLIF(?, ''), ?, ?, 'active')`,
		id, catalogID, strings.TrimSpace(req.ImageID), strings.TrimSpace(req.ProductID),
		strings.TrimSpace(req.Title), strings.TrimSpace(req.Remark), strings.TrimSpace(req.ImageURL),
		strings.TrimSpace(req.ThumbURL), box, req.SortOrder)
	if err != nil {
		return nil, err
	}
	return r.GetItem(ctx, id)
}

func (r *Repository) GetItem(ctx context.Context, id string) (*CatalogItem, error) {
	item, err := scanItem(r.queryRow(ctx,
		`SELECT id, catalog_id, COALESCE(image_id, ''), COALESCE(product_id, ''),
		 COALESCE(title, ''), COALESCE(remark, ''), image_url, COALESCE(thumb_url, ''),
		 COALESCE(box, 'null'::jsonb), COALESCE(sort_order, 0), status, created_at, updated_at
		 FROM `+tableCatalogItem+` WHERE id = ?`, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return item, nil
}

func (r *Repository) UpdateItem(ctx context.Context, id string, req *UpdateItemReq) (*CatalogItem, error) {
	existing, err := r.GetItem(ctx, id)
	if err != nil {
		return nil, err
	}
	productID := existing.ProductID
	title := existing.Title
	remark := existing.Remark
	imageURL := existing.ImageURL
	thumbURL := existing.ThumbURL
	box := existing.Box
	sortOrder := existing.SortOrder
	status := existing.Status
	if req.ProductID != nil {
		productID = strings.TrimSpace(*req.ProductID)
	}
	if req.Title != nil {
		title = strings.TrimSpace(*req.Title)
	}
	if req.Remark != nil {
		remark = strings.TrimSpace(*req.Remark)
	}
	if req.ImageURL != nil && strings.TrimSpace(*req.ImageURL) != "" {
		imageURL = strings.TrimSpace(*req.ImageURL)
	}
	if req.ThumbURL != nil {
		thumbURL = strings.TrimSpace(*req.ThumbURL)
	}
	if len(req.Box) > 0 {
		box = req.Box
	}
	if req.SortOrder != nil {
		sortOrder = *req.SortOrder
	}
	if req.Status != nil && strings.TrimSpace(*req.Status) != "" {
		status = strings.TrimSpace(*req.Status)
	}
	_, err = r.exec(ctx,
		`UPDATE `+tableCatalogItem+`
		 SET product_id = NULLIF(?, ''), title = ?, remark = ?, image_url = ?, thumb_url = NULLIF(?, ''),
		     box = ?, sort_order = ?, status = ?, updated_at = now()
		 WHERE id = ?`,
		productID, title, remark, imageURL, thumbURL, box, sortOrder, status, id)
	if err != nil {
		return nil, err
	}
	return r.GetItem(ctx, id)
}

func (r *Repository) DeleteItem(ctx context.Context, id string) error {
	res, err := r.exec(ctx, `DELETE FROM `+tableCatalogItem+` WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanCatalog(row interface{ Scan(...any) error }) (*Catalog, error) {
	var item Catalog
	err := row.Scan(&item.ID, &item.InventoryID, &item.Title, &item.Remark, &item.CoverURL,
		&item.Status, &item.CreatedAt, &item.UpdatedAt, &item.ImageCount, &item.ItemCount)
	return &item, err
}

func scanImage(row interface{ Scan(...any) error }) (*CatalogImage, error) {
	var item CatalogImage
	err := row.Scan(&item.ID, &item.CatalogID, &item.ImageURL, &item.Width, &item.Height,
		&item.CropStatus, &item.CropError, &item.CreatedAt)
	return &item, err
}

func scanItem(row interface{ Scan(...any) error }) (*CatalogItem, error) {
	var item CatalogItem
	var box []byte
	err := row.Scan(&item.ID, &item.CatalogID, &item.ImageID, &item.ProductID,
		&item.Title, &item.Remark, &item.ImageURL, &item.ThumbURL, &box,
		&item.SortOrder, &item.Status, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if len(box) > 0 {
		item.Box = json.RawMessage(box)
	}
	return &item, nil
}

func nowPtr() *time.Time {
	now := time.Now().UTC()
	return &now
}
