package components

import (
	"context"
	"fmt"

	infras3 "github.com/jsolteam/tenex-platform/internal/infrastructure/s3"
	"github.com/jsolteam/tenex-platform/internal/platform/logger/facade"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/tracing"
)

type S3Component struct {
	cfg    *ConfigComponent
	tracer *TracingComponent
	client *infras3.Client
}

func NewS3(cfg *ConfigComponent, tracer *TracingComponent) *S3Component {
	return &S3Component{cfg: cfg, tracer: tracer}
}

func (s *S3Component) Start(ctx context.Context) error {
	appCfg := s.cfg.Get()
	log := facade.L()

	var tr tracing.Tracer
	if s.tracer != nil {
		tr = s.tracer.Tracer()
	} else {
		tr = tracing.NewNoop()
	}

	cfg := infras3.Config{
		Endpoint: appCfg.S3.Endpoint,
		Key:      appCfg.S3.Key,
		Secret:   appCfg.S3.Secret,
		Bucket:   appCfg.S3.Bucket,
		UseSSL:   appCfg.S3.UseSSL,
	}

	client, err := infras3.New(ctx, cfg, log, tr)
	if err != nil {
		return fmt.Errorf("s3 component: %w", err)
	}

	s.client = client
	return nil
}

func (s *S3Component) Stop(_ context.Context) error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}

func (s *S3Component) Client() *infras3.Client {
	if s.client == nil {
		panic("S3Component.Client() called before Start()")
	}
	return s.client
}
