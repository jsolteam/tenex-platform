package tracing

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type noopTracer struct{}

func NewNoop() Tracer {
	return &noopTracer{}
}

func (t *noopTracer) Start(ctx context.Context, _ string, _ ...trace.SpanStartOption) (context.Context, Span) {
	return ctx, noopSpan{}
}

type noopSpan struct{}

func (noopSpan) End()                                  {}
func (noopSpan) RecordError(_ error)                   {}
func (noopSpan) SetStatus(_ codes.Code, _ string)      {}
func (noopSpan) SetAttributes(_ ...attribute.KeyValue) {}
