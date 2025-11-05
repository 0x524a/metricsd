package collector

import (
	"context"
	"github.com/prometheus/client_golang/prometheus"
)

// Metric represents a collected metric
type Metric struct {
	Name   string
	Labels map[string]string
	Value  float64
	Type   string // "gauge" or "counter"
}

// Collector is the interface that all metric collectors must implement (Interface Segregation Principle)
type Collector interface {
	Collect(ctx context.Context) ([]Metric, error)
	Name() string
}

// Registry holds all registered collectors (Dependency Inversion Principle)
type Registry struct {
	collectors []Collector
}

// NewRegistry creates a new collector registry
func NewRegistry() *Registry {
	return &Registry{
		collectors: make([]Collector, 0),
	}
}

// Register adds a collector to the registry
func (r *Registry) Register(collector Collector) {
	r.collectors = append(r.collectors, collector)
}

// CollectAll collects metrics from all registered collectors
func (r *Registry) CollectAll(ctx context.Context) ([]Metric, error) {
	allMetrics := make([]Metric, 0)

	for _, collector := range r.collectors {
		metrics, err := collector.Collect(ctx)
		if err != nil {
			// Continue collecting from other collectors even if one fails
			// Log the error but don't stop
			continue
		}
		allMetrics = append(allMetrics, metrics...)
	}

	return allMetrics, nil
}

// ToPrometheusMetrics converts collected metrics to Prometheus metric format
func ToPrometheusMetrics(metrics []Metric) []prometheus.Metric {
	promMetrics := make([]prometheus.Metric, 0, len(metrics))

	for _, m := range metrics {
		labels := make([]string, 0, len(m.Labels))
		values := make([]string, 0, len(m.Labels))

		for k, v := range m.Labels {
			labels = append(labels, k)
			values = append(values, v)
		}

		var valueType prometheus.ValueType
		if m.Type == "counter" {
			valueType = prometheus.CounterValue
		} else {
			valueType = prometheus.GaugeValue
		}

		desc := prometheus.NewDesc(m.Name, "", labels, nil)
		metric, err := prometheus.NewConstMetric(desc, valueType, m.Value, values...)
		if err == nil {
			promMetrics = append(promMetrics, metric)
		}
	}

	return promMetrics
}
