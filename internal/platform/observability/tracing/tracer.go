package tracing

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type Tracer interface {
	Start(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, Span)
}

type Span interface {
	End()
	RecordError(err error)
	SetStatus(code codes.Code, description string)
	SetAttributes(kv ...attribute.KeyValue)
}
