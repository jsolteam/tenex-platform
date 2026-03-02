package metrics

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

func StartServer(_ context.Context, port int, h http.Handler) func(context.Context) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", h)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("[metrics] server error: %v\n", err)
		}
	}()

	return func(shutdownCtx context.Context) error {
		return srv.Shutdown(shutdownCtx)
	}
}
