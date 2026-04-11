package collector

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func newTestHTTPCollector(endpoints []EndpointConfig) *HTTPCollector {
	return NewHTTPCollector(endpoints, 5*time.Second)
}

// findMetric returns the first metric in the slice whose Name matches, or nil.
func findMetric(metrics []Metric, name string) *Metric {
	for i := range metrics {
		if metrics[i].Name == name {
			return &metrics[i]
		}
	}
	return nil
}

// metricNames returns a sorted slice of metric names for deterministic assertions.
func metricNames(metrics []Metric) []string {
	names := make([]string, len(metrics))
	for i, m := range metrics {
		names[i] = m.Name
	}
	sort.Strings(names)
	return names
}

// ---------------------------------------------------------------------------
// 1. Prometheus text format scrape
// ---------------------------------------------------------------------------

func TestHTTPCollector_PrometheusTextFormat(t *testing.T) {
	body := `# HELP go_goroutines Number of goroutines.
# TYPE go_goroutines gauge
go_goroutines 42
# HELP process_cpu_seconds_total Total CPU seconds.
# TYPE process_cpu_seconds_total counter
process_cpu_seconds_total 1.5
`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		w.Write([]byte(body))
	}))
	defer srv.Close()

	col := newTestHTTPCollector([]EndpointConfig{{Name: "test_prom", URL: srv.URL}})
	metrics, err := col.Collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(metrics) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(metrics))
	}

	goroutines := findMetric(metrics, "go_goroutines")
	if goroutines == nil {
		t.Fatal("metric go_goroutines not found")
	}
	if goroutines.Value != 42 {
		t.Errorf("go_goroutines: expected value 42, got %v", goroutines.Value)
	}
	if goroutines.Labels["endpoint"] != "test_prom" {
		t.Errorf("go_goroutines: expected endpoint label 'test_prom', got %q", goroutines.Labels["endpoint"])
	}

	cpu := findMetric(metrics, "process_cpu_seconds_total")
	if cpu == nil {
		t.Fatal("metric process_cpu_seconds_total not found")
	}
	if cpu.Value != 1.5 {
		t.Errorf("process_cpu_seconds_total: expected value 1.5, got %v", cpu.Value)
	}
}

// ---------------------------------------------------------------------------
// 2. JSON format scrape
// ---------------------------------------------------------------------------

func TestHTTPCollector_JSONFormat(t *testing.T) {
	body := `{"cpu_usage": 42.5, "memory_used": 8589934592}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
	defer srv.Close()

	col := newTestHTTPCollector([]EndpointConfig{{Name: "myapp", URL: srv.URL}})
	metrics, err := col.Collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(metrics) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(metrics))
	}

	cpu := findMetric(metrics, "app_cpu_usage")
	if cpu == nil {
		t.Fatal("metric app_cpu_usage not found")
	}
	if cpu.Value != 42.5 {
		t.Errorf("app_cpu_usage: expected 42.5, got %v", cpu.Value)
	}
	if cpu.Labels["endpoint"] != "myapp" {
		t.Errorf("app_cpu_usage: expected endpoint 'myapp', got %q", cpu.Labels["endpoint"])
	}

	mem := findMetric(metrics, "app_memory_used")
	if mem == nil {
		t.Fatal("metric app_memory_used not found")
	}
	if mem.Value != 8589934592 {
		t.Errorf("app_memory_used: expected 8589934592, got %v", mem.Value)
	}
}

// ---------------------------------------------------------------------------
// 3. Auto-detect Prometheus vs JSON via isPrometheusFormat
// ---------------------------------------------------------------------------

func TestIsPrometheusFormat(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "prometheus with HELP comment",
			input: "# HELP go_goroutines Number of goroutines.\ngo_goroutines 5\n",
			want:  true,
		},
		{
			name:  "prometheus metric without comments",
			input: "go_goroutines 5\n",
			want:  true,
		},
		{
			name:  "prometheus metric with labels",
			input: `http_requests_total{method="GET",status="200"} 1234` + "\n",
			want:  true,
		},
		{
			name:  "json object",
			input: `{"cpu_usage": 42.5}`,
			want:  false,
		},
		{
			name:  "json array",
			input: `[1, 2, 3]`,
			want:  false,
		},
		{
			name:  "empty string",
			input: "",
			want:  false,
		},
		{
			name:  "html content",
			input: "<html><body>hello</body></html>",
			want:  false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isPrometheusFormat([]byte(tc.input))
			if got != tc.want {
				t.Errorf("isPrometheusFormat(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 4. Prometheus with labels
// ---------------------------------------------------------------------------

func TestHTTPCollector_PrometheusWithLabels(t *testing.T) {
	body := `http_requests_total{method="GET",status="200"} 1234` + "\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}))
	defer srv.Close()

	col := newTestHTTPCollector([]EndpointConfig{{Name: "api", URL: srv.URL}})
	metrics, err := col.Collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}

	m := metrics[0]
	if m.Name != "http_requests_total" {
		t.Errorf("expected name 'http_requests_total', got %q", m.Name)
	}
	if m.Value != 1234 {
		t.Errorf("expected value 1234, got %v", m.Value)
	}
	if m.Labels["method"] != "GET" {
		t.Errorf("expected method label 'GET', got %q", m.Labels["method"])
	}
	if m.Labels["status"] != "200" {
		t.Errorf("expected status label '200', got %q", m.Labels["status"])
	}
	if m.Labels["endpoint"] != "api" {
		t.Errorf("expected endpoint label 'api', got %q", m.Labels["endpoint"])
	}
}

// ---------------------------------------------------------------------------
// 5. Empty response
// ---------------------------------------------------------------------------

func TestHTTPCollector_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// write nothing
	}))
	defer srv.Close()

	col := newTestHTTPCollector([]EndpointConfig{{Name: "empty", URL: srv.URL}})
	metrics, err := col.Collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(metrics) != 0 {
		t.Errorf("expected 0 metrics for empty response, got %d", len(metrics))
	}
}

// ---------------------------------------------------------------------------
// 6. Server error (500)
// ---------------------------------------------------------------------------

func TestHTTPCollector_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	// scrapeEndpoint should return an error; Collect swallows it and returns empty slice.
	col := newTestHTTPCollector([]EndpointConfig{{Name: "failing", URL: srv.URL}})
	metrics, err := col.Collect(context.Background())

	// Collect never propagates endpoint errors — it logs and continues.
	if err != nil {
		t.Fatalf("Collect should not return an error, got: %v", err)
	}
	if len(metrics) != 0 {
		t.Errorf("expected 0 metrics on 500 response, got %d", len(metrics))
	}

	// Verify scrapeEndpoint itself returns the error.
	scrapeErr := func() error {
		_, e := col.scrapeEndpoint(context.Background(), EndpointConfig{Name: "failing", URL: srv.URL})
		return e
	}()
	if scrapeErr == nil {
		t.Error("expected scrapeEndpoint to return an error for 500 status, got nil")
	}
}

// ---------------------------------------------------------------------------
// 7. Invalid response (not JSON or Prometheus)
// ---------------------------------------------------------------------------

func TestHTTPCollector_InvalidResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html><body><h1>Hello</h1></body></html>"))
	}))
	defer srv.Close()

	col := newTestHTTPCollector([]EndpointConfig{{Name: "html_srv", URL: srv.URL}})

	// Collect swallows the error; verify via scrapeEndpoint directly.
	_, err := col.scrapeEndpoint(context.Background(), EndpointConfig{Name: "html_srv", URL: srv.URL})
	if err == nil {
		t.Error("expected an error for HTML response, got nil")
	}

	// Collect should return empty and no error.
	metrics, collectErr := col.Collect(context.Background())
	if collectErr != nil {
		t.Fatalf("Collect should not propagate the error, got: %v", collectErr)
	}
	if len(metrics) != 0 {
		t.Errorf("expected 0 metrics for HTML response, got %d", len(metrics))
	}
}

// ---------------------------------------------------------------------------
// 8. Multiple endpoints
// ---------------------------------------------------------------------------

func TestHTTPCollector_MultipleEndpoints(t *testing.T) {
	srv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("node_cpu_usage 0.75\n"))
	}))
	defer srv1.Close()

	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"req_rate": 120.0}`))
	}))
	defer srv2.Close()

	col := newTestHTTPCollector([]EndpointConfig{
		{Name: "node1", URL: srv1.URL},
		{Name: "node2", URL: srv2.URL},
	})

	metrics, err := col.Collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(metrics) != 2 {
		t.Fatalf("expected 2 metrics (one per endpoint), got %d: %v", len(metrics), metricNames(metrics))
	}

	cpuMetric := findMetric(metrics, "node_cpu_usage")
	if cpuMetric == nil {
		t.Fatal("metric node_cpu_usage not found")
	}
	if cpuMetric.Labels["endpoint"] != "node1" {
		t.Errorf("node_cpu_usage: expected endpoint 'node1', got %q", cpuMetric.Labels["endpoint"])
	}
	if cpuMetric.Value != 0.75 {
		t.Errorf("node_cpu_usage: expected 0.75, got %v", cpuMetric.Value)
	}

	reqMetric := findMetric(metrics, "app_req_rate")
	if reqMetric == nil {
		t.Fatal("metric app_req_rate not found")
	}
	if reqMetric.Labels["endpoint"] != "node2" {
		t.Errorf("app_req_rate: expected endpoint 'node2', got %q", reqMetric.Labels["endpoint"])
	}
	if reqMetric.Value != 120.0 {
		t.Errorf("app_req_rate: expected 120.0, got %v", reqMetric.Value)
	}
}

// ---------------------------------------------------------------------------
// 9. HTTPCollector.Name
// ---------------------------------------------------------------------------

func TestHTTPCollector_Name(t *testing.T) {
	c := NewHTTPCollector(nil, 5*time.Second)
	if c.Name() != "http" {
		t.Errorf("expected Name() == \"http\", got %q", c.Name())
	}
}

// ---------------------------------------------------------------------------
// 10. parseMetrics — all type-switch branches
// ---------------------------------------------------------------------------

// TestParseMetrics_AllTypes exercises the int, int64, int32 and float32
// branches of parseMetrics, which are not reached through the HTTP path
// because JSON always deserialises numbers as float64.
func TestParseMetrics_AllTypes(t *testing.T) {
	col := NewHTTPCollector(nil, 5*time.Second)

	rawMetrics := map[string]interface{}{
		"int_val":     int(7),
		"int64_val":   int64(8),
		"int32_val":   int32(9),
		"float32_val": float32(3.14),
		"float64_val": float64(2.71),
		"string_val":  "skip_me", // should be ignored
	}

	metrics := col.parseMetrics("test_ep", rawMetrics)

	// 5 numeric keys → 5 metrics (the string should be skipped).
	if len(metrics) != 5 {
		t.Fatalf("expected 5 metrics, got %d", len(metrics))
	}

	find := func(name string) *Metric {
		for i := range metrics {
			if metrics[i].Name == "app_"+name {
				return &metrics[i]
			}
		}
		return nil
	}

	cases := []struct {
		key  string
		want float64
	}{
		{"int_val", 7},
		{"int64_val", 8},
		{"int32_val", 9},
	}
	for _, tc := range cases {
		m := find(tc.key)
		if m == nil {
			t.Errorf("metric app_%s not found", tc.key)
			continue
		}
		if m.Value != tc.want {
			t.Errorf("app_%s: expected %v, got %v", tc.key, tc.want, m.Value)
		}
		if m.Labels["endpoint"] != "test_ep" {
			t.Errorf("app_%s: expected endpoint label 'test_ep', got %q", tc.key, m.Labels["endpoint"])
		}
	}

	// float32 loses some precision when converted; just check it's close.
	f32m := find("float32_val")
	if f32m == nil {
		t.Error("metric app_float32_val not found")
	} else if f32m.Value < 3.0 || f32m.Value > 4.0 {
		t.Errorf("app_float32_val: unexpected value %v", f32m.Value)
	}

	// string key must NOT appear.
	if find("string_val") != nil {
		t.Error("app_string_val should have been skipped")
	}
}

// ---------------------------------------------------------------------------
// 11. splitLabels — quoted values containing commas
// ---------------------------------------------------------------------------

func TestSplitLabels(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "two simple labels",
			input: `method="GET",status="200"`,
			want:  []string{`method="GET"`, `status="200"`},
		},
		{
			name:  "value with comma inside quotes",
			input: `env="prod,staging",region="us-east-1"`,
			want:  []string{`env="prod,staging"`, `region="us-east-1"`},
		},
		{
			name:  "single label",
			input: `job="node_exporter"`,
			want:  []string{`job="node_exporter"`},
		},
		{
			name:  "multiple commas inside quoted value",
			input: `tags="a,b,c",name="foo"`,
			want:  []string{`tags="a,b,c"`, `name="foo"`},
		},
		{
			name:  "empty string",
			input: "",
			want:  nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := splitLabels(tc.input)
			if len(got) != len(tc.want) {
				t.Fatalf("splitLabels(%q) = %v (len %d), want %v (len %d)",
					tc.input, got, len(got), tc.want, len(tc.want))
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("splitLabels(%q)[%d] = %q, want %q", tc.input, i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestParsePrometheusLine_EdgeCases(t *testing.T) {
	c := &HTTPCollector{}

	t.Run("no value field", func(t *testing.T) {
		result := c.parsePrometheusLine("test", "metric_name_only")
		if result != nil {
			t.Error("expected nil for line with no value")
		}
	})

	t.Run("unparseable value", func(t *testing.T) {
		result := c.parsePrometheusLine("test", "metric_name notanumber")
		if result != nil {
			t.Error("expected nil for non-numeric value")
		}
	})

	t.Run("malformed labels no closing brace", func(t *testing.T) {
		result := c.parsePrometheusLine("test", "metric{label=\"value\" 123")
		if result != nil {
			t.Error("expected nil for malformed labels")
		}
	})

	t.Run("labels with no value after brace", func(t *testing.T) {
		result := c.parsePrometheusLine("test", "metric{label=\"value\"}")
		if result != nil {
			t.Error("expected nil for labels with no value")
		}
	})
}
