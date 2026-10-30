// Package servicenav 服务导航模块，提供服务条目（服务导航）和服务类型的增删改查。
package servicenav

import (
	"fmt"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/Allinost/go-backend-core/internal/database"
	"github.com/gin-gonic/gin"
)

// 数据表名
const (
	tableItem = "zzz_servicenav_item"
	tableType = "zzz_servicenav_type"
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

// Init 初始化模块：获取 MySQL 连接并初始化依赖（建表由 migrations 统一管理）
func (m *Module) Init(cfg *config.Config) error {
	m.cfg = cfg

	pool := database.DB.MySQL["main"]
	if pool == nil {
		return fmt.Errorf("servicenav: MySQL 连接不可用")
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

var _ interface {
	Name() string
	Init(*config.Config) error
	Close() error
	RegisterRoutes(*gin.RouterGroup)
} = (*Module)(nil)
