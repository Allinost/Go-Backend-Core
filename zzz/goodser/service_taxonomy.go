package goodser

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/Allinost/go-backend-core/internal/pkg/logger"
	"github.com/Allinost/go-backend-core/internal/services/outbox"
)

// --- 受控维度字典 ---

// ListDictionaries 查询字典项
func (s *Service) ListDictionaries(ctx context.Context, req *ListDictionariesReq) ([]Dictionary, error) {
	if req == nil {
		req = &ListDictionariesReq{}
	}
	if req.Kind != "" && !ValidDictionaryKind(req.Kind) {
		return nil, NewGoodserError(ErrDictionaryKind)
	}
	return s.repo.ListDictionaries(ctx, req.Kind, req.Enabled)
}

// CreateDictionary 创建字典项
func (s *Service) CreateDictionary(ctx context.Context, req *CreateDictionaryReq) (*Dictionary, error) {
	if req == nil || !ValidDictionaryKind(req.Kind) {
		return nil, NewGoodserError(ErrDictionaryKind)
	}
	if strings.TrimSpace(req.Name) == "" {
		return nil, NewGoodserError(ErrInvalidDimension, "字典名称不能为空")
	}
	if err := s.validateDictionaryParent(ctx, req.Kind, req.ParentID); err != nil {
		return nil, err
	}
	return s.repo.CreateDictionary(ctx, req)
}

// UpdateDictionary 更新字典项，kind 不允许修改
func (s *Service) UpdateDictionary(ctx context.Context, req *UpdateDictionaryReq) (*Dictionary, error) {
	if req == nil || strings.TrimSpace(req.ID) == "" {
		return nil, NewGoodserError(ErrDictionaryNotFound)
	}
	existing, err := s.repo.GetDictionary(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	if req.Name != nil && strings.TrimSpace(*req.Name) == "" {
		return nil, NewGoodserError(ErrInvalidDimension, "字典名称不能为空")
	}
	if req.ParentID != nil {
		if err := s.validateDictionaryParent(ctx, existing.Kind, req.ParentID); err != nil {
			return nil, err
		}
	}
	return s.repo.UpdateDictionary(ctx, req)
}

// DeleteDictionary 删除字典项，被引用时拒绝
func (s *Service) DeleteDictionary(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return NewGoodserError(ErrDictionaryNotFound)
	}
	return s.repo.DeleteDictionary(ctx, id)
}

func (s *Service) validateDictionaryParent(ctx context.Context, kind string, parentID *string) error {
	if parentID == nil || strings.TrimSpace(*parentID) == "" {
		return nil
	}
	if kind != string(DictionaryKindCharacter) {
		return NewGoodserError(ErrInvalidDimension, "只有角色可以设置上级系列")
	}
	parent, err := s.repo.GetDictionary(ctx, *parentID)
	if err != nil {
		return err
	}
	if parent.Kind != string(DictionaryKindSeries) {
		return NewGoodserError(ErrInvalidDimension, "角色的上级必须是系列")
	}
	return nil
}

// ListSeriesSummary 返回某库存目录下的系列聚合视图
func (s *Service) ListSeriesSummary(ctx context.Context, inventoryID string) ([]SeriesSummary, error) {
	if strings.TrimSpace(inventoryID) == "" {
		return nil, NewGoodserError(ErrInventoryNotFound)
	}
	if _, err := s.repo.GetInventory(ctx, inventoryID); err != nil {
		if err == sql.ErrNoRows {
			return nil, NewGoodserError(ErrInventoryNotFound)
		}
		return nil, err
	}
	return s.repo.ListSeriesSummary(ctx, inventoryID)
}

// validateProductDimensions 校验商品维度引用是否存在且类型正确
func (s *Service) validateProductDimensions(ctx context.Context, seriesID, setTypeID, productTypeID *string, characterIDs *[]string) error {
	checks := []struct {
		id   *string
		kind string
		key  string
	}{
		{seriesID, string(DictionaryKindSeries), "系列"},
		{setTypeID, string(DictionaryKindSetType), "套型"},
		{productTypeID, string(DictionaryKindProductType), "商品类型"},
	}
	for _, check := range checks {
		if check.id == nil || strings.TrimSpace(*check.id) == "" {
			continue
		}
		item, err := s.repo.GetDictionary(ctx, *check.id)
		if err != nil {
			return err
		}
		if item.Kind != check.kind {
			return NewGoodserError(ErrInvalidDimension, check.key+"字典类型不匹配")
		}
	}
	if characterIDs != nil {
		seen := make(map[string]struct{}, len(*characterIDs))
		for _, id := range *characterIDs {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if _, exists := seen[id]; exists {
				continue
			}
			seen[id] = struct{}{}
			item, err := s.repo.GetDictionary(ctx, id)
			if err != nil {
				return err
			}
			if item.Kind != string(DictionaryKindCharacter) {
				return NewGoodserError(ErrInvalidDimension, "角色字典类型不匹配")
			}
		}
	}
	return nil
}

func (s *Service) syncProductCharactersTx(ctx context.Context, tx *sql.Tx, productID string, characterIDs *[]string) error {
	if characterIDs == nil {
		return nil
	}
	return s.repo.ReplaceProductCharactersTx(ctx, tx, productID, *characterIDs)
}

func (s *Service) attachProductCharacters(ctx context.Context, products []Product) error {
	ids := make([]string, 0, len(products))
	for _, product := range products {
		ids = append(ids, product.ID)
	}
	mapping, err := s.repo.ListProductCharacterIDs(ctx, ids)
	if err != nil {
		return err
	}
	for i := range products {
		products[i].CharacterIDs = mapping[products[i].ID]
	}
	return nil
}

// --- 组合包 ---

// ListCombos 查询组合包，并计算可售数量
func (s *Service) ListCombos(ctx context.Context, inventoryID, status string) ([]Combo, error) {
	combos, err := s.repo.ListCombos(ctx, inventoryID, status)
	if err != nil {
		return nil, err
	}
	for i := range combos {
		combos[i].Availability = comboAvailability(combos[i].Items)
	}
	return combos, nil
}

// GetCombo 获取组合包详情
func (s *Service) GetCombo(ctx context.Context, id string) (*Combo, error) {
	combo, err := s.repo.GetCombo(ctx, id)
	if err != nil {
		return nil, err
	}
	combo.Availability = comboAvailability(combo.Items)
	return combo, nil
}

// CreateCombo 创建组合包
func (s *Service) CreateCombo(ctx context.Context, req *CreateComboReq) (*Combo, error) {
	if req == nil || strings.TrimSpace(req.Name) == "" {
		return nil, NewGoodserError(ErrInvalidDimension, "组合包名称不能为空")
	}
	if strings.TrimSpace(req.InventoryID) == "" {
		return nil, NewGoodserError(ErrInventoryNotFound)
	}
	if err := s.validateComboItems(ctx, req.InventoryID, req.Items); err != nil {
		return nil, err
	}
	if err := validateComboStatus(req.Status); err != nil {
		return nil, err
	}
	combo, err := s.repo.CreateCombo(ctx, req)
	if err != nil {
		return nil, err
	}
	return s.GetCombo(ctx, combo.ID)
}

// UpdateCombo 更新组合包
func (s *Service) UpdateCombo(ctx context.Context, req *UpdateComboReq) (*Combo, error) {
	if req == nil || strings.TrimSpace(req.ID) == "" {
		return nil, NewGoodserError(ErrComboNotFound)
	}
	existing, err := s.GetCombo(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	if req.Items != nil {
		if err := s.validateComboItems(ctx, existing.InventoryID, *req.Items); err != nil {
			return nil, err
		}
	}
	if req.Status != nil {
		if err := validateComboStatus(*req.Status); err != nil {
			return nil, err
		}
	}
	if _, err := s.repo.UpdateCombo(ctx, req); err != nil {
		return nil, err
	}
	return s.GetCombo(ctx, req.ID)
}

// DeleteCombo 删除组合包
func (s *Service) DeleteCombo(ctx context.Context, id string) error {
	return s.repo.DeleteCombo(ctx, id)
}

func validateComboStatus(status string) error {
	if status == "" || status == ComboStatusActive || status == ComboStatusInactive {
		return nil
	}
	return NewGoodserError(ErrInvalidDimension, "组合包状态仅支持 active/inactive")
}

func (s *Service) validateComboItems(ctx context.Context, inventoryID string, items []ComboItemReq) error {
	if len(items) == 0 {
		return NewGoodserError(ErrComboItemsRequired)
	}
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.ProductID) == "" {
			return NewGoodserError(ErrProductNotFound)
		}
		if item.Quantity <= 0 {
			return NewGoodserError(ErrInvalidQuantity)
		}
		if _, exists := seen[item.ProductID]; exists {
			return NewGoodserError(ErrInvalidDimension, "组合包组件商品重复")
		}
		seen[item.ProductID] = struct{}{}
		product, err := s.repo.GetProduct(ctx, item.ProductID)
		if err != nil {
			if err == sql.ErrNoRows {
				return NewGoodserError(ErrProductNotFound)
			}
			return err
		}
		if product.InventoryID != inventoryID {
			return NewGoodserError(ErrProductInventory)
		}
	}
	return nil
}

// expandCombo 把组合包展开为组件商品明细
func (s *Service) expandCombo(ctx context.Context, inventoryID, comboID string, comboQuantity int) ([]OrderItem, error) {
	if comboQuantity <= 0 {
		comboQuantity = 1
	}
	combo, err := s.repo.GetCombo(ctx, comboID)
	if err != nil {
		return nil, err
	}
	if combo.InventoryID != inventoryID {
		return nil, NewGoodserError(ErrProductInventory, "组合包不属于该库存目录")
	}
	if combo.Status != ComboStatusActive {
		return nil, NewGoodserError(ErrComboNotFound, "组合包已停用")
	}
	if len(combo.Items) == 0 {
		return nil, NewGoodserError(ErrComboItemsRequired)
	}
	items := make([]OrderItem, 0, len(combo.Items))
	for _, component := range combo.Items {
		items = append(items, OrderItem{
			ProductID:   component.ProductID,
			ProductName: component.ProductName,
			ProductCode: component.ProductCode,
			Quantity:    component.Quantity * comboQuantity,
			ImageURL:    component.ImageURL,
		})
	}
	return items, nil
}

func comboAvailability(items []ComboItem) int {
	if len(items) == 0 {
		return 0
	}
	availability := math.MaxInt
	for _, item := range items {
		if item.Quantity <= 0 {
			return 0
		}
		current := item.Available / item.Quantity
		if current < availability {
			availability = current
		}
	}
	if availability == math.MaxInt {
		return 0
	}
	return availability
}

// --- 客户服务 ---

// CustomerStockQuery 客户只读库存查询，支持系列/角色/类型/状态筛选
func (s *Service) CustomerStockQuery(ctx context.Context, req *CustomerStockQueryReq) (*CustomerStockQueryResp, error) {
	if req == nil {
		req = &CustomerStockQueryReq{}
	}
	if strings.TrimSpace(req.InventoryID) == "" {
		return nil, NewGoodserError(ErrInventoryNotFound)
	}
	req.Normalize()

	query := &QueryProductsReq{
		InventoryID:   req.InventoryID,
		Keyword:       strings.TrimSpace(req.Keyword),
		SeriesID:      strings.TrimSpace(req.SeriesID),
		CharacterID:   strings.TrimSpace(req.CharacterID),
		SetTypeID:     strings.TrimSpace(req.SetTypeID),
		ProductTypeID: strings.TrimSpace(req.ProductTypeID),
		StatusCode:    strings.TrimSpace(req.StatusCode),
		OnlyAvailable: !req.IncludeOutOfStock,
		SortBy:        "quantity",
		SortOrder:     "desc",
		Page:          req.Page,
		PageSize:      req.PageSize,
	}

	var err error
	if query.SeriesID == "" && req.SeriesName != "" {
		query.SeriesID, err = s.dictionaryIDByName(ctx, DictionaryKindSeries, req.SeriesName)
		if err != nil {
			if isDictionaryMissing(err) {
				return s.emptyCustomerStockResult(req), nil
			}
			return nil, err
		}
	}
	if query.CharacterID == "" && req.CharacterName != "" {
		query.CharacterID, err = s.dictionaryIDByName(ctx, DictionaryKindCharacter, req.CharacterName)
		if err != nil {
			if isDictionaryMissing(err) {
				return s.emptyCustomerStockResult(req), nil
			}
			return nil, err
		}
	}
	if query.ProductTypeID == "" && req.ProductType != "" {
		query.ProductTypeID, err = s.dictionaryIDByName(ctx, DictionaryKindProductType, req.ProductType)
		if err != nil {
			if isDictionaryMissing(err) {
				return s.emptyCustomerStockResult(req), nil
			}
			return nil, err
		}
	}

	products, hasMore, total, err := s.SearchProductsPaginated(ctx, query)
	if err != nil {
		return nil, err
	}

	dictionaryIDs := make([]string, 0, len(products)*4)
	for _, product := range products {
		if product.SeriesID != nil {
			dictionaryIDs = append(dictionaryIDs, *product.SeriesID)
		}
		if product.SetTypeID != nil {
			dictionaryIDs = append(dictionaryIDs, *product.SetTypeID)
		}
		if product.ProductTypeID != nil {
			dictionaryIDs = append(dictionaryIDs, *product.ProductTypeID)
		}
		dictionaryIDs = append(dictionaryIDs, product.CharacterIDs...)
	}
	names, err := s.repo.GetDictionaryNames(ctx, dictionaryIDs)
	if err != nil {
		return nil, err
	}

	items := make([]CustomerProductView, 0, len(products))
	for _, product := range products {
		view := CustomerProductView{
			ID:               product.ID,
			InventoryID:      product.InventoryID,
			Name:             product.Name,
			Code:             product.Code,
			StatusCode:       product.StatusCode,
			Quantity:         product.Quantity,
			ReservedQuantity: product.ReservedQuantity,
			TotalQuantity:    product.Quantity + product.ReservedQuantity,
			ExpectedPrice:    product.ExpectedPrice,
			ImageURL:         product.ImageURL,
			Available:        product.Quantity > 0,
		}
		if product.SeriesID != nil {
			view.Series = names[*product.SeriesID]
		}
		if product.SetTypeID != nil {
			view.SetType = names[*product.SetTypeID]
		}
		if product.ProductTypeID != nil {
			view.ProductType = names[*product.ProductTypeID]
		}
		for _, characterID := range product.CharacterIDs {
			if name := names[characterID]; name != "" {
				view.Characters = append(view.Characters, name)
			}
		}
		items = append(items, view)
	}

	combos, err := s.ListCombos(ctx, req.InventoryID, ComboStatusActive)
	if err != nil {
		return nil, err
	}
	keyword := strings.ToLower(strings.TrimSpace(req.Keyword))
	comboViews := make([]CustomerComboView, 0, len(combos))
	for _, combo := range combos {
		if !req.IncludeOutOfStock && combo.Availability <= 0 {
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(combo.Name), keyword) &&
			!strings.Contains(strings.ToLower(combo.Code), keyword) {
			continue
		}
		comboViews = append(comboViews, CustomerComboView{
			ID:           combo.ID,
			InventoryID:  combo.InventoryID,
			Name:         combo.Name,
			Code:         combo.Code,
			Description:  combo.Description,
			ImageURL:     combo.ImageURL,
			Price:        combo.Price,
			Availability: combo.Availability,
			Components:   combo.Items,
		})
	}

	bundles, err := s.CustomerBundles(ctx, req.InventoryID, req.Keyword)
	if err != nil {
		return nil, err
	}

	return &CustomerStockQueryResp{
		Items:    items,
		Combos:   comboViews,
		Bundles:  bundles,
		Total:    total,
		HasMore:  hasMore,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

func (s *Service) dictionaryIDByName(ctx context.Context, kind DictionaryKind, name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", nil
	}
	item, err := s.repo.FindDictionaryByName(ctx, string(kind), name)
	if err != nil {
		return "", err
	}
	return item.ID, nil
}

func isDictionaryMissing(err error) bool {
	var target *GoodserError
	return errors.As(err, &target) && target.Code == ErrDictionaryNotFound
}

func (s *Service) emptyCustomerStockResult(req *CustomerStockQueryReq) *CustomerStockQueryResp {
	return &CustomerStockQueryResp{
		Items:    []CustomerProductView{},
		Combos:   []CustomerComboView{},
		Bundles:  []CustomerBundleView{},
		Page:     req.Page,
		PageSize: req.PageSize,
	}
}

// CustomerOrderRequest 客户提交订单请求。
// 复用预留单：创建即锁定库存，店主确认后转出库。
func (s *Service) CustomerOrderRequest(ctx context.Context, req *CustomerOrderRequestReq) (*CustomerOrderRequestResp, error) {
	if req == nil || strings.TrimSpace(req.InventoryID) == "" {
		return nil, NewGoodserError(ErrInventoryNotFound)
	}
	if len(req.Items) == 0 && strings.TrimSpace(req.ComboID) == "" && req.Bundle == nil {
		return nil, NewGoodserError(ErrCustomerItemsNeeded)
	}
	if req.Bundle != nil && (len(req.Items) > 0 || strings.TrimSpace(req.ComboID) != "") {
		return nil, NewGoodserError(ErrInvalidDimension, "使用捆下单时 items 和 combo_id 必须留空")
	}

	info := map[string]any{
		"source":        "langbot",
		"customer_name": strings.TrimSpace(req.CustomerName),
		"contact":       strings.TrimSpace(req.Contact),
	}
	if req.OrderInfo != nil && strings.TrimSpace(*req.OrderInfo) != "" {
		info["note"] = strings.TrimSpace(*req.OrderInfo)
	}
	infoJSON, err := json.Marshal(info)
	if err != nil {
		return nil, err
	}
	infoStr := string(infoJSON)

	orderType := string(OrderTypeReserve)
	customerItems := make([]OrderItem, 0, len(req.Items))
	for _, item := range req.Items {
		customerItems = append(customerItems, OrderItem{
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
		})
	}
	createReq := &CreateOutboundReq{
		InventoryID:   req.InventoryID,
		Type:          &orderType,
		OrderInfo:     &infoStr,
		Remark:        req.Remark,
		Items:         customerItems,
		ComboQuantity: req.ComboQuantity,
		Bundle:        req.Bundle,
	}
	if strings.TrimSpace(req.ComboID) != "" {
		createReq.ComboID = &req.ComboID
	}
	order, err := s.CreateOutboundOrder(ctx, createReq)
	if err != nil {
		return nil, err
	}

	var orderItems []OrderItem
	_ = json.Unmarshal(order.Items, &orderItems)
	totalQty := 0
	for _, item := range orderItems {
		totalQty += item.Quantity
	}
	totalPrice := 0.0
	bundleID := ""
	if len(order.BundleSnapshot) > 0 {
		var snapshot BundleOrderSnapshot
		if err := json.Unmarshal(order.BundleSnapshot, &snapshot); err == nil {
			totalPrice = snapshot.TotalPrice
			bundleID = snapshot.BundleID
		}
	}

	payload := map[string]any{
		"order_id":       order.ID,
		"order_no":       order.OrderNo,
		"inventory_id":   order.InventoryID,
		"status":         order.Status,
		"customer_name":  info["customer_name"],
		"contact":        info["contact"],
		"items":          orderItems,
		"combo_id":       req.ComboID,
		"combo_quantity": req.ComboQuantity,
		"bundle_id":      bundleID,
		"total_quantity": totalQty,
		"total_price":    totalPrice,
		"created_at":     time.Now().UTC(),
	}
	if len(order.BundleSnapshot) > 0 {
		payload["bundle_snapshot"] = order.BundleSnapshot
	}

	resp := &CustomerOrderRequestResp{
		OrderID:    order.ID,
		OrderNo:    order.OrderNo,
		Status:     order.Status,
		TotalQty:   totalQty,
		TotalPrice: totalPrice,
		BundleID:   bundleID,
		NotifyOK:   true,
	}
	resp.NotifyOK, resp.NotifyErr = s.dispatchCustomerOrderEvent(ctx, payload)
	return resp, nil
}

// dispatchCustomerOrderEvent 先写外盒事件，再尝试即时通知店主。
// 两者都是尽力而为，失败不会回滚已经创建的预留单。
func (s *Service) dispatchCustomerOrderEvent(ctx context.Context, payload map[string]any) (bool, string) {
	const topic = "goodser.customer.order_requested"
	ok := true
	message := ""

	if s.events != nil {
		if _, err := s.events.Emit(ctx, outbox.EmitInput{
			Topic:         topic,
			Source:        "goodser",
			AggregateType: "outbound_order",
			AggregateID:   toStringValue(payload["order_id"]),
			Payload:       payload,
		}); err != nil {
			ok = false
			message = "事件写入失败: " + err.Error()
			logger.Error().Err(err).Msg("goodser: 客户订单事件写入失败")
		}
	}

	if s.ownerNotifier != nil {
		if err := s.ownerNotifier(ctx, topic, payload); err != nil {
			ok = false
			if message == "" {
				message = "店主通知失败: " + err.Error()
			} else {
				message += "；店主通知失败: " + err.Error()
			}
			logger.Error().Err(err).Msg("goodser: 店主通知失败")
		}
	}
	return ok, message
}

func toStringValue(v any) string {
	switch value := v.(type) {
	case string:
		return value
	case nil:
		return ""
	default:
		data, err := json.Marshal(value)
		if err != nil {
			return ""
		}
		return string(data)
	}
}
