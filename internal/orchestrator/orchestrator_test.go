package orchestrator

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/0x524A/metricsd/internal/collector"
)

// mockShipper implements shipper.Shipper for tests.
type mockShipper struct {
	mu      sync.Mutex
	shipped [][]collector.Metric
	err     error
}

func (m *mockShipper) Ship(_ context.Context, metrics []collector.Metric) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shipped = append(m.shipped, metrics)
	return m.err
}

func (m *mockShipper) Close() error { return nil }

func (m *mockShipper) calls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.shipped)
}

func (m *mockShipper) firstBatch() []collector.Metric {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.shipped) == 0 {
		return nil
	}
	return m.shipped[0]
}

// mockCollector implements collector.Collector for tests.
type mockCollector struct {
	name    string
	metrics []collector.Metric
	err     error
}

func (m *mockCollector) Name() string { return m.name }
func (m *mockCollector) Collect(_ context.Context) ([]collector.Metric, error) {
	return m.metrics, m.err
}

// TestNewOrchestrator verifies that NewOrchestrator sets fields correctly.
func TestNewOrchestrator(t *testing.T) {
	reg := collector.NewRegistry()
	shpr := &mockShipper{}
	interval := 5 * time.Second

	o := NewOrchestrator(reg, shpr, interval)

	if o == nil {
		t.Fatal("expected non-nil orchestrator")
	}
	if o.registry != reg {
		t.Error("registry field not set correctly")
	}
	if o.shipper != shpr {
		t.Error("shipper field not set correctly")
	}
	if o.interval != interval {
		t.Errorf("interval: got %v, want %v", o.interval, interval)
	}
	if o.stopChan == nil {
		t.Error("stopChan should be initialised")
	}
}

// TestCollectAndShipSuccess verifies that Start() drives collectAndShip and the
// shipper receives the expected metrics (user metrics + internal duration metric).
func TestCollectAndShipSuccess(t *testing.T) {
	userMetrics := []collector.Metric{
		{Name: "cpu_usage", Value: 42.0, Type: "gauge", Labels: map[string]string{"host": "a"}},
		{Name: "mem_usage", Value: 8192.0, Type: "gauge", Labels: map[string]string{"host": "a"}},
	}

	reg := collector.NewRegistry()
	reg.Register(&mockCollector{name: "test", metrics: userMetrics})

	shpr := &mockShipper{}

	o := NewOrchestrator(reg, shpr, 100*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	// Run Start in a goroutine; it returns when ctx expires.
	done := make(chan error, 1)
	go func() {
		done <- o.Start(ctx)
	}()

	// Wait for Start to finish (context deadline).
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after context cancellation")
	}

	// At 100 ms interval with 250 ms context we expect at least 2 Ship calls
	// (immediate + at least one ticker tick).
	if shpr.calls() < 1 {
		t.Fatalf("expected at least 1 Ship call, got %d", shpr.calls())
	}

	// Inspect the first batch.
	batch := shpr.firstBatch()
	if batch == nil {
		t.Fatal("first shipped batch is nil")
	}

	// Must contain the 2 user metrics plus metricsd_collection_duration_seconds.
	foundDuration := false
	userCount := 0
	for _, m := range batch {
		if m.Name == "metricsd_collection_duration_seconds" {
			foundDuration = true
		}
		for _, um := range userMetrics {
			if m.Name == um.Name {
				userCount++
			}
		}
	}

	if !foundDuration {
		t.Error("expected metricsd_collection_duration_seconds in shipped metrics")
	}
	if userCount != len(userMetrics) {
		t.Errorf("expected %d user metrics in first batch, found %d", len(userMetrics), userCount)
	}
	// Total must be at least len(userMetrics)+1 (duration metric).
	minExpected := len(userMetrics) + 1
	if len(batch) < minExpected {
		t.Errorf("expected at least %d metrics in batch, got %d", minExpected, len(batch))
	}
}

// TestStop verifies that Stop() causes Start() to return without waiting for
// the context to expire.
func TestStop(t *testing.T) {
	reg := collector.NewRegistry()
	shpr := &mockShipper{}

	// Use a very long interval so the test is not timing-sensitive.
	o := NewOrchestrator(reg, shpr, 10*time.Minute)

	ctx := context.Background() // no deadline — only Stop() should terminate Start.

	done := make(chan error, 1)
	go func() {
		done <- o.Start(ctx)
	}()

	// Give Start time to reach the select loop.
	time.Sleep(50 * time.Millisecond)

	o.Stop()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Start returned non-nil error after Stop: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after Stop()")
	}
}
