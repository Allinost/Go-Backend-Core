package logger

import (
	"context"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type ctxKey string

const (
	traceKey ctxKey = "trace_id"
	spanKey  ctxKey = "span_id"
)

func NewTraceID() string {
	return uuid.New().String()
}

func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceKey, traceID)
}

func GetTraceID(ctx context.Context) string {
	if id, ok := ctx.Value(traceKey).(string); ok {
		return id
	}
	return ""
}

func WithSpanID(ctx context.Context, spanID string) context.Context {
	return context.WithValue(ctx, spanKey, spanID)
}

func GetSpanID(ctx context.Context) string {
	if id, ok := ctx.Value(spanKey).(string); ok {
		return id
	}
	return ""
}

func FromContext(ctx context.Context) *zerolog.Logger {
	logger := zerolog.Ctx(ctx)
	if logger == nil {
		return &L.Logger
	}

	traceID := GetTraceID(ctx)
	spanID := GetSpanID(ctx)

	if traceID != "" {
		logger.UpdateContext(func(c zerolog.Context) zerolog.Context {
			return c.Str("trace_id", traceID)
		})
	}
	if spanID != "" {
		logger.UpdateContext(func(c zerolog.Context) zerolog.Context {
			return c.Str("span_id", spanID)
		})
	}

	return logger
}
