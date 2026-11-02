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
	"github.com/Allinost/go-backend-core/internal/services/outbox"
	"github.com/Allinost/go-backend-core/internal/services/storage"
	"github.com/google/uuid"
)

// Service 业务逻辑层
type Service struct {
	repo          *Repository
	store         storage.Storage
	events        outbox.Emitter
	ownerNotifier OwnerNotifier
}

// OwnerNotifier 向店主推送客户订单请求，失败不影响订单写入
type OwnerNotifier func(ctx context.Context, event string, payload any) error

// NewService 创建 Service
func NewService(repo *Repository, store storage.Storage) *Service {
	return &Service{repo: repo, store: store}
}

// SetEventEmitter 注入外盒事件写入器，由集成模块在初始化时提供
func (s *Service) SetEventEmitter(emitter outbox.Emitter) {
	s.events = emitter
}

// SetOwnerNotifier 注入店主通知器，由集成模块在初始化时提供
func (s *Service) SetOwnerNotifier(notifier OwnerNotifier) {
	s.ownerNotifier = notifier
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
	items, err := s.repo.ListProducts(ctx, inventoryID)
	if err != nil {
		return nil, err
	}
	if err := s.attachProductCharacters(ctx, items); err != nil {
		return nil, err
	}
	return items, nil
}

// ListProductsPaginated 分页获取商品列表
func (s *Service) ListProductsPaginated(ctx context.Context, inventoryID string, page, pageSize int) ([]Product, bool, int, error) {
	items, hasMore, total, err := s.repo.ListProductsPaginated(ctx, inventoryID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, false, 0, err
	}
	if err := s.attachProductCharacters(ctx, items); err != nil {
		return nil, false, 0, err
	}
	return items, hasMore, total, nil
}

// SearchProductsPaginated 搜索商品（支持筛选、排序、分页）
func (s *Service) SearchProductsPaginated(ctx context.Context, req *QueryProductsReq) ([]Product, bool, int, error) {
	items, hasMore, total, err := s.repo.SearchProductsPaginated(ctx, req)
	if err != nil {
		return nil, false, 0, err
	}
	if err := s.attachProductCharacters(ctx, items); err != nil {
		return nil, false, 0, err
	}
	return items, hasMore, total, nil
}

// AllocateSeq 分配序号（优先复用回收序号）
func (s *Service) AllocateSeq(ctx context.Context, req *AllocateSeqReq) (int, error) {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	seq, err := s.repo.PeekRecycledSeq(ctx, tx, req.InventoryID, req.MainZone, req.SubZone)
	if err == nil {
		if err := s.repo.ConsumeRecycledSeq(ctx, tx, req.InventoryID, req.MainZone, req.SubZone, seq); err != nil {
			return 0, err
		}
		if err := tx.Commit(); err != nil {
			return 0, err
		}
		return seq, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
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
	if err := s.validateProductDimensions(ctx, req.SeriesID, req.SetTypeID, req.ProductTypeID, req.CharacterIDs); err != nil {
		return nil, err
	}

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	useSeq := req.SeqNumber
	recycled := false
	if useSeq <= 0 {
		seq, err := s.repo.PeekRecycledSeq(ctx, tx, req.InventoryID, req.MainZone, req.SubZone)
		if err == nil {
			useSeq = seq
			recycled = true
		} else if err != sql.ErrNoRows {
			return nil, err
		}
	}
	if useSeq <= 0 {
		seq, err := s.repo.IncrementSeqCounter(ctx, tx, req.InventoryID, req.MainZone, req.SubZone)
		if err != nil {
			return nil, err
		}
		useSeq = seq
	}
	req.SeqNumber = useSeq

	if req.Code == "" {
		req.Code = fmt.Sprintf("%s-%s-%04d-%04d-%s", req.MainZone, req.SubZone, useSeq, 0, req.StatusCode)
	}
	if _, err := s.repo.FindProductByCode(ctx, req.InventoryID, req.Code); err == nil {
		return nil, NewGoodserError(ErrDuplicateCode)
	} else if err != sql.ErrNoRows {
		return nil, err
	}

	result, err := s.repo.CreateProductTx(ctx, tx, req)
	if err != nil {
		return nil, err
	}
	if err := s.syncProductCharactersTx(ctx, tx, result.ID, req.CharacterIDs); err != nil {
		return nil, err
	}
	if req.CharacterIDs != nil {
		result.CharacterIDs = *req.CharacterIDs
	}

	if recycled {
		if err := s.repo.ConsumeRecycledSeq(ctx, tx, req.InventoryID, req.MainZone, req.SubZone, useSeq); err != nil {
			return nil, err
		}
	}
	if err := s.repo.EnsureSeqCounter(ctx, tx, req.InventoryID, req.MainZone, req.SubZone, useSeq); err != nil {
		return nil, err
	}

	if result.Quantity > 0 {
		if err := s.repo.AddStockMovementTx(ctx, tx, &StockMovement{
			InventoryID:   result.InventoryID,
			ProductID:     result.ID,
			QuantityDelta: result.Quantity,
			QuantityAfter: result.Quantity,
			ReservedAfter: result.ReservedQuantity,
			Reason:        StockReasonInboundManual,
			RefType:       "product",
			RefID:         result.ID,
		}); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

// GetProduct 获取商品详情
func (s *Service) GetProduct(ctx context.Context, id string) (*Product, error) {
	item, err := s.repo.GetProduct(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, NewGoodserError(ErrProductNotFound)
		}
		return nil, err
	}
	mapping, err := s.repo.ListProductCharacterIDs(ctx, []string{item.ID})
	if err != nil {
		return nil, err
	}
	item.CharacterIDs = mapping[item.ID]
	return item, nil
}

// UpdateProduct 更新商品基础信息。
// 直接修改 quantity 会按差值写入库存流水，避免手工改数无法追溯。
func (s *Service) UpdateProduct(ctx context.Context, req *UpdateProductReq) (*Product, error) {
	if err := s.validateProductDimensions(ctx, req.SeriesID, req.SetTypeID, req.ProductTypeID, req.CharacterIDs); err != nil {
		return nil, err
	}

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	before, err := s.repo.GetProductForUpdateTx(ctx, tx, req.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, NewGoodserError(ErrProductNotFound)
		}
		return nil, err
	}

	if req.Code != nil && *req.Code != "" && *req.Code != before.Code {
		if _, err := s.repo.FindProductByCode(ctx, before.InventoryID, *req.Code); err == nil {
			return nil, NewGoodserError(ErrDuplicateCode)
		} else if err != sql.ErrNoRows {
			return nil, err
		}
	}

	product, err := s.repo.UpdateProductTx(ctx, tx, req)
	if err != nil {
		return nil, err
	}
	if err := s.syncProductCharactersTx(ctx, tx, product.ID, req.CharacterIDs); err != nil {
		return nil, err
	}
	if req.CharacterIDs != nil {
		product.CharacterIDs = *req.CharacterIDs
	}

	deltaQty := product.Quantity - before.Quantity
	if deltaQty != 0 {
		if err := s.repo.AddStockMovementTx(ctx, tx, &StockMovement{
			InventoryID:   product.InventoryID,
			ProductID:     product.ID,
			QuantityDelta: deltaQty,
			ReservedDelta: 0,
			QuantityAfter: product.Quantity,
			ReservedAfter: product.ReservedQuantity,
			Reason:        StockReasonManualAdjust,
			RefType:       "product",
			RefID:         product.ID,
			Remark:        "手工调整可用库存",
		}); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return product, nil
}

// resolveInboundQty 解析入库数量：缺省为 1，非正数视为非法
func resolveInboundQty(q *int) (int, error) {
	if q == nil {
		return 1, nil
	}
	if *q <= 0 {
		return 0, NewGoodserError(ErrInvalidQuantity)
	}
	return *q, nil
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
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	product, err := s.repo.GetProductForUpdateTx(ctx, tx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return NewGoodserError(ErrProductNotFound)
		}
		return err
	}

	// D1 下 quantity 是可用库存，reserved 是锁定库存，两者都为 0 才允许删除
	if product.Quantity != 0 || product.ReservedQuantity != 0 {
		return NewGoodserError(ErrProductInUse)
	}

	comboRefs, err := s.repo.CountProductComboRefs(ctx, id)
	if err != nil {
		return err
	}
	if comboRefs > 0 {
		return NewGoodserError(ErrProductInUse, "商品已被组合包引用")
	}
	bundleRefs, err := s.repo.CountProductBundleRefs(ctx, id)
	if err != nil {
		return err
	}
	if bundleRefs > 0 {
		return NewGoodserError(ErrProductInUse, "商品已被捆引用")
	}

	if err := s.repo.DeleteProductTx(ctx, tx, id); err != nil {
		return err
	}
	if product.SeqNumber > 0 {
		if err := s.repo.AddRecycledSeqTx(ctx, tx, product.InventoryID, product.MainZone, product.SubZone, product.SeqNumber); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	// 事务提交成功后再删除图片，避免回滚后图片已被删掉
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
	return nil
}

// InboundSingle 单品入库
func (s *Service) InboundSingle(ctx context.Context, req *InboundSingleReq) (*Product, error) {
	qty, err := resolveInboundQty(req.Quantity)
	if err != nil {
		return nil, err
	}
	if err := s.validateProductDimensions(ctx, req.SeriesID, req.SetTypeID, req.ProductTypeID, req.CharacterIDs); err != nil {
		return nil, err
	}

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	useSeq, recycled, err := s.resolveInboundSeq(ctx, tx, req.InventoryID, req.MainZone, req.SubZone, req.SeqNumber)
	if err != nil {
		return nil, err
	}

	code := req.Code
	if code == "" {
		code = fmt.Sprintf("%s-%s-%04d-%04d-%s", req.MainZone, req.SubZone, useSeq, 0, req.StatusCode)
	}
	product, err := s.repo.FindProductByCode(ctx, req.InventoryID, code)
	var result *Product

	if err == sql.ErrNoRows {
		result, err = s.repo.CreateProductTx(ctx, tx, &CreateProductReq{
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
			SeriesID:        req.SeriesID,
			SetTypeID:       req.SetTypeID,
			ProductTypeID:   req.ProductTypeID,
			CharacterIDs:    req.CharacterIDs,
		})
		if err != nil {
			return nil, err
		}
		if err := s.syncProductCharactersTx(ctx, tx, result.ID, req.CharacterIDs); err != nil {
			return nil, err
		}
		if recycled {
			if err := s.repo.ConsumeRecycledSeq(ctx, tx, req.InventoryID, req.MainZone, req.SubZone, useSeq); err != nil {
				return nil, err
			}
		}
		if err := s.repo.EnsureSeqCounter(ctx, tx, req.InventoryID, req.MainZone, req.SubZone, useSeq); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else {
		// 商品已存在，直接增加可用库存
		result, err = s.repo.ApplyStockDeltaTx(ctx, tx, product.ID, qty, 0)
		if err != nil {
			return nil, err
		}
		// 回收序号指向了已存在的商品，说明该序号已失效，移除避免反复命中
		if recycled {
			if err := s.repo.ConsumeRecycledSeq(ctx, tx, req.InventoryID, req.MainZone, req.SubZone, useSeq); err != nil {
				return nil, err
			}
		}
	}

	if err := s.repo.AddStockMovementTx(ctx, tx, &StockMovement{
		InventoryID:   result.InventoryID,
		ProductID:     result.ID,
		QuantityDelta: qty,
		QuantityAfter: result.Quantity,
		ReservedAfter: result.ReservedQuantity,
		Reason:        StockReasonInboundSingle,
		RefType:       "product",
		RefID:         result.ID,
	}); err != nil {
		return nil, err
	}

	items, _ := json.Marshal([]OrderItem{{
		ProductID:   result.ID,
		ProductName: result.Name,
		ProductCode: result.Code,
		Quantity:    qty,
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
	var orderItems []OrderItem
	for _, item := range req.Items {
		qty, err := resolveInboundQty(item.Quantity)
		if err != nil {
			return nil, err
		}
		if err := s.validateProductDimensions(ctx, item.SeriesID, item.SetTypeID, item.ProductTypeID, item.CharacterIDs); err != nil {
			return nil, err
		}

		useSeq, recycled, err := s.resolveInboundSeq(ctx, tx, req.InventoryID, item.MainZone, item.SubZone, item.SeqNumber)
		if err != nil {
			return nil, err
		}

		code := item.Code
		if code == "" {
			code = fmt.Sprintf("%s-%s-%04d-%04d-%s", item.MainZone, item.SubZone, useSeq, 0, item.StatusCode)
		}
		product, err := s.repo.FindProductByCode(ctx, req.InventoryID, code)
		var p *Product
		if err == sql.ErrNoRows {
			p, err = s.repo.CreateProductTx(ctx, tx, &CreateProductReq{
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
				SeriesID:        item.SeriesID,
				SetTypeID:       item.SetTypeID,
				ProductTypeID:   item.ProductTypeID,
				CharacterIDs:    item.CharacterIDs,
			})
			if err != nil {
				return nil, err
			}
			if err := s.syncProductCharactersTx(ctx, tx, p.ID, item.CharacterIDs); err != nil {
				return nil, err
			}
			if recycled {
				if err := s.repo.ConsumeRecycledSeq(ctx, tx, req.InventoryID, item.MainZone, item.SubZone, useSeq); err != nil {
					return nil, err
				}
			}
			if err := s.repo.EnsureSeqCounter(ctx, tx, req.InventoryID, item.MainZone, item.SubZone, useSeq); err != nil {
				return nil, err
			}
		} else if err != nil {
			return nil, err
		} else {
			p, err = s.repo.ApplyStockDeltaTx(ctx, tx, product.ID, qty, 0)
			if err != nil {
				return nil, err
			}
			if recycled {
				if err := s.repo.ConsumeRecycledSeq(ctx, tx, req.InventoryID, item.MainZone, item.SubZone, useSeq); err != nil {
					return nil, err
				}
			}
		}

		if err := s.repo.AddStockMovementTx(ctx, tx, &StockMovement{
			InventoryID:   p.InventoryID,
			ProductID:     p.ID,
			QuantityDelta: qty,
			QuantityAfter: p.Quantity,
			ReservedAfter: p.ReservedQuantity,
			Reason:        StockReasonInboundBatch,
			RefType:       "product",
			RefID:         p.ID,
		}); err != nil {
			return nil, err
		}

		results = append(results, *p)
		orderItems = append(orderItems, OrderItem{
			ProductID: p.ID, ProductName: p.Name, ProductCode: p.Code, Quantity: qty,
		})
	}

	itemsJSON, _ := json.Marshal(orderItems)
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
	var orderItems []OrderItem
	for _, item := range req.Items {
		if item.Quantity <= 0 {
			return nil, NewGoodserError(ErrInvalidQuantity)
		}
		product, err := s.repo.GetProductForUpdateTx(ctx, tx, item.ProductID)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, NewGoodserError(ErrProductNotFound)
			}
			return nil, err
		}
		if product.InventoryID != req.InventoryID {
			return nil, NewGoodserError(ErrProductInventory)
		}
		product, err = s.repo.ApplyStockDeltaTx(ctx, tx, product.ID, item.Quantity, 0)
		if err != nil {
			return nil, err
		}
		if err := s.repo.AddStockMovementTx(ctx, tx, &StockMovement{
			InventoryID:   product.InventoryID,
			ProductID:     product.ID,
			QuantityDelta: item.Quantity,
			QuantityAfter: product.Quantity,
			ReservedAfter: product.ReservedQuantity,
			Reason:        StockReasonInboundImport,
			RefType:       "product",
			RefID:         product.ID,
		}); err != nil {
			return nil, err
		}
		results = append(results, *product)
		orderItems = append(orderItems, OrderItem{
			ProductID: product.ID, ProductName: product.Name, ProductCode: product.Code, Quantity: item.Quantity,
		})
	}

	itemsJSON, _ := json.Marshal(orderItems)
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

// CreateOutboundOrder 创建出库单或预留单。
// D1：创建时 quantity -= n、reserved_quantity += n，锁定库存。
func (s *Service) CreateOutboundOrder(ctx context.Context, req *CreateOutboundReq) (*OutboundOrder, error) {
	orderType, err := resolveOrderType(req.Type)
	if err != nil {
		return nil, err
	}
	if req.SourceReserveID != nil && *req.SourceReserveID != "" {
		return nil, NewGoodserError(ErrReserveConvertFail, "来源预留单请使用转出库接口")
	}
	if req.InventoryID == "" {
		return nil, NewGoodserError(ErrInventoryNotFound)
	}
	if req.Bundle != nil && req.ComboID != nil && strings.TrimSpace(*req.ComboID) != "" {
		return nil, NewGoodserError(ErrInvalidDimension, "组合包与捆不能同时下单")
	}
	if req.Bundle != nil && len(req.Items) > 0 {
		return nil, NewGoodserError(ErrOrderItemsRequired, "使用捆下单时 items 必须留空，由捆配置生成")
	}

	items := req.Items
	comboQuantity := req.ComboQuantity
	if req.ComboID != nil && strings.TrimSpace(*req.ComboID) != "" {
		comboItems, err := s.expandCombo(ctx, req.InventoryID, *req.ComboID, comboQuantity)
		if err != nil {
			return nil, err
		}
		if comboQuantity <= 0 {
			comboQuantity = 1
		}
		items = append(items, comboItems...)
	}

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var bundleSnapshot *BundleOrderSnapshot
	var bundleID, bundleMainProductID *string
	bundleQuantity := 0
	if req.Bundle != nil {
		var err error
		items, bundleSnapshot, err = s.prepareBundleOrder(ctx, tx, req.InventoryID, req.Bundle)
		if err != nil {
			return nil, err
		}
		bundleID = strPtr(bundleSnapshot.BundleID)
		bundleMainProductID = strPtr(bundleSnapshot.MainProductID)
		bundleQuantity = bundleSnapshot.Quantity
	}

	items, err = s.prepareOrderItems(ctx, tx, req.InventoryID, items)
	if err != nil {
		return nil, err
	}

	if req.OrderNo == "" {
		req.OrderNo = generateOrderNo(orderType)
	}
	order, err := s.repo.CreateOutboundOrderTx(ctx, tx, &CreateOutboundReq{
		InventoryID:         req.InventoryID,
		OrderNo:             req.OrderNo,
		Type:                strPtr(string(orderType)),
		OrderInfo:           req.OrderInfo,
		Remark:              req.Remark,
		Items:               items,
		ComboID:             req.ComboID,
		ComboQuantity:       comboQuantity,
		BundleID:            bundleID,
		BundleMainProductID: bundleMainProductID,
		BundleQuantity:      bundleQuantity,
		BundleSnapshot:      bundleSnapshotJSON(bundleSnapshot),
	})
	if err != nil {
		return nil, err
	}

	reason := StockReasonOutboundCreated
	if orderType == OrderTypeReserve {
		reason = StockReasonReserveCreated
	}
	for _, item := range items {
		updated, err := s.repo.ApplyStockDeltaTx(ctx, tx, item.ProductID, -item.Quantity, item.Quantity)
		if err != nil {
			return nil, err
		}
		if err := s.repo.AddStockMovementTx(ctx, tx, &StockMovement{
			InventoryID:   updated.InventoryID,
			ProductID:     updated.ID,
			QuantityDelta: -item.Quantity,
			ReservedDelta: item.Quantity,
			QuantityAfter: updated.Quantity,
			ReservedAfter: updated.ReservedQuantity,
			Reason:        reason,
			RefType:       "outbound_order",
			RefID:         order.ID,
		}); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	order.Items, _ = json.Marshal(items)
	s.emitEvent(ctx, EventOrderCreated, "outbound_order", order.ID, map[string]any{
		"order": order,
		"type":  order.Type,
	})
	return order, nil
}

// ConfirmOutbound 确认出库。
// D1：只释放锁定库存，不再扣减 quantity，因为创建单据时已经扣过可用库存。
func (s *Service) ConfirmOutbound(ctx context.Context, req *ConfirmOutboundReq) (*OutboundOrder, error) {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	order, err := s.repo.GetOutboundOrderForUpdateTx(ctx, tx, req.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, NewGoodserError(ErrOrderNotFound)
		}
		return nil, err
	}
	if order.Type != string(OrderTypeOutbound) || order.Status != string(OrderStatusPending) {
		return nil, NewGoodserError(ErrInvalidOrderStatus)
	}

	var orderItems []OrderItem
	if err := json.Unmarshal(order.Items, &orderItems); err != nil {
		return nil, err
	}

	for _, item := range orderItems {
		updated, err := s.repo.ApplyStockDeltaTx(ctx, tx, item.ProductID, 0, -item.Quantity)
		if err != nil {
			return nil, err
		}
		if err := s.repo.AddStockMovementTx(ctx, tx, &StockMovement{
			InventoryID:   updated.InventoryID,
			ProductID:     updated.ID,
			ReservedDelta: -item.Quantity,
			QuantityAfter: updated.Quantity,
			ReservedAfter: updated.ReservedQuantity,
			Reason:        StockReasonOutboundConfirm,
			RefType:       "outbound_order",
			RefID:         order.ID,
		}); err != nil {
			return nil, err
		}
	}

	now := time.Now()
	if err := s.repo.UpdateOutboundOrderStatusTx(ctx, tx, order.ID, string(OrderStatusConfirmed), &now, nil, nil, nil); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	order.Status = string(OrderStatusConfirmed)
	order.ConfirmedAt = &now
	s.emitEvent(ctx, EventOrderConfirmed, "outbound_order", order.ID, map[string]any{
		"order_id": order.ID,
		"order_no": order.OrderNo,
		"items":    orderItems,
	})
	return order, nil
}

// CancelOutbound 取消出库单。
// D1：quantity += n、reserved_quantity -= n，把锁定库存释放回可用库存。
func (s *Service) CancelOutbound(ctx context.Context, req *CancelOutboundReq) (*OutboundOrder, error) {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	order, err := s.repo.GetOutboundOrderForUpdateTx(ctx, tx, req.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, NewGoodserError(ErrOrderNotFound)
		}
		return nil, err
	}
	if order.Type != string(OrderTypeOutbound) || order.Status != string(OrderStatusPending) {
		return nil, NewGoodserError(ErrInvalidOrderStatus)
	}

	var orderItems []OrderItem
	if err := json.Unmarshal(order.Items, &orderItems); err != nil {
		return nil, err
	}

	for _, item := range orderItems {
		updated, err := s.repo.ApplyStockDeltaTx(ctx, tx, item.ProductID, item.Quantity, -item.Quantity)
		if err != nil {
			return nil, err
		}
		if err := s.repo.AddStockMovementTx(ctx, tx, &StockMovement{
			InventoryID:   updated.InventoryID,
			ProductID:     updated.ID,
			QuantityDelta: item.Quantity,
			ReservedDelta: -item.Quantity,
			QuantityAfter: updated.Quantity,
			ReservedAfter: updated.ReservedQuantity,
			Reason:        StockReasonOutboundCancel,
			RefType:       "outbound_order",
			RefID:         order.ID,
		}); err != nil {
			return nil, err
		}
	}

	now := time.Now()
	if err := s.repo.UpdateOutboundOrderStatusTx(ctx, tx, order.ID, string(OrderStatusCancelled), nil, &now, nil, nil); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	order.Status = string(OrderStatusCancelled)
	order.CancelledAt = &now
	s.emitEvent(ctx, EventOrderCancelled, "outbound_order", order.ID, map[string]any{
		"order_id": order.ID,
		"order_no": order.OrderNo,
		"items":    orderItems,
	})
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

// CancelReserve 取消预留单。
// D1：quantity += n、reserved_quantity -= n，锁定库存回到可用库存。
func (s *Service) CancelReserve(ctx context.Context, req *CancelReserveReq) (*OutboundOrder, error) {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	order, err := s.repo.GetOutboundOrderForUpdateTx(ctx, tx, req.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, NewGoodserError(ErrOrderNotFound)
		}
		return nil, err
	}
	if order.Type != string(OrderTypeReserve) {
		return nil, NewGoodserError(ErrInvalidOrderStatus)
	}
	if order.Status == string(OrderStatusConverted) {
		return nil, NewGoodserError(ErrInvalidOrderStatus, "预留单已转出库，请取消对应出库单")
	}
	if order.Status != string(OrderStatusPending) {
		return nil, NewGoodserError(ErrInvalidOrderStatus)
	}

	var orderItems []OrderItem
	if err := json.Unmarshal(order.Items, &orderItems); err != nil {
		return nil, err
	}

	for _, item := range orderItems {
		updated, err := s.repo.ApplyStockDeltaTx(ctx, tx, item.ProductID, item.Quantity, -item.Quantity)
		if err != nil {
			return nil, err
		}
		if err := s.repo.AddStockMovementTx(ctx, tx, &StockMovement{
			InventoryID:   updated.InventoryID,
			ProductID:     updated.ID,
			QuantityDelta: item.Quantity,
			ReservedDelta: -item.Quantity,
			QuantityAfter: updated.Quantity,
			ReservedAfter: updated.ReservedQuantity,
			Reason:        StockReasonReserveCancel,
			RefType:       "outbound_order",
			RefID:         order.ID,
		}); err != nil {
			return nil, err
		}
	}

	now := time.Now()
	if err := s.repo.UpdateOutboundOrderStatusTx(ctx, tx, order.ID, string(OrderStatusCancelled), nil, &now, nil, nil); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	order.Status = string(OrderStatusCancelled)
	order.CancelledAt = &now
	s.emitEvent(ctx, EventReserveCancelled, "outbound_order", order.ID, map[string]any{
		"order_id": order.ID,
		"order_no": order.OrderNo,
		"items":    orderItems,
	})
	return order, nil
}

// ReserveToOutbound 预留单转出库单。
// D1：转单只转移单据关系，不改变库存；确认出库时才释放锁定库存。
// 幂等：同一预留单重复转单时返回已生成的出库单。
func (s *Service) ReserveToOutbound(ctx context.Context, req *ReserveToOutboundReq) (*OutboundOrder, error) {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	order, err := s.repo.GetOutboundOrderForUpdateTx(ctx, tx, req.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, NewGoodserError(ErrOrderNotFound)
		}
		return nil, err
	}
	if order.Type != string(OrderTypeReserve) {
		return nil, NewGoodserError(ErrInvalidOrderStatus)
	}

	// 已转单：直接返回子出库单，保证双击不会生成两张
	if order.Status == string(OrderStatusConverted) {
		child, err := s.repo.FindChildOutboundTx(ctx, tx, order.ID)
		if err == nil {
			return child, nil
		}
		if err == sql.ErrNoRows {
			return nil, NewGoodserError(ErrReserveConvertFail, "预留单状态异常，找不到对应出库单")
		}
		return nil, err
	}
	if order.Status != string(OrderStatusPending) {
		return nil, NewGoodserError(ErrInvalidOrderStatus)
	}

	// 转单必须沿用原预留单的库存目录和明细，P0 不允许换商品/改数量
	inventoryID := req.InventoryID
	if inventoryID == "" {
		inventoryID = order.InventoryID
	}
	if inventoryID != order.InventoryID {
		return nil, NewGoodserError(ErrProductInventory, "预留单与库存目录不一致")
	}

	var reserveItems []OrderItem
	if err := json.Unmarshal(order.Items, &reserveItems); err != nil {
		return nil, err
	}
	items, err := s.prepareOrderItems(ctx, tx, inventoryID, reserveItems)
	if err != nil {
		return nil, err
	}

	orderNo := req.OrderNo
	if orderNo == "" {
		orderNo = generateOrderNo(OrderTypeOutbound)
	}
	orderInfo := req.OrderInfo
	if orderInfo == nil {
		orderInfo = order.OrderInfo
	}
	remark := req.Remark
	if remark == nil {
		remark = order.Remark
	}

	newOrder, err := s.repo.CreateOutboundOrderTx(ctx, tx, &CreateOutboundReq{
		InventoryID:     inventoryID,
		OrderNo:         orderNo,
		Type:            strPtr(string(OrderTypeOutbound)),
		OrderInfo:       orderInfo,
		Remark:          remark,
		Items:           items,
		SourceReserveID: strPtr(order.ID),
	})
	if err != nil {
		return nil, err
	}

	now := time.Now()
	if err := s.repo.UpdateOutboundOrderStatusTx(ctx, tx, order.ID, string(OrderStatusConverted), nil, nil, &now, nil); err != nil {
		return nil, err
	}

	// 记录 0 增量的转单流水，便于审计和事件回放
	for _, item := range items {
		product, err := s.repo.GetProductForUpdateTx(ctx, tx, item.ProductID)
		if err != nil {
			return nil, err
		}
		if err := s.repo.AddStockMovementTx(ctx, tx, &StockMovement{
			InventoryID:   product.InventoryID,
			ProductID:     product.ID,
			QuantityAfter: product.Quantity,
			ReservedAfter: product.ReservedQuantity,
			Reason:        StockReasonReserveConvert,
			RefType:       "outbound_order",
			RefID:         newOrder.ID,
			Remark:        "预留单 " + order.OrderNo + " 转出库",
		}); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	newOrder.Items, _ = json.Marshal(items)
	s.emitEvent(ctx, EventReserveConverted, "outbound_order", newOrder.ID, map[string]any{
		"order_id":          newOrder.ID,
		"order_no":          newOrder.OrderNo,
		"source_reserve_id": order.ID,
		"items":             items,
	})
	return newOrder, nil
}

// GetOutboundOrder 获取出库单/预留单详情
func (s *Service) GetOutboundOrder(ctx context.Context, id string) (*OutboundOrder, error) {
	order, err := s.repo.GetOutboundOrder(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, NewGoodserError(ErrOrderNotFound)
		}
		return nil, err
	}
	return order, nil
}

// ListReturns 获取退货单列表
func (s *Service) ListReturns(ctx context.Context, inventoryID, outboundOrderID string) ([]ReturnOrder, error) {
	return s.repo.ListReturns(ctx, inventoryID, outboundOrderID)
}

// GetReturn 获取退货单详情
func (s *Service) GetReturn(ctx context.Context, id string) (*ReturnOrder, error) {
	ret, err := s.repo.GetReturn(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, NewGoodserError(ErrReturnNotFound)
		}
		return nil, err
	}
	return ret, nil
}

// CreateReturn 创建退货单。
// 可对同一张已确认出库单多选商品、部分退货、多次退货。
// 创建时只校验数量，不改变库存；确认时才按 restock 决定是否回库。
func (s *Service) CreateReturn(ctx context.Context, req *CreateReturnReq) (*ReturnOrder, error) {
	if len(req.Items) == 0 {
		return nil, NewGoodserError(ErrOrderItemsRequired)
	}
	reason, err := resolveReturnReason(req.Reason)
	if err != nil {
		return nil, err
	}

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	order, err := s.repo.GetOutboundOrderForUpdateTx(ctx, tx, req.OutboundOrderID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, NewGoodserError(ErrOrderNotFound)
		}
		return nil, err
	}
	if order.Type != string(OrderTypeOutbound) {
		return nil, NewGoodserError(ErrReturnNotAllowed, "只有出库单可以退货")
	}
	if order.Status != string(OrderStatusConfirmed) && order.Status != string(OrderStatusPartiallyReturned) && order.Status != string(OrderStatusReturned) {
		return nil, NewGoodserError(ErrReturnNotAllowed, "出库单尚未确认或已取消")
	}

	var orderItems []OrderItem
	if err := json.Unmarshal(order.Items, &orderItems); err != nil {
		return nil, err
	}
	orderedQty := make(map[string]int)
	meta := make(map[string]OrderItem)
	for _, item := range orderItems {
		orderedQty[item.ProductID] += item.Quantity
		meta[item.ProductID] = item
	}

	// 聚合本次提交的商品，避免同一商品拆成多行绕过数量校验
	aggregated := make(map[string]*ReturnItem)
	var productOrder []string
	for _, item := range req.Items {
		if item.Quantity <= 0 {
			return nil, NewGoodserError(ErrInvalidQuantity)
		}
		if _, ok := orderedQty[item.ProductID]; !ok {
			return nil, NewGoodserError(ErrReturnNotAllowed, "商品不在该出库单中")
		}
		if existing, ok := aggregated[item.ProductID]; ok {
			existing.Quantity += item.Quantity
			continue
		}
		m := meta[item.ProductID]
		aggregated[item.ProductID] = &ReturnItem{
			ProductID:   item.ProductID,
			ProductName: m.ProductName,
			ProductCode: m.ProductCode,
			Quantity:    item.Quantity,
			ImageURL:    m.ImageURL,
		}
		productOrder = append(productOrder, item.ProductID)
	}

	// pending 也计入已退数量，防止并发提交超过可退数量
	alreadyReturned, err := s.repo.SumReturnedQtyTx(ctx, tx, order.ID, []string{
		string(ReturnStatusPending), string(ReturnStatusConfirmed),
	})
	if err != nil {
		return nil, err
	}
	for pid, item := range aggregated {
		if item.Quantity+alreadyReturned[pid] > orderedQty[pid] {
			return nil, NewGoodserError(ErrReturnQtyExceeded)
		}
	}

	defaultRestock := reason == ReturnReasonReturnRefund
	var items []ReturnItem
	for _, pid := range productOrder {
		item := aggregated[pid]
		v := defaultRestock
		if item.Restock != nil {
			v = *item.Restock
		}
		if reason == ReturnReasonRefundOnly {
			v = false
		}
		item.Restock = &v
		items = append(items, *item)
	}

	itemsJSON, _ := json.Marshal(items)
	ret := &ReturnOrder{
		ID:              uuid.New().String(),
		InventoryID:     order.InventoryID,
		OutboundOrderID: order.ID,
		OrderNo:         order.OrderNo,
		Status:          string(ReturnStatusPending),
		Reason:          string(reason),
		Remark:          req.Remark,
		Items:           itemsJSON,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	if err := s.repo.CreateReturnTx(ctx, tx, ret); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	s.emitEvent(ctx, EventReturnCreated, "return_order", ret.ID, map[string]any{
		"return_id":         ret.ID,
		"outbound_order_id": ret.OutboundOrderID,
		"order_no":          ret.OrderNo,
		"reason":            ret.Reason,
		"items":             items,
	})
	return ret, nil
}

// ConfirmReturn 确认退货单。
// restock=true 的商品回可用库存，restock=false 只记录退货不改变库存。
func (s *Service) ConfirmReturn(ctx context.Context, req *ConfirmReturnReq) (*ReturnOrder, error) {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	ret, err := s.repo.GetReturnForUpdateTx(ctx, tx, req.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, NewGoodserError(ErrReturnNotFound)
		}
		return nil, err
	}
	if ret.Status != string(ReturnStatusPending) {
		return nil, NewGoodserError(ErrInvalidOrderStatus, "退货单已处理")
	}

	var returnItems []ReturnItem
	if err := json.Unmarshal(ret.Items, &returnItems); err != nil {
		return nil, err
	}

	now := time.Now()
	for _, item := range returnItems {
		restock := item.Restock != nil && *item.Restock
		if restock {
			updated, err := s.repo.ApplyStockDeltaTx(ctx, tx, item.ProductID, item.Quantity, 0)
			if err != nil {
				return nil, err
			}
			if err := s.repo.AddStockMovementTx(ctx, tx, &StockMovement{
				InventoryID:   updated.InventoryID,
				ProductID:     updated.ID,
				QuantityDelta: item.Quantity,
				QuantityAfter: updated.Quantity,
				ReservedAfter: updated.ReservedQuantity,
				Reason:        StockReasonReturnRestock,
				RefType:       "return",
				RefID:         ret.ID,
			}); err != nil {
				return nil, err
			}
			continue
		}

		// 不回库存也要写一笔 0 增量流水，保证退货可追溯
		product, err := s.repo.GetProductForUpdateTx(ctx, tx, item.ProductID)
		if err != nil {
			if err == sql.ErrNoRows {
				continue
			}
			return nil, err
		}
		if err := s.repo.AddStockMovementTx(ctx, tx, &StockMovement{
			InventoryID:   product.InventoryID,
			ProductID:     product.ID,
			QuantityAfter: product.Quantity,
			ReservedAfter: product.ReservedQuantity,
			Reason:        StockReasonReturnNoRestock,
			RefType:       "return",
			RefID:         ret.ID,
		}); err != nil {
			return nil, err
		}
	}

	if err := s.repo.UpdateReturnStatusTx(ctx, tx, ret.ID, string(ReturnStatusConfirmed), &now, nil); err != nil {
		return nil, err
	}

	order, err := s.repo.GetOutboundOrderForUpdateTx(ctx, tx, ret.OutboundOrderID)
	if err != nil {
		return nil, err
	}
	if err := s.refreshOrderReturnStatus(ctx, tx, order, now); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	ret.Status = string(ReturnStatusConfirmed)
	ret.ConfirmedAt = &now
	s.emitEvent(ctx, EventReturnConfirmed, "return_order", ret.ID, map[string]any{
		"return_id":         ret.ID,
		"outbound_order_id": ret.OutboundOrderID,
		"order_no":          ret.OrderNo,
		"reason":            ret.Reason,
		"items":             returnItems,
	})
	return ret, nil
}

// CancelReturn 取消待确认的退货单
func (s *Service) CancelReturn(ctx context.Context, req *CancelReturnReq) (*ReturnOrder, error) {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	ret, err := s.repo.GetReturnForUpdateTx(ctx, tx, req.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, NewGoodserError(ErrReturnNotFound)
		}
		return nil, err
	}
	if ret.Status != string(ReturnStatusPending) {
		return nil, NewGoodserError(ErrInvalidOrderStatus, "退货单已处理")
	}

	now := time.Now()
	if err := s.repo.UpdateReturnStatusTx(ctx, tx, ret.ID, string(ReturnStatusCancelled), nil, &now); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	ret.Status = string(ReturnStatusCancelled)
	ret.CancelledAt = &now
	return ret, nil
}

// refreshOrderReturnStatus 根据已确认退货数量刷新出库单退货状态
func (s *Service) refreshOrderReturnStatus(ctx context.Context, tx *sql.Tx, order *OutboundOrder, now time.Time) error {
	var orderItems []OrderItem
	if err := json.Unmarshal(order.Items, &orderItems); err != nil {
		return err
	}
	returned, err := s.repo.SumReturnedQtyTx(ctx, tx, order.ID, []string{string(ReturnStatusConfirmed)})
	if err != nil {
		return err
	}

	// 同一商品可能因历史数据出现在多行，先按商品聚合再判断是否全部退完
	ordered := make(map[string]int)
	for _, item := range orderItems {
		ordered[item.ProductID] += item.Quantity
	}

	full := true
	totalOrdered := 0
	totalReturned := 0
	for pid, qty := range ordered {
		totalOrdered += qty
		totalReturned += returned[pid]
		if returned[pid] < qty {
			full = false
		}
	}

	status := string(OrderStatusPartiallyReturned)
	var returnedAt *time.Time
	if full && totalReturned >= totalOrdered {
		status = string(OrderStatusReturned)
		returnedAt = &now
	}
	return s.repo.UpdateOutboundOrderStatusTx(ctx, tx, order.ID, status, order.ConfirmedAt, nil, order.ConvertedAt, returnedAt)
}

func resolveReturnReason(raw string) (ReturnReason, error) {
	if raw == "" {
		return ReturnReasonReturnRefund, nil
	}
	switch ReturnReason(raw) {
	case ReturnReasonRefundOnly, ReturnReasonReturnRefund, ReturnReasonShippingDamage, ReturnReasonOther:
		return ReturnReason(raw), nil
	default:
		return "", NewGoodserError(ErrInvalidReturnReason)
	}
}

func resolveOrderType(raw *string) (OrderType, error) {
	if raw == nil || *raw == "" || *raw == string(OrderTypeOutbound) {
		return OrderTypeOutbound, nil
	}
	if *raw == string(OrderTypeReserve) {
		return OrderTypeReserve, nil
	}
	return "", NewGoodserError(ErrInvalidOrderStatus, "无效的单据类型")
}

func generateOrderNo(t OrderType) string {
	prefix := "OUT"
	if t == OrderTypeReserve {
		prefix = "RSV"
	}
	return fmt.Sprintf("%s-%s-%s", prefix, time.Now().Format("20060102-150405"), uuid.New().String()[:4])
}

// prepareOrderItems 校验并聚合单据明细，同时锁定商品行。
// 商品名/编码/图片以数据库为准，避免客户端伪造。
func (s *Service) prepareOrderItems(ctx context.Context, tx *sql.Tx, inventoryID string, items []OrderItem) ([]OrderItem, error) {
	if len(items) == 0 {
		return nil, NewGoodserError(ErrOrderItemsRequired)
	}
	aggregated := make(map[string]*OrderItem)
	var order []string
	for _, item := range items {
		if item.ProductID == "" {
			return nil, NewGoodserError(ErrProductNotFound)
		}
		if item.Quantity <= 0 {
			return nil, NewGoodserError(ErrInvalidQuantity)
		}
		product, err := s.repo.GetProductForUpdateTx(ctx, tx, item.ProductID)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, NewGoodserError(ErrProductNotFound)
			}
			return nil, err
		}
		if product.InventoryID != inventoryID {
			return nil, NewGoodserError(ErrProductInventory)
		}
		if existing, ok := aggregated[product.ID]; ok {
			existing.Quantity += item.Quantity
			continue
		}
		aggregated[product.ID] = &OrderItem{
			ProductID:   product.ID,
			ProductName: product.Name,
			ProductCode: product.Code,
			Quantity:    item.Quantity,
			ImageURL:    product.ImageURL,
		}
		order = append(order, product.ID)
	}

	result := make([]OrderItem, 0, len(order))
	for _, id := range order {
		result = append(result, *aggregated[id])
	}
	return result, nil
}

// resolveInboundSeq 解析入库序号：显式序号 > 回收序号 > 递增计数器。
// 回收序号只窥视不删除，真正使用后再由调用方 Consume，避免校验失败导致序号泄漏。
func (s *Service) resolveInboundSeq(ctx context.Context, tx *sql.Tx, inventoryID, mainZone, subZone string, requested int) (int, bool, error) {
	if requested > 0 {
		return requested, false, nil
	}
	seq, err := s.repo.PeekRecycledSeq(ctx, tx, inventoryID, mainZone, subZone)
	if err == nil {
		return seq, true, nil
	}
	if err != sql.ErrNoRows {
		return 0, false, err
	}
	seq, err = s.repo.IncrementSeqCounter(ctx, tx, inventoryID, mainZone, subZone)
	if err != nil {
		return 0, false, err
	}
	return seq, false, nil
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
