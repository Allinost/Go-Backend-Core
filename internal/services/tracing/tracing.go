package tracing

import (
	"context"
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"github.com/Allinost/go-backend-core/internal/pkg/logger"
)

func init() {
	_ = rand.Reader
}

// SpanStatus Span 状态类型
type SpanStatus int

const (
	SpanStatusOK    SpanStatus = 0 // 正常
	SpanStatusError SpanStatus = 1 // 错误
)

// SpanContext Span 上下文（包含追踪与父 Span ID）
type SpanContext struct {
	TraceID      string // 追踪 ID
	SpanID       string // Span ID
	ParentSpanID string // 父 Span ID
}

// Span 追踪跨度
type Span struct {
	SpanContext                        // 嵌入的 Span 上下文
	Name       string                  // Span 名称
	Status     SpanStatus              // Span 状态
	Attributes map[string]string        // 属性键值对
	Events     []SpanEvent             // 事件列表
	StartTime  int64                   // 开始时间（毫秒时间戳）
	EndTime    int64                   // 结束时间（毫秒时间戳）
}

// SpanEvent Span 事件
type SpanEvent struct {
	Name       string            // 事件名称
	Attributes map[string]string // 事件属性
}

// Tracer 追踪器接口
type Tracer interface {
	Start(ctx context.Context, name string, opts ...StartOption) (context.Context, *Span)   // 启动 Span
	End(span *Span, opts ...EndOption)                                                         // 结束 Span
	Inject(ctx context.Context) SpanContext                                                    // 从上下文中提取 SpanContext
	Extract(ctx context.Context, sc SpanContext) context.Context                               // 将 SpanContext 注入上下文
}

// StartOption Span 启动选项函数
type StartOption func(*Span)

// EndOption Span 结束选项函数
type EndOption func(*Span)

// WithAttribute 添加属性启动选项
func WithAttribute(key, value string) StartOption {
	return func(s *Span) {
		if s.Attributes == nil {
			s.Attributes = make(map[string]string)
		}
		s.Attributes[key] = value
	}
}

// WithParent 设置父 Span 启动选项
func WithParent(parent *Span) StartOption {
	return func(s *Span) {
		if parent != nil {
			s.ParentSpanID = parent.SpanID
		}
	}
}

// WithStatus 设置状态结束选项
func WithStatus(status SpanStatus) EndOption {
	return func(s *Span) {
		s.Status = status
	}
}

// noopTracer 空操作追踪器（默认实现）
type noopTracer struct{}

func (n *noopTracer) Start(ctx context.Context, name string, opts ...StartOption) (context.Context, *Span) {
	span := &Span{Name: name, Attributes: make(map[string]string)}
	for _, opt := range opts {
		opt(span)
	}
	return ctx, span
}

func (n *noopTracer) End(span *Span, opts ...EndOption) {
	for _, opt := range opts {
		opt(span)
	}
}

func (n *noopTracer) Inject(ctx context.Context) SpanContext {
	return SpanContext{}
}

func (n *noopTracer) Extract(ctx context.Context, sc SpanContext) context.Context {
	return ctx
}

var globalTracer Tracer = &noopTracer{}
var mu sync.RWMutex

// SetGlobalTracer 设置全局追踪器
func SetGlobalTracer(t Tracer) {
	mu.Lock()
	defer mu.Unlock()
	globalTracer = t
}

// GetGlobalTracer 获取全局追踪器
func GetGlobalTracer() Tracer {
	mu.RLock()
	defer mu.RUnlock()
	return globalTracer
}

// Start 启动全局追踪器的新 Span
func Start(ctx context.Context, name string, opts ...StartOption) (context.Context, *Span) {
	return GetGlobalTracer().Start(ctx, name, opts...)
}

// End 结束全局追踪器的一个 Span
func End(span *Span, opts ...EndOption) {
	GetGlobalTracer().End(span, opts...)
}

// Inject 从上下文提取 SpanContext
func Inject(ctx context.Context) SpanContext {
	return GetGlobalTracer().Inject(ctx)
}

// Extract 将 SpanContext 注入上下文
func Extract(ctx context.Context, sc SpanContext) context.Context {
	return GetGlobalTracer().Extract(ctx, sc)
}

// logTracer 基于日志的追踪器实现
type logTracer struct {
	service string // 服务名称
}

// NewLogTracer 创建基于日志的追踪器
func NewLogTracer(svc string) Tracer {
	serviceName = svc
	return &logTracer{service: svc}
}

func (l *logTracer) Start(ctx context.Context, name string, opts ...StartOption) (context.Context, *Span) {
	span := &Span{
		Name:       name,
		Attributes: make(map[string]string),
		StartTime:  nowMs(),
	}
	for _, opt := range opts {
		opt(span)
	}
	parent := SpanContext{}
	if sc, ok := ctx.Value(spanCtxKey{}).(SpanContext); ok {
		parent = sc
		span.ParentSpanID = sc.SpanID
	}
	span.TraceID = generateID()
	if parent.TraceID != "" {
		span.TraceID = parent.TraceID
	}
	span.SpanID = generateID()

	ctx = context.WithValue(ctx, spanCtxKey{}, span.SpanContext)
	logger.Debug().
		Str("trace_id", span.TraceID).
		Str("span_id", span.SpanID).
		Str("parent_span_id", span.ParentSpanID).
		Str("span_name", name).
		Msg("tracing: span start")
	return ctx, span
}

func (l *logTracer) End(span *Span, opts ...EndOption) {
	for _, opt := range opts {
		opt(span)
	}
	span.EndTime = nowMs()
	duration := span.EndTime - span.StartTime
	logger.Debug().
		Str("trace_id", span.TraceID).
		Str("span_id", span.SpanID).
		Str("span_name", span.Name).
		Int64("duration_ms", duration).
		Int("status", int(span.Status)).
		Msg("tracing: span end")
}

func (l *logTracer) Inject(ctx context.Context) SpanContext {
	if sc, ok := ctx.Value(spanCtxKey{}).(SpanContext); ok {
		return sc
	}
	return SpanContext{}
}

func (l *logTracer) Extract(ctx context.Context, sc SpanContext) context.Context {
	return context.WithValue(ctx, spanCtxKey{}, sc)
}

// spanCtxKey 上下文键类型
type spanCtxKey struct{}

// serviceName 全局服务名称
var serviceName string

// generateID 生成随机 ID（8 字节十六进制）
func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// nowMs 获取当前毫秒时间戳
func nowMs() int64 {
	return time.Now().UnixMilli()
}