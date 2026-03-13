package components

import (
	"context"
	"fmt"

	"github.com/jsolteam/tenex-platform/internal/platform/config"
)

type ConfigComponent struct {
	filePath string

	mgr      *config.Manager
	shutdown func()
}

func NewConfig(filePath string) *ConfigComponent {
	return &ConfigComponent{filePath: filePath}
}

func (c *ConfigComponent) Start(ctx context.Context) error {
	mgr, shutdown, err := config.Bootstrap(ctx, c.filePath)
	if err != nil {
		return fmt.Errorf("config bootstrap: %w", err)
	}
	c.mgr = mgr
	c.shutdown = shutdown
	return nil
}

func (c *ConfigComponent) Stop(_ context.Context) error {
	if c.shutdown != nil {
		c.shutdown()
	}
	return nil
}

func (c *ConfigComponent) Manager() *config.Manager {
	if c.mgr == nil {
		panic("ConfigComponent.Manager() called before Start()")
	}
	return c.mgr
}

func (c *ConfigComponent) Get() *config.AppConfig {
	return c.Manager().Get()
}
