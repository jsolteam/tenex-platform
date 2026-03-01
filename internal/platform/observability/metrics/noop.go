package metrics

import (
	"context"
	"net/http"
)

type noopCounter struct{}

func (noopCounter) Inc(_ context.Context, _ ...Label)          {}
func (noopCounter) Add(_ context.Context, _ int64, _ ...Label) {}

type noopHistogram struct{}

func (noopHistogram) Record(_ context.Context, _ float64, _ ...Label) {}

type noopRegistry struct{}

func NewNoop() Registry {
	return noopRegistry{}
}

func (noopRegistry) Counter(_ string, _ string, _ ...string) (Counter, error) {
	return noopCounter{}, nil
}

func (noopRegistry) Histogram(_ string, _ string, _ []float64, _ ...string) (Histogram, error) {
	return noopHistogram{}, nil
}

func (noopRegistry) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "metrics disabled", http.StatusNotFound)
	})
}
