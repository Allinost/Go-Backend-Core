// Package collector 提供价格采集器接口与注册表
package collector

import (
	"context"
	"time"
)

// PriceResult 价格抓取结果
type PriceResult struct {
	CurrentPrice  float64   // 当前价格
	OriginalPrice float64   // 原价
	Title         string    // 商品标题
	StockStatus   string    // 库存状态：in_stock / out_of_stock / unknown
	FetchedAt     time.Time // 抓取时间
}

// Collector 价格采集器接口，每种平台需实现该接口
type Collector interface {
	// Platform 返回平台标识，如 "jd"、"taobao"、"pdd"
	Platform() string
	// FetchPrice 根据商品 URL 抓取当前价格，返回 PriceResult
	FetchPrice(ctx context.Context, url string) (*PriceResult, error)
}

// Registry 采集器注册表，用于管理和获取各平台的采集器
type Registry struct {
	collectors map[string]Collector
}

// NewRegistry 创建空的采集器注册表
func NewRegistry() *Registry {
	return &Registry{
		collectors: make(map[string]Collector),
	}
}

// Register 注册指定平台的采集器
func (r *Registry) Register(platform string, c Collector) {
	r.collectors[platform] = c
}

// Get 获取指定平台的采集器，第二个返回值为是否存在
func (r *Registry) Get(platform string) (Collector, bool) {
	c, ok := r.collectors[platform]
	return c, ok
}
