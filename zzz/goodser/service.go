package goodser

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/Allinost/go-backend-core/internal/pkg/logger"
	"github.com/Allinost/go-backend-core/internal/services/storage"
)

// Service 业务逻辑层
type Service struct {
	repo  *Repository
	store storage.Storage
}

// NewService 创建 Service
func NewService(repo *Repository, store storage.Storage) *Service {
	return &Service{repo: repo, store: store}
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

// ListProductsPaginated 分页获取商品列表
func (s *Service) ListProductsPaginated(ctx context.Context, inventoryID string, page, pageSize int) ([]Product, bool, int, error) {
	return s.repo.ListProductsPaginated(ctx, inventoryID, pageSize, (page-1)*pageSize)
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
		if err := tx.Commit(); err != nil {
			return 0, err
		}
		return seq, nil
	}

	seq, err = s.repo.IncrementSeqCounter(ctx, tx, req.InventoryID, req.MainZone, req.SubZone)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return seq, nil
}

// CreateProduct 手动创建商品
func (s *Service) CreateProduct(ctx context.Context, req *CreateProductReq) (*Product, error) {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// 优先复用回收序号
	seq, err := s.repo.PopRecycledSeq(ctx, tx, req.InventoryID, req.MainZone, req.SubZone)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	useSeq := req.SeqNumber
	if err == nil {
		useSeq = seq
	}
	req.SeqNumber = useSeq

	result, err := s.repo.CreateProductTx(ctx, tx, req)
	if err != nil {
		return nil, err
	}

	if err := s.repo.EnsureSeqCounter(ctx, tx, req.InventoryID, req.MainZone, req.SubZone, useSeq); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
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

// imageKeyFromURL 从 S3 预签名 URL 中解析出对象存储 key
func imageKeyFromURL(imageURL string) string {
	if imageURL == "" {
		return ""
	}
	u, err := url.Parse(imageURL)
	if err != nil {
		return ""
	}
	idx := strings.Index(u.Path, "goodser/images/")
	if idx < 0 {
		return ""
	}
	return u.Path[idx:]
}

// DeleteProduct 删除商品（含 RustFS 图片清理）
func (s *Service) DeleteProduct(ctx context.Context, id string) error {
	product, err := s.repo.GetProduct(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return NewGoodserError(ErrProductNotFound)
		}
		return err
	}

	if s.store != nil {
		if product.ImageURL != nil && *product.ImageURL != "" {
			key := imageKeyFromURL(*product.ImageURL)
			if key != "" {
				if err := s.store.Delete(ctx, key); err != nil {
					logger.Error().Err(err).Str("key", key).Msg("删除商品图片失败")
				}
			}
		}
		if len(product.Images) > 0 {
			var urls []string
			if err := json.Unmarshal(product.Images, &urls); err == nil {
				for _, u := range urls {
					if u == "" {
						continue
					}
					key := imageKeyFromURL(u)
					if key != "" {
						if err := s.store.Delete(ctx, key); err != nil {
							logger.Error().Err(err).Str("key", key).Msg("删除商品多图失败")
						}
					}
				}
			}
		}
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
	return tx.Commit()
}

// InboundSingle 单品入库
func (s *Service) InboundSingle(ctx context.Context, req *InboundSingleReq) (*Product, error) {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// 优先复用回收序号
	seq, err := s.repo.PopRecycledSeq(ctx, tx, req.InventoryID, req.MainZone, req.SubZone)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	useSeq := req.SeqNumber
	if err == nil {
		useSeq = seq
	}

	code := fmt.Sprintf("%s-%s-%04d-%04d-%s", req.MainZone, req.SubZone, useSeq, 0, req.StatusCode)
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
			SeqNumber:       useSeq,
			Quantity:        &qty,
			StatusCode:      req.StatusCode,
			Name:            req.Name,
			OriginalPrice:   req.OriginalPrice,
			MarketPrice:     req.MarketPrice,
			ExpectedPrice:   req.ExpectedPrice,
			Remark:          req.Remark,
			StorageLocation: req.StorageLocation,
			ImageURL:        req.ImageURL,
			Images:          req.Images,
			Tags:            req.Tags,
		}
		result, err = s.repo.CreateProductTx(ctx, tx, createReq)
		if err != nil {
			return nil, err
		}
		if err := s.repo.EnsureSeqCounter(ctx, tx, req.InventoryID, req.MainZone, req.SubZone, useSeq); err != nil {
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

	if err := tx.Commit(); err != nil {
		return nil, err
	}
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
		// 优先复用回收序号
		seq, err := s.repo.PopRecycledSeq(ctx, tx, req.InventoryID, item.MainZone, item.SubZone)
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
		useSeq := item.SeqNumber
		if err == nil {
			useSeq = seq
		}

		code := fmt.Sprintf("%s-%s-%04d-%04d-%s", item.MainZone, item.SubZone, useSeq, 0, item.StatusCode)
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
				SeqNumber:       useSeq,
				Quantity:        &qty,
				StatusCode:      item.StatusCode,
				Name:            item.Name,
				OriginalPrice:   item.OriginalPrice,
				MarketPrice:     item.MarketPrice,
				ExpectedPrice:   item.ExpectedPrice,
				Remark:          item.Remark,
				StorageLocation: item.StorageLocation,
				ImageURL:        item.ImageURL,
				Images:          item.Images,
				Tags:            item.Tags,
			}
			p, err := s.repo.CreateProductTx(ctx, tx, createReq)
			if err != nil {
				return nil, err
			}
			if err := s.repo.EnsureSeqCounter(ctx, tx, req.InventoryID, item.MainZone, item.SubZone, useSeq); err != nil {
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

	if err := tx.Commit(); err != nil {
		return nil, err
	}
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

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &InboundSearchImportResp{Items: results, Count: len(results)}, nil
}

// ListOutboundOrders 获取出库单列表
func (s *Service) ListOutboundOrders(ctx context.Context, inventoryID string) ([]OutboundOrder, error) {
	return s.repo.ListOutboundOrders(ctx, inventoryID)
}

// ListOutboundOrdersPaginated 分页获取出库单列表
func (s *Service) ListOutboundOrdersPaginated(ctx context.Context, inventoryID string, page, pageSize int) ([]OutboundOrder, bool, int, error) {
	return s.repo.ListOutboundOrdersPaginated(ctx, inventoryID, pageSize, (page-1)*pageSize)
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

	if err := tx.Commit(); err != nil {
		return nil, err
	}
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

	if err := tx.Commit(); err != nil {
		return nil, err
	}
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

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	order.Status = string(OrderStatusCancelled)
	return order, nil
}

// ListInboundLogs 获取入库日志
func (s *Service) ListInboundLogs(ctx context.Context, inventoryID string) ([]InboundLog, error) {
	return s.repo.ListInboundLogs(ctx, inventoryID)
}

// ListInboundLogsPaginated 分页获取入库日志
func (s *Service) ListInboundLogsPaginated(ctx context.Context, inventoryID string, page, pageSize int) ([]InboundLog, bool, int, error) {
	return s.repo.ListInboundLogsPaginated(ctx, inventoryID, pageSize, (page-1)*pageSize)
}

// ListTags 获取标签列表
func (s *Service) ListTags(ctx context.Context) ([]Tag, error) {
	return s.repo.ListTags(ctx)
}

// CreateTag 创建标签
func (s *Service) CreateTag(ctx context.Context, req *CreateTagReq) (*Tag, error) {
	return s.repo.CreateTag(ctx, req)
}

// UpdateTag 更新标签
func (s *Service) UpdateTag(ctx context.Context, req *UpdateTagReq) (*Tag, error) {
	item, err := s.repo.UpdateTag(ctx, req)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, NewGoodserError(ErrTagNotFound)
		}
		return nil, err
	}
	return item, nil
}

// DeleteTag 删除标签
func (s *Service) DeleteTag(ctx context.Context, id string) error {
	if err := s.repo.DeleteTag(ctx, id); err != nil {
		return err
	}
	return nil
}

// ListStatusCodes 获取状态编码列表
func (s *Service) ListStatusCodes(ctx context.Context) ([]StatusCode, error) {
	return s.repo.ListStatusCodes(ctx)
}

// CreateStatusCode 创建状态码
func (s *Service) CreateStatusCode(ctx context.Context, req *AddStatusCodeReq) (*StatusCode, error) {
	return s.repo.CreateStatusCode(ctx, req)
}

// UpdateStatusCode 更新状态码
func (s *Service) UpdateStatusCode(ctx context.Context, req *UpdateStatusCodeReq) (*StatusCode, error) {
	item, err := s.repo.UpdateStatusCode(ctx, req)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, NewGoodserError(ErrStatusCodeNotFound)
		}
		return nil, err
	}
	return item, nil
}

// DeleteStatusCode 删除状态码
func (s *Service) DeleteStatusCode(ctx context.Context, id string) error {
	sc, err := s.repo.GetStatusCode(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return NewGoodserError(ErrStatusCodeNotFound)
		}
		return err
	}
	count, err := s.repo.CountProductsByStatusCode(ctx, sc.Code)
	if err != nil {
		return err
	}
	if count > 0 {
		return NewGoodserError(ErrStatusCodeInUse)
	}
	if err := s.repo.DeleteStatusCode(ctx, id); err != nil {
		return err
	}
	return nil
}

// CreateInboundLog 创建入库日志
func (s *Service) CreateInboundLog(ctx context.Context, req *CreateInboundLogReq) (*InboundLog, error) {
	return s.repo.CreateInboundLog(ctx, req)
}

// UpdateInboundLog 更新入库日志
func (s *Service) UpdateInboundLog(ctx context.Context, req *UpdateInboundLogReq) (*InboundLog, error) {
	item, err := s.repo.UpdateInboundLog(ctx, req)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, NewGoodserError(ErrInboundLogNotFound)
		}
		return nil, err
	}
	return item, nil
}

// DeleteInboundLog 删除入库日志
func (s *Service) DeleteInboundLog(ctx context.Context, id string) error {
	if err := s.repo.DeleteInboundLog(ctx, id); err != nil {
		return err
	}
	return nil
}

// CancelReserve 取消预留单
func (s *Service) CancelReserve(ctx context.Context, req *CancelReserveReq) (*OutboundOrder, error) {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	order, err := s.repo.GetOutboundOrderForUpdateTx(ctx, tx, req.ID)
	if err != nil {
		return nil, err
	}
	if order.Type != string(OrderTypeReserve) {
		return nil, NewGoodserError(ErrInvalidOrderStatus)
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

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	order.Status = string(OrderStatusCancelled)
	return order, nil
}

// ReserveToOutbound 预留单转出库单
func (s *Service) ReserveToOutbound(ctx context.Context, req *ReserveToOutboundReq) (*OutboundOrder, error) {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	order, err := s.repo.GetOutboundOrderForUpdateTx(ctx, tx, req.ID)
	if err != nil {
		return nil, err
	}
	if order.Type != string(OrderTypeReserve) {
		return nil, NewGoodserError(ErrInvalidOrderStatus)
	}
	if order.Status != string(OrderStatusPending) && order.Status != string(OrderStatusReserved) {
		return nil, NewGoodserError(ErrInvalidOrderStatus)
	}

	// 释放原预留的库存，确认预留单
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
	if err := s.repo.UpdateOutboundOrderStatusTx(ctx, tx, order.ID, string(OrderStatusConfirmed), &now, nil); err != nil {
		return nil, err
	}

	// 创建新出库单
	newOrder, err := s.repo.CreateOutboundOrderTx(ctx, tx, &CreateOutboundReq{
		InventoryID:     req.InventoryID,
		OrderNo:         req.OrderNo,
		Type:            strPtr(string(OrderTypeOutbound)),
		Status:          strPtr(string(OrderStatusPending)),
		OrderInfo:       req.OrderInfo,
		Remark:          req.Remark,
		Items:           req.Items,
		SourceReserveID: strPtr(req.ID),
	})
	if err != nil {
		return nil, err
	}

	// 锁定新出库单的库存
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

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return newOrder, nil
}

func strPtr(s string) *string { return &s }

// SyncAll 全量同步所有数据
func (s *Service) SyncAll(ctx context.Context) (*SyncAllResp, error) {
	inventories, err := s.repo.ListInventories(ctx)
	if err != nil {
		return nil, err
	}
	if inventories == nil {
		inventories = []Inventory{}
	}

	products := make(map[string][]Product)
	outboundOrders := make(map[string][]OutboundOrder)
	inboundLogs := make(map[string][]InboundLog)

	for _, inv := range inventories {
		id := inv.ID

		prods, err := s.repo.ListProducts(ctx, id)
		if err != nil {
			return nil, err
		}
		if prods == nil {
			prods = []Product{}
		}
		products[id] = prods

		orders, err := s.repo.ListOutboundOrders(ctx, id)
		if err != nil {
			return nil, err
		}
		if orders == nil {
			orders = []OutboundOrder{}
		}
		outboundOrders[id] = orders

		logs, err := s.repo.ListInboundLogs(ctx, id)
		if err != nil {
			return nil, err
		}
		if logs == nil {
			logs = []InboundLog{}
		}
		inboundLogs[id] = logs
	}

	tags, err := s.repo.ListTags(ctx)
	if err != nil {
		return nil, err
	}
	if tags == nil {
		tags = []Tag{}
	}

	statusCodes, err := s.repo.ListStatusCodes(ctx)
	if err != nil {
		return nil, err
	}
	if statusCodes == nil {
		statusCodes = []StatusCode{}
	}

	return &SyncAllResp{
		Inventories:    inventories,
		Products:       products,
		OutboundOrders: outboundOrders,
		InboundLogs:    inboundLogs,
		Tags:           tags,
		StatusCodes:    statusCodes,
	}, nil
}

// timeNowFunc 便于测试注入
type timeNowFunc func() time.Time
