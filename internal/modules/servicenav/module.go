// Package servicenav 服务导航模块，提供服务条目（服务导航）和服务类型的增删改查。
package servicenav

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/Allinost/go-backend-core/internal/database"
	"github.com/Allinost/go-backend-core/internal/pkg/logger"
	"github.com/gin-gonic/gin"
)

// 数据表名
const (
	tableItem = "services_nav_item"
	tableType = "services_nav_type"
)

// Module 服务导航模块，实现 modules.Module 接口
type Module struct {
	cfg  *config.Config
	h    *Handler
	svc  *Service
	repo *Repository
}

// Name 返回模块名称，用于路由前缀 /api/v1/servicenav/*
func (m *Module) Name() string {
	return "servicenav"
}

// Init 初始化模块：获取 MySQL 连接，自动建表并初始化依赖
func (m *Module) Init(cfg *config.Config) error {
	m.cfg = cfg

	pool := database.DB.MySQL["main"]
	if pool == nil {
		return fmt.Errorf("servicenav: MySQL 连接不可用")
	}

	if err := initTables(pool.DB); err != nil {
		return fmt.Errorf("servicenav: 初始化数据表失败: %w", err)
	}

	m.repo = NewRepository(pool.DB)
	m.svc = NewService(m.repo)
	m.h = NewHandler(m.svc)
	return nil
}

// Close 关闭模块资源
func (m *Module) Close() error {
	return nil
}

// RegisterRoutes 注册服务导航模块路由到 /api/v1/servicenav/* 下
func (m *Module) RegisterRoutes(r *gin.RouterGroup) {
	// 服务类型管理
	r.GET("/types", m.h.ListTypes)
	r.POST("/types", m.h.CreateType)
	r.PUT("/types/:id", m.h.UpdateType)
	r.DELETE("/types/:id", m.h.DeleteType)

	// 服务条目（服务导航）管理
	r.GET("/services", m.h.ListItems)
	r.POST("/services", m.h.CreateItem)
	r.POST("/import", m.h.ImportFromProjects)
	r.GET("/services/:id", m.h.GetItem)
	r.PUT("/services/:id", m.h.UpdateItem)
	r.DELETE("/services/:id", m.h.DeleteItem)
}

// initTables 自动建表：services_nav_item（服务条目）和 services_nav_type（服务类型）
func initTables(db *sql.DB) error {
	ctx := context.Background()
	tables := []string{
		`CREATE TABLE IF NOT EXISTS services_nav_type (
			id VARCHAR(36) PRIMARY KEY,
			name VARCHAR(50) NOT NULL,
			remark VARCHAR(200) DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			UNIQUE KEY uk_name (name)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		`CREATE TABLE IF NOT EXISTS services_nav_item (
			id VARCHAR(36) PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			types JSON NOT NULL,
			protocol VARCHAR(20) NOT NULL DEFAULT 'http',
			address VARCHAR(2048) DEFAULT '',
			intranet_port INT NULL,
			easytier_port INT NULL,
			account VARCHAR(255) DEFAULT '',
			password_hint VARCHAR(255) DEFAULT '',
			remark VARCHAR(500) DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			UNIQUE KEY uk_name (name)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}
	for _, ddl := range tables {
		if _, err := db.ExecContext(ctx, ddl); err != nil {
			return err
		}
	}
	// 兼容已存在的历史表：若缺少 protocol 列则补充（CREATE TABLE IF NOT EXISTS 不会改动已存在的表）
	if err := ensureColumn(db, tableItem, "protocol", "VARCHAR(20) NOT NULL DEFAULT 'http' AFTER types"); err != nil {
		return err
	}
	// developmentManager 关联列：项目 / 容器 / 网络接入点（可空，NULL=未关联）
	linkColumns := []struct {
		name       string
		definition string
	}{
		{"project_id", "BIGINT NULL AFTER remark"},
		{"container_id", "BIGINT NULL AFTER project_id"},
		{"access_point_id", "BIGINT NULL AFTER container_id"},
	}
	for _, col := range linkColumns {
		if err := ensureColumn(db, tableItem, col.name, col.definition); err != nil {
			return err
		}
	}
	logger.Info().Msg("servicenav: 数据表初始化完成")
	return nil
}

// ensureColumn 检查目标表是否已存在指定列，不存在则执行 ALTER TABLE 添加
func ensureColumn(db *sql.DB, table, column, definition string) error {
	ctx := context.Background()
	var count int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM information_schema.COLUMNS
		 WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?`,
		table, column).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err = db.ExecContext(ctx, fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN `%s` %s", table, column, definition))
	return err
}

var _ interface {
	Name() string
	Init(*config.Config) error
	Close() error
	RegisterRoutes(*gin.RouterGroup)
} = (*Module)(nil)
