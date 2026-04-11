package collector

import (
	"strings"
	"testing"

	dto "github.com/prometheus/client_model/go"
)

func TestToPrometheusMetrics(t *testing.T) {
	t.Run("gauge metric", func(t *testing.T) {
		input := []Metric{
			{
				Name:   "test_gauge_metric",
				Labels: map[string]string{},
				Value:  3.14,
				Type:   "gauge",
			},
		}
		result := ToPrometheusMetrics(input)
		if len(result) != 1 {
			t.Fatalf("expected 1 prometheus metric, got %d", len(result))
		}
		var pb dto.Metric
		if err := result[0].Write(&pb); err != nil {
			t.Fatalf("Write() failed: %v", err)
		}
		if pb.Gauge == nil {
			t.Fatal("expected a gauge metric, got nil gauge")
		}
		if got := pb.Gauge.GetValue(); got != 3.14 {
			t.Errorf("gauge value = %v, want 3.14", got)
		}
	})

	t.Run("counter metric", func(t *testing.T) {
		input := []Metric{
			{
				Name:   "test_counter_total",
				Labels: map[string]string{},
				Value:  42.0,
				Type:   "counter",
			},
		}
		result := ToPrometheusMetrics(input)
		if len(result) != 1 {
			t.Fatalf("expected 1 prometheus metric, got %d", len(result))
		}
		var pb dto.Metric
		if err := result[0].Write(&pb); err != nil {
			t.Fatalf("Write() failed: %v", err)
		}
		if pb.Counter == nil {
			t.Fatal("expected a counter metric, got nil counter")
		}
		if got := pb.Counter.GetValue(); got != 42.0 {
			t.Errorf("counter value = %v, want 42.0", got)
		}
	})

	t.Run("metric with labels", func(t *testing.T) {
		input := []Metric{
			{
				Name:   "test_labeled_metric",
				Labels: map[string]string{"env": "prod", "region": "us-east-1"},
				Value:  7.0,
				Type:   "gauge",
			},
		}
		result := ToPrometheusMetrics(input)
		if len(result) != 1 {
			t.Fatalf("expected 1 prometheus metric, got %d", len(result))
		}
		var pb dto.Metric
		if err := result[0].Write(&pb); err != nil {
			t.Fatalf("Write() failed: %v", err)
		}
		if len(pb.Label) != 2 {
			t.Fatalf("expected 2 labels, got %d", len(pb.Label))
		}
		labelMap := make(map[string]string, len(pb.Label))
		for _, lp := range pb.Label {
			labelMap[lp.GetName()] = lp.GetValue()
		}
		if labelMap["env"] != "prod" {
			t.Errorf("label env = %q, want %q", labelMap["env"], "prod")
		}
		if labelMap["region"] != "us-east-1" {
			t.Errorf("label region = %q, want %q", labelMap["region"], "us-east-1")
		}
	})

	t.Run("multiple metrics correct count", func(t *testing.T) {
		input := []Metric{
			{Name: "metric_a", Labels: map[string]string{}, Value: 1.0, Type: "gauge"},
			{Name: "metric_b", Labels: map[string]string{}, Value: 2.0, Type: "counter"},
			{Name: "metric_c", Labels: map[string]string{"k": "v"}, Value: 3.0, Type: "gauge"},
		}
		result := ToPrometheusMetrics(input)
		if len(result) != 3 {
			t.Errorf("expected 3 prometheus metrics, got %d", len(result))
		}
	})

	t.Run("unknown type defaults to gauge", func(t *testing.T) {
		input := []Metric{
			{
				Name:   "test_unknown_type",
				Labels: map[string]string{},
				Value:  1.5,
				Type:   "histogram", // not gauge or counter
			},
		}
		result := ToPrometheusMetrics(input)
		if len(result) != 1 {
			t.Fatalf("expected 1 prometheus metric, got %d", len(result))
		}
		var pb dto.Metric
		if err := result[0].Write(&pb); err != nil {
			t.Fatalf("Write() failed: %v", err)
		}
		// ToPrometheusMetrics defaults non-counter types to gauge
		if pb.Gauge == nil {
			t.Error("expected gauge for unknown type, got nil gauge")
		}
	})
}

func TestToPrometheusMetrics_Empty(t *testing.T) {
	result := ToPrometheusMetrics([]Metric{})
	if result == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(result) != 0 {
		t.Errorf("expected 0 prometheus metrics, got %d", len(result))
	}
}

// TestToPrometheusMetrics_DescString verifies that a converted metric's
// descriptor string contains the original metric name.
func TestToPrometheusMetrics_DescString(t *testing.T) {
	input := []Metric{
		{Name: "my_special_metric", Labels: map[string]string{}, Value: 9.9, Type: "gauge"},
	}
	result := ToPrometheusMetrics(input)
	if len(result) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(result))
	}
	desc := result[0].Desc().String()
	if !strings.Contains(desc, "my_special_metric") {
		t.Errorf("descriptor string %q does not contain metric name 'my_special_metric'", desc)
	}
}
