package tracing

import (
	"context"
)

// OpenTelemetryTracer OpenTelemetry 兼容追踪器封装
type OpenTelemetryTracer struct {
	inner Tracer // 内部委托的追踪器
}

// NewOpenTelemetryTracer 创建 OpenTelemetry 追踪器
func NewOpenTelemetryTracer(svc string) *OpenTelemetryTracer {
	return &OpenTelemetryTracer{inner: NewLogTracer(svc)}
}

// Start 启动 Span
func (o *OpenTelemetryTracer) Start(ctx context.Context, name string, opts ...StartOption) (context.Context, *Span) {
	return o.inner.Start(ctx, name, opts...)
}

// End 结束 Span
func (o *OpenTelemetryTracer) End(span *Span, opts ...EndOption) {
	o.inner.End(span, opts...)
}

// Inject 从上下文提取 SpanContext
func (o *OpenTelemetryTracer) Inject(ctx context.Context) SpanContext {
	return o.inner.Inject(ctx)
}

// Extract 将 SpanContext 注入上下文
func (o *OpenTelemetryTracer) Extract(ctx context.Context, sc SpanContext) context.Context {
	return o.inner.Extract(ctx, sc)
}

// StartSpan 在 goroutine 中启动 Span（带属性）
func (o *OpenTelemetryTracer) StartSpan(ctx context.Context, name string, attributes map[string]string) (context.Context, *Span) {
	opts := make([]StartOption, 0)
	for k, v := range attributes {
		opts = append(opts, WithAttribute(k, v))
	}
	ch := make(chan struct {
		ctx  context.Context
		span *Span
	})
	go func() {
		c, s := o.Start(ctx, name, opts...)
		ch <- struct {
			ctx  context.Context
			span *Span
		}{c, s}
	}()
	result := <-ch
	return result.ctx, result.span
}

// EndSpan 结束 Span 并根据错误设置状态
func (o *OpenTelemetryTracer) EndSpan(span *Span, err error) {
	opts := []EndOption{WithStatus(SpanStatusOK)}
	if err != nil {
		opts = []EndOption{WithStatus(SpanStatusError)}
		span.Attributes["error"] = err.Error()
	}
	o.End(span, opts...)
}

var _ Tracer = (*OpenTelemetryTracer)(nil)

// GetOTelTracer 获取全局 OpenTelemetry 追踪器实例
func GetOTelTracer() *OpenTelemetryTracer {
	if t, ok := GetGlobalTracer().(*OpenTelemetryTracer); ok {
		return t
	}
	return NewOpenTelemetryTracer("default")
}