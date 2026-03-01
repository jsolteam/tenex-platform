package metrics

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

func StartServer(ctx context.Context, port int, h http.Handler) func(context.Context) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", h)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	serveErr := make(chan error, 1)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serveErr <- err
		} else {
			serveErr <- nil
		}
	}()

	go func() {
		select {
		case <-ctx.Done():
			shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = srv.Shutdown(shutCtx)
		case <-serveErr:
		}
	}()

	return func(shutdownCtx context.Context) error {
		return srv.Shutdown(shutdownCtx)
	}
}
