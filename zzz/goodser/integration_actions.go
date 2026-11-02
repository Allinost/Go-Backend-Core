package goodser

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Allinost/go-backend-core/internal/pkg/authz"
	"github.com/Allinost/go-backend-core/internal/services/action"
)

// RegisterActions 把 Goodser 能力注册到集成动作网关。
// n8n / OpenClaw 使用 owner 作用域，LangBot 使用 customer 作用域。
func RegisterActions(registry *action.Registry, svc *Service) error {
	definitions := []action.Definition{
		{
			Name:        "goodser.inventory.list",
			Description: "列出库存目录",
			Scopes:      []string{authz.ScopeOwnerRead},
			Handler: func(ctx context.Context, _ action.Request) (any, error) {
				return svc.ListInventories(ctx)
			},
		},
		{
			Name:        "goodser.stock.query",
			Description: "店主库存查询，支持维度、状态与关键词筛选",
			Scopes:      []string{authz.ScopeOwnerRead},
			Handler: func(ctx context.Context, req action.Request) (any, error) {
				var params CustomerStockQueryReq
				if err := decodeParams(req, &params); err != nil {
					return nil, err
				}
				return svc.CustomerStockQuery(ctx, &params)
			},
		},
		{
			Name:        "goodser.order.list",
			Description: "列出出库/预留单",
			Scopes:      []string{authz.ScopeOwnerRead},
			Handler: func(ctx context.Context, req action.Request) (any, error) {
				var params struct {
					InventoryID string `json:"inventory_id"`
					Page        int    `json:"page"`
					PageSize    int    `json:"page_size"`
				}
				if err := decodeParams(req, &params); err != nil {
					return nil, err
				}
				if strings.TrimSpace(params.InventoryID) == "" {
					return nil, NewGoodserError(ErrInventoryNotFound)
				}
				if params.Page < 1 {
					params.Page = 1
				}
				if params.PageSize < 1 || params.PageSize > 100 {
					params.PageSize = 20
				}
				items, hasMore, total, err := svc.ListOutboundOrdersPaginated(ctx, params.InventoryID, params.Page, params.PageSize)
				if err != nil {
					return nil, err
				}
				return map[string]any{"items": items, "has_more": hasMore, "total": total}, nil
			},
		},
		{
			Name:        "goodser.order.create",
			Description: "创建出库或预留单，支持组合包展开",
			Scopes:      []string{authz.ScopeOwnerWrite},
			Mutating:    true,
			Handler: func(ctx context.Context, req action.Request) (any, error) {
				var params CreateOutboundReq
				if err := decodeParams(req, &params); err != nil {
					return nil, err
				}
				return svc.CreateOutboundOrder(ctx, &params)
			},
		},
		{
			Name:        "goodser.order.confirm",
			Description: "确认出库",
			Scopes:      []string{authz.ScopeOwnerWrite},
			Mutating:    true,
			Handler: func(ctx context.Context, req action.Request) (any, error) {
				var params ConfirmOutboundReq
				if err := decodeParams(req, &params); err != nil {
					return nil, err
				}
				return svc.ConfirmOutbound(ctx, &params)
			},
		},
		{
			Name:        "goodser.order.cancel",
			Description: "取消出库/预留单并释放锁定库存",
			Scopes:      []string{authz.ScopeOwnerWrite},
			Mutating:    true,
			Handler: func(ctx context.Context, req action.Request) (any, error) {
				var params CancelOutboundReq
				if err := decodeParams(req, &params); err != nil {
					return nil, err
				}
				return svc.CancelOutbound(ctx, &params)
			},
		},
		{
			Name:        "goodser.return.create",
			Description: "创建退货单，支持多商品与回库开关",
			Scopes:      []string{authz.ScopeOwnerWrite},
			Mutating:    true,
			Handler: func(ctx context.Context, req action.Request) (any, error) {
				var params CreateReturnReq
				if err := decodeParams(req, &params); err != nil {
					return nil, err
				}
				return svc.CreateReturn(ctx, &params)
			},
		},
		{
			Name:        "goodser.return.confirm",
			Description: "确认退货并按明细回库",
			Scopes:      []string{authz.ScopeOwnerWrite},
			Mutating:    true,
			Handler: func(ctx context.Context, req action.Request) (any, error) {
				var params ConfirmReturnReq
				if err := decodeParams(req, &params); err != nil {
					return nil, err
				}
				return svc.ConfirmReturn(ctx, &params)
			},
		},
		{
			Name:        "goodser.combo.list",
			Description: "列出组合包及可售数量",
			Scopes:      []string{authz.ScopeOwnerRead},
			Handler: func(ctx context.Context, req action.Request) (any, error) {
				var params struct {
					InventoryID string `json:"inventory_id"`
					Status      string `json:"status"`
				}
				if err := decodeParams(req, &params); err != nil {
					return nil, err
				}
				return svc.ListCombos(ctx, params.InventoryID, params.Status)
			},
		},
		{
			Name:        "goodser.combo.create",
			Description: "创建组合包（BOM）",
			Scopes:      []string{authz.ScopeOwnerWrite},
			Mutating:    true,
			Handler: func(ctx context.Context, req action.Request) (any, error) {
				var params CreateComboReq
				if err := decodeParams(req, &params); err != nil {
					return nil, err
				}
				return svc.CreateCombo(ctx, &params)
			},
		},
		{
			Name:        "goodser.bundle.list",
			Description: "列出捆模板及主/副商品配置",
			Scopes:      []string{authz.ScopeOwnerRead},
			Handler: func(ctx context.Context, req action.Request) (any, error) {
				var params struct {
					InventoryID string `json:"inventory_id"`
					Status      string `json:"status"`
				}
				if err := decodeParams(req, &params); err != nil {
					return nil, err
				}
				return svc.ListBundles(ctx, params.InventoryID, params.Status)
			},
		},
		{
			Name:        "goodser.bundle.get",
			Description: "获取捆模板详情",
			Scopes:      []string{authz.ScopeOwnerRead},
			Handler: func(ctx context.Context, req action.Request) (any, error) {
				var params struct {
					BundleID string `json:"bundle_id"`
				}
				if err := decodeParams(req, &params); err != nil {
					return nil, err
				}
				return svc.GetBundle(ctx, params.BundleID)
			},
		},
		{
			Name:        "goodser.bundle.create",
			Description: "创建捆模板，配置主商品条件与副区商品",
			Scopes:      []string{authz.ScopeOwnerWrite},
			Mutating:    true,
			Handler: func(ctx context.Context, req action.Request) (any, error) {
				var params CreateBundleReq
				if err := decodeParams(req, &params); err != nil {
					return nil, err
				}
				return svc.CreateBundle(ctx, &params)
			},
		},
		{
			Name:        "goodser.bundle.update",
			Description: "更新捆模板",
			Scopes:      []string{authz.ScopeOwnerWrite},
			Mutating:    true,
			Handler: func(ctx context.Context, req action.Request) (any, error) {
				var params UpdateBundleReq
				if err := decodeParams(req, &params); err != nil {
					return nil, err
				}
				return svc.UpdateBundle(ctx, &params)
			},
		},
		{
			Name:        "goodser.bundle.delete",
			Description: "删除捆模板",
			Scopes:      []string{authz.ScopeOwnerWrite},
			Mutating:    true,
			Handler: func(ctx context.Context, req action.Request) (any, error) {
				var params struct {
					BundleID string `json:"bundle_id"`
				}
				if err := decodeParams(req, &params); err != nil {
					return nil, err
				}
				return map[string]any{"deleted": true}, svc.DeleteBundle(ctx, params.BundleID)
			},
		},
		{
			Name:        "goodser.bundle.preview",
			Description: "校验捆选择并计算价格，不改变库存",
			Scopes:      []string{authz.ScopeOwnerRead},
			Handler: func(ctx context.Context, req action.Request) (any, error) {
				var params struct {
					BundleID string `json:"bundle_id"`
					BundlePreviewReq
				}
				if err := decodeParams(req, &params); err != nil {
					return nil, err
				}
				return svc.PreviewBundle(ctx, params.BundleID, &params.BundlePreviewReq)
			},
		},
		{
			Name:        "goodser.dictionary.list",
			Description: "列出受控维度字典",
			Scopes:      []string{authz.ScopeOwnerRead},
			Handler: func(ctx context.Context, req action.Request) (any, error) {
				var params ListDictionariesReq
				if err := decodeParams(req, &params); err != nil {
					return nil, err
				}
				return svc.ListDictionaries(ctx, &params)
			},
		},
		{
			Name:        "goodser.customer.stock_query",
			Description: "客户库存查询，只读",
			Scopes:      []string{authz.ScopeCustomerRead},
			Handler: func(ctx context.Context, req action.Request) (any, error) {
				var params CustomerStockQueryReq
				if err := decodeParams(req, &params); err != nil {
					return nil, err
				}
				return svc.CustomerStockQuery(ctx, &params)
			},
		},
		{
			Name:        "goodser.customer.order_request",
			Description: "客户提交订单请求，创建 pending 预留并锁定库存",
			Scopes:      []string{authz.ScopeCustomerOrderReq},
			Mutating:    true,
			Handler: func(ctx context.Context, req action.Request) (any, error) {
				var params CustomerOrderRequestReq
				if err := decodeParams(req, &params); err != nil {
					return nil, err
				}
				return svc.CustomerOrderRequest(ctx, &params)
			},
		},
	}

	for _, def := range definitions {
		if err := registry.Register(def); err != nil {
			return fmt.Errorf("注册 %s 失败: %w", def.Name, err)
		}
	}
	return nil
}

func decodeParams(req action.Request, target any) error {
	if len(req.Params) == 0 {
		return nil
	}
	if err := json.Unmarshal(req.Params, target); err != nil {
		return NewGoodserError(ErrInvalidDimension, "参数解析失败: "+err.Error())
	}
	return nil
}
