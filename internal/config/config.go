package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config represents the application configuration
type Config struct {
	Server     ServerConfig     `json:"server"`
	Collector  CollectorConfig  `json:"collector"`
	Shipper    ShipperConfig    `json:"shipper"`
	Endpoints  []EndpointConfig `json:"endpoints"`
}

// ServerConfig contains HTTP server settings
type ServerConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// CollectorConfig contains metrics collection settings
type CollectorConfig struct {
	IntervalSeconds int                `json:"interval_seconds"`
	EnableCPU       bool               `json:"enable_cpu"`
	EnableMemory    bool               `json:"enable_memory"`
	EnableDisk      bool               `json:"enable_disk"`
	EnableNetwork   bool               `json:"enable_network"`
	EnableGPU       bool               `json:"enable_gpu"`
	Plugins         PluginSystemConfig `json:"plugins,omitempty"`
}

// GoPluginEntry configures a compile-time registered Go plugin.
type GoPluginEntry struct {
	Name   string                 `json:"name"`
	Config map[string]interface{} `json:"config,omitempty"`
}

// PluginSystemConfig contains plugin system settings
type PluginSystemConfig struct {
	Enabled               bool            `json:"enabled"`
	PluginsDir            string          `json:"plugins_dir"`
	DefaultTimeoutSeconds int             `json:"default_timeout_seconds,omitempty"`
	ValidateOnStartup     bool            `json:"validate_on_startup,omitempty"`
	GoPlugins             []GoPluginEntry `json:"go_plugins,omitempty"`
}

// ShipperConfig contains remote endpoint settings
type ShipperConfig struct {
	Type     string        `json:"type"` // "prometheus_remote_write", "http_json", "json_file", or "splunk_hec"
	Endpoint string        `json:"endpoint"`
	TLS      TLSConfig     `json:"tls"`
	Timeout  time.Duration `json:"timeout"`
	// File shipper specific settings
	File FileShipperConfig `json:"file,omitempty"`
	// Splunk HEC specific settings
	HECToken     string `json:"hec_token,omitempty"`
	DebugLogFile string `json:"debug_log_file,omitempty"` // Optional file path to log payloads for debugging
}

// FileShipperConfig contains file shipper settings for Splunk Universal Forwarder integration
type FileShipperConfig struct {
	Path      string `json:"path"`        // Path to the output file
	MaxSizeMB int    `json:"max_size_mb"` // Max file size before rotation (default: 100MB)
	MaxFiles  int    `json:"max_files"`   // Number of rotated files to keep (default: 5)
	Format    string `json:"format"`      // Output format: "single" (one metric per line) or "multi" (Splunk multi-metric)
}

// TLSConfig contains TLS settings
type TLSConfig struct {
	Enabled            bool   `json:"enabled"`
	CertFile           string `json:"cert_file"`
	KeyFile            string `json:"key_file"`
	CAFile             string `json:"ca_file"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify"`
}

// EndpointConfig represents an application endpoint to scrape
type EndpointConfig struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Load reads configuration from a JSON file and applies environment variable overrides
func Load(configPath string) (*Config, error) {
	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply environment variable overrides
	applyEnvOverrides(&cfg)

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// applyEnvOverrides applies environment variable overrides to the configuration
func applyEnvOverrides(cfg *Config) {
	if val := os.Getenv("MC_SERVER_HOST"); val != "" {
		cfg.Server.Host = val
	}
	if val := os.Getenv("MC_SERVER_PORT"); val != "" {
		if port, err := strconv.Atoi(val); err == nil {
			cfg.Server.Port = port
		}
	}
	if val := os.Getenv("MC_COLLECTOR_INTERVAL"); val != "" {
		if interval, err := strconv.Atoi(val); err == nil {
			cfg.Collector.IntervalSeconds = interval
		}
	}
	if val := os.Getenv("MC_SHIPPER_TYPE"); val != "" {
		cfg.Shipper.Type = val
	}
	if val := os.Getenv("MC_SHIPPER_ENDPOINT"); val != "" {
		cfg.Shipper.Endpoint = val
	}
	if val := os.Getenv("MC_TLS_ENABLED"); val != "" {
		if enabled, err := strconv.ParseBool(val); err == nil {
			cfg.Shipper.TLS.Enabled = enabled
		}
	}
	if val := os.Getenv("MC_TLS_CERT_FILE"); val != "" {
		cfg.Shipper.TLS.CertFile = val
	}
	if val := os.Getenv("MC_TLS_KEY_FILE"); val != "" {
		cfg.Shipper.TLS.KeyFile = val
	}
	if val := os.Getenv("MC_TLS_CA_FILE"); val != "" {
		cfg.Shipper.TLS.CAFile = val
	}
	// Splunk HEC token environment variable override
	if val := os.Getenv("MC_HEC_TOKEN"); val != "" {
		cfg.Shipper.HECToken = val
	}
	// Plugin configuration environment variable overrides
	if val := os.Getenv("MC_PLUGINS_ENABLED"); val != "" {
		if enabled, err := strconv.ParseBool(val); err == nil {
			cfg.Collector.Plugins.Enabled = enabled
		}
	}
	if val := os.Getenv("MC_PLUGINS_DIR"); val != "" {
		cfg.Collector.Plugins.PluginsDir = val
	}
	if val := os.Getenv("MC_PLUGINS_DEFAULT_TIMEOUT"); val != "" {
		if timeout, err := strconv.Atoi(val); err == nil {
			cfg.Collector.Plugins.DefaultTimeoutSeconds = timeout
		}
	}
	if val := os.Getenv("MC_PLUGINS_VALIDATE"); val != "" {
		if validate, err := strconv.ParseBool(val); err == nil {
			cfg.Collector.Plugins.ValidateOnStartup = validate
		}
	}
	// File shipper environment variable overrides
	if val := os.Getenv("MC_FILE_PATH"); val != "" {
		cfg.Shipper.File.Path = val
	}
	if val := os.Getenv("MC_FILE_MAX_SIZE_MB"); val != "" {
		if size, err := strconv.Atoi(val); err == nil {
			cfg.Shipper.File.MaxSizeMB = size
		}
	}
	if val := os.Getenv("MC_FILE_MAX_FILES"); val != "" {
		if count, err := strconv.Atoi(val); err == nil {
			cfg.Shipper.File.MaxFiles = count
		}
	}
	if val := os.Getenv("MC_FILE_FORMAT"); val != "" {
		cfg.Shipper.File.Format = val
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	if c.Collector.IntervalSeconds <= 0 {
		return fmt.Errorf("collector interval must be positive")
	}

	if c.Shipper.Type != "prometheus_remote_write" && c.Shipper.Type != "http_json" && c.Shipper.Type != "json_file" && c.Shipper.Type != "splunk_hec" {
		return fmt.Errorf("invalid shipper type: %s (must be 'prometheus_remote_write', 'http_json', 'json_file', or 'splunk_hec')", c.Shipper.Type)
	}

	// Validate based on shipper type
	if c.Shipper.Type == "json_file" {
		if c.Shipper.File.Path == "" {
			return fmt.Errorf("file shipper requires a file path")
		}
		// Validate format (default to "single" if not specified)
		if c.Shipper.File.Format != "" && c.Shipper.File.Format != "single" && c.Shipper.File.Format != "multi" {
			return fmt.Errorf("invalid file format: %s (must be 'single' or 'multi')", c.Shipper.File.Format)
		}
	} else {
		if c.Shipper.Endpoint == "" {
			return fmt.Errorf("shipper endpoint is required")
		}
		// Validate Splunk HEC token
		if c.Shipper.Type == "splunk_hec" && c.Shipper.HECToken == "" {
			return fmt.Errorf("splunk_hec shipper requires a HEC token")
		}
	}

	if c.Shipper.TLS.Enabled {
		if c.Shipper.TLS.CertFile == "" || c.Shipper.TLS.KeyFile == "" {
			return fmt.Errorf("TLS cert and key files are required when TLS is enabled")
		}
	}

	// Apply plugin configuration defaults
	if c.Collector.Plugins.Enabled {
		if c.Collector.Plugins.PluginsDir == "" {
			c.Collector.Plugins.PluginsDir = "plugins"
		}
		if c.Collector.Plugins.DefaultTimeoutSeconds <= 0 {
			c.Collector.Plugins.DefaultTimeoutSeconds = 30
		}
	}

	return nil
}

// GetCollectionInterval returns the collection interval as a duration
func (c *Config) GetCollectionInterval() time.Duration {
	return time.Duration(c.Collector.IntervalSeconds) * time.Second
}
