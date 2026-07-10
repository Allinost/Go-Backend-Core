package goodser

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Service 业务逻辑层
type Service struct {
	repo *Repository
}

// NewService 创建 Service
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// ListInventories 获取库存目录列表
func (s *Service) ListInventories(ctx context.Context) ([]Inventory, error) {
	return s.repo.ListInventories(ctx)
}

// GetInventory 获取单个库存目录
func (s *Service) GetInventory(ctx context.Context, id string) (*Inventory, error) {
	item, err := s.repo.GetInventory(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, NewGoodserError(ErrInventoryNotFound)
		}
		return nil, err
	}
	return item, nil
}

// CreateInventory 创建库存目录
func (s *Service) CreateInventory(ctx context.Context, req *CreateInventoryReq) (*Inventory, error) {
	return s.repo.CreateInventory(ctx, req)
}

// UpdateInventory 更新库存目录
func (s *Service) UpdateInventory(ctx context.Context, req *UpdateInventoryReq) (*Inventory, error) {
	item, err := s.repo.GetInventory(ctx, req.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, NewGoodserError(ErrInventoryNotFound)
		}
		return nil, err
	}
	if req.Name != nil {
		item.Name = *req.Name
	}
	return s.repo.UpdateInventory(ctx, req)
}

// DeleteInventory 删除库存目录
func (s *Service) DeleteInventory(ctx context.Context, id string) error {
	return s.repo.DeleteInventory(ctx, id)
}

// ListProducts 获取商品列表
func (s *Service) ListProducts(ctx context.Context, inventoryID string) ([]Product, error) {
	return s.repo.ListProducts(ctx, inventoryID)
}

// SearchProducts 搜索商品
func (s *Service) SearchProducts(ctx context.Context, inventoryID, keyword string) ([]Product, error) {
	return s.repo.SearchProducts(ctx, inventoryID, keyword)
}

// AllocateSeq 分配序号（优先复用回收序号）
func (s *Service) AllocateSeq(ctx context.Context, req *AllocateSeqReq) (int, error) {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	seq, err := s.repo.PopRecycledSeq(ctx, tx, req.InventoryID, req.MainZone, req.SubZone)
	if err == nil {
		tx.Commit()
		return seq, nil
	}

	seq, err = s.repo.IncrementSeqCounter(ctx, tx, req.InventoryID, req.MainZone, req.SubZone)
	if err != nil {
		return 0, err
	}

	tx.Commit()
	return seq, nil
}

// CreateProduct 手动创建商品
func (s *Service) CreateProduct(ctx context.Context, req *CreateProductReq) (*Product, error) {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	result, err := s.repo.CreateProductTx(ctx, tx, req)
	if err != nil {
		return nil, err
	}

	tx.Commit()
	return result, nil
}

// UpdateProduct 更新商品
func (s *Service) UpdateProduct(ctx context.Context, req *UpdateProductReq) (*Product, error) {
	product, err := s.repo.UpdateProduct(ctx, req)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, NewGoodserError(ErrProductNotFound)
		}
		return nil, err
	}
	return product, nil
}

// DeleteProduct 删除商品
func (s *Service) DeleteProduct(ctx context.Context, id string) error {
	product, err := s.repo.GetProduct(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return NewGoodserError(ErrProductNotFound)
		}
		return err
	}
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := s.repo.DeleteProduct(ctx, id); err != nil {
		return err
	}
	if err := s.repo.AddRecycledSeqTx(ctx, tx, product.InventoryID, product.MainZone, product.SubZone, product.SeqNumber); err != nil {
		return err
	}
	tx.Commit()
	return nil
}

// InboundSingle 单品入库
func (s *Service) InboundSingle(ctx context.Context, req *InboundSingleReq) (*Product, error) {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	code := fmt.Sprintf("%s-%s-%04d-%04d-%s", req.MainZone, req.SubZone, req.SeqNumber, 0, req.StatusCode)
	product, err := s.repo.FindProductByCode(ctx, req.InventoryID, code)
	var result *Product

	if err == sql.ErrNoRows {
		qty := 0
		if req.Quantity != nil {
			qty = *req.Quantity
		}
		createReq := &CreateProductReq{
			InventoryID:     req.InventoryID,
			Code:            code,
			MainZone:        req.MainZone,
			SubZone:         req.SubZone,
			SeqNumber:       req.SeqNumber,
			Quantity:        &qty,
			StatusCode:      req.StatusCode,
			Name:            req.Name,
			OriginalPrice:   req.OriginalPrice,
			MarketPrice:     req.MarketPrice,
			ExpectedPrice:   req.ExpectedPrice,
			Remark:          req.Remark,
			StorageLocation: req.StorageLocation,
			ImageURL:        req.ImageURL,
			Tags:            req.Tags,
		}
		result, err = s.repo.CreateProductTx(ctx, tx, createReq)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else {
		qty := 1
		if req.Quantity != nil {
			qty = *req.Quantity
		}
		if err := s.repo.AddProductQuantityTx(ctx, tx, product.ID, qty); err != nil {
			return nil, err
		}
		result = product
		result.Quantity += qty
	}

	items, _ := json.Marshal([]OrderItem{{
		ProductID:   result.ID,
		ProductName: result.Name,
		ProductCode: result.Code,
		Quantity:    result.Quantity,
	}})
	if _, err := s.repo.CreateInboundLogItemTx(ctx, tx, req.InventoryID, "single", items); err != nil {
		return nil, err
	}

	tx.Commit()
	return result, nil
}

// InboundBatch 批量入库
func (s *Service) InboundBatch(ctx context.Context, req *InboundBatchReq) (*InboundBatchResp, error) {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var results []Product
	for _, item := range req.Items {
		code := fmt.Sprintf("%s-%s-%04d-%04d-%s", item.MainZone, item.SubZone, item.SeqNumber, 0, item.StatusCode)
		product, err := s.repo.FindProductByCode(ctx, req.InventoryID, code)
		if err == sql.ErrNoRows {
			qty := 0
			if item.Quantity != nil {
				qty = *item.Quantity
			}
			createReq := &CreateProductReq{
				InventoryID:     req.InventoryID,
				Code:            code,
				MainZone:        item.MainZone,
				SubZone:         item.SubZone,
				SeqNumber:       item.SeqNumber,
				Quantity:        &qty,
				StatusCode:      item.StatusCode,
				Name:            item.Name,
				OriginalPrice:   item.OriginalPrice,
				MarketPrice:     item.MarketPrice,
				ExpectedPrice:   item.ExpectedPrice,
				Remark:          item.Remark,
				StorageLocation: item.StorageLocation,
				ImageURL:        item.ImageURL,
				Tags:            item.Tags,
			}
			p, err := s.repo.CreateProductTx(ctx, tx, createReq)
			if err != nil {
				return nil, err
			}
			results = append(results, *p)
		} else if err != nil {
			return nil, err
		} else {
			qty := 1
			if item.Quantity != nil {
				qty = *item.Quantity
			}
			if err := s.repo.AddProductQuantityTx(ctx, tx, product.ID, qty); err != nil {
				return nil, err
			}
			product.Quantity += qty
			results = append(results, *product)
		}
	}

	itemsJSON, _ := json.Marshal(results)
	if _, err := s.repo.CreateInboundLogItemTx(ctx, tx, req.InventoryID, "batch", itemsJSON); err != nil {
		return nil, err
	}

	tx.Commit()
	return &InboundBatchResp{Items: results, Count: len(results)}, nil
}

// InboundSearchImport 搜索导入入库
func (s *Service) InboundSearchImport(ctx context.Context, req *InboundSearchImportReq) (*InboundSearchImportResp, error) {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var results []Product
	for _, item := range req.Items {
		product, err := s.repo.GetProductForUpdateTx(ctx, tx, item.ProductID)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, NewGoodserError(ErrProductNotFound)
			}
			return nil, err
		}
		if err := s.repo.AddProductQuantityTx(ctx, tx, product.ID, item.Quantity); err != nil {
			return nil, err
		}
		product.Quantity += item.Quantity
		results = append(results, *product)
	}

	itemsJSON, _ := json.Marshal(results)
	if _, err := s.repo.CreateInboundLogItemTx(ctx, tx, req.InventoryID, "search", itemsJSON); err != nil {
		return nil, err
	}

	tx.Commit()
	return &InboundSearchImportResp{Items: results, Count: len(results)}, nil
}

// ListOutboundOrders 获取出库单列表
func (s *Service) ListOutboundOrders(ctx context.Context, inventoryID string) ([]OutboundOrder, error) {
	return s.repo.ListOutboundOrders(ctx, inventoryID)
}

// CreateOutboundOrder 创建出库单（同时锁定预留库存）
func (s *Service) CreateOutboundOrder(ctx context.Context, req *CreateOutboundReq) (*OutboundOrder, error) {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// 锁定库存
	for _, item := range req.Items {
		product, err := s.repo.GetProductForUpdateTx(ctx, tx, item.ProductID)
		if err != nil {
			return nil, err
		}
		if product.Quantity-product.ReservedQuantity < item.Quantity {
			return nil, NewGoodserError(ErrInsufficientStock)
		}
		if err := s.repo.UpdateProductReservedTx(ctx, tx, item.ProductID, 0, item.Quantity); err != nil {
			return nil, err
		}
	}

	order, err := s.repo.CreateOutboundOrderTx(ctx, tx, req)
	if err != nil {
		return nil, err
	}

	tx.Commit()
	return order, nil
}

// ConfirmOutbound 确认出库
func (s *Service) ConfirmOutbound(ctx context.Context, req *ConfirmOutboundReq) (*OutboundOrder, error) {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	order, err := s.repo.GetOutboundOrderForUpdateTx(ctx, tx, req.ID)
	if err != nil {
		return nil, err
	}
	if order.Status != string(OrderStatusPending) && order.Status != string(OrderStatusReserved) {
		return nil, NewGoodserError(ErrInvalidOrderStatus)
	}

	var orderItems []OrderItem
	if err := json.Unmarshal(order.Items, &orderItems); err != nil {
		return nil, err
	}

	for _, item := range orderItems {
		if err := s.repo.UpdateProductReservedTx(ctx, tx, item.ProductID, -item.Quantity, -item.Quantity); err != nil {
			return nil, err
		}
	}

	now := time.Now()
	if err := s.repo.UpdateOutboundOrderStatusTx(ctx, tx, order.ID, string(OrderStatusConfirmed), &now, nil); err != nil {
		return nil, err
	}

	tx.Commit()
	order.Status = string(OrderStatusConfirmed)
	return order, nil
}

// CancelOutbound 取消出库
func (s *Service) CancelOutbound(ctx context.Context, req *CancelOutboundReq) (*OutboundOrder, error) {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	order, err := s.repo.GetOutboundOrderForUpdateTx(ctx, tx, req.ID)
	if err != nil {
		return nil, err
	}
	if order.Status != string(OrderStatusPending) && order.Status != string(OrderStatusReserved) {
		return nil, NewGoodserError(ErrInvalidOrderStatus)
	}

	var orderItems []OrderItem
	if err := json.Unmarshal(order.Items, &orderItems); err != nil {
		return nil, err
	}

	for _, item := range orderItems {
		if err := s.repo.UpdateProductReservedTx(ctx, tx, item.ProductID, 0, -item.Quantity); err != nil {
			return nil, err
		}
	}

	now := time.Now()
	if err := s.repo.UpdateOutboundOrderStatusTx(ctx, tx, order.ID, string(OrderStatusCancelled), nil, &now); err != nil {
		return nil, err
	}

	tx.Commit()
	order.Status = string(OrderStatusCancelled)
	return order, nil
}

// ListInboundLogs 获取入库日志
func (s *Service) ListInboundLogs(ctx context.Context, inventoryID string) ([]InboundLog, error) {
	return s.repo.ListInboundLogs(ctx, inventoryID)
}

// ListTags 获取标签列表
func (s *Service) ListTags(ctx context.Context) ([]Tag, error) {
	return s.repo.ListTags(ctx)
}

// CreateTag 创建标签
func (s *Service) CreateTag(ctx context.Context, req *CreateTagReq) (*Tag, error) {
	return s.repo.CreateTag(ctx, req)
}

// ListStatusCodes 获取状态编码列表
func (s *Service) ListStatusCodes(ctx context.Context) ([]StatusCode, error) {
	return s.repo.ListStatusCodes(ctx)
}

// timeNowFunc 便于测试注入
type timeNowFunc func() time.Time
