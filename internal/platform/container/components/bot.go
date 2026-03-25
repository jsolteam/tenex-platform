package components

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync/atomic"

	"github.com/jsolteam/tenex-platform/internal/platform/logger/facade"
)

type BotComponent struct {
	cfg  *ConfigComponent
	repo *RepositoriesComponent

	messenger string
	token     string

	acceptingUpdates atomic.Bool
	inflight         atomic.Int64
}

func NewBot(cfg *ConfigComponent, repos *RepositoriesComponent, messenger, token string) *BotComponent {
	return &BotComponent{
		cfg:       cfg,
		repo:      repos,
		messenger: strings.TrimSpace(strings.ToLower(messenger)),
		token:     strings.TrimSpace(token),
	}
}

func (b *BotComponent) Start(_ context.Context) error {
	if b.messenger == "" {
		return fmt.Errorf("bot component: messenger is empty (set MESSENGER)")
	}
	if b.token == "" {
		return fmt.Errorf("bot component: token is empty (set MESSENGER_TOKEN)")
	}

	_ = b.cfg.Get()
	_ = b.repo.Repos()

	b.acceptingUpdates.Store(true)
	facade.L().Info("bot component started")
	return nil
}

func (b *BotComponent) Stop(_ context.Context) error {
	b.acceptingUpdates.Store(false)
	facade.L().Info("bot component stopped")
	return nil
}

func (b *BotComponent) StopUpdates() {
	b.acceptingUpdates.Store(false)
}

func (b *BotComponent) WaitWorkers(_ context.Context) {
	for b.inflight.Load() > 0 {
		runtime.Gosched()
	}
}
