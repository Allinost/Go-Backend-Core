package logger

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTraceID(t *testing.T) {
	ctx := WithTraceID(context.Background(), "trace-abc")
	assert.Equal(t, "trace-abc", GetTraceID(ctx))
}

func TestTraceID_Empty(t *testing.T) {
	assert.Empty(t, GetTraceID(context.Background()))
}

func TestSpanID(t *testing.T) {
	ctx := WithSpanID(context.Background(), "span-xyz")
	assert.Equal(t, "span-xyz", GetSpanID(ctx))
}

func TestSpanID_Empty(t *testing.T) {
	assert.Empty(t, GetSpanID(context.Background()))
}

func TestBothIDs(t *testing.T) {
	ctx := context.Background()
	ctx = WithTraceID(ctx, "trace-1")
	ctx = WithSpanID(ctx, "span-2")

	assert.Equal(t, "trace-1", GetTraceID(ctx))
	assert.Equal(t, "span-2", GetSpanID(ctx))
}

func TestNewTraceID_NotEmpty(t *testing.T) {
	id := NewTraceID()
	assert.NotEmpty(t, id)
	assert.Len(t, id, 36)
}

func TestFromContext_NilLogger(t *testing.T) {
	ctx := context.Background()
	l := FromContext(ctx)
	assert.NotNil(t, l)
}

func TestFromContext_WithTraceID(t *testing.T) {
	ctx := WithTraceID(context.Background(), "trace-test")
	l := FromContext(ctx)
	assert.NotNil(t, l)
}
