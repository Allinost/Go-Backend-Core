// Package collector 提供价格采集器接口与注册表
package collector

import (
	"context"
	"strings"
	"time"
)

// PriceResult 价格抓取结果
type PriceResult struct {
	CurrentPrice  float64   // 当前价格（已扣国补/平台优惠后的到手价）
	OriginalPrice float64   // 原价/划线价
	BasePrice     float64   // 计算国补前的基础价（京东价/日常价）
	SubsidyAmount float64   // 本次抵扣的国补金额
	SubsidyRate   float64   // 国补比例（如 15 表示 15%）
	PriceSource   string    // 价格来源：gov_subsidy / label / price_api / dom 等
	Title         string    // 商品标题
	StockStatus   string    // 库存状态：in_stock / out_of_stock / unknown
	ProductURL    string    // 采集到的规范商品页地址（用于回写商品 URL，避免后续跳转）
	RawData       any       // 采集原始数据（用于排查，写入 price_records.raw_data）
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

// IsRateLimitError 判断错误是否来自平台访问频率限制（需要退避，不是登录失效）。
func IsRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, kw := range []string{"频控", "pc-frequent", "访问频率", "too many requests", "http 403", "status 403"} {
		if strings.Contains(msg, kw) {
			return true
		}
	}
	return false
}
