package metrics

import "net/http"

type LabelSet []Label

type Label struct {
	Name  string
	Value string
}

func L(name, value string) Label {
	return Label{Name: name, Value: value}
}

type Counter interface {
	Inc(labels ...Label)
	Add(delta float64, labels ...Label)
}

type Histogram interface {
	Observe(value float64, labels ...Label)
}

type Registry interface {
	Counter(name, help string, labelNames ...string) (Counter, error)

	Histogram(name, help string, buckets []float64, labelNames ...string) (Histogram, error)

	Handler() http.Handler
}
