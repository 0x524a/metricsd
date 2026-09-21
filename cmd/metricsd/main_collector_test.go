package main

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/0x524A/metricsd/internal/collector"
	"github.com/0x524A/metricsd/internal/config"
	"github.com/0x524A/metricsd/internal/plugin"
	"github.com/0x524A/metricsd/internal/server"
)

func TestSetupCollectors_AllDisabled_NoEndpoints_PluginsDisabled(t *testing.T) {
	cfg := &config.Config{
		Collector: config.CollectorConfig{
			EnableCPU:     false,
			EnableMemory:  false,
			EnableDisk:    false,
			EnableNetwork: false,
			EnableGPU:     false,
			Plugins: config.PluginSystemConfig{
				Enabled: false,
			},
		},
		Endpoints: []config.EndpointConfig{},
	}

	registry, pluginMgr := setupCollectors(cfg)

	if registry == nil {
		t.Fatal("setupCollectors returned nil registry")
	}

	if pluginMgr != nil {
		t.Error("setupCollectors returned non-nil pluginMgr when plugins are disabled")
	}
}

func TestSetupCollectors_EnableCPU(t *testing.T) {
	cfg := &config.Config{
		Collector: config.CollectorConfig{
			EnableCPU:     true,
			EnableMemory:  false,
			EnableDisk:    false,
			EnableNetwork: false,
			EnableGPU:     false,
			Plugins: config.PluginSystemConfig{
				Enabled: false,
			},
		},
		Endpoints: []config.EndpointConfig{},
	}

	registry, pluginMgr := setupCollectors(cfg)

	if registry == nil {
		t.Fatal("setupCollectors returned nil registry")
	}

	if pluginMgr != nil {
		t.Error("setupCollectors returned non-nil pluginMgr when plugins are disabled")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	metrics, err := registry.CollectAll(ctx)
	if err != nil {
		t.Fatalf("CollectAll failed: %v", err)
	}

	if len(metrics) == 0 {
		t.Error("expected at least some metrics from system collector with CPU enabled")
	}
}

func TestSetupCollectors_EnableMemory(t *testing.T) {
	cfg := &config.Config{
		Collector: config.CollectorConfig{
			EnableCPU:     false,
			EnableMemory:  true,
			EnableDisk:    false,
			EnableNetwork: false,
			EnableGPU:     false,
			Plugins: config.PluginSystemConfig{
				Enabled: false,
			},
		},
		Endpoints: []config.EndpointConfig{},
	}

	registry, pluginMgr := setupCollectors(cfg)

	if registry == nil {
		t.Fatal("setupCollectors returned nil registry")
	}

	if pluginMgr != nil {
		t.Error("setupCollectors returned non-nil pluginMgr when plugins are disabled")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	metrics, err := registry.CollectAll(ctx)
	if err != nil {
		t.Fatalf("CollectAll failed: %v", err)
	}

	if len(metrics) == 0 {
		t.Error("expected at least some metrics from system collector with memory enabled")
	}
}

func TestSetupCollectors_EnableDisk(t *testing.T) {
	cfg := &config.Config{
		Collector: config.CollectorConfig{
			EnableCPU:     false,
			EnableMemory:  false,
			EnableDisk:    true,
			EnableNetwork: false,
			EnableGPU:     false,
			Plugins: config.PluginSystemConfig{
				Enabled: false,
			},
		},
		Endpoints: []config.EndpointConfig{},
	}

	registry, _ := setupCollectors(cfg)

	if registry == nil {
		t.Fatal("setupCollectors returned nil registry")
	}
}

func TestSetupCollectors_EnableNetwork(t *testing.T) {
	cfg := &config.Config{
		Collector: config.CollectorConfig{
			EnableCPU:     false,
			EnableMemory:  false,
			EnableDisk:    false,
			EnableNetwork: true,
			EnableGPU:     false,
			Plugins: config.PluginSystemConfig{
				Enabled: false,
			},
		},
		Endpoints: []config.EndpointConfig{},
	}

	registry, _ := setupCollectors(cfg)

	if registry == nil {
		t.Fatal("setupCollectors returned nil registry")
	}
}

func TestSetupCollectors_EnableGPU(t *testing.T) {
	cfg := &config.Config{
		Collector: config.CollectorConfig{
			EnableCPU:     false,
			EnableMemory:  false,
			EnableDisk:    false,
			EnableNetwork: false,
			EnableGPU:     true,
			Plugins: config.PluginSystemConfig{
				Enabled: false,
			},
		},
		Endpoints: []config.EndpointConfig{},
	}

	registry, _ := setupCollectors(cfg)

	if registry == nil {
		t.Fatal("setupCollectors returned nil registry")
	}
}

func TestSetupCollectors_WithEndpoints(t *testing.T) {
	cfg := &config.Config{
		Collector: config.CollectorConfig{
			EnableCPU:     false,
			EnableMemory:  false,
			EnableDisk:    false,
			EnableNetwork: false,
			EnableGPU:     false,
			Plugins: config.PluginSystemConfig{
				Enabled: false,
			},
		},
		Endpoints: []config.EndpointConfig{
			{Name: "app1", URL: "http://localhost:8080/metrics"},
			{Name: "app2", URL: "http://localhost:8081/metrics"},
		},
		Shipper: config.ShipperConfig{
			Timeout: 10 * time.Second,
		},
	}

	registry, _ := setupCollectors(cfg)

	if registry == nil {
		t.Fatal("setupCollectors returned nil registry")
	}
}

func TestSetupCollectors_WithMultipleEndpoints(t *testing.T) {
	cfg := &config.Config{
		Collector: config.CollectorConfig{
			Plugins: config.PluginSystemConfig{
				Enabled: false,
			},
		},
		Endpoints: []config.EndpointConfig{
			{Name: "service1", URL: "http://localhost:9090/metrics"},
			{Name: "service2", URL: "http://localhost:9091/metrics"},
			{Name: "service3", URL: "http://localhost:9092/metrics"},
		},
		Shipper: config.ShipperConfig{
			Timeout: 5 * time.Second,
		},
	}

	registry, _ := setupCollectors(cfg)

	if registry == nil {
		t.Fatal("setupCollectors returned nil registry")
	}
}

func TestSetupCollectors_PluginsEnabled_EmptyDirectory(t *testing.T) {
	emptyDir := t.TempDir()

	cfg := &config.Config{
		Collector: config.CollectorConfig{
			EnableCPU:     false,
			EnableMemory:  false,
			EnableDisk:    false,
			EnableNetwork: false,
			EnableGPU:     false,
			Plugins: config.PluginSystemConfig{
				Enabled:               true,
				PluginsDir:            emptyDir,
				DefaultTimeoutSeconds: 30,
				ValidateOnStartup:     false,
			},
		},
		Endpoints: []config.EndpointConfig{},
	}

	registry, pluginMgr := setupCollectors(cfg)

	if registry == nil {
		t.Fatal("setupCollectors returned nil registry")
	}

	if pluginMgr != nil && pluginMgr.PluginCount() > 0 {
		t.Fatalf("expected no plugins to be loaded from empty dir, got %d", pluginMgr.PluginCount())
	}
}

func TestSetupCollectors_PluginsEnabled_WithPluginDir_NoPluginsFound(t *testing.T) {
	emptyDir := t.TempDir()

	cfg := &config.Config{
		Collector: config.CollectorConfig{
			EnableCPU:     false,
			EnableMemory:  false,
			EnableDisk:    false,
			EnableNetwork: false,
			EnableGPU:     false,
			Plugins: config.PluginSystemConfig{
				Enabled:               true,
				PluginsDir:            emptyDir,
				DefaultTimeoutSeconds: 30,
				ValidateOnStartup:     false,
				GoPlugins:             []config.GoPluginEntry{},
			},
		},
		Endpoints: []config.EndpointConfig{},
	}

	registry, pluginMgr := setupCollectors(cfg)

	if registry == nil {
		t.Fatal("setupCollectors returned nil registry")
	}

	if pluginMgr != nil {
		if pluginMgr.PluginCount() == 0 {
			t.Logf("pluginMgr is not nil but has 0 plugins (acceptable)")
		}
	}
}

func TestSetupCollectors_MultipleMetricsEnabled(t *testing.T) {
	cfg := &config.Config{
		Collector: config.CollectorConfig{
			EnableCPU:     true,
			EnableMemory:  true,
			EnableDisk:    true,
			EnableNetwork: false,
			EnableGPU:     false,
			Plugins: config.PluginSystemConfig{
				Enabled: false,
			},
		},
		Endpoints: []config.EndpointConfig{
			{Name: "app", URL: "http://localhost:9090/metrics"},
		},
		Shipper: config.ShipperConfig{
			Timeout: 10 * time.Second,
		},
	}

	registry, _ := setupCollectors(cfg)

	if registry == nil {
		t.Fatal("setupCollectors returned nil registry")
	}
}

func TestPluginHealthAdapter_GetHealthData_NilManager(t *testing.T) {
	adapter := &pluginHealthAdapter{mgr: nil}

	result := adapter.GetHealthData()

	if result != nil {
		t.Error("expected nil result when mgr is nil, got non-nil map")
	}
}

func TestPluginHealthAdapter_GetHealthData_EmptyManager(t *testing.T) {
	mgr := plugin.NewManager()

	adapter := &pluginHealthAdapter{mgr: mgr}

	result := adapter.GetHealthData()

	if result == nil {
		t.Error("expected non-nil result, got nil")
	}

	if len(result) != 0 {
		t.Errorf("expected empty health map, got %d entries", len(result))
	}
}

func TestPluginHealthAdapter_GetHealthData_WithMockCollector(t *testing.T) {
	mgr := plugin.NewManager()

	mockCollector := &mockCollector{
		name:    "mock_plugin",
		metrics: []collector.Metric{{Name: "test_metric", Value: 42, Type: "gauge"}},
	}
	mgr.AddGoPlugin("mock_plugin", mockCollector)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := mgr.Collect(ctx)
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	adapter := &pluginHealthAdapter{mgr: mgr}

	result := adapter.GetHealthData()

	if result == nil {
		t.Error("expected non-nil result, got nil")
	}

	if len(result) != 1 {
		t.Errorf("expected 1 plugin health entry, got %d", len(result))
	}

	if health, ok := result["mock_plugin"]; !ok {
		t.Error("expected 'mock_plugin' in health data")
	} else {
		if health.Status != "ok" {
			t.Errorf("expected status 'ok', got %q", health.Status)
		}

		if health.MetricCount != 1 {
			t.Errorf("expected MetricCount 1, got %d", health.MetricCount)
		}

		expectedType := "CollectorHealth"
		if reflect.TypeOf(health).Name() != expectedType {
			t.Logf("health struct type: %T", health)
		}
	}
}

func TestPluginHealthAdapter_MapStructConversion(t *testing.T) {
	mgr := plugin.NewManager()

	mockCollector := &mockCollector{
		name:    "test",
		metrics: []collector.Metric{{Name: "m1", Value: 1, Type: "gauge"}},
	}
	mgr.AddGoPlugin("test", mockCollector)

	adapter := &pluginHealthAdapter{mgr: mgr}
	result := adapter.GetHealthData()

	if result == nil {
		t.Fatal("result should not be nil")
	}

	health, ok := result["test"]
	if !ok {
		t.Fatal("expected 'test' key in result")
	}

	if health.Status == "" {
		t.Error("Status should not be empty")
	}

	if health.MetricCount < 0 {
		t.Error("MetricCount should be non-negative")
	}
}

func TestCleanupGPUCollector_WithRegistry(t *testing.T) {
	registry := collector.NewRegistry()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("cleanupGPUCollector panicked: %v", r)
		}
	}()

	cleanupGPUCollector(registry)
}

func TestCleanupGPUCollector_WithGPUCollector(t *testing.T) {
	registry := collector.NewRegistry()

	gpuCollector := collector.NewGPUCollector()
	registry.Register(gpuCollector)

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("cleanupGPUCollector panicked: %v", r)
		}
	}()

	cleanupGPUCollector(registry)
}

func TestPluginHealthAdapter_ServerHealthProviderInterface(t *testing.T) {
	var _ server.HealthProvider = &pluginHealthAdapter{}
}

type mockCollector struct {
	name    string
	metrics []collector.Metric
	err     error
}

func (m *mockCollector) Name() string {
	return m.name
}

func (m *mockCollector) Collect(ctx context.Context) ([]collector.Metric, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.metrics, nil
}
