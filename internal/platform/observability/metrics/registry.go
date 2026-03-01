package metrics

import (
	"context"
	"net/http"
)

type Label struct {
	Name  string
	Value string
}

func L(name, value string) Label {
	return Label{Name: name, Value: value}
}

type Counter interface {
	Inc(ctx context.Context, labels ...Label)
	Add(ctx context.Context, delta int64, labels ...Label)
}

type Histogram interface {
	Record(ctx context.Context, value float64, labels ...Label)
}

type Registry interface {
	Counter(name, description string, labelNames ...string) (Counter, error)
	Histogram(name, description string, buckets []float64, labelNames ...string) (Histogram, error)
	Handler() http.Handler
}
