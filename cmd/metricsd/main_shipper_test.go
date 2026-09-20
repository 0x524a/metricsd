package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/0x524A/metricsd/internal/config"
)

func TestFormatTime(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected string
	}{
		{
			name:     "zero time returns empty string",
			input:    time.Time{},
			expected: "",
		},
		{
			name:     "non-zero time returns RFC3339 format",
			input:    time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC),
			expected: "2024-01-15T10:30:45Z",
		},
		{
			name:     "time with different timezone",
			input:    time.Date(2024, 6, 20, 14, 22, 33, 0, time.FixedZone("EST", -5*3600)),
			expected: "2024-06-20T14:22:33-05:00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTime(tt.input)
			if result != tt.expected {
				t.Errorf("formatTime(%v) = %q, expected %q", tt.input, result, tt.expected)
			}

			if result != "" {
				parsed, err := time.Parse(time.RFC3339, result)
				if err != nil {
					t.Errorf("result is not valid RFC3339: %v", err)
				}
				if !parsed.Equal(tt.input) {
					t.Errorf("parsed time %v does not match input %v", parsed, tt.input)
				}
			}
		})
	}
}

func TestSetupShipper_HTTPJson(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{
		Shipper: config.ShipperConfig{
			Type:     "http_json",
			Endpoint: srv.URL,
			Timeout:  5 * time.Second,
		},
	}

	shipper := setupShipper(cfg)
	if shipper == nil {
		t.Fatal("setupShipper returned nil shipper for http_json")
	}
	if err := shipper.Close(); err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}

func TestSetupShipper_PrometheusRemoteWrite(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{
		Shipper: config.ShipperConfig{
			Type:     "prometheus_remote_write",
			Endpoint: srv.URL,
			Timeout:  5 * time.Second,
		},
	}

	shipper := setupShipper(cfg)
	if shipper == nil {
		t.Fatal("setupShipper returned nil shipper for prometheus_remote_write")
	}
	if err := shipper.Close(); err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}

func TestSetupShipper_SplunkHEC(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{
		Shipper: config.ShipperConfig{
			Type:     "splunk_hec",
			Endpoint: srv.URL,
			HECToken: "test-token-12345",
			Timeout:  5 * time.Second,
		},
	}

	shipper := setupShipper(cfg)
	if shipper == nil {
		t.Fatal("setupShipper returned nil shipper for splunk_hec")
	}
	if err := shipper.Close(); err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}

func TestSetupShipper_JsonFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := tmpDir + "/metrics.json"

	cfg := &config.Config{
		Shipper: config.ShipperConfig{
			Type: "json_file",
			File: config.FileShipperConfig{
				Path:      filePath,
				MaxSizeMB: 100,
				MaxFiles:  5,
				Format:    "single",
			},
		},
	}

	shipper := setupShipper(cfg)
	if shipper == nil {
		t.Fatal("setupShipper returned nil shipper for json_file")
	}
	if err := shipper.Close(); err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}

func TestSetupShipper_TimeoutDefaults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{
		Shipper: config.ShipperConfig{
			Type:     "http_json",
			Endpoint: srv.URL,
			Timeout:  0,
		},
	}

	shipper := setupShipper(cfg)
	if shipper == nil {
		t.Fatal("setupShipper returned nil shipper when timeout is 0")
	}
	if err := shipper.Close(); err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}
