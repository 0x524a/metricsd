package shipper

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"

	"github.com/0x524A/metricsd/internal/collector"
)

// newTestPrometheusShipper is a helper that creates a shipper pointed at the
// given server URL with no TLS and a 5-second timeout.
func newTestPrometheusShipper(t *testing.T, serverURL string) *PrometheusRemoteWriteShipper {
	t.Helper()
	s, err := NewPrometheusRemoteWriteShipper(
		serverURL,
		false, // tlsEnabled
		"",    // certFile
		"",    // keyFile
		"",    // caFile
		false, // insecureSkipVerify
		5*time.Second,
	)
	if err != nil {
		t.Fatalf("NewPrometheusRemoteWriteShipper: %v", err)
	}
	return s
}

// decodeWriteRequest decompresses a Snappy-encoded body and unmarshals it into
// a prompb.WriteRequest, failing the test on any error.
func decodeWriteRequest(t *testing.T, body []byte) prompb.WriteRequest {
	t.Helper()
	decoded, err := snappy.Decode(nil, body)
	if err != nil {
		t.Fatalf("snappy.Decode: %v", err)
	}
	var wr prompb.WriteRequest
	if err := wr.Unmarshal(decoded); err != nil {
		t.Fatalf("prompb.WriteRequest.Unmarshal: %v", err)
	}
	return wr
}

// labelValue returns the value for the given label name inside a TimeSeries,
// or an empty string when the label is absent.
func labelValue(ts prompb.TimeSeries, name string) string {
	for _, l := range ts.Labels {
		if l.Name == name {
			return l.Value
		}
	}
	return ""
}

// TestPrometheusShipper_ShipSuccess verifies that Ship sends a correctly encoded
// Prometheus remote-write request with the expected headers, metric names,
// label values, and sample values.
func TestPrometheusShipper_ShipSuccess(t *testing.T) {
	var (
		capturedEncoding string
		capturedCT       string
		capturedVersion  string
		capturedBody     []byte
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedEncoding = r.Header.Get("Content-Encoding")
		capturedCT = r.Header.Get("Content-Type")
		capturedVersion = r.Header.Get("X-Prometheus-Remote-Write-Version")
		capturedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	s := newTestPrometheusShipper(t, srv.URL)

	metrics := []collector.Metric{
		{Name: "cpu_usage", Value: 42.5, Type: "gauge", Labels: map[string]string{"core": "0"}},
		{Name: "http_requests_total", Value: 1000.0, Type: "counter", Labels: map[string]string{"method": "GET", "code": "200"}},
	}

	if err := s.Ship(context.Background(), metrics); err != nil {
		t.Fatalf("Ship returned error: %v", err)
	}

	// --- header assertions ---
	if capturedEncoding != "snappy" {
		t.Errorf("Content-Encoding: want %q, got %q", "snappy", capturedEncoding)
	}
	if capturedCT != "application/x-protobuf" {
		t.Errorf("Content-Type: want %q, got %q", "application/x-protobuf", capturedCT)
	}
	if capturedVersion != "0.1.0" {
		t.Errorf("X-Prometheus-Remote-Write-Version: want %q, got %q", "0.1.0", capturedVersion)
	}

	// --- body assertions ---
	wr := decodeWriteRequest(t, capturedBody)

	if len(wr.Timeseries) != 2 {
		t.Fatalf("expected 2 timeseries, got %d", len(wr.Timeseries))
	}

	// Find each time series by __name__ label.
	tsMap := make(map[string]prompb.TimeSeries, 2)
	for _, ts := range wr.Timeseries {
		tsMap[labelValue(ts, "__name__")] = ts
	}

	// cpu_usage checks
	cpu, ok := tsMap["cpu_usage"]
	if !ok {
		t.Fatal("time series cpu_usage not found")
	}
	if got := labelValue(cpu, "core"); got != "0" {
		t.Errorf("cpu_usage label core: want %q, got %q", "0", got)
	}
	if len(cpu.Samples) != 1 || cpu.Samples[0].Value != 42.5 {
		t.Errorf("cpu_usage sample value: want 42.5, got %v", cpu.Samples)
	}
	if cpu.Samples[0].Timestamp <= 0 {
		t.Errorf("cpu_usage sample timestamp should be positive, got %d", cpu.Samples[0].Timestamp)
	}

	// http_requests_total checks
	reqs, ok := tsMap["http_requests_total"]
	if !ok {
		t.Fatal("time series http_requests_total not found")
	}
	if got := labelValue(reqs, "method"); got != "GET" {
		t.Errorf("http_requests_total label method: want %q, got %q", "GET", got)
	}
	if got := labelValue(reqs, "code"); got != "200" {
		t.Errorf("http_requests_total label code: want %q, got %q", "200", got)
	}
	if len(reqs.Samples) != 1 || reqs.Samples[0].Value != 1000.0 {
		t.Errorf("http_requests_total sample value: want 1000.0, got %v", reqs.Samples)
	}
}

// TestPrometheusShipper_ShipEmpty verifies that an empty metrics slice returns
// nil without making any HTTP request.
func TestPrometheusShipper_ShipEmpty(t *testing.T) {
	requestReceived := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestReceived = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := newTestPrometheusShipper(t, srv.URL)

	if err := s.Ship(context.Background(), []collector.Metric{}); err != nil {
		t.Errorf("expected nil error for empty metrics, got: %v", err)
	}
	if requestReceived {
		t.Error("expected no HTTP request for empty metrics slice")
	}
}

// TestPrometheusShipper_ServerError verifies that a non-2xx response causes
// Ship to return a non-nil error.
func TestPrometheusShipper_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	s := newTestPrometheusShipper(t, srv.URL)

	metrics := []collector.Metric{
		{Name: "cpu_usage", Value: 10.0, Type: "gauge"},
	}

	err := s.Ship(context.Background(), metrics)
	if err == nil {
		t.Error("expected error for 500 response, got nil")
	}
}

// TestPrometheusShipper_ConvertToTimeSeries exercises convertToTimeSeries
// directly, verifying __name__ labels, custom labels, sample values, and that
// timestamps are non-zero.
func TestPrometheusShipper_ConvertToTimeSeries(t *testing.T) {
	s := &PrometheusRemoteWriteShipper{}

	metrics := []collector.Metric{
		{Name: "mem_used_bytes", Value: 8589934592.0, Type: "gauge", Labels: map[string]string{"host": "server1"}},
		{Name: "disk_reads_total", Value: 42.0, Type: "counter", Labels: map[string]string{"device": "sda", "type": "read"}},
		{Name: "uptime_seconds", Value: 3600.0, Type: "gauge", Labels: map[string]string{}},
	}

	ts := s.convertToTimeSeries(metrics)

	if len(ts) != 3 {
		t.Fatalf("expected 3 timeseries, got %d", len(ts))
	}

	nameMap := make(map[string]prompb.TimeSeries, 3)
	for _, t2 := range ts {
		nameMap[labelValue(t2, "__name__")] = t2
	}

	// mem_used_bytes
	mem, ok := nameMap["mem_used_bytes"]
	if !ok {
		t.Fatal("mem_used_bytes not found in converted timeseries")
	}
	if got := labelValue(mem, "host"); got != "server1" {
		t.Errorf("mem_used_bytes label host: want %q, got %q", "server1", got)
	}
	if len(mem.Samples) != 1 {
		t.Fatalf("mem_used_bytes: expected 1 sample, got %d", len(mem.Samples))
	}
	if mem.Samples[0].Value != 8589934592.0 {
		t.Errorf("mem_used_bytes sample value: want 8589934592, got %v", mem.Samples[0].Value)
	}
	if mem.Samples[0].Timestamp <= 0 {
		t.Errorf("mem_used_bytes sample timestamp should be positive, got %d", mem.Samples[0].Timestamp)
	}

	// disk_reads_total
	disk, ok := nameMap["disk_reads_total"]
	if !ok {
		t.Fatal("disk_reads_total not found in converted timeseries")
	}
	if got := labelValue(disk, "device"); got != "sda" {
		t.Errorf("disk_reads_total label device: want %q, got %q", "sda", got)
	}
	if got := labelValue(disk, "type"); got != "read" {
		t.Errorf("disk_reads_total label type: want %q, got %q", "read", got)
	}
	if disk.Samples[0].Value != 42.0 {
		t.Errorf("disk_reads_total sample value: want 42, got %v", disk.Samples[0].Value)
	}

	// uptime_seconds (no extra labels)
	up, ok := nameMap["uptime_seconds"]
	if !ok {
		t.Fatal("uptime_seconds not found in converted timeseries")
	}
	if up.Samples[0].Value != 3600.0 {
		t.Errorf("uptime_seconds sample value: want 3600, got %v", up.Samples[0].Value)
	}
}

// TestPrometheusShipper_Close verifies that Close does not panic and returns nil.
func TestPrometheusShipper_Close(t *testing.T) {
	s := newTestPrometheusShipper(t, "http://127.0.0.1:9999")
	if err := s.Close(); err != nil {
		t.Errorf("Close() returned unexpected error: %v", err)
	}
}

// TestConvertToPrometheusMetrics verifies that the exported wrapper function
// returns a non-empty slice for valid gauge and counter inputs.
func TestConvertToPrometheusMetrics(t *testing.T) {
	metrics := []collector.Metric{
		{Name: "cpu_usage", Value: 55.0, Type: "gauge", Labels: map[string]string{"core": "0"}},
		{Name: "requests_total", Value: 200.0, Type: "counter", Labels: map[string]string{"status": "ok"}},
	}

	result := ConvertToPrometheusMetrics(metrics)
	if len(result) == 0 {
		t.Error("expected non-empty result from ConvertToPrometheusMetrics")
	}
}

// TestNewPrometheusRemoteWriteShipper_NoTLS verifies that the constructor succeeds with TLS disabled.
func TestNewPrometheusRemoteWriteShipper_NoTLS(t *testing.T) {
	s, err := NewPrometheusRemoteWriteShipper("http://localhost:9999", false, "", "", "", false, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if s == nil {
		t.Fatal("expected non-nil shipper")
	}
	s.Close()
}

// TestNewPrometheusRemoteWriteShipper_WithTLS verifies that the constructor succeeds with a valid self-signed cert.
func TestNewPrometheusRemoteWriteShipper_WithTLS(t *testing.T) {
	certFile, keyFile, cleanup := generateTestCert(t)
	defer cleanup()
	s, err := NewPrometheusRemoteWriteShipper("https://localhost:9999", true, certFile, keyFile, "", false, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
}

// TestNewPrometheusRemoteWriteShipper_BadCert verifies that a missing cert/key pair causes an error.
func TestNewPrometheusRemoteWriteShipper_BadCert(t *testing.T) {
	_, err := NewPrometheusRemoteWriteShipper("https://localhost:9999", true, "/nonexistent", "/nonexistent", "", false, 5*time.Second)
	if err == nil {
		t.Error("expected error for bad cert")
	}
}
