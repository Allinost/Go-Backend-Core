package goodser

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// 捆默认值
const (
	defaultBundleMainQuantity = 1
	maxBundleMainCandidates   = 200
)

// ListBundles 查询捆模板，附带主商品与副区商品
func (s *Service) ListBundles(ctx context.Context, inventoryID, status string) ([]Bundle, error) {
	if strings.TrimSpace(inventoryID) == "" {
		return nil, NewGoodserError(ErrInventoryNotFound)
	}
	if status != "" {
		if err := validateBundleStatus(status); err != nil {
			return nil, err
		}
	}
	items, err := s.repo.ListBundles(ctx, inventoryID, status)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if err := s.hydrateBundle(ctx, &items[i]); err != nil {
			return nil, err
		}
	}
	return items, nil
}

// GetBundle 获取捆模板详情
func (s *Service) GetBundle(ctx context.Context, id string) (*Bundle, error) {
	bundle, err := s.repo.GetBundle(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.hydrateBundle(ctx, bundle); err != nil {
		return nil, err
	}
	return bundle, nil
}

func (s *Service) hydrateBundle(ctx context.Context, bundle *Bundle) error {
	if bundle == nil {
		return nil
	}
	explicit, err := s.repo.LoadBundleMainProducts(ctx, bundle.ID)
	if err != nil {
		return err
	}
	bundle.MainProductIDs = make([]string, 0, len(explicit))
	for _, item := range explicit {
		bundle.MainProductIDs = append(bundle.MainProductIDs, item.ProductID)
	}
	mainProducts, err := s.repo.ResolveBundleMainProducts(ctx, bundle, "", maxBundleMainCandidates)
	if err != nil {
		return err
	}
	if err := s.attachBundleMainCharacters(ctx, mainProducts); err != nil {
		return err
	}
	subProducts, err := s.repo.LoadBundleSubProducts(ctx, bundle.ID, "", false)
	if err != nil {
		return err
	}
	bundle.MainProducts = mainProducts
	bundle.SubProducts = subProducts
	return nil
}

func (s *Service) attachBundleMainCharacters(ctx context.Context, products []BundleMainProduct) error {
	ids := make([]string, 0, len(products))
	for _, product := range products {
		ids = append(ids, product.ProductID)
	}
	mapping, err := s.repo.ListProductCharacterIDs(ctx, ids)
	if err != nil {
		return err
	}
	for i := range products {
		products[i].CharacterIDs = mapping[products[i].ProductID]
	}
	return nil
}

// CreateBundle 创建捆模板
func (s *Service) CreateBundle(ctx context.Context, req *CreateBundleReq) (*Bundle, error) {
	if req == nil {
		return nil, NewGoodserError(ErrBundleNotFound)
	}
	if strings.TrimSpace(req.InventoryID) == "" {
		return nil, NewGoodserError(ErrInventoryNotFound)
	}
	if err := s.validateBundleCreate(ctx, req); err != nil {
		return nil, err
	}
	bundle, err := s.repo.CreateBundle(ctx, req)
	if err != nil {
		return nil, err
	}
	return s.GetBundle(ctx, bundle.ID)
}

// UpdateBundle 更新捆模板
func (s *Service) UpdateBundle(ctx context.Context, req *UpdateBundleReq) (*Bundle, error) {
	if req == nil || strings.TrimSpace(req.ID) == "" {
		return nil, NewGoodserError(ErrBundleNotFound)
	}
	existing, err := s.repo.GetBundle(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	explicit, err := s.repo.LoadBundleMainProducts(ctx, existing.ID)
	if err != nil {
		return nil, err
	}
	subs, err := s.repo.LoadBundleSubProducts(ctx, existing.ID, "", false)
	if err != nil {
		return nil, err
	}

	mainProductIDs := make([]string, 0, len(explicit))
	for _, item := range explicit {
		mainProductIDs = append(mainProductIDs, item.ProductID)
	}
	subReqs := make([]BundleSubItemReq, 0, len(subs))
	for _, item := range subs {
		subReqs = append(subReqs, BundleSubItemReq{
			ProductID:       item.ProductID,
			DefaultQuantity: item.DefaultQuantity,
		})
	}
	candidate := &CreateBundleReq{
		InventoryID:     existing.InventoryID,
		Code:            existing.Code,
		Name:            existing.Name,
		Description:     existing.Description,
		Status:          existing.Status,
		MainFilter:      &existing.MainFilter,
		MainProductIDs:  mainProductIDs,
		SubProducts:     subReqs,
		MinMainQuantity: existing.MinMainQuantity,
		MinMainAmount:   existing.MinMainAmount,
		MinSubQuantity:  existing.MinSubQuantity,
		MaxSubQuantity:  existing.MaxSubQuantity,
		MinSubAmount:    existing.MinSubAmount,
		PriceMode:       existing.PriceMode,
		FixedPrice:      existing.FixedPrice,
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
	if req.MainFilter != nil {
		candidate.MainFilter = req.MainFilter
	}
	if req.MainProductIDs != nil {
		candidate.MainProductIDs = *req.MainProductIDs
	}
	if req.SubProducts != nil {
		candidate.SubProducts = *req.SubProducts
	}
	if req.MinMainQuantity != nil {
		candidate.MinMainQuantity = *req.MinMainQuantity
	}
	if req.MinMainAmount != nil {
		candidate.MinMainAmount = *req.MinMainAmount
	}
	if req.MinSubQuantity != nil {
		candidate.MinSubQuantity = *req.MinSubQuantity
	}
	if req.MaxSubQuantity != nil {
		candidate.MaxSubQuantity = *req.MaxSubQuantity
	}
	if req.MinSubAmount != nil {
		candidate.MinSubAmount = *req.MinSubAmount
	}
	if req.PriceMode != nil {
		candidate.PriceMode = *req.PriceMode
	}
	if req.FixedPrice != nil {
		candidate.FixedPrice = *req.FixedPrice
	}

	if err := s.validateBundleCreate(ctx, candidate); err != nil {
		return nil, err
	}
	// 把校验后的规范化值写回更新请求，避免默认值和去重结果丢失
	req.Code = &candidate.Code
	req.Name = &candidate.Name
	req.Description = candidate.Description
	req.Status = &candidate.Status
	req.MainFilter = candidate.MainFilter
	req.MainProductIDs = &candidate.MainProductIDs
	req.SubProducts = &candidate.SubProducts
	req.MinMainQuantity = &candidate.MinMainQuantity
	req.MinMainAmount = &candidate.MinMainAmount
	req.MinSubQuantity = &candidate.MinSubQuantity
	req.MaxSubQuantity = &candidate.MaxSubQuantity
	req.MinSubAmount = &candidate.MinSubAmount
	req.PriceMode = &candidate.PriceMode
	req.FixedPrice = &candidate.FixedPrice
	if _, err := s.repo.UpdateBundle(ctx, req); err != nil {
		return nil, err
	}
	return s.GetBundle(ctx, req.ID)
}

// DeleteBundle 删除捆模板
func (s *Service) DeleteBundle(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return NewGoodserError(ErrBundleNotFound)
	}
	return s.repo.DeleteBundle(ctx, id)
}

func (s *Service) validateBundleCreate(ctx context.Context, req *CreateBundleReq) error {
	if strings.TrimSpace(req.Name) == "" {
		return NewGoodserError(ErrInvalidDimension, "捆名称不能为空")
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
		req.Code = "BND-" + strings.ReplaceAll(uuid.New().String(), "-", "")[:8]
	}
	if req.Status == "" {
		req.Status = BundleStatusActive
	}
	if err := validateBundleStatus(req.Status); err != nil {
		return err
	}
	if req.PriceMode == "" {
		req.PriceMode = BundlePriceModeSum
	}
	if err := validateBundlePriceMode(req.PriceMode); err != nil {
		return err
	}
	if req.PriceMode == BundlePriceModeFixed && req.FixedPrice <= 0 {
		return NewGoodserError(ErrBundleRule, "固定价模式必须设置大于 0 的固定价格")
	}
	if req.MinMainQuantity <= 0 {
		req.MinMainQuantity = defaultBundleMainQuantity
	}
	if req.MinMainAmount < 0 || req.MinSubAmount < 0 || req.FixedPrice < 0 {
		return NewGoodserError(ErrBundleRule, "金额规则不能为负数")
	}
	if req.MinSubQuantity < 0 || req.MaxSubQuantity < 0 {
		return NewGoodserError(ErrBundleRule, "数量规则不能为负数")
	}
	if req.MaxSubQuantity > 0 && req.MaxSubQuantity < req.MinSubQuantity {
		return NewGoodserError(ErrBundleRule, "副区商品最多数量不能小于最少数量")
	}

	if req.MainFilter != nil {
		req.MainFilter.Normalize()
		if err := s.validateBundleFilter(ctx, *req.MainFilter); err != nil {
			return err
		}
	}
	filter := BundleMainFilter{}
	if req.MainFilter != nil {
		filter = *req.MainFilter
	}
	if len(req.MainProductIDs) == 0 && filter.IsZero() {
		return NewGoodserError(ErrBundleMainRequired)
	}

	seenMain := make(map[string]struct{}, len(req.MainProductIDs))
	mainIDs := make([]string, 0, len(req.MainProductIDs))
	for _, productID := range req.MainProductIDs {
		productID = strings.TrimSpace(productID)
		if productID == "" {
			continue
		}
		if _, exists := seenMain[productID]; exists {
			continue
		}
		seenMain[productID] = struct{}{}
		product, err := s.GetProduct(ctx, productID)
		if err != nil {
			return err
		}
		if product.InventoryID != req.InventoryID {
			return NewGoodserError(ErrProductInventory, "主商品不属于该库存目录")
		}
		if !filter.IsZero() && !bundleMainFilterMatches(filter, product, product.CharacterIDs) {
			return NewGoodserError(ErrBundleMainNotAllowed, "主商品 "+product.Name+" 不满足筛选条件")
		}
		mainIDs = append(mainIDs, productID)
	}
	req.MainProductIDs = mainIDs
	if len(mainIDs) == 0 && filter.IsZero() {
		return NewGoodserError(ErrBundleMainRequired)
	}

	if len(req.SubProducts) == 0 {
		return NewGoodserError(ErrBundleSubRequired)
	}
	seenSub := make(map[string]struct{}, len(req.SubProducts))
	subs := make([]BundleSubItemReq, 0, len(req.SubProducts))
	for _, item := range req.SubProducts {
		productID := strings.TrimSpace(item.ProductID)
		if productID == "" {
			continue
		}
		if _, exists := seenSub[productID]; exists {
			return NewGoodserError(ErrBundleRule, "副区商品重复")
		}
		seenSub[productID] = struct{}{}
		product, err := s.repo.GetProduct(ctx, productID)
		if err != nil {
			if err == sql.ErrNoRows {
				return NewGoodserError(ErrProductNotFound)
			}
			return err
		}
		if product.InventoryID != req.InventoryID {
			return NewGoodserError(ErrProductInventory, "副区商品不属于该库存目录")
		}
		if item.DefaultQuantity <= 0 {
			item.DefaultQuantity = 1
		}
		subs = append(subs, BundleSubItemReq{ProductID: productID, DefaultQuantity: item.DefaultQuantity})
	}
	req.SubProducts = subs
	if len(subs) == 0 {
		return NewGoodserError(ErrBundleSubRequired)
	}
	return nil
}

func (s *Service) validateBundleFilter(ctx context.Context, filter BundleMainFilter) error {
	seriesID, setTypeID, productTypeID := filter.SeriesID, filter.SetTypeID, filter.ProductTypeID
	var characterIDs []string
	if filter.CharacterID != "" {
		characterIDs = []string{filter.CharacterID}
	}
	var seriesPtr, setTypePtr, productTypePtr *string
	if seriesID != "" {
		seriesPtr = &seriesID
	}
	if setTypeID != "" {
		setTypePtr = &setTypeID
	}
	if productTypeID != "" {
		productTypePtr = &productTypeID
	}
	var charsPtr *[]string
	if len(characterIDs) > 0 {
		charsPtr = &characterIDs
	}
	return s.validateProductDimensions(ctx, seriesPtr, setTypePtr, productTypePtr, charsPtr)
}

func validateBundleStatus(status string) error {
	if status == "" || status == BundleStatusActive || status == BundleStatusInactive {
		return nil
	}
	return NewGoodserError(ErrInvalidDimension, "捆状态仅支持 active/inactive")
}

func validateBundlePriceMode(mode string) error {
	switch mode {
	case BundlePriceModeSum, BundlePriceModeFixed, BundlePriceModeMainOnly:
		return nil
	default:
		return NewGoodserError(ErrBundleRule, "价格模式仅支持 sum/fixed/main_only")
	}
}

// ListBundleMainProducts 查询捆允许的主商品
func (s *Service) ListBundleMainProducts(ctx context.Context, bundleID, keyword string) ([]BundleMainProduct, error) {
	bundle, err := s.repo.GetBundle(ctx, bundleID)
	if err != nil {
		return nil, err
	}
	items, err := s.repo.ResolveBundleMainProducts(ctx, bundle, keyword, maxBundleMainCandidates)
	if err != nil {
		return nil, err
	}
	if err := s.attachBundleMainCharacters(ctx, items); err != nil {
		return nil, err
	}
	return items, nil
}

// ListBundleSubProducts 查询捆允许的副区商品
func (s *Service) ListBundleSubProducts(ctx context.Context, bundleID, keyword string, onlyAvailable bool) ([]BundleSubProduct, error) {
	if _, err := s.repo.GetBundle(ctx, bundleID); err != nil {
		return nil, err
	}
	return s.repo.LoadBundleSubProducts(ctx, bundleID, keyword, onlyAvailable)
}

// PreviewBundle 校验一次捆选择并返回价格，不改变库存
func (s *Service) PreviewBundle(ctx context.Context, bundleID string, req *BundlePreviewReq) (*BundlePreviewResp, error) {
	bundle, err := s.GetBundle(ctx, bundleID)
	if err != nil {
		return nil, err
	}
	if req == nil {
		req = &BundlePreviewReq{}
	}
	selection := &BundleOrderSelection{
		BundleID:      bundleID,
		MainProductID: req.MainProductID,
		MainQuantity:  req.MainQuantity,
		Quantity:      req.Quantity,
		SubItems:      req.SubItems,
	}
	resp, _, err := s.buildBundleSelection(ctx, bundle, selection, func(productID string) (*Product, error) {
		return s.repo.GetProduct(ctx, productID)
	})
	return resp, err
}

// prepareBundleOrder 在出库事务中展开捆明细
func (s *Service) prepareBundleOrder(ctx context.Context, tx *sql.Tx, inventoryID string, selection *BundleOrderSelection) ([]OrderItem, *BundleOrderSnapshot, error) {
	if selection == nil {
		return nil, nil, NewGoodserError(ErrBundleNotFound)
	}
	selection.Normalize()
	if selection.BundleID == "" {
		return nil, nil, NewGoodserError(ErrBundleNotFound)
	}
	bundle, err := s.GetBundle(ctx, selection.BundleID)
	if err != nil {
		return nil, nil, err
	}
	if bundle.InventoryID != inventoryID {
		return nil, nil, NewGoodserError(ErrProductInventory, "捆不属于该库存目录")
	}
	_, snapshot, err := s.buildBundleSelection(ctx, bundle, selection, func(productID string) (*Product, error) {
		return s.repo.GetProductForUpdateTx(ctx, tx, productID)
	})
	if err != nil {
		return nil, nil, err
	}
	items := make([]OrderItem, 0, len(snapshot.SubItems)+1)
	items = append(items, OrderItem{
		ProductID:   snapshot.MainProductID,
		ProductName: snapshot.MainProductName,
		Quantity:    snapshot.MainQuantity * snapshot.Quantity,
	})
	for _, item := range snapshot.SubItems {
		items = append(items, OrderItem{
			ProductID:   item.ProductID,
			ProductName: item.ProductName,
			ProductCode: item.ProductCode,
			Quantity:    item.Quantity * snapshot.Quantity,
			ImageURL:    item.ImageURL,
		})
	}
	// 主商品编码和图片以数据库为准，由 prepareOrderItems 统一补齐
	return items, snapshot, nil
}

// buildBundleSelection 校验捆选择并计算价格与快照
func (s *Service) buildBundleSelection(
	ctx context.Context,
	bundle *Bundle,
	selection *BundleOrderSelection,
	getProduct func(string) (*Product, error),
) (*BundlePreviewResp, *BundleOrderSnapshot, error) {
	if bundle == nil {
		return nil, nil, NewGoodserError(ErrBundleNotFound)
	}
	if bundle.Status != BundleStatusActive {
		return nil, nil, NewGoodserError(ErrBundleNotFound, "捆已停用")
	}
	selection.Normalize()
	if selection.MainProductID == "" {
		return nil, nil, NewGoodserError(ErrBundleMainRequired)
	}

	mainProduct, err := getProduct(selection.MainProductID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, NewGoodserError(ErrProductNotFound)
		}
		return nil, nil, err
	}
	if mainProduct.InventoryID != bundle.InventoryID {
		return nil, nil, NewGoodserError(ErrProductInventory, "主商品不属于该库存目录")
	}
	allowed, err := s.bundleAllowsMainProduct(ctx, bundle, mainProduct)
	if err != nil {
		return nil, nil, err
	}
	if !allowed {
		return nil, nil, NewGoodserError(ErrBundleMainNotAllowed)
	}
	mainQty := selection.MainQuantity
	if mainQty < bundle.MinMainQuantity {
		return nil, nil, NewGoodserError(ErrBundleRule,
			fmt.Sprintf("主商品数量不足，至少 %d 件", bundle.MinMainQuantity))
	}
	mainPrice := productSalePrice(mainProduct)
	mainAmount := roundMoney(mainPrice * float64(mainQty))
	if mainAmount < bundle.MinMainAmount {
		return nil, nil, NewGoodserError(ErrBundleRule,
			fmt.Sprintf("主商品金额不足，至少 %.2f 元", bundle.MinMainAmount))
	}

	allowedSubs := make(map[string]BundleSubProduct, len(bundle.SubProducts))
	for _, item := range bundle.SubProducts {
		allowedSubs[item.ProductID] = item
	}
	aggregated := make(map[string]int)
	subOrder := make([]string, 0, len(selection.SubItems))
	for _, item := range selection.SubItems {
		productID := strings.TrimSpace(item.ProductID)
		if productID == "" {
			continue
		}
		if item.Quantity <= 0 {
			return nil, nil, NewGoodserError(ErrInvalidQuantity)
		}
		if _, ok := allowedSubs[productID]; !ok {
			return nil, nil, NewGoodserError(ErrBundleSubNotAllowed, "副区商品不在捆配置中")
		}
		if _, exists := aggregated[productID]; !exists {
			subOrder = append(subOrder, productID)
		}
		aggregated[productID] += item.Quantity
	}

	subQty := 0
	for _, qty := range aggregated {
		subQty += qty
	}
	if subQty < bundle.MinSubQuantity {
		return nil, nil, NewGoodserError(ErrBundleRule,
			fmt.Sprintf("副区商品数量不足，至少 %d 件", bundle.MinSubQuantity))
	}
	if bundle.MaxSubQuantity > 0 && subQty > bundle.MaxSubQuantity {
		return nil, nil, NewGoodserError(ErrBundleRule,
			fmt.Sprintf("副区商品数量超出上限，最多 %d 件", bundle.MaxSubQuantity))
	}

	subItems := make([]BundleSelectedSubItem, 0, len(subOrder))
	subAmount := 0.0
	for _, productID := range subOrder {
		qty := aggregated[productID]
		product, err := getProduct(productID)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, nil, NewGoodserError(ErrProductNotFound)
			}
			return nil, nil, err
		}
		if product.InventoryID != bundle.InventoryID {
			return nil, nil, NewGoodserError(ErrProductInventory, "副区商品不属于该库存目录")
		}
		price := productSalePrice(product)
		amount := roundMoney(price * float64(qty))
		subAmount += amount
		subItems = append(subItems, BundleSelectedSubItem{
			ProductID:   product.ID,
			ProductName: product.Name,
			ProductCode: product.Code,
			Quantity:    qty,
			UnitPrice:   price,
			Amount:      amount,
			Available:   product.Quantity,
			ImageURL:    product.ImageURL,
		})
	}
	subAmount = roundMoney(subAmount)
	if subAmount < bundle.MinSubAmount {
		return nil, nil, NewGoodserError(ErrBundleRule,
			fmt.Sprintf("副区商品金额不足，至少 %.2f 元", bundle.MinSubAmount))
	}

	unitPrice := 0.0
	switch bundle.PriceMode {
	case BundlePriceModeFixed:
		unitPrice = bundle.FixedPrice
	case BundlePriceModeMainOnly:
		unitPrice = mainPrice * float64(mainQty)
	default:
		unitPrice = mainPrice*float64(mainQty) + subAmount
	}
	unitPrice = roundMoney(unitPrice)
	totalPrice := roundMoney(unitPrice * float64(selection.Quantity))

	items := make([]OrderItem, 0, len(subItems)+1)
	items = append(items, OrderItem{
		ProductID:   mainProduct.ID,
		ProductName: mainProduct.Name,
		ProductCode: mainProduct.Code,
		Quantity:    mainQty * selection.Quantity,
		ImageURL:    mainProduct.ImageURL,
	})
	for _, item := range subItems {
		items = append(items, OrderItem{
			ProductID:   item.ProductID,
			ProductName: item.ProductName,
			ProductCode: item.ProductCode,
			Quantity:    item.Quantity * selection.Quantity,
			ImageURL:    item.ImageURL,
		})
	}

	mainView := BundleMainProduct{
		ProductID:     mainProduct.ID,
		ProductName:   mainProduct.Name,
		ProductCode:   mainProduct.Code,
		ImageURL:      mainProduct.ImageURL,
		ExpectedPrice: mainPrice,
		Available:     mainProduct.Quantity,
		ReservedQty:   mainProduct.ReservedQuantity,
		TotalQuantity: mainProduct.TotalQuantity,
		SeriesID:      mainProduct.SeriesID,
		SetTypeID:     mainProduct.SetTypeID,
		ProductTypeID: mainProduct.ProductTypeID,
		CharacterIDs:  mainProduct.CharacterIDs,
	}

	snapshot := &BundleOrderSnapshot{
		BundleID:        bundle.ID,
		BundleCode:      bundle.Code,
		BundleName:      bundle.Name,
		MainProductID:   mainProduct.ID,
		MainProductName: mainProduct.Name,
		MainQuantity:    mainQty,
		Quantity:        selection.Quantity,
		SubItems:        subItems,
		PriceMode:       bundle.PriceMode,
		MinMainQuantity: bundle.MinMainQuantity,
		MinMainAmount:   bundle.MinMainAmount,
		MinSubQuantity:  bundle.MinSubQuantity,
		MaxSubQuantity:  bundle.MaxSubQuantity,
		MinSubAmount:    bundle.MinSubAmount,
		UnitPrice:       unitPrice,
		TotalPrice:      totalPrice,
	}

	resp := &BundlePreviewResp{
		BundleID:     bundle.ID,
		BundleCode:   bundle.Code,
		BundleName:   bundle.Name,
		InventoryID:  bundle.InventoryID,
		PriceMode:    bundle.PriceMode,
		Quantity:     selection.Quantity,
		MainProduct:  mainView,
		MainQuantity: mainQty,
		MainAmount:   mainAmount,
		SubItems:     subItems,
		SubQuantity:  subQty,
		SubAmount:    subAmount,
		UnitPrice:    unitPrice,
		TotalPrice:   totalPrice,
		Items:        items,
		Snapshot:     *snapshot,
	}
	return resp, snapshot, nil
}

func (s *Service) bundleAllowsMainProduct(ctx context.Context, bundle *Bundle, product *Product) (bool, error) {
	explicit, err := s.repo.LoadBundleMainProducts(ctx, bundle.ID)
	if err != nil {
		return false, err
	}
	if len(explicit) > 0 {
		found := false
		for _, item := range explicit {
			if item.ProductID == product.ID {
				found = true
				break
			}
		}
		if !found {
			return false, nil
		}
	}
	if !bundle.MainFilter.IsZero() {
		characterIDs := product.CharacterIDs
		if len(characterIDs) == 0 {
			mapping, err := s.repo.ListProductCharacterIDs(ctx, []string{product.ID})
			if err != nil {
				return false, err
			}
			characterIDs = mapping[product.ID]
		}
		if !bundleMainFilterMatches(bundle.MainFilter, product, characterIDs) {
			return false, nil
		}
	}
	if len(explicit) == 0 && bundle.MainFilter.IsZero() {
		return false, nil
	}
	return true, nil
}

// CustomerBundles 返回客户可见的启用捆模板，库存严格只读
func (s *Service) CustomerBundles(ctx context.Context, inventoryID, keyword string) ([]CustomerBundleView, error) {
	bundles, err := s.ListBundles(ctx, inventoryID, BundleStatusActive)
	if err != nil {
		return nil, err
	}
	kw := strings.ToLower(strings.TrimSpace(keyword))
	views := make([]CustomerBundleView, 0, len(bundles))
	for _, bundle := range bundles {
		if kw != "" && !strings.Contains(strings.ToLower(bundle.Name), kw) &&
			!strings.Contains(strings.ToLower(bundle.Code), kw) {
			continue
		}
		views = append(views, CustomerBundleView{
			ID:              bundle.ID,
			InventoryID:     bundle.InventoryID,
			Name:            bundle.Name,
			Code:            bundle.Code,
			Description:     bundle.Description,
			MainFilter:      bundle.MainFilter,
			MainProducts:    bundle.MainProducts,
			SubProducts:     bundle.SubProducts,
			MinMainQuantity: bundle.MinMainQuantity,
			MinMainAmount:   bundle.MinMainAmount,
			MinSubQuantity:  bundle.MinSubQuantity,
			MaxSubQuantity:  bundle.MaxSubQuantity,
			MinSubAmount:    bundle.MinSubAmount,
			PriceMode:       bundle.PriceMode,
			FixedPrice:      bundle.FixedPrice,
		})
	}
	return views, nil
}

// productSalePrice 选择对外展示价格，优先期望售价
func productSalePrice(product *Product) float64 {
	if product == nil {
		return 0
	}
	if product.ExpectedPrice > 0 {
		return product.ExpectedPrice
	}
	if product.MarketPrice > 0 {
		return product.MarketPrice
	}
	if product.OriginalPrice > 0 {
		return product.OriginalPrice
	}
	return 0
}

func roundMoney(value float64) float64 {
	if value >= 0 {
		return float64(int64(value*100+0.5)) / 100
	}
	return float64(int64(value*100-0.5)) / 100
}
