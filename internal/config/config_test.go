package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// minimalValidConfig returns a Config struct with the minimum valid fields set.
func minimalValidConfig() Config {
	return Config{
		Server: ServerConfig{
			Port: 8080,
		},
		Collector: CollectorConfig{
			IntervalSeconds: 10,
		},
		Shipper: ShipperConfig{
			Type:     "http_json",
			Endpoint: "http://localhost:9000",
		},
	}
}

// writeTempJSON writes content to a temp file and returns its path.
func writeTempJSON(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}
	return path
}

// ---------------------------------------------------------------------------
// Load() tests
// ---------------------------------------------------------------------------

func TestLoad_ValidConfig(t *testing.T) {
	json := `{
		"server":    {"host": "0.0.0.0", "port": 9090},
		"collector": {"interval_seconds": 15, "enable_cpu": true, "enable_memory": true},
		"shipper":   {"type": "http_json", "endpoint": "http://example.com/metrics"}
	}`
	path := writeTempJSON(t, json)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Server.Host = %q, want %q", cfg.Server.Host, "0.0.0.0")
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("Server.Port = %d, want 9090", cfg.Server.Port)
	}
	if cfg.Collector.IntervalSeconds != 15 {
		t.Errorf("Collector.IntervalSeconds = %d, want 15", cfg.Collector.IntervalSeconds)
	}
	if !cfg.Collector.EnableCPU {
		t.Error("Collector.EnableCPU should be true")
	}
	if !cfg.Collector.EnableMemory {
		t.Error("Collector.EnableMemory should be true")
	}
	if cfg.Shipper.Type != "http_json" {
		t.Errorf("Shipper.Type = %q, want %q", cfg.Shipper.Type, "http_json")
	}
	if cfg.Shipper.Endpoint != "http://example.com/metrics" {
		t.Errorf("Shipper.Endpoint = %q, want %q", cfg.Shipper.Endpoint, "http://example.com/metrics")
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	path := writeTempJSON(t, `{not valid json`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() expected error for invalid JSON, got nil")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load("/nonexistent/path/to/config.json")
	if err == nil {
		t.Fatal("Load() expected error for missing file, got nil")
	}
}

// ---------------------------------------------------------------------------
// Validate() tests
// ---------------------------------------------------------------------------

func TestValidate_ValidShipperTypes(t *testing.T) {
	types := []struct {
		shipperType string
		endpoint    string
		filePath    string
		hecToken    string
	}{
		{"http_json", "http://localhost:9000", "", ""},
		{"prometheus_remote_write", "http://localhost:9090/api/v1/write", "", ""},
		{"json_file", "", "/tmp/metrics.json", ""},
		{"splunk_hec", "http://splunk:8088/services/collector", "", "mytoken"},
	}

	for _, tc := range types {
		t.Run(tc.shipperType, func(t *testing.T) {
			cfg := minimalValidConfig()
			cfg.Shipper.Type = tc.shipperType
			cfg.Shipper.Endpoint = tc.endpoint
			cfg.Shipper.File.Path = tc.filePath
			cfg.Shipper.HECToken = tc.hecToken

			if err := cfg.Validate(); err != nil {
				t.Errorf("Validate() unexpected error for type %q: %v", tc.shipperType, err)
			}
		})
	}
}

func TestValidate_InvalidPort(t *testing.T) {
	for _, port := range []int{0, 99999, -1} {
		t.Run("port_"+itoa(port), func(t *testing.T) {
			cfg := minimalValidConfig()
			cfg.Server.Port = port
			if err := cfg.Validate(); err == nil {
				t.Errorf("Validate() expected error for port %d, got nil", port)
			}
		})
	}
}

func TestValidate_InvalidInterval(t *testing.T) {
	cfg := minimalValidConfig()
	cfg.Collector.IntervalSeconds = 0
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() expected error for interval_seconds=0, got nil")
	}
}

func TestValidate_InvalidShipperType(t *testing.T) {
	cfg := minimalValidConfig()
	cfg.Shipper.Type = "unknown_type"
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() expected error for invalid shipper type, got nil")
	}
}

func TestValidate_JsonFileRequiresPath(t *testing.T) {
	cfg := minimalValidConfig()
	cfg.Shipper.Type = "json_file"
	cfg.Shipper.Endpoint = ""
	cfg.Shipper.File.Path = "" // deliberately empty
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() expected error when json_file has no file path, got nil")
	}
}

func TestValidate_SplunkHECRequiresToken(t *testing.T) {
	cfg := minimalValidConfig()
	cfg.Shipper.Type = "splunk_hec"
	cfg.Shipper.Endpoint = "http://splunk:8088/services/collector"
	cfg.Shipper.HECToken = "" // deliberately empty
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() expected error when splunk_hec has no hec_token, got nil")
	}
}

func TestValidate_TLSRequiresCertAndKey(t *testing.T) {
	tests := []struct {
		name     string
		certFile string
		keyFile  string
	}{
		{"missing_both", "", ""},
		{"missing_key", "cert.pem", ""},
		{"missing_cert", "", "key.pem"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := minimalValidConfig()
			cfg.Shipper.TLS.Enabled = true
			cfg.Shipper.TLS.CertFile = tc.certFile
			cfg.Shipper.TLS.KeyFile = tc.keyFile
			if err := cfg.Validate(); err == nil {
				t.Errorf("Validate() expected TLS error for cert=%q key=%q, got nil", tc.certFile, tc.keyFile)
			}
		})
	}
}

func TestValidate_PluginDefaultsApplied(t *testing.T) {
	cfg := minimalValidConfig()
	cfg.Collector.Plugins.Enabled = true
	cfg.Collector.Plugins.PluginsDir = "" // empty — should be defaulted to "plugins"

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	if cfg.Collector.Plugins.PluginsDir != "plugins" {
		t.Errorf("PluginsDir = %q, want %q", cfg.Collector.Plugins.PluginsDir, "plugins")
	}
	if cfg.Collector.Plugins.DefaultTimeoutSeconds != 30 {
		t.Errorf("DefaultTimeoutSeconds = %d, want 30", cfg.Collector.Plugins.DefaultTimeoutSeconds)
	}
}

// ---------------------------------------------------------------------------
// applyEnvOverrides tests
// ---------------------------------------------------------------------------

func TestApplyEnvOverrides_ServerPort(t *testing.T) {
	t.Setenv("MC_SERVER_PORT", "7777")

	cfg := minimalValidConfig()
	applyEnvOverrides(&cfg)

	if cfg.Server.Port != 7777 {
		t.Errorf("Server.Port = %d after env override, want 7777", cfg.Server.Port)
	}
}

// ---------------------------------------------------------------------------
// GetCollectionInterval tests
// ---------------------------------------------------------------------------

func TestGetCollectionInterval(t *testing.T) {
	cfg := minimalValidConfig()
	cfg.Collector.IntervalSeconds = 30

	got := cfg.GetCollectionInterval()
	want := 30 * time.Second
	if got != want {
		t.Errorf("GetCollectionInterval() = %v, want %v", got, want)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func TestApplyEnvOverrides_AllVars(t *testing.T) {
	t.Setenv("MC_SERVER_HOST", "10.0.0.1")
	t.Setenv("MC_COLLECTOR_INTERVAL", "42")
	t.Setenv("MC_SHIPPER_TYPE", "http_json")
	t.Setenv("MC_SHIPPER_ENDPOINT", "http://override.example.com/metrics")
	t.Setenv("MC_TLS_ENABLED", "true")
	t.Setenv("MC_TLS_CERT_FILE", "/etc/certs/client.crt")
	t.Setenv("MC_TLS_KEY_FILE", "/etc/certs/client.key")
	t.Setenv("MC_TLS_CA_FILE", "/etc/certs/ca.crt")
	t.Setenv("MC_HEC_TOKEN", "my-hec-token")
	t.Setenv("MC_PLUGINS_ENABLED", "true")
	t.Setenv("MC_PLUGINS_DIR", "/opt/plugins")
	t.Setenv("MC_PLUGINS_DEFAULT_TIMEOUT", "60")
	t.Setenv("MC_PLUGINS_VALIDATE", "true")
	t.Setenv("MC_FILE_PATH", "/var/log/metrics.json")
	t.Setenv("MC_FILE_MAX_SIZE_MB", "200")
	t.Setenv("MC_FILE_MAX_FILES", "10")
	t.Setenv("MC_FILE_FORMAT", "multi")

	cfg := minimalValidConfig()
	applyEnvOverrides(&cfg)

	if cfg.Server.Host != "10.0.0.1" {
		t.Errorf("Server.Host = %q, want %q", cfg.Server.Host, "10.0.0.1")
	}
	if cfg.Collector.IntervalSeconds != 42 {
		t.Errorf("Collector.IntervalSeconds = %d, want 42", cfg.Collector.IntervalSeconds)
	}
	if cfg.Shipper.Type != "http_json" {
		t.Errorf("Shipper.Type = %q, want %q", cfg.Shipper.Type, "http_json")
	}
	if cfg.Shipper.Endpoint != "http://override.example.com/metrics" {
		t.Errorf("Shipper.Endpoint = %q, want %q", cfg.Shipper.Endpoint, "http://override.example.com/metrics")
	}
	if !cfg.Shipper.TLS.Enabled {
		t.Error("Shipper.TLS.Enabled should be true")
	}
	if cfg.Shipper.TLS.CertFile != "/etc/certs/client.crt" {
		t.Errorf("Shipper.TLS.CertFile = %q, want %q", cfg.Shipper.TLS.CertFile, "/etc/certs/client.crt")
	}
	if cfg.Shipper.TLS.KeyFile != "/etc/certs/client.key" {
		t.Errorf("Shipper.TLS.KeyFile = %q, want %q", cfg.Shipper.TLS.KeyFile, "/etc/certs/client.key")
	}
	if cfg.Shipper.TLS.CAFile != "/etc/certs/ca.crt" {
		t.Errorf("Shipper.TLS.CAFile = %q, want %q", cfg.Shipper.TLS.CAFile, "/etc/certs/ca.crt")
	}
	if cfg.Shipper.HECToken != "my-hec-token" {
		t.Errorf("Shipper.HECToken = %q, want %q", cfg.Shipper.HECToken, "my-hec-token")
	}
	if !cfg.Collector.Plugins.Enabled {
		t.Error("Collector.Plugins.Enabled should be true")
	}
	if cfg.Collector.Plugins.PluginsDir != "/opt/plugins" {
		t.Errorf("Collector.Plugins.PluginsDir = %q, want %q", cfg.Collector.Plugins.PluginsDir, "/opt/plugins")
	}
	if cfg.Collector.Plugins.DefaultTimeoutSeconds != 60 {
		t.Errorf("Collector.Plugins.DefaultTimeoutSeconds = %d, want 60", cfg.Collector.Plugins.DefaultTimeoutSeconds)
	}
	if !cfg.Collector.Plugins.ValidateOnStartup {
		t.Error("Collector.Plugins.ValidateOnStartup should be true")
	}
	if cfg.Shipper.File.Path != "/var/log/metrics.json" {
		t.Errorf("Shipper.File.Path = %q, want %q", cfg.Shipper.File.Path, "/var/log/metrics.json")
	}
	if cfg.Shipper.File.MaxSizeMB != 200 {
		t.Errorf("Shipper.File.MaxSizeMB = %d, want 200", cfg.Shipper.File.MaxSizeMB)
	}
	if cfg.Shipper.File.MaxFiles != 10 {
		t.Errorf("Shipper.File.MaxFiles = %d, want 10", cfg.Shipper.File.MaxFiles)
	}
	if cfg.Shipper.File.Format != "multi" {
		t.Errorf("Shipper.File.Format = %q, want %q", cfg.Shipper.File.Format, "multi")
	}
}

func TestValidate_FileFormatValidation(t *testing.T) {
	tests := []struct {
		format  string
		wantErr bool
	}{
		{"single", false},
		{"multi", false},
		{"invalid", true},
	}

	for _, tc := range tests {
		t.Run("format_"+tc.format, func(t *testing.T) {
			cfg := minimalValidConfig()
			cfg.Shipper.Type = "json_file"
			cfg.Shipper.Endpoint = ""
			cfg.Shipper.File.Path = "/tmp/metrics.json"
			cfg.Shipper.File.Format = tc.format

			err := cfg.Validate()
			if tc.wantErr && err == nil {
				t.Errorf("Validate() expected error for format %q, got nil", tc.format)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Validate() unexpected error for format %q: %v", tc.format, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// itoa is a minimal int-to-string helper to avoid importing strconv in tests.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := make([]byte, 0, 10)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}
