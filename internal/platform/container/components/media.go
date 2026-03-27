package components

import (
	"context"

	"github.com/jsolteam/tenex-platform/internal/media"
	"github.com/jsolteam/tenex-platform/internal/platform/logger/facade"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/tracing"
)

type MediaComponent struct {
	s3    *S3Component
	redis *RedisComponent
	repos *RepositoriesComponent

	service media.Manager
}

func NewMedia(s3 *S3Component, redis *RedisComponent, repos *RepositoriesComponent) *MediaComponent {
	return &MediaComponent{s3: s3, redis: redis, repos: repos}
}

func (m *MediaComponent) Start(_ context.Context) error {
	log := facade.L()

	var tr tracing.Tracer
	if m.s3.tracer != nil {
		tr = m.s3.tracer.Tracer()
	} else {
		tr = tracing.NewNoop()
	}

	m.service = media.NewService(
		m.repos.Repos().Media,
		m.s3.Client(),
		m.redis.Cache,
		log,
		tr,
	)
	return nil
}

func (m *MediaComponent) Stop(_ context.Context) error { return nil }

func (m *MediaComponent) Service() media.Manager {
	if m.service == nil {
		panic("MediaComponent.Service() called before Start()")
	}
	return m.service
}
