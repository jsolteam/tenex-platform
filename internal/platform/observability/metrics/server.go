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

	startErr := make(chan error, 1)

	go func() {
		fmt.Printf("[metrics] server starting on %s\n", srv.Addr)
		err := srv.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			fmt.Printf("[metrics] server error: %v\n", err)
			select {
			case startErr <- err:
			default:
			}
		}
	}()

	select {
	case err := <-startErr:
		fmt.Printf("[metrics] failed to start server: %v\n", err)
		return func(context.Context) error { return err }
	case <-time.After(2 * time.Second):
		fmt.Printf("[metrics] server started successfully on %s\n", srv.Addr)
	case <-ctx.Done():
		_ = srv.Close()
		return func(context.Context) error { return ctx.Err() }
	}

	return func(shutdownCtx context.Context) error {
		fmt.Printf("[metrics] shutting down server on %s\n", srv.Addr)
		return srv.Shutdown(shutdownCtx)
	}
}
