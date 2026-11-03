package goodser

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/Allinost/go-backend-core/internal/database"
	"github.com/Allinost/go-backend-core/internal/middleware"
	"github.com/Allinost/go-backend-core/internal/modules"
	coreAuth "github.com/Allinost/go-backend-core/internal/modules/auth"
	"github.com/Allinost/go-backend-core/internal/pkg/logger"
	"github.com/Allinost/go-backend-core/internal/services/action"
	"github.com/Allinost/go-backend-core/internal/services/connector"
	"github.com/Allinost/go-backend-core/internal/services/outbox"
	"github.com/Allinost/go-backend-core/internal/services/storage"
	"github.com/gin-gonic/gin"
)

// Module Goodser 业务模块
type Module struct {
	cfg    *config.Config
	h      *Handler
	svc    *Service
	repo   *Repository
	store  storage.Storage
	authMW gin.HandlerFunc
}

// Name 返回模块名称，路由自动挂载至 /api/v1/goodser
func (m *Module) Name() string {
	return "zzz-goodser"
}

// Init 初始化模块（获取数据库连接、初始化分层）
func (m *Module) Init(cfg *config.Config) error {
	m.cfg = cfg

	// 获取 Goodser 专用 MySQL 连接，优先使用 goodser 实例，回退到 main
	pool := database.DB.MySQL["goodser"]
	if pool == nil {
		pool = database.DB.MySQL["main"]
		if pool == nil {
			return fmt.Errorf("goodser: MySQL 连接不可用")
		}
		logger.Warn().Msg("goodser: 使用 main 数据库实例替代 zzz_goodser")
	}

	// 初始化图片存储（优先使用 RustFS）— must be before NewService
	if client, ok := database.DB.RustFS["rustfs"]; ok {
		m.store = storage.NewS3StoreFromClient(client.Client, client.Bucket)
		logger.Info().Msg("goodser: 使用 RustFS 作为图片存储后端")
	} else {
		logger.Warn().Msg("goodser: RustFS 不可用，图片上传将不可用")
	}

	m.repo = NewRepository(pool.DB)
	m.svc = NewService(m.repo, m.store)
	m.wireIntegration()

	m.h = NewHandler(m.svc, m.store)

	// 依赖 auth 模块（main.go 注册顺序保证 auth 先于 goodser 初始化）
	authMod := modules.Get("auth")
	if authMod == nil {
		return fmt.Errorf("goodser: auth 模块未注册")
	}
	am, ok := authMod.(*coreAuth.Module)
	if !ok {
		return fmt.Errorf("goodser: auth 模块类型异常")
	}
	m.authMW = am.APIKeyOrJWTMiddleware()

	return nil
}

// wireIntegration 接入集成基座：外盒事件、店主通知与动作注册。
func (m *Module) wireIntegration() {
	mod := modules.Get("integration")
	if mod == nil {
		return
	}

	type outboxProvider interface {
		Outbox() *outbox.Service
	}
	if provider, ok := mod.(outboxProvider); ok {
		if svc := provider.Outbox(); svc != nil {
			m.svc.SetEventEmitter(svc)
		}
	}

	type connectorProvider interface {
		Connectors() *connector.Registry
	}
	if provider, ok := mod.(connectorProvider); ok {
		if registry := provider.Connectors(); registry != nil {
			m.svc.SetOwnerNotifier(func(ctx context.Context, event string, payload any) error {
				return notifyOwner(ctx, registry, event, payload)
			})
		}
	}

	type actionProvider interface {
		Actions() *action.Registry
	}
	if provider, ok := mod.(actionProvider); ok {
		if registry := provider.Actions(); registry != nil {
			if err := RegisterActions(registry, m.svc); err != nil {
				logger.Error().Err(err).Msg("goodser: 注册集成动作失败")
			}
		}
	}
}

// notifyOwner 把客户订单事件推送到所有已注册连接器。
// 没有任何连接器时视为成功；至少一个成功即视为通知成功。
func notifyOwner(ctx context.Context, registry *connector.Registry, event string, payload any) error {
	connectors := registry.All()
	if len(connectors) == 0 {
		return nil
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	message := connector.Message{
		EventID:    extractPayloadID(payload),
		Event:      event,
		OccurredAt: time.Now().UTC(),
		Payload:    body,
	}
	var lastErr error
	success := 0
	for _, item := range connectors {
		if err := item.Send(ctx, message); err != nil {
			lastErr = err
			logger.Error().Err(err).Str("connector", item.ID()).Msg("goodser: 连接器推送失败")
			continue
		}
		success++
	}
	if success == 0 && lastErr != nil {
		return lastErr
	}
	return nil
}

func extractPayloadID(payload any) string {
	if values, ok := payload.(map[string]any); ok {
		if id, ok := values["order_id"].(string); ok {
			return id
		}
	}
	return ""
}

// Close 关闭模块
func (m *Module) Close() error {
	return nil
}

// RegisterRoutes 注册所有路由。
// JWT 登录用户视为 owner 控制台；API Key 必须携带对应作用域。
func (m *Module) RegisterRoutes(r *gin.RouterGroup) {
	r.Use(m.authMW)
	read := middleware.RequireScope(middleware.ScopeOwnerRead)
	write := middleware.RequireScope(middleware.ScopeOwnerWrite)

	// === Legacy 兼容端点 ===
	legacy := r.Group("/legacy")
	legacy.POST("/loadInventories", read, m.h.LoadInventories)
	legacy.POST("/loadProducts", read, m.h.LoadProducts)
	legacy.POST("/queryProducts", read, m.h.QueryProducts)
	legacy.POST("/loadOutboundOrders", read, m.h.LoadOutboundOrders)
	legacy.POST("/loadInboundLogs", read, m.h.LoadInboundLogs)
	legacy.POST("/loadTags", read, m.h.LoadTags)
	legacy.POST("/loadStatusCodes", read, m.h.LoadStatusCodes)
	legacy.POST("/createInventory", write, m.h.CreateInventory)
	legacy.POST("/updateInventory", write, m.h.UpdateInventory)
	legacy.POST("/deleteInventory", write, m.h.DeleteInventory)
	legacy.POST("/createProduct", write, m.h.CreateProduct)
	legacy.POST("/deleteProduct", write, m.h.DeleteProduct)
	legacy.POST("/allocateSeq", write, m.h.AllocateSeq)
	legacy.POST("/inboundSingle", write, m.h.InboundSingle)
	legacy.POST("/inboundBatch", write, m.h.InboundBatch)
	legacy.POST("/inboundSearchImport", write, m.h.InboundSearchImport)
	legacy.POST("/createOutbound", write, m.h.CreateOutbound)
	legacy.POST("/confirmOutbound", write, m.h.ConfirmOutboundLegacy)
	legacy.POST("/cancelOutbound", write, m.h.CancelOutboundLegacy)
	legacy.POST("/createTag", write, m.h.CreateTag)
	legacy.POST("/updateTag", write, m.h.UpdateTagLegacy)
	legacy.POST("/deleteTag", write, m.h.DeleteTagLegacy)
	legacy.POST("/updateProduct", write, m.h.UpdateProductLegacy)
	legacy.POST("/addStatusCode", write, m.h.AddStatusCode)
	legacy.POST("/updateStatusCode", write, m.h.UpdateStatusCode)
	legacy.POST("/removeStatusCode", write, m.h.RemoveStatusCode)
	legacy.POST("/createInboundLog", write, m.h.CreateInboundLogHandler)
	legacy.POST("/updateInboundLog", write, m.h.UpdateInboundLogHandler)
	legacy.POST("/deleteInboundLog", write, m.h.DeleteInboundLogHandler)
	legacy.POST("/cancelReserve", write, m.h.CancelReserveHandler)
	legacy.POST("/reserveToOutbound", write, m.h.ReserveToOutboundHandler)
	legacy.POST("/uploadImage", write, m.h.UploadImage)

	// 全量同步
	r.POST("/syncAll", read, m.h.SyncAll)

	// === RESTful 端点 ===
	r.GET("/inventories", read, m.h.ListInventoriesREST)
	r.POST("/inventories", write, m.h.CreateInventoryREST)

	inv := r.Group("/inventories/:id")
	inv.GET("", read, m.h.GetInventoryREST)
	inv.PUT("", write, m.h.UpdateInventoryREST)
	inv.DELETE("", write, m.h.DeleteInventoryREST)
	inv.GET("/products", read, m.h.ListProductsREST)
	inv.POST("/products", write, m.h.CreateProduct)
	inv.POST("/allocate-seq", write, m.h.AllocateSeq)
	inv.POST("/inbound/single", write, m.h.InboundSingle)
	inv.POST("/inbound/batch", write, m.h.InboundBatch)
	inv.POST("/inbound/search-import", write, m.h.InboundSearchImport)
	inv.POST("/outbound-orders", write, m.h.CreateOutbound)
	inv.GET("/outbound-orders", read, m.h.ListOutboundOrdersREST)
	inv.GET("/outbound-orders/:oid", read, m.h.GetOutboundOrderREST)
	inv.POST("/outbound-orders/:oid/confirm", write, m.h.ConfirmOutboundREST)
	inv.POST("/outbound-orders/:oid/cancel", write, m.h.CancelOutboundREST)
	inv.POST("/outbound-orders/:oid/returns", write, m.h.CreateReturnREST)
	inv.POST("/reserves", write, m.h.CreateReserveREST)
	inv.POST("/reserves/:rid/convert", write, m.h.ConvertReserveREST)
	inv.POST("/reserves/:rid/cancel", write, m.h.CancelReserveREST)
	inv.GET("/returns", read, m.h.ListReturnsREST)
	inv.GET("/returns/:rid", read, m.h.GetReturnREST)
	inv.POST("/returns/:rid/confirm", write, m.h.ConfirmReturnREST)
	inv.POST("/returns/:rid/cancel", write, m.h.CancelReturnREST)
	inv.GET("/inbound-logs", read, m.h.ListInboundLogsREST)

	// 受控维度、系列视图与组合包
	inv.GET("/series", read, m.h.ListSeriesSummaryREST)
	inv.GET("/stats", read, m.h.GetInventoryStatsREST)
	inv.GET("/combos", read, m.h.ListCombosREST)
	inv.POST("/combos", write, m.h.CreateComboREST)
	r.GET("/combos/:cid", read, m.h.GetComboREST)
	r.PUT("/combos/:cid", write, m.h.UpdateComboREST)
	r.DELETE("/combos/:cid", write, m.h.DeleteComboREST)

	// 捆：主商品条件约束下搭配副区商品
	inv.GET("/bundles", read, m.h.ListBundlesREST)
	inv.POST("/bundles", write, m.h.CreateBundleREST)
	r.GET("/bundles/:bid", read, m.h.GetBundleREST)
	r.PUT("/bundles/:bid", write, m.h.UpdateBundleREST)
	r.DELETE("/bundles/:bid", write, m.h.DeleteBundleREST)
	r.GET("/bundles/:bid/main-products", read, m.h.ListBundleMainProductsREST)
	r.GET("/bundles/:bid/sub-products", read, m.h.ListBundleSubProductsREST)
	r.POST("/bundles/:bid/preview", read, m.h.PreviewBundleREST)

	// 捆物池：按库存目录隔离，可被多个捆复用
	r.GET("/bundle-pools", read, m.h.ListBundlePoolsREST)
	r.POST("/bundle-pools", write, m.h.CreateBundlePoolREST)
	r.GET("/bundle-pools/:pid", read, m.h.GetBundlePoolREST)
	r.PUT("/bundle-pools/:pid", write, m.h.UpdateBundlePoolREST)
	r.DELETE("/bundle-pools/:pid", write, m.h.DeleteBundlePoolREST)

	// Products (standalone CRUD)
	r.GET("/products/:id", read, m.h.GetProductREST)
	r.PUT("/products/:id", write, m.h.UpdateProductREST)
	r.DELETE("/products/:id", write, m.h.DeleteProductREST)

	// Settings
	stg := r.Group("/settings")
	stg.GET("/tags", read, m.h.LoadTags)
	stg.GET("/status-codes", read, m.h.LoadStatusCodes)
	stg.GET("/dictionaries", read, m.h.ListDictionariesREST)
	stg.POST("/dictionaries", write, m.h.CreateDictionaryREST)
	stg.PUT("/dictionaries/:did", write, m.h.UpdateDictionaryREST)
	stg.DELETE("/dictionaries/:did", write, m.h.DeleteDictionaryREST)

	// 客户服务（只读查询 + 订单请求），库存严格只读
	customerRead := middleware.RequireScope(middleware.ScopeCustomerRead)
	customerOrder := middleware.RequireScope(middleware.ScopeCustomerOrderReq)
	cust := r.Group("/customer")
	cust.POST("/stock-query", customerRead, m.h.CustomerStockQueryREST)
	cust.POST("/order-requests", customerOrder, m.h.CustomerOrderRequestREST)
}

var _ interface {
	Name() string
	Init(*config.Config) error
	Close() error
	RegisterRoutes(*gin.RouterGroup)
} = (*Module)(nil)
