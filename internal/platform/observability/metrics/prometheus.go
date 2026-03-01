package metrics

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type prometheusCounter struct {
	vec *prometheus.CounterVec
}

func (c *prometheusCounter) Inc(labels ...Label) {
	c.vec.With(toPromLabels(labels)).Inc()
}

func (c *prometheusCounter) Add(delta float64, labels ...Label) {
	c.vec.With(toPromLabels(labels)).Add(delta)
}

type prometheusHistogram struct {
	vec *prometheus.HistogramVec
}

func (h *prometheusHistogram) Observe(value float64, labels ...Label) {
	h.vec.With(toPromLabels(labels)).Observe(value)
}

type prometheusRegistry struct {
	reg *prometheus.Registry
}

func NewPrometheus() Registry {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		prometheus.NewGoCollector(),
		prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}),
	)

	return &prometheusRegistry{reg: reg}
}

func (r *prometheusRegistry) Counter(name, help string, labelNames ...string) (Counter, error) {
	vec := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: name,
		Help: help,
	}, labelNames)

	if err := r.reg.Register(vec); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			existing, ok2 := are.ExistingCollector.(*prometheus.CounterVec)
			if !ok2 {
				return nil, fmt.Errorf("metrics: %q already registered as different type", name)
			}
			return &prometheusCounter{vec: existing}, nil
		}
		return nil, fmt.Errorf("metrics: register counter %q: %w", name, err)
	}

	return &prometheusCounter{vec: vec}, nil
}

func (r *prometheusRegistry) Histogram(name, help string, buckets []float64, labelNames ...string) (Histogram, error) {
	if len(buckets) == 0 {
		buckets = prometheus.DefBuckets
	}

	vec := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    name,
		Help:    help,
		Buckets: buckets,
	}, labelNames)

	if err := r.reg.Register(vec); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			existing, ok2 := are.ExistingCollector.(*prometheus.HistogramVec)
			if !ok2 {
				return nil, fmt.Errorf("metrics: %q already registered as different type", name)
			}
			return &prometheusHistogram{vec: existing}, nil
		}
		return nil, fmt.Errorf("metrics: register histogram %q: %w", name, err)
	}

	return &prometheusHistogram{vec: vec}, nil
}

func (r *prometheusRegistry) Handler() http.Handler {
	return promhttp.HandlerFor(r.reg, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}

func toPromLabels(labels []Label) prometheus.Labels {
	m := make(prometheus.Labels, len(labels))
	for _, l := range labels {
		m[l.Name] = l.Value
	}
	return m
}
