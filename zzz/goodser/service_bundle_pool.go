package goodser

import (
	"context"
	"database/sql"
	"strings"

	"github.com/google/uuid"
)

// ListBundlePools 查询捆物池
func (s *Service) ListBundlePools(ctx context.Context, inventoryID, status string) ([]BundlePool, error) {
	if strings.TrimSpace(inventoryID) == "" {
		return nil, NewGoodserError(ErrInventoryNotFound)
	}
	if status != "" && status != BundlePoolStatusActive && status != BundlePoolStatusInactive {
		return nil, NewGoodserError(ErrInvalidDimension, "捆物池状态仅支持 active/inactive")
	}
	return s.repo.ListBundlePools(ctx, inventoryID, status)
}

// GetBundlePool 获取捆物池详情
func (s *Service) GetBundlePool(ctx context.Context, id string) (*BundlePool, error) {
	if strings.TrimSpace(id) == "" {
		return nil, NewGoodserError(ErrBundlePoolNotFound)
	}
	return s.repo.GetBundlePool(ctx, id)
}

// CreateBundlePool 创建捆物池
func (s *Service) CreateBundlePool(ctx context.Context, req *CreateBundlePoolReq) (*BundlePool, error) {
	if req == nil {
		return nil, NewGoodserError(ErrBundlePoolNotFound)
	}
	if err := s.validateBundlePoolCreate(ctx, req); err != nil {
		return nil, err
	}
	return s.repo.CreateBundlePool(ctx, req)
}

// UpdateBundlePool 更新捆物池
func (s *Service) UpdateBundlePool(ctx context.Context, req *UpdateBundlePoolReq) (*BundlePool, error) {
	if req == nil || strings.TrimSpace(req.ID) == "" {
		return nil, NewGoodserError(ErrBundlePoolNotFound)
	}
	existing, err := s.repo.GetBundlePool(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	items := make([]BundlePoolItemReq, 0, len(existing.Items))
	for _, item := range existing.Items {
		items = append(items, BundlePoolItemReq{
			ProductID:       item.ProductID,
			DefaultQuantity: item.DefaultQuantity,
		})
	}
	candidate := &CreateBundlePoolReq{
		InventoryID:  existing.InventoryID,
		Code:         existing.Code,
		Name:         existing.Name,
		Description:  existing.Description,
		Status:       existing.Status,
		MinItemCount: existing.MinItemCount,
		MinAmount:    existing.MinAmount,
		Items:        items,
	}
	if req.Code != nil {
		candidate.Code = *req.Code
	}
	if req.Name != nil {
		candidate.Name = *req.Name
	}
	if req.Description != nil {
		candidate.Description = req.Description
	}
	if req.Status != nil {
		candidate.Status = *req.Status
	}
	if req.MinItemCount != nil {
		candidate.MinItemCount = *req.MinItemCount
	}
	if req.MinAmount != nil {
		candidate.MinAmount = *req.MinAmount
	}
	if req.Items != nil {
		candidate.Items = *req.Items
	}
	if err := s.validateBundlePoolCreate(ctx, candidate); err != nil {
		return nil, err
	}
	req.Code = &candidate.Code
	req.Name = &candidate.Name
	req.Description = candidate.Description
	req.Status = &candidate.Status
	req.MinItemCount = &candidate.MinItemCount
	req.MinAmount = &candidate.MinAmount
	req.Items = &candidate.Items
	return s.repo.UpdateBundlePool(ctx, req)
}

// DeleteBundlePool 删除捆物池
func (s *Service) DeleteBundlePool(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return NewGoodserError(ErrBundlePoolNotFound)
	}
	return s.repo.DeleteBundlePool(ctx, id)
}

func (s *Service) validateBundlePoolCreate(ctx context.Context, req *CreateBundlePoolReq) error {
	if strings.TrimSpace(req.Name) == "" {
		return NewGoodserError(ErrInvalidDimension, "捆物池名称不能为空")
	}
	if _, err := s.repo.GetInventory(ctx, req.InventoryID); err != nil {
		if err == sql.ErrNoRows {
			return NewGoodserError(ErrInventoryNotFound)
		}
		return err
	}
	req.Code = strings.TrimSpace(req.Code)
	req.Name = strings.TrimSpace(req.Name)
	if req.Code == "" {
		req.Code = "BP-" + strings.ReplaceAll(uuid.New().String(), "-", "")[:8]
	}
	if req.Status == "" {
		req.Status = BundlePoolStatusActive
	}
	if req.Status != BundlePoolStatusActive && req.Status != BundlePoolStatusInactive {
		return NewGoodserError(ErrInvalidDimension, "捆物池状态仅支持 active/inactive")
	}
	if req.MinItemCount <= 0 {
		req.MinItemCount = 1
	}
	if req.MinAmount < 0 {
		return NewGoodserError(ErrBundleRule, "最低捆物金额不能为负数")
	}
	if len(req.Items) == 0 {
		return NewGoodserError(ErrBundlePoolItemsEmpty)
	}

	seen := make(map[string]struct{}, len(req.Items))
	items := make([]BundlePoolItemReq, 0, len(req.Items))
	for _, item := range req.Items {
		productID := strings.TrimSpace(item.ProductID)
		if productID == "" {
			continue
		}
		if _, exists := seen[productID]; exists {
			return NewGoodserError(ErrBundleRule, "捆物池成员重复")
		}
		seen[productID] = struct{}{}
		product, err := s.repo.GetProduct(ctx, productID)
		if err != nil {
			if err == sql.ErrNoRows {
				return NewGoodserError(ErrProductNotFound)
			}
			return err
		}
		if product.InventoryID != req.InventoryID {
			return NewGoodserError(ErrProductInventory, "捆物池成员不属于该库存目录")
		}
		if item.DefaultQuantity <= 0 {
			item.DefaultQuantity = 1
		}
		items = append(items, BundlePoolItemReq{ProductID: productID, DefaultQuantity: item.DefaultQuantity})
	}
	if len(items) == 0 {
		return NewGoodserError(ErrBundlePoolItemsEmpty)
	}
	req.Items = items
	return nil
}

// bundlePoolSubProducts 把池成员转换为副区商品结构，供既有校验与下单逻辑复用
func bundlePoolSubProducts(pool *BundlePool) []BundleSubProduct {
	if pool == nil {
		return nil
	}
	items := make([]BundleSubProduct, 0, len(pool.Items))
	for _, item := range pool.Items {
		items = append(items, BundleSubProduct{
			ProductID:       item.ProductID,
			ProductName:     item.ProductName,
			ProductCode:     item.ProductCode,
			ImageURL:        item.ImageURL,
			DefaultQuantity: item.DefaultQuantity,
			ExpectedPrice:   item.ExpectedPrice,
			Available:       item.Available,
			ReservedQty:     item.ReservedQty,
			TotalQuantity:   item.TotalQuantity,
		})
	}
	return items
}
