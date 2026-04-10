package plugin

import (
	"context"
	"testing"

	"github.com/0x524A/metricsd/internal/collector"
)

type mockCollector struct {
	name    string
	metrics []collector.Metric
	err     error
}

func (m *mockCollector) Name() string { return m.name }
func (m *mockCollector) Collect(ctx context.Context) ([]collector.Metric, error) {
	return m.metrics, m.err
}

func TestGoPluginRegistry(t *testing.T) {
	goPluginRegistry = make(map[string]GoPluginFactory)

	t.Run("register and retrieve", func(t *testing.T) {
		factory := func(cfg map[string]interface{}) (collector.Collector, error) {
			return &mockCollector{name: "test_go_plugin"}, nil
		}
		RegisterGoPlugin("test", factory)

		reg := GetRegisteredGoPlugins()
		if _, ok := reg["test"]; !ok {
			t.Error("expected 'test' factory in registry")
		}
	})

	t.Run("factory creates collector", func(t *testing.T) {
		factory := func(cfg map[string]interface{}) (collector.Collector, error) {
			addr := "default"
			if v, ok := cfg["addr"].(string); ok {
				addr = v
			}
			return &mockCollector{
				name:    "redis",
				metrics: []collector.Metric{{Name: "redis_up", Value: 1, Labels: map[string]string{"addr": addr}}},
			}, nil
		}
		RegisterGoPlugin("redis", factory)

		reg := GetRegisteredGoPlugins()
		c, err := reg["redis"](map[string]interface{}{"addr": "localhost:6379"})
		if err != nil {
			t.Fatalf("factory failed: %v", err)
		}
		if c.Name() != "redis" {
			t.Errorf("expected name 'redis', got %s", c.Name())
		}
		metrics, _ := c.Collect(context.Background())
		if len(metrics) != 1 || metrics[0].Labels["addr"] != "localhost:6379" {
			t.Errorf("unexpected metrics: %v", metrics)
		}
	})

	goPluginRegistry = make(map[string]GoPluginFactory)
}
