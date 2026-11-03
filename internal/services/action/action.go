// Package action 定义统一的集成动作注册表与调用网关。
//
// 动作由业务模块注册，调用方（n8n、OpenClaw、LangBot 等）通过网关触发。
// 网关只负责作用域校验与调用编排，业务规则仍由各模块的 Service 负责。
package action

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Allinost/go-backend-core/internal/pkg/authz"
	"github.com/Allinost/go-backend-core/internal/pkg/logger"
	"golang.org/x/sync/singleflight"
)

var (
	// ErrNotFound 动作未注册
	ErrNotFound = errors.New("action: not found")
	// ErrForbidden 调用方缺少动作所需作用域
	ErrForbidden = errors.New("action: forbidden")
	// ErrAlreadyRegistered 动作重复注册
	ErrAlreadyRegistered = errors.New("action: already registered")
	// ErrInvalidDefinition 动作定义不合法
	ErrInvalidDefinition = errors.New("action: invalid definition")
)

// Request 一次动作调用请求
type Request struct {
	Action    string            `json:"action"`
	Actor     string            `json:"actor,omitempty"`
	Scopes    string            `json:"-"`
	Params    json.RawMessage   `json:"params,omitempty"`
	RequestID string            `json:"request_id,omitempty"` // 写操作幂等键
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// Handler 动作处理函数
type Handler func(ctx context.Context, req Request) (any, error)

// Definition 动作定义
type Definition struct {
	Name         string          `json:"name"`
	Description  string          `json:"description,omitempty"`
	Scopes       []string        `json:"scopes,omitempty"`
	Mutating     bool            `json:"mutating"`
	ParamsSchema json.RawMessage `json:"params_schema,omitempty"` // params 的 JSON Schema，供 MCP tools/list 使用
	Handler      Handler         `json:"-"`
}

// Result 动作调用结果
type Result struct {
	Action   string        `json:"action"`
	Data     any           `json:"data,omitempty"`
	Duration time.Duration `json:"duration"`
	Replayed bool          `json:"replayed,omitempty"` // 命中幂等缓存
}

// Registry 动作注册表
type Registry struct {
	mu    sync.RWMutex
	items map[string]Definition
}

// NewRegistry 创建空注册表
func NewRegistry() *Registry {
	return &Registry{items: make(map[string]Definition)}
}

// Register 注册动作，名称重复或定义不合法时返回错误
func (r *Registry) Register(def Definition) error {
	def.Name = strings.TrimSpace(def.Name)
	if def.Name == "" || def.Handler == nil {
		return ErrInvalidDefinition
	}
	for i, scope := range def.Scopes {
		def.Scopes[i] = strings.TrimSpace(scope)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.items[def.Name]; exists {
		return fmt.Errorf("%w: %s", ErrAlreadyRegistered, def.Name)
	}
	r.items[def.Name] = def
	return nil
}

// MustRegister 注册失败直接 panic，用于模块初始化阶段
func (r *Registry) MustRegister(def Definition) {
	if err := r.Register(def); err != nil {
		panic(err)
	}
}

// Get 读取动作定义
func (r *Registry) Get(name string) (Definition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, ok := r.items[strings.TrimSpace(name)]
	return def, ok
}

// List 返回全部动作定义，按名称排序
func (r *Registry) List() []Definition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Definition, 0, len(r.items))
	for _, def := range r.items {
		result = append(result, def)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

// Gateway 动作调用网关
type Gateway struct {
	registry  *Registry
	now       func() time.Time
	idem      IdempotencyStore
	idemTTL   time.Duration
	idemGroup singleflight.Group
}

// GatewayOption 网关可选能力
type GatewayOption func(*Gateway)

// WithIdempotency 为写操作启用幂等缓存。
// 只有 Mutating 动作且请求带 RequestID 时生效。
func WithIdempotency(store IdempotencyStore, ttl time.Duration) GatewayOption {
	return func(g *Gateway) {
		if store == nil {
			return
		}
		if ttl <= 0 {
			ttl = 24 * time.Hour
		}
		g.idem = store
		g.idemTTL = ttl
	}
}

// NewGateway 创建动作网关
func NewGateway(registry *Registry, opts ...GatewayOption) *Gateway {
	g := &Gateway{registry: registry, now: time.Now}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

// Registry 返回动作注册表，供模块注册与列表查询
func (g *Gateway) Registry() *Registry {
	return g.registry
}

// Invoke 校验作用域并执行动作。
// 写操作带 RequestID 时，会按 actor + action + request_id 做幂等缓存。
func (g *Gateway) Invoke(ctx context.Context, req Request) (Result, error) {
	name := strings.TrimSpace(req.Action)
	def, ok := g.registry.Get(name)
	if !ok {
		return Result{}, fmt.Errorf("%w: %s", ErrNotFound, name)
	}
	req.Action = name
	if !authz.Has(req.Scopes, def.Scopes...) {
		return Result{}, fmt.Errorf("%w: %s 需要 %s", ErrForbidden, name, strings.Join(def.Scopes, " 或 "))
	}

	if !def.Mutating || strings.TrimSpace(req.RequestID) == "" || g.idem == nil {
		return g.invoke(ctx, def, req)
	}

	key := idempotencyKey(req, name)
	if result, ok := g.loadIdempotentResult(ctx, key, name); ok {
		return result, nil
	}

	value, err, _ := g.idemGroup.Do(key, func() (any, error) {
		// singleflight 内二次检查，避免并发请求都执行写操作。
		if result, ok := g.loadIdempotentResult(ctx, key, name); ok {
			return result, nil
		}
		result, invokeErr := g.invoke(ctx, def, req)
		if invokeErr != nil {
			return result, invokeErr
		}
		payload, marshalErr := json.Marshal(result.Data)
		if marshalErr != nil {
			logger.Warn().Err(marshalErr).Str("action", name).Msg("action: 幂等结果序列化失败")
			return result, nil
		}
		if saveErr := g.idem.Save(ctx, key, payload, g.idemTTL); saveErr != nil {
			logger.Warn().Err(saveErr).Str("action", name).Msg("action: 幂等结果保存失败")
		}
		return result, nil
	})
	result, _ := value.(Result)
	return result, err
}

func (g *Gateway) invoke(ctx context.Context, def Definition, req Request) (Result, error) {
	start := g.now()
	data, err := def.Handler(ctx, req)
	result := Result{
		Action:   def.Name,
		Data:     data,
		Duration: g.now().Sub(start),
	}
	if err != nil {
		return result, err
	}
	return result, nil
}

func (g *Gateway) loadIdempotentResult(ctx context.Context, key, actionName string) (Result, bool) {
	payload, ok, err := g.idem.Get(ctx, key)
	if err != nil {
		logger.Warn().Err(err).Str("action", actionName).Msg("action: 幂等缓存读取失败")
		return Result{}, false
	}
	if !ok {
		return Result{}, false
	}
	var data any
	if err := json.Unmarshal(payload, &data); err != nil {
		logger.Warn().Err(err).Str("action", actionName).Msg("action: 幂等缓存解析失败")
		return Result{}, false
	}
	return Result{Action: actionName, Data: data, Replayed: true}, true
}

func idempotencyKey(req Request, actionName string) string {
	sum := sha256.Sum256([]byte(req.Actor + "\x00" + actionName + "\x00" + req.RequestID))
	return hex.EncodeToString(sum[:])
}
