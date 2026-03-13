package config

import (
	"context"
	"errors"
	"fmt"
	"os"

	cfgproviders "github.com/jsolteam/tenex-platform/internal/platform/config/providers"
)

func Bootstrap(ctx context.Context, filePath string) (*Manager, func(), error) {
	providers := []Provider{
		&cfgproviders.EnvProvider{},
	}
	if filePath != "" {
		providers = append([]Provider{&cfgproviders.FileProvider{Path: filePath}}, providers...)
	}

	loader := NewLoader(providers...)
	mgr := NewManager(loader)

	if err := mgr.Load(); err != nil {
		mgr.Close()
		return nil, nil, fmt.Errorf("initial config load: %w", err)
	}

	ctx, cancelCtx := context.WithCancel(ctx)

	watcherDone := make(chan struct{})

	if filePath != "" {
		watcher := NewWatcher(filePath, mgr)
		go func() {
			defer close(watcherDone)
			if err := watcher.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				fmt.Fprintf(os.Stderr, "[config] watcher stopped: %v\n", err)
			}
		}()
	} else {
		close(watcherDone)
	}

	shutdown := func() {
		cancelCtx()
		<-watcherDone
		mgr.Close()
	}

	return mgr, shutdown, nil
}
