package container

import (
	"context"
	"errors"
	"fmt"
)

type entry struct {
	name      string
	component Component
}

type Container struct {
	entries []entry
	started int
}

func New() *Container {
	return &Container{}
}

func (c *Container) Register(name string, comp Component) {
	c.entries = append(c.entries, entry{name: name, component: comp})
}

func (c *Container) Get(name string) Component {
	for _, e := range c.entries {
		if e.name == name {
			return e.component
		}
	}
	return nil
}

func (c *Container) Start(ctx context.Context) error {
	for i, e := range c.entries {
		if err := e.component.Start(ctx); err != nil {
			startErr := fmt.Errorf("container: start %q: %w", e.name, err)

			rollbackErr := c.stopN(ctx, i)
			c.started = 0

			if rollbackErr != nil {
				return errors.Join(startErr, fmt.Errorf("container: rollback errors: %w", rollbackErr))
			}
			return startErr
		}
		c.started = i + 1
	}
	return nil
}

func (c *Container) Stop(ctx context.Context) error {
	return c.stopN(ctx, c.started)
}

func (c *Container) stopN(ctx context.Context, n int) error {
	var errs []error
	for i := n - 1; i >= 0; i-- {
		e := c.entries[i]
		if err := e.component.Stop(ctx); err != nil {
			errs = append(errs, fmt.Errorf("stop %q: %w", e.name, err))
		}
	}
	return errors.Join(errs...)
}
