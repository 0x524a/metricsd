package shipper

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/0x524A/metricsd/internal/collector"
)

func newTestHTTPJSONShipper(t *testing.T, serverURL string) *HTTPJSONShipper {
	t.Helper()
	s, err := NewHTTPJSONShipper(
		serverURL,
		false, // tlsEnabled
		"",    // certFile
		"",    // keyFile
		"",    // caFile
		false, // insecureSkipVerify
		5*time.Second,
	)
	if err != nil {
		t.Fatalf("NewHTTPJSONShipper: %v", err)
	}
	return s
}

// TestHTTPJSONShipper_ShipSuccess verifies that a successful POST sets the correct
// Content-Type and sends a payload with a timestamp and a non-empty metrics array.
func TestHTTPJSONShipper_ShipSuccess(t *testing.T) {
	var (
		capturedCT   string
		capturedBody []byte
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCT = r.Header.Get("Content-Type")
		capturedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := newTestHTTPJSONShipper(t, srv.URL)

	metrics := []collector.Metric{
		{Name: "cpu_usage", Value: 55.0, Type: "gauge", Labels: map[string]string{"core": "1"}},
		{Name: "disk_io", Value: 1024.0, Type: "counter", Labels: map[string]string{"device": "sda"}},
	}

	err := s.Ship(context.Background(), metrics)
	if err != nil {
		t.Fatalf("Ship returned error: %v", err)
	}

	// Verify Content-Type
	if capturedCT != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", capturedCT)
	}

	// Verify payload structure
	var payload MetricPayload
	if err := json.Unmarshal(capturedBody, &payload); err != nil {
		t.Fatalf("failed to parse response body as MetricPayload: %v", err)
	}

	if payload.Timestamp <= 0 {
		t.Errorf("expected positive timestamp, got %d", payload.Timestamp)
	}
	if len(payload.Metrics) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(payload.Metrics))
	}

	// Spot-check first metric
	found := false
	for _, m := range payload.Metrics {
		if m.Name == "cpu_usage" {
			found = true
			if m.Value != 55.0 {
				t.Errorf("expected cpu_usage value 55.0, got %v", m.Value)
			}
			if m.Type != "gauge" {
				t.Errorf("expected type 'gauge', got %q", m.Type)
			}
			if m.Labels["core"] != "1" {
				t.Errorf("expected label core='1', got %q", m.Labels["core"])
			}
		}
	}
	if !found {
		t.Error("cpu_usage metric not found in payload")
	}
}

// TestHTTPJSONShipper_ShipEmptyMetrics verifies that an empty slice returns nil
// without performing any HTTP request.
func TestHTTPJSONShipper_ShipEmptyMetrics(t *testing.T) {
	requestReceived := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestReceived = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := newTestHTTPJSONShipper(t, srv.URL)

	err := s.Ship(context.Background(), []collector.Metric{})
	if err != nil {
		t.Errorf("expected nil error for empty metrics, got: %v", err)
	}
	if requestReceived {
		t.Error("expected no HTTP request for empty metrics slice")
	}
}

// TestHTTPJSONShipper_NaNInfFiltered verifies that metrics with NaN/Inf values are
// omitted from the shipped payload while valid metrics are included.
func TestHTTPJSONShipper_NaNInfFiltered(t *testing.T) {
	var capturedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := newTestHTTPJSONShipper(t, srv.URL)

	metrics := []collector.Metric{
		{Name: "valid_metric", Value: 7.0, Type: "gauge"},
		{Name: "nan_metric", Value: math.NaN(), Type: "gauge"},
		{Name: "inf_metric", Value: math.Inf(1), Type: "gauge"},
	}

	err := s.Ship(context.Background(), metrics)
	if err != nil {
		t.Fatalf("Ship returned error: %v", err)
	}

	var payload MetricPayload
	if err := json.Unmarshal(capturedBody, &payload); err != nil {
		t.Fatalf("failed to parse payload: %v", err)
	}

	if len(payload.Metrics) != 1 {
		t.Fatalf("expected 1 metric in payload (NaN/Inf filtered), got %d", len(payload.Metrics))
	}
	if payload.Metrics[0].Name != "valid_metric" {
		t.Errorf("expected valid_metric, got %q", payload.Metrics[0].Name)
	}
}

// TestHTTPJSONShipper_ServerError verifies that a non-2xx response causes Ship to
// return a non-nil error.
func TestHTTPJSONShipper_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	s := newTestHTTPJSONShipper(t, srv.URL)

	metrics := []collector.Metric{
		{Name: "mem_usage", Value: 80.0, Type: "gauge"},
	}

	err := s.Ship(context.Background(), metrics)
	if err == nil {
		t.Error("expected error for 503 response, got nil")
	}
}

// TestHTTPJSONShipper_Close verifies that Close does not panic and returns nil.
func TestHTTPJSONShipper_Close(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := newTestHTTPJSONShipper(t, srv.URL)
	if err := s.Close(); err != nil {
		t.Errorf("Close() returned unexpected error: %v", err)
	}
}

// TestNewHTTPJSONShipper_NoTLS verifies that the constructor succeeds with TLS disabled.
func TestNewHTTPJSONShipper_NoTLS(t *testing.T) {
	s, err := NewHTTPJSONShipper("http://localhost:9999", false, "", "", "", false, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if s == nil {
		t.Fatal("expected non-nil shipper")
	}
	s.Close()
}

// TestNewHTTPJSONShipper_WithTLS verifies that the constructor succeeds with a valid self-signed cert.
func TestNewHTTPJSONShipper_WithTLS(t *testing.T) {
	certFile, keyFile, cleanup := generateTestCert(t)
	defer cleanup()
	s, err := NewHTTPJSONShipper("https://localhost:9999", true, certFile, keyFile, "", false, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
}

// TestNewHTTPJSONShipper_BadCert verifies that a missing cert/key pair causes an error.
func TestNewHTTPJSONShipper_BadCert(t *testing.T) {
	_, err := NewHTTPJSONShipper("https://localhost:9999", true, "/nonexistent", "/nonexistent", "", false, 5*time.Second)
	if err == nil {
		t.Error("expected error for bad cert")
	}
}
