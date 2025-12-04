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
	Server       ServerConfig      `json:"server"`
	Collector    CollectorConfig   `json:"collector"`
	Shipper      ShipperConfig     `json:"shipper"`
	Endpoints    []EndpointConfig  `json:"endpoints"`
	GlobalLabels map[string]string `json:"global_labels,omitempty"` // Optional global labels added to all metrics
}

// ServerConfig contains HTTP server settings
type ServerConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// CollectorConfig contains metrics collection settings
type CollectorConfig struct {
	IntervalSeconds int  `json:"interval_seconds"`
	EnableCPU       bool `json:"enable_cpu"`
	EnableMemory    bool `json:"enable_memory"`
	EnableDisk      bool `json:"enable_disk"`
	EnableNetwork   bool `json:"enable_network"`
	EnableGPU       bool `json:"enable_gpu"`
}

// ShipperConfig contains remote endpoint settings
type ShipperConfig struct {
	Type     string        `json:"type"` // "prometheus_remote_write" or "http_json"
	Endpoint string        `json:"endpoint"`
	TLS      TLSConfig     `json:"tls"`
	Timeout  time.Duration `json:"timeout"`
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
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	if c.Collector.IntervalSeconds <= 0 {
		return fmt.Errorf("collector interval must be positive")
	}

	if c.Shipper.Type != "prometheus_remote_write" && c.Shipper.Type != "http_json" {
		return fmt.Errorf("invalid shipper type: %s (must be 'prometheus_remote_write' or 'http_json')", c.Shipper.Type)
	}

	if c.Shipper.Endpoint == "" {
		return fmt.Errorf("shipper endpoint is required")
	}

	if c.Shipper.TLS.Enabled {
		if c.Shipper.TLS.CertFile == "" || c.Shipper.TLS.KeyFile == "" {
			return fmt.Errorf("TLS cert and key files are required when TLS is enabled")
		}
	}

	return nil
}

// GetCollectionInterval returns the collection interval as a duration
func (c *Config) GetCollectionInterval() time.Duration {
	return time.Duration(c.Collector.IntervalSeconds) * time.Second
}
