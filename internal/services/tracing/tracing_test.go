package tracing

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNoopTracer(t *testing.T) {
	tracer := &noopTracer{}
	ctx, span := tracer.Start(context.Background(), "test")
	assert.NotNil(t, span)
	assert.Equal(t, "test", span.Name)

	tracer.End(span)
	sc := tracer.Inject(ctx)
	assert.Empty(t, sc.TraceID)

	newCtx := tracer.Extract(ctx, SpanContext{TraceID: "abc", SpanID: "def"})
	assert.NotNil(t, newCtx)
}

func TestSetGlobalTracer(t *testing.T) {
	SetGlobalTracer(&noopTracer{})
	assert.NotNil(t, GetGlobalTracer())
}

func TestStartEnd(t *testing.T) {
	SetGlobalTracer(&noopTracer{})
	_, span := Start(context.Background(), "global-test")
	assert.Equal(t, "global-test", span.Name)
	End(span)
}

func TestStartWithAttributes(t *testing.T) {
	SetGlobalTracer(&noopTracer{})
	_, span := Start(context.Background(), "attr-test",
		WithAttribute("key1", "val1"),
		WithAttribute("key2", "val2"))
	assert.Equal(t, "val1", span.Attributes["key1"])
	assert.Equal(t, "val2", span.Attributes["key2"])
}

func TestInjectExtract(t *testing.T) {
	SetGlobalTracer(&noopTracer{})
	sc := Inject(context.Background())
	assert.Empty(t, sc.TraceID)

	ctx := Extract(context.Background(), SpanContext{TraceID: "123"})
	assert.NotNil(t, ctx)
}

func TestLogTracer(t *testing.T) {
	tracer := NewLogTracer("test-service")
	ctx, span := tracer.Start(context.Background(), "log-span")
	assert.NotNil(t, span)
	assert.Equal(t, "log-span", span.Name)

	sc := tracer.Inject(ctx)
	assert.NotEmpty(t, sc.TraceID)

	tracer.End(span, WithStatus(SpanStatusOK))
	assert.Equal(t, SpanStatusOK, span.Status)
}

func TestLogTracerExtract(t *testing.T) {
	tracer := NewLogTracer("test")
	sc := SpanContext{TraceID: "abc", SpanID: "def"}
	ctx := tracer.Extract(context.Background(), sc)
	out := tracer.Inject(ctx)
	assert.Equal(t, "abc", out.TraceID)
	assert.Equal(t, "def", out.SpanID)
}

func TestStartEndWithStatus(t *testing.T) {
	SetGlobalTracer(&noopTracer{})
	_, span := Start(context.Background(), "status-test")
	End(span, WithStatus(SpanStatusError))
	assert.Equal(t, SpanStatusError, span.Status)
}

func TestLogTracerParentSpan(t *testing.T) {
	tracer := NewLogTracer("svc")
	ctx, parent := tracer.Start(context.Background(), "parent")
	_, child := tracer.Start(ctx, "child", WithParent(parent))
	assert.Equal(t, parent.SpanID, child.ParentSpanID)
}
