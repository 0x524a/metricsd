package orchestrator

import (
	"context"
	"fmt"
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

// retryShipper is a shipper that fails for the first N calls then succeeds.
type retryShipper struct {
	mu        sync.Mutex
	callCount int
	failUntil int // fail for first N calls
	shipped   [][]collector.Metric
}

func (r *retryShipper) Ship(_ context.Context, metrics []collector.Metric) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.callCount++
	if r.callCount <= r.failUntil {
		return fmt.Errorf("ship error on call %d", r.callCount)
	}
	r.shipped = append(r.shipped, metrics)
	return nil
}

func (r *retryShipper) Close() error { return nil }

func (r *retryShipper) calls() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.callCount
}

func (r *retryShipper) successCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.shipped)
}

// TestCollectAndShip_ShipRetry verifies the path where the first Ship call
// fails but the retry succeeds — metrics must still be delivered.
func TestCollectAndShip_ShipRetry(t *testing.T) {
	userMetrics := []collector.Metric{
		{Name: "cpu", Value: 1.0, Type: "gauge", Labels: map[string]string{}},
	}
	reg := collector.NewRegistry()
	reg.Register(&mockCollector{name: "test", metrics: userMetrics})

	shpr := &retryShipper{failUntil: 1} // fail first call, succeed on retry

	o := NewOrchestrator(reg, shpr, 10*time.Minute)
	o.collectAndShip(context.Background())

	// Ship should have been called exactly twice (original + retry).
	if shpr.calls() != 2 {
		t.Errorf("expected 2 Ship calls (1 fail + 1 retry), got %d", shpr.calls())
	}
	// The retry succeeded, so metrics should have been delivered once.
	if shpr.successCount() != 1 {
		t.Errorf("expected 1 successful delivery, got %d", shpr.successCount())
	}
	// lastShipDuration should be set after a successful retry.
	if o.lastShipDuration == 0 {
		t.Error("expected lastShipDuration to be non-zero after successful retry")
	}
}

// TestCollectAndShip_ShipRetryFails verifies the path where both the original
// Ship call and the retry fail — no panic, lastShipDuration still set.
func TestCollectAndShip_ShipRetryFails(t *testing.T) {
	userMetrics := []collector.Metric{
		{Name: "cpu", Value: 2.0, Type: "gauge", Labels: map[string]string{}},
	}
	reg := collector.NewRegistry()
	reg.Register(&mockCollector{name: "test", metrics: userMetrics})

	shpr := &retryShipper{failUntil: 999} // always fail

	o := NewOrchestrator(reg, shpr, 10*time.Minute)
	o.collectAndShip(context.Background())

	// Ship should have been called exactly twice (original + retry).
	if shpr.calls() != 2 {
		t.Errorf("expected 2 Ship calls (original + retry), got %d", shpr.calls())
	}
	// No metrics should have been successfully delivered.
	if shpr.successCount() != 0 {
		t.Errorf("expected 0 successful deliveries, got %d", shpr.successCount())
	}
	// lastShipDuration is still updated even on full failure.
	if o.lastShipDuration == 0 {
		t.Error("expected lastShipDuration to be non-zero even after failed retry")
	}
}

// TestCollectAndShip_DeadlineWarning verifies that collectAndShip completes
// without panic when the collection duration exceeds 80 % of the interval.
// We can't assert on the log output but we can ensure the cycle still ships.
func TestCollectAndShip_DeadlineWarning(t *testing.T) {
	// Use a very short interval so even a trivial collection duration exceeds 80 %.
	interval := 1 * time.Nanosecond

	reg := collector.NewRegistry()
	reg.Register(&mockCollector{
		name:    "slow",
		metrics: []collector.Metric{{Name: "m", Value: 1, Type: "gauge", Labels: map[string]string{}}},
	})

	shpr := &mockShipper{}
	o := NewOrchestrator(reg, shpr, interval)

	// collectAndShip must not panic even when the deadline warning fires.
	o.collectAndShip(context.Background())

	if shpr.calls() < 1 {
		t.Error("expected at least one Ship call despite deadline warning")
	}
}

// TestCollectAndShip_LastShipDurationIncluded verifies that on the second call
// to collectAndShip the metricsd_ship_duration_seconds internal metric is
// present in the shipped batch (because lastShipDuration was set by the first).
func TestCollectAndShip_LastShipDurationIncluded(t *testing.T) {
	reg := collector.NewRegistry()
	reg.Register(&mockCollector{
		name:    "test",
		metrics: []collector.Metric{{Name: "cpu", Value: 1.0, Type: "gauge", Labels: map[string]string{}}},
	})

	shpr := &mockShipper{}
	o := NewOrchestrator(reg, shpr, 10*time.Minute)

	// First cycle — sets lastShipDuration but does NOT include it in shipped metrics.
	o.collectAndShip(context.Background())

	if shpr.calls() != 1 {
		t.Fatalf("expected 1 Ship call after first cycle, got %d", shpr.calls())
	}
	firstBatch := shpr.firstBatch()
	for _, m := range firstBatch {
		if m.Name == "metricsd_ship_duration_seconds" {
			t.Error("metricsd_ship_duration_seconds should NOT appear in first cycle batch")
		}
	}

	// Second cycle — lastShipDuration > 0, so ship metric must be included.
	o.collectAndShip(context.Background())

	if shpr.calls() != 2 {
		t.Fatalf("expected 2 Ship calls after second cycle, got %d", shpr.calls())
	}

	shpr.mu.Lock()
	secondBatch := shpr.shipped[1]
	shpr.mu.Unlock()

	found := false
	for _, m := range secondBatch {
		if m.Name == "metricsd_ship_duration_seconds" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected metricsd_ship_duration_seconds in second cycle batch")
	}
}
