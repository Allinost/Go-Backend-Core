package goodser

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/Allinost/go-backend-core/internal/database"
	"github.com/Allinost/go-backend-core/internal/pkg/logger"
	"github.com/Allinost/go-backend-core/internal/services/storage"
	"github.com/gin-gonic/gin"
)

// Module Goodser 业务模块
type Module struct {
	cfg   *config.Config
	h     *Handler
	svc   *Service
	repo  *Repository
	store storage.Storage
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

	m.repo = NewRepository(pool.DB)
	m.svc = NewService(m.repo)

	// 自动建表
	if err := initTables(pool.DB); err != nil {
		return fmt.Errorf("goodser: 初始化数据表失败: %w", err)
	}

	// 初始化图片存储（优先使用 RustFS）
	if client, ok := database.DB.RustFS["rustfs"]; ok {
		m.store = storage.NewS3StoreFromClient(client.Client, client.Bucket)
		logger.Info().Msg("goodser: 使用 RustFS 作为图片存储后端")
	} else {
		logger.Warn().Msg("goodser: RustFS 不可用，图片上传将不可用")
	}

	m.h = NewHandler(m.svc, m.store)

	return nil
}

// Close 关闭模块
func (m *Module) Close() error {
	return nil
}

// RegisterRoutes 注册所有路由
func (m *Module) RegisterRoutes(r *gin.RouterGroup) {
	// === Legacy 兼容端点 ===
	legacy := r.Group("/legacy")
	legacy.POST("/loadInventories", m.h.LoadInventories)
	legacy.POST("/loadProducts", m.h.LoadProducts)
	legacy.POST("/queryProducts", m.h.QueryProducts)
	legacy.POST("/loadOutboundOrders", m.h.LoadOutboundOrders)
	legacy.POST("/loadInboundLogs", m.h.LoadInboundLogs)
	legacy.POST("/loadTags", m.h.LoadTags)
	legacy.POST("/loadStatusCodes", m.h.LoadStatusCodes)
	legacy.POST("/createInventory", m.h.CreateInventory)
	legacy.POST("/updateInventory", m.h.UpdateInventory)
	legacy.POST("/deleteInventory", m.h.DeleteInventory)
	legacy.POST("/createProduct", m.h.CreateProduct)
	legacy.POST("/deleteProduct", m.h.DeleteProduct)
	legacy.POST("/allocateSeq", m.h.AllocateSeq)
	legacy.POST("/inboundSingle", m.h.InboundSingle)
	legacy.POST("/inboundBatch", m.h.InboundBatch)
	legacy.POST("/inboundSearchImport", m.h.InboundSearchImport)
	legacy.POST("/createOutbound", m.h.CreateOutbound)
	legacy.POST("/confirmOutbound", m.h.ConfirmOutboundLegacy)
	legacy.POST("/cancelOutbound", m.h.CancelOutboundLegacy)
	legacy.POST("/createTag", m.h.CreateTag)
	legacy.POST("/updateTag", m.h.UpdateTagLegacy)
	legacy.POST("/deleteTag", m.h.DeleteTagLegacy)
	legacy.POST("/updateProduct", m.h.UpdateProductLegacy)
	legacy.POST("/addStatusCode", m.h.AddStatusCode)
	legacy.POST("/updateStatusCode", m.h.UpdateStatusCode)
	legacy.POST("/removeStatusCode", m.h.RemoveStatusCode)
	legacy.POST("/createInboundLog", m.h.CreateInboundLogHandler)
	legacy.POST("/updateInboundLog", m.h.UpdateInboundLogHandler)
	legacy.POST("/deleteInboundLog", m.h.DeleteInboundLogHandler)
	legacy.POST("/cancelReserve", m.h.CancelReserveHandler)
	legacy.POST("/reserveToOutbound", m.h.ReserveToOutboundHandler)
	legacy.POST("/uploadImage", m.h.UploadImage)

	// === RESTful 端点 ===
	r.GET("/inventories", m.h.ListInventoriesREST)
	r.POST("/inventories", m.h.CreateInventoryREST)

	inv := r.Group("/inventories/:id")
	inv.GET("", m.h.GetInventoryREST)
	inv.PUT("", m.h.UpdateInventoryREST)
	inv.DELETE("", m.h.DeleteInventoryREST)
	inv.GET("/products", m.h.ListProductsREST)
	inv.POST("/products", m.h.CreateProduct)
	inv.POST("/allocate-seq", m.h.AllocateSeq)
	inv.POST("/inbound/single", m.h.InboundSingle)
	inv.POST("/inbound/batch", m.h.InboundBatch)
	inv.POST("/inbound/search-import", m.h.InboundSearchImport)
	inv.POST("/outbound-orders", m.h.CreateOutbound)
	inv.GET("/outbound-orders", m.h.LoadOutboundOrders)
	inv.GET("/inbound-logs", m.h.LoadInboundLogs)

	// Products (standalone CRUD)
	r.PUT("/products/:id", m.h.UpdateProductREST)
	r.DELETE("/products/:id", m.h.DeleteProductREST)

	// Settings
	stg := r.Group("/settings")
	stg.GET("/tags", m.h.LoadTags)
	stg.GET("/status-codes", m.h.LoadStatusCodes)
}

// initTables 自动创建数据表（CREATE TABLE IF NOT EXISTS）
func initTables(db *sql.DB) error {
	ctx := context.Background()
	tables := []string{
		`CREATE TABLE IF NOT EXISTS zzz_goodser_inventories (
			id VARCHAR(36) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			sort_order INT NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		`CREATE TABLE IF NOT EXISTS zzz_goodser_products (
			id VARCHAR(36) PRIMARY KEY,
			inventory_id VARCHAR(36) NOT NULL,
			code VARCHAR(50) NOT NULL,
			main_zone VARCHAR(10) NOT NULL,
			sub_zone VARCHAR(10) NOT NULL,
			seq_number INT NOT NULL,
			quantity INT NOT NULL DEFAULT 0,
			reserved_quantity INT NOT NULL DEFAULT 0,
			status_code VARCHAR(10) NOT NULL DEFAULT 'A',
			name VARCHAR(500) NOT NULL,
			original_price DECIMAL(12,2) NOT NULL DEFAULT 0,
			market_price DECIMAL(12,2) NOT NULL DEFAULT 0,
			expected_price DECIMAL(12,2) NOT NULL DEFAULT 0,
			remark TEXT,
			storage_location VARCHAR(255) DEFAULT '',
			image_url VARCHAR(1024) DEFAULT '',
			tags JSON DEFAULT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_inventory_id (inventory_id),
			INDEX idx_code (inventory_id, code),
			INDEX idx_zone (inventory_id, main_zone, sub_zone),
			INDEX idx_seq_number (inventory_id, seq_number),
			FULLTEXT INDEX ft_name (name)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		`CREATE TABLE IF NOT EXISTS zzz_goodser_recycled_seq_numbers (
			id VARCHAR(36) PRIMARY KEY,
			inventory_id VARCHAR(36) NOT NULL,
			main_zone VARCHAR(10) NOT NULL,
			sub_zone VARCHAR(10) NOT NULL,
			seq_number INT NOT NULL,
			recycled_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_zone_seq (inventory_id, main_zone, sub_zone, seq_number)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		`CREATE TABLE IF NOT EXISTS zzz_goodser_seq_counters (
			id VARCHAR(36) PRIMARY KEY,
			inventory_id VARCHAR(36) NOT NULL,
			main_zone VARCHAR(10) NOT NULL,
			sub_zone VARCHAR(10) NOT NULL,
			current_max INT NOT NULL DEFAULT 0,
			UNIQUE INDEX idx_unique_zone (inventory_id, main_zone, sub_zone)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		`CREATE TABLE IF NOT EXISTS zzz_goodser_outbound_orders (
			id VARCHAR(36) PRIMARY KEY,
			inventory_id VARCHAR(36) NOT NULL,
			order_no VARCHAR(50) NOT NULL,
			type VARCHAR(20) NOT NULL,
			status VARCHAR(20) NOT NULL,
			order_info TEXT,
			remark TEXT,
			items JSON NOT NULL,
			source_reserve_id VARCHAR(36) DEFAULT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			confirmed_at DATETIME DEFAULT NULL,
			cancelled_at DATETIME DEFAULT NULL,
			INDEX idx_inventory_id (inventory_id),
			INDEX idx_order_no (order_no)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		`CREATE TABLE IF NOT EXISTS zzz_goodser_inbound_logs (
			id VARCHAR(36) PRIMARY KEY,
			inventory_id VARCHAR(36) NOT NULL,
			order_no VARCHAR(50) DEFAULT '',
			type VARCHAR(20) NOT NULL,
			remark TEXT,
			items JSON NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_inventory_id (inventory_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		`CREATE TABLE IF NOT EXISTS zzz_goodser_status_codes (
			id VARCHAR(36) PRIMARY KEY,
			code VARCHAR(10) NOT NULL,
			label VARCHAR(100) NOT NULL,
			is_system TINYINT(1) NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE INDEX idx_code (code)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		`CREATE TABLE IF NOT EXISTS zzz_goodser_tags (
			id VARCHAR(36) PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			color VARCHAR(7) NOT NULL DEFAULT '#1890ff',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE INDEX idx_name (name)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}
	for _, ddl := range tables {
		if _, err := db.ExecContext(ctx, ddl); err != nil {
			return err
		}
	}

	// 插入系统预设状态编码（首次启动）
	seed := `INSERT IGNORE INTO zzz_goodser_status_codes (id, code, label, is_system) VALUES
		('sys-a', 'A', '正常', 1),
		('sys-b', 'B', '预留', 1),
		('sys-c', 'C', '已拆', 1),
		('sys-d', 'D', '损坏', 1),
		('sys-e', 'E', '过期', 1),
		('sys-f', 'F', '停用', 1),
		('sys-n', 'N', '全新', 1)`
	if _, err := db.ExecContext(ctx, seed); err != nil {
		return err
	}

	logger.Info().Msg("goodser: 数据表初始化完成")
	return nil
}

var _ interface {
	Name() string
	Init(*config.Config) error
	Close() error
	RegisterRoutes(*gin.RouterGroup)
} = (*Module)(nil)
