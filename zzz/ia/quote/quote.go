// Package quote 行情采集子包，提供证券与基金净值的多源接入能力。
//
// 当前内置数据源：
//   - eastmoney：A股/ETF/港股/美股实时行情（push2 接口）
//   - fund：场外基金/QDII 净值与实时估值（天天基金 fundgz 接口）
//
// 存款/理财无公开行情，由模块内自动计息任务估值，不经此包。
package quote

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Allinost/go-backend-core/internal/config"
	pkgNet "github.com/Allinost/go-backend-core/pkg/net"
)

// AssetInfo 行情获取所需的标的摘要信息
type AssetInfo struct {
	Code          string `json:"code"`
	Market        string `json:"market"`         // CN-SH / CN-SZ / HK / US / OTC
	Currency      string `json:"currency"`       // 交易币种
	TrackingIndex string `json:"tracking_index"` // QDII 参考指数（预留）
	AssetType     string `json:"asset_type"`     // stock / etf / fund
}

// Quote 行情快照结果
type Quote struct {
	Code      string  `json:"code"`
	Name      string  `json:"name,omitempty"`
	Price     float64 `json:"price"`                // 最新价（证券）或当前参考净值（基金：估值优先）
	PrevClose float64 `json:"prev_close,omitempty"` // 昨收/昨日净值
	Nav       float64 `json:"nav,omitempty"`        // 官方单位净值（基金类，证券为 0）
	NavDate   string  `json:"nav_date,omitempty"`   // 官方净值日期 YYYY-MM-DD（基金类）
	Estimate  float64 `json:"estimate,omitempty"`   // 实时估算净值（基金类，0=暂无估值）
	Source    string  `json:"source"`               // official / estimated
}

// Provider 行情提供方接口
type Provider interface {
	// Name 数据源名称
	Name() string
	// Support 是否支持该标的
	Support(asset AssetInfo) bool
	// Fetch 拉取行情快照
	Fetch(ctx context.Context, asset AssetInfo) (*Quote, error)
}

// Registry 行情提供方注册表，按注册顺序匹配第一个支持的数据源。
// 内部持有独立的 HTTP 客户端：主客户端用于实时行情/净值，筛选客户端用于全市场快照，
// 两者拥有独立的断路器，避免筛选拉取失败影响持仓行情刷新。
type Registry struct {
	mu           sync.RWMutex
	httpClient   *pkgNet.HTTPClient // 主客户端：实时行情、K线、基金净值、汇率
	screenClient *pkgNet.HTTPClient // 筛选专用：全市场快照（clist）
	providers    []Provider
}

// NewRegistry 创建注册表并内置默认数据源。
// httpClient 为主客户端（可为 nil，内部创建默认配置）；筛选客户端自动创建独立断路器。
func NewRegistry(httpClient *pkgNet.HTTPClient) *Registry {
	if httpClient == nil {
		cfg := pkgNet.DefaultHTTPConfigWithBreaker()
		cfg.Timeout = 15 * time.Second
		httpClient = pkgNet.NewHTTPClient(cfg)
	}
	// 筛选客户端：独立断路器（阈值更高，熔断不影响主客户端）
	screenCfg := pkgNet.DefaultHTTPConfigWithBreaker()
	screenCfg.Timeout = 30 * time.Second
	screenCfg.Breaker = pkgNet.NewCircuitBreaker(pkgNet.CircuitBreakerConfig{
		FailureThreshold: 10,
		SuccessThreshold: 2,
		Timeout:          60 * time.Second,
	})

	r := &Registry{
		httpClient:   httpClient,
		screenClient: pkgNet.NewHTTPClient(screenCfg),
	}
	r.Register(NewEastmoneyProvider(httpClient))
	r.Register(NewFundProvider(httpClient))
	return r
}

// NewRegistryWithConfig 创建注册表，筛选客户端断路器从 config 外部化配置读取
func NewRegistryWithConfig(httpClient *pkgNet.HTTPClient, cfg *config.Config) *Registry {
	if httpClient == nil {
		httpClient = pkgNet.NewHTTPClient(pkgNet.DefaultHTTPConfigWithBreaker())
	}
	screenCfg := pkgNet.DefaultHTTPConfig()
	screenCfg.Timeout = 30 * time.Second
	sbrk := config.DefaultScreenBreakerConfig()
	if cfg != nil && cfg.Net.HTTP.ScreenBreaker.FailureThreshold > 0 {
		sbrk.FailureThreshold = cfg.Net.HTTP.ScreenBreaker.FailureThreshold
	}
	if cfg != nil && cfg.Net.HTTP.ScreenBreaker.SuccessThreshold > 0 {
		sbrk.SuccessThreshold = cfg.Net.HTTP.ScreenBreaker.SuccessThreshold
	}
	if cfg != nil && cfg.Net.HTTP.ScreenBreaker.Timeout > 0 {
		sbrk.Timeout = cfg.Net.HTTP.ScreenBreaker.Timeout
	}
	screenCfg.Breaker = pkgNet.NewCircuitBreaker(pkgNet.CircuitBreakerConfig{
		FailureThreshold: sbrk.FailureThreshold,
		SuccessThreshold: sbrk.SuccessThreshold,
		Timeout:          sbrk.Timeout,
	})

	r := &Registry{
		httpClient:   httpClient,
		screenClient: pkgNet.NewHTTPClient(screenCfg),
	}
	r.Register(NewEastmoneyProvider(httpClient))
	r.Register(NewFundProvider(httpClient))
	return r
}

// Register 注册数据源
func (r *Registry) Register(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers = append(r.providers, p)
}

// HTTP 暴露主 HTTP 客户端（实时行情、K线、基金净值、汇率复用）
func (r *Registry) HTTP() *pkgNet.HTTPClient {
	return r.httpClient
}

// ScreenHTTP 暴露筛选专用 HTTP 客户端（独立断路器，熔断不影响主客户端）
func (r *Registry) ScreenHTTP() *pkgNet.HTTPClient {
	return r.screenClient
}

// Fetch 按注册顺序匹配数据源并拉取行情
func (r *Registry) Fetch(ctx context.Context, asset AssetInfo) (*Quote, error) {
	r.mu.RLock()
	providers := make([]Provider, len(r.providers))
	copy(providers, r.providers)
	r.mu.RUnlock()

	for _, p := range providers {
		if !p.Support(asset) {
			continue
		}
		q, err := p.Fetch(ctx, asset)
		if err == nil && q != nil && q.Price > 0 {
			return q, nil
		}
		if err != nil {
			// 单一数据源失败不尝试跨源兜底（不同源代码体系不一致），直接返回错误
			return nil, fmt.Errorf("quote: %s 拉取 %s.%s 失败: %w", p.Name(), asset.Market, asset.Code, err)
		}
	}
	return nil, fmt.Errorf("quote: 无可用数据源支持 %s.%s", asset.Market, asset.Code)
}
