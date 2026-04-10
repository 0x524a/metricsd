package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// PluginDiscoveryConfig holds settings for plugin discovery
type PluginDiscoveryConfig struct {
	PluginsDir        string
	Enabled           bool
	DefaultTimeout    time.Duration
	ValidateOnStartup bool
}

// DiscoverPlugins scans the plugins directory and returns plugin collectors
func DiscoverPlugins(cfg PluginDiscoveryConfig) ([]*PluginCollector, error) {
	if !cfg.Enabled {
		log.Debug().Msg("Plugin discovery is disabled")
		return nil, nil
	}

	if cfg.DefaultTimeout == 0 {
		cfg.DefaultTimeout = 30 * time.Second
	}

	// Ensure plugins directory exists
	info, err := os.Stat(cfg.PluginsDir)
	if os.IsNotExist(err) {
		log.Info().
			Str("dir", cfg.PluginsDir).
			Msg("Plugins directory does not exist, skipping plugin discovery")
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to stat plugins directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("plugins path is not a directory: %s", cfg.PluginsDir)
	}

	var collectors []*PluginCollector

	// Scan directory for executables
	entries, err := os.ReadDir(cfg.PluginsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugins directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		pluginPath := filepath.Join(cfg.PluginsDir, name)

		// Skip non-plugin files (config, docs, backups, examples)
		skipExtensions := []string{".json", ".md", ".txt", ".example", ".bak", ".log", ".old", ".swp", ".tmp"}
		shouldSkip := false
		for _, ext := range skipExtensions {
			if strings.HasSuffix(name, ext) {
				log.Debug().
					Str("file", name).
					Str("reason", "non-plugin extension").
					Msg("Skipping file")
				shouldSkip = true
				break
			}
		}
		if shouldSkip {
			continue
		}

		// Check if file is executable
		fileInfo, err := entry.Info()
		if err != nil {
			log.Warn().
				Str("file", name).
				Err(err).
				Msg("Failed to get file info")
			continue
		}

		// Check for executable permission (Unix-style)
		mode := fileInfo.Mode()
		if mode&0111 == 0 {
			log.Debug().
				Str("file", name).
				Msg("Skipping non-executable file")
			continue
		}

		// Load plugin config if exists
		config := PluginConfig{
			Name:    strings.TrimSuffix(name, filepath.Ext(name)),
			Path:    pluginPath,
			Timeout: cfg.DefaultTimeout,
			Enabled: true,
		}

		configPath := pluginPath + ".json"
		if configData, err := os.ReadFile(configPath); err == nil {
			// Use a map to check which fields are explicitly set
			var rawConfig map[string]interface{}
			if err := json.Unmarshal(configData, &rawConfig); err != nil {
				log.Warn().
					Str("plugin", name).
					Str("config", configPath).
					Err(err).
					Msg("Failed to parse plugin config file")
			} else {
				// Apply config file values, preserving path
				if val, ok := rawConfig["name"].(string); ok && val != "" {
					config.Name = val
				}
				if val, ok := rawConfig["args"].([]interface{}); ok && len(val) > 0 {
					config.Args = make([]string, 0, len(val))
					for _, arg := range val {
						if s, ok := arg.(string); ok {
							config.Args = append(config.Args, s)
						}
					}
				}
				if val, ok := rawConfig["timeout"].(float64); ok && val > 0 {
					config.Timeout = time.Duration(val)
				}
				if val, ok := rawConfig["env"].([]interface{}); ok && len(val) > 0 {
					config.Env = make([]string, 0, len(val))
					for _, env := range val {
						if s, ok := env.(string); ok {
							config.Env = append(config.Env, s)
						}
					}
				}
				if val, ok := rawConfig["working_dir"].(string); ok && val != "" {
					config.WorkingDir = val
				}
				// Only update enabled if explicitly set in config
				if val, ok := rawConfig["enabled"].(bool); ok {
					config.Enabled = val
				}
			}
		}

		if !config.Enabled {
			log.Info().
				Str("plugin", config.Name).
				Msg("Plugin is disabled, skipping")
			continue
		}

		collector := NewPluginCollector(config)

		// Validate plugin on startup if enabled
		if cfg.ValidateOnStartup {
			ctx, cancel := context.WithTimeout(context.Background(), cfg.DefaultTimeout)
			err := collector.Validate(ctx)
			cancel()

			if err != nil {
				log.Warn().
					Str("plugin", config.Name).
					Err(err).
					Msg("Plugin failed validation, skipping")
				continue
			}
		}

		collectors = append(collectors, collector)
		log.Info().
			Str("plugin", config.Name).
			Str("path", config.Path).
			Dur("timeout", config.Timeout).
			Msg("Discovered plugin")
	}

	return collectors, nil
}
