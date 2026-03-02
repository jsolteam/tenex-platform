package tracing

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/jsol/tenex-platform/internal/platform/config"
)

const (
	forceFlushTimeout = 10 * time.Second
)

func Bootstrap(ctx context.Context, cfg *config.AppConfig) (Tracer, func(context.Context) error, error) {
	if cfg.Observability.OTLPEndpoint == "" {
		noop := func(_ context.Context) error { return nil }
		return NewNoop(), noop, nil
	}

	conn, err := grpc.NewClient(
		cfg.Observability.OTLPEndpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("tracing: grpc dial %q: %w", cfg.Observability.OTLPEndpoint, err)
	}

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithGRPCConn(conn),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("tracing: otlp exporter: %w", err)
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.App.Name),
			attribute.String("env", cfg.App.Env),
		),
	)
	if err != nil {
		res = resource.Default()
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)

	shutdown := func(shutCtx context.Context) error {
		flushCtx, cancel := context.WithTimeout(shutCtx, forceFlushTimeout)
		defer cancel()
		if err := tp.ForceFlush(flushCtx); err != nil {
			fmt.Printf("[tracing] ForceFlush: %v\n", err)
		}
		return tp.Shutdown(shutCtx)
	}

	return &sdkTracer{inner: tp.Tracer(cfg.App.Name)}, shutdown, nil
}

type sdkTracer struct {
	inner trace.Tracer
}

func (t *sdkTracer) Start(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, Span) {
	ctx, span := t.inner.Start(ctx, spanName, opts...)
	return ctx, &sdkSpan{inner: span}
}

type sdkSpan struct {
	inner trace.Span
}

func (s *sdkSpan) End()                  { s.inner.End() }
func (s *sdkSpan) RecordError(err error) { s.inner.RecordError(err) }
func (s *sdkSpan) SetStatus(code codes.Code, description string) {
	s.inner.SetStatus(code, description)
}
func (s *sdkSpan) SetAttributes(kv ...attribute.KeyValue) { s.inner.SetAttributes(kv...) }
