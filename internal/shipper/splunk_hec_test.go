package shipper

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/0x524A/metricsd/internal/collector"
)

func newTestSplunkShipper(t *testing.T, serverURL string) *SplunkHECShipper {
	t.Helper()
	s, err := NewSplunkHECShipper(
		serverURL,
		"test-token",
		false, // tlsEnabled
		"",    // certFile
		"",    // keyFile
		"",    // caFile
		false, // insecureSkipVerify
		5*time.Second,
		"", // debugLogFile
	)
	if err != nil {
		t.Fatalf("NewSplunkHECShipper: %v", err)
	}
	return s
}

// TestSplunkHECShipper_ShipSuccess verifies that a successful POST reaches the
// correct path, carries the expected headers, and contains well-formed JSON events.
func TestSplunkHECShipper_ShipSuccess(t *testing.T) {
	var (
		capturedPath   string
		capturedAuth   string
		capturedCT     string
		capturedBody   []byte
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedAuth = r.Header.Get("Authorization")
		capturedCT = r.Header.Get("Content-Type")
		capturedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := newTestSplunkShipper(t, srv.URL)

	metrics := []collector.Metric{
		{Name: "cpu_usage", Value: 42.5, Type: "gauge", Labels: map[string]string{"core": "0"}},
		{Name: "mem_used", Value: 1024.0, Type: "gauge", Labels: map[string]string{"host": "localhost"}},
	}

	err := s.Ship(context.Background(), metrics)
	if err != nil {
		t.Fatalf("Ship returned error: %v", err)
	}

	// Verify endpoint path
	if capturedPath != "/services/collector/event" {
		t.Errorf("expected path /services/collector/event, got %q", capturedPath)
	}

	// Verify Authorization header
	expectedAuth := "Splunk test-token"
	if capturedAuth != expectedAuth {
		t.Errorf("expected Authorization %q, got %q", expectedAuth, capturedAuth)
	}

	// Verify Content-Type
	if capturedCT != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", capturedCT)
	}

	// Body is newline-delimited JSON — parse each line
	lines := strings.Split(strings.TrimRight(string(capturedBody), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 JSON lines, got %d", len(lines))
	}

	for i, line := range lines {
		var event SplunkHECEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Errorf("line %d: invalid JSON: %v", i, err)
			continue
		}
		if event.Time <= 0 {
			t.Errorf("line %d: expected positive time, got %v", i, event.Time)
		}
		if event.Source != "metricsd" {
			t.Errorf("line %d: expected source 'metricsd', got %q", i, event.Source)
		}
		if _, ok := event.Event["metric_name"]; !ok {
			t.Errorf("line %d: missing metric_name in event", i)
		}
		if _, ok := event.Event["_value"]; !ok {
			t.Errorf("line %d: missing _value in event", i)
		}
	}
}

// TestSplunkHECShipper_ShipEmptyMetrics verifies that an empty slice returns nil
// without performing any HTTP request.
func TestSplunkHECShipper_ShipEmptyMetrics(t *testing.T) {
	requestReceived := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestReceived = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := newTestSplunkShipper(t, srv.URL)

	err := s.Ship(context.Background(), []collector.Metric{})
	if err != nil {
		t.Errorf("expected nil error for empty metrics, got: %v", err)
	}
	if requestReceived {
		t.Error("expected no HTTP request for empty metrics slice")
	}
}

// TestSplunkHECShipper_ShipNaNInf verifies that metrics with NaN/Inf values are
// skipped while remaining valid metrics are still shipped successfully.
func TestSplunkHECShipper_ShipNaNInf(t *testing.T) {
	var capturedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := newTestSplunkShipper(t, srv.URL)

	metrics := []collector.Metric{
		{Name: "valid_metric", Value: 99.9, Type: "gauge"},
		{Name: "nan_metric", Value: math.NaN(), Type: "gauge"},
		{Name: "inf_metric", Value: math.Inf(1), Type: "gauge"},
		{Name: "neg_inf_metric", Value: math.Inf(-1), Type: "gauge"},
	}

	err := s.Ship(context.Background(), metrics)
	if err != nil {
		t.Fatalf("Ship returned error: %v", err)
	}

	// Only the valid metric should appear in the body
	lines := strings.Split(strings.TrimRight(string(capturedBody), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected exactly 1 JSON line (valid metric only), got %d", len(lines))
	}

	var event SplunkHECEvent
	if err := json.Unmarshal([]byte(lines[0]), &event); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if event.Event["metric_name"] != "valid_metric" {
		t.Errorf("expected metric_name 'valid_metric', got %v", event.Event["metric_name"])
	}
}

// TestSplunkHECShipper_ServerError verifies that a non-2xx response causes Ship
// to return a non-nil error.
func TestSplunkHECShipper_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	s := newTestSplunkShipper(t, srv.URL)

	metrics := []collector.Metric{
		{Name: "cpu_usage", Value: 10.0, Type: "gauge"},
	}

	err := s.Ship(context.Background(), metrics)
	if err == nil {
		t.Error("expected error for 500 response, got nil")
	}
}

// TestNewSplunkHECShipper_EmptyEndpoint verifies that providing an empty endpoint
// causes NewSplunkHECShipper to return an error.
func TestNewSplunkHECShipper_EmptyEndpoint(t *testing.T) {
	_, err := NewSplunkHECShipper(
		"",
		"test-token",
		false,
		"", "", "",
		false,
		5*time.Second,
		"",
	)
	if err == nil {
		t.Error("expected error for empty endpoint, got nil")
	}
}

// TestSplunkHECShipper_Close verifies that Close does not panic and returns nil.
func TestSplunkHECShipper_Close(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := newTestSplunkShipper(t, srv.URL)
	if err := s.Close(); err != nil {
		t.Errorf("Close() returned unexpected error: %v", err)
	}
}

// TestSplunkHECShipper_LogPayloadToFile verifies that, when debugLogFile is set,
// Ship writes a payload entry that contains the timestamp header and the
// metric_name of each shipped metric.
func TestSplunkHECShipper_LogPayloadToFile(t *testing.T) {
	// Spin up a server that always returns 200.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Create a temp file for the debug log.
	tmpFile, err := os.CreateTemp("", "splunk-debug-*.log")
	if err != nil {
		t.Fatalf("os.CreateTemp: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	// Build a shipper with the debug log path set.
	s, err := NewSplunkHECShipper(
		srv.URL,
		"test-token",
		false,
		"", "", "",
		false,
		5*time.Second,
		tmpFile.Name(),
	)
	if err != nil {
		t.Fatalf("NewSplunkHECShipper: %v", err)
	}

	metrics := []collector.Metric{
		{Name: "cpu_usage", Value: 77.0, Type: "gauge", Labels: map[string]string{"host": "box1"}},
	}

	if err := s.Ship(context.Background(), metrics); err != nil {
		t.Fatalf("Ship returned error: %v", err)
	}

	// Read the debug log and verify it contains the expected content.
	data, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("os.ReadFile: %v", err)
	}

	content := string(data)

	// The header written by logPayloadToFile should include the marker text.
	if !strings.Contains(content, "Splunk HEC Payload at") {
		t.Error("debug log missing timestamp header '=== Splunk HEC Payload at ...' ===")
	}
	// The payload itself should reference the metric name.
	if !strings.Contains(content, "cpu_usage") {
		t.Error("debug log missing metric name 'cpu_usage'")
	}
}
