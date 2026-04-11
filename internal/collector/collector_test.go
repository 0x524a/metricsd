package collector

import (
	"context"
	"fmt"
	"testing"
)

type mockCollector struct {
	name    string
	metrics []Metric
	err     error
}

func (m *mockCollector) Name() string { return m.name }
func (m *mockCollector) Collect(ctx context.Context) ([]Metric, error) {
	return m.metrics, m.err
}

func TestCollectAllParallel(t *testing.T) {
	t.Run("merges results from multiple collectors", func(t *testing.T) {
		r := NewRegistry()
		r.Register(&mockCollector{
			name:    "a",
			metrics: []Metric{{Name: "m1", Value: 1, Type: "gauge"}},
		})
		r.Register(&mockCollector{
			name:    "b",
			metrics: []Metric{{Name: "m2", Value: 2, Type: "gauge"}},
		})

		metrics, err := r.CollectAllParallel(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(metrics) != 2 {
			t.Errorf("expected 2 metrics, got %d", len(metrics))
		}
	})

	t.Run("failing collector does not block others", func(t *testing.T) {
		r := NewRegistry()
		r.Register(&mockCollector{
			name: "failing",
			err:  fmt.Errorf("broke"),
		})
		r.Register(&mockCollector{
			name:    "good",
			metrics: []Metric{{Name: "m1", Value: 1, Type: "gauge"}},
		})

		metrics, err := r.CollectAllParallel(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(metrics) != 1 {
			t.Errorf("expected 1 metric, got %d", len(metrics))
		}
	})

	t.Run("empty registry returns empty", func(t *testing.T) {
		r := NewRegistry()
		metrics, err := r.CollectAllParallel(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(metrics) != 0 {
			t.Errorf("expected 0 metrics, got %d", len(metrics))
		}
	})

	t.Run("all failing returns empty", func(t *testing.T) {
		r := NewRegistry()
		r.Register(&mockCollector{name: "f1", err: fmt.Errorf("err1")})
		r.Register(&mockCollector{name: "f2", err: fmt.Errorf("err2")})

		metrics, err := r.CollectAllParallel(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(metrics) != 0 {
			t.Errorf("expected 0 metrics, got %d", len(metrics))
		}
	})
}

func TestCollectAll(t *testing.T) {
	t.Run("sequential collection works", func(t *testing.T) {
		r := NewRegistry()
		r.Register(&mockCollector{
			name:    "a",
			metrics: []Metric{{Name: "m1", Value: 1, Type: "gauge"}},
		})
		r.Register(&mockCollector{
			name: "failing",
			err:  fmt.Errorf("broke"),
		})
		r.Register(&mockCollector{
			name:    "b",
			metrics: []Metric{{Name: "m2", Value: 2, Type: "gauge"}},
		})

		metrics, err := r.CollectAll(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(metrics) != 2 {
			t.Errorf("expected 2 metrics, got %d", len(metrics))
		}
	})
}
