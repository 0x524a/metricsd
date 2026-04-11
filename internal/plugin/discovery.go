package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

var skipExtensions = []string{".json", ".md", ".txt", ".example", ".bak", ".log", ".old", ".swp", ".tmp"}

// DiscoverPlugins scans pluginsDir for executable files and returns ExecPlugin instances.
func DiscoverPlugins(pluginsDir string, defaultTimeout time.Duration, validate bool) ([]*ExecPlugin, error) {
	info, err := os.Stat(pluginsDir)
	if os.IsNotExist(err) {
		log.Info().Str("dir", pluginsDir).Msg("Plugins directory does not exist, skipping")
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, nil
	}

	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return nil, err
	}

	var plugins []*ExecPlugin

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		rawPath := filepath.Join(pluginsDir, name)

		skip := false
		for _, ext := range skipExtensions {
			if strings.HasSuffix(name, ext) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		resolvedPath, err := ValidatePluginPath(rawPath, pluginsDir)
		if err != nil {
			log.Warn().Str("file", name).Err(err).Msg("Skipping plugin — path validation failed")
			continue
		}

		fileInfo, err := os.Stat(resolvedPath)
		if err != nil {
			log.Warn().Str("file", name).Err(err).Msg("Skipping plugin — stat failed")
			continue
		}
		if fileInfo.Mode()&0111 == 0 {
			continue
		}

		config := PluginConfig{
			Name: strings.TrimSuffix(name, filepath.Ext(name)),
			Path: resolvedPath,
		}

		configPath := rawPath + ".json"
		if data, err := os.ReadFile(configPath); err == nil {
			var fileCfg PluginConfig
			if err := json.Unmarshal(data, &fileCfg); err != nil {
				log.Warn().Str("config", configPath).Err(err).Msg("Failed to parse plugin config")
			} else {
				if fileCfg.Name != "" {
					config.Name = fileCfg.Name
				}
				if fileCfg.Timeout > 0 {
					config.Timeout = fileCfg.Timeout
				}
				if len(fileCfg.Args) > 0 {
					config.Args = fileCfg.Args
				}
				if len(fileCfg.Env) > 0 {
					config.Env = fileCfg.Env
				}
				if fileCfg.WorkingDir != "" {
					config.WorkingDir = fileCfg.WorkingDir
				}
				if fileCfg.Interval > 0 {
					config.Interval = fileCfg.Interval
				}
				if fileCfg.Enabled != nil {
					config.Enabled = fileCfg.Enabled
				}
			}
		}

		if !config.IsEnabled() {
			log.Info().Str("plugin", config.Name).Msg("Plugin disabled, skipping")
			continue
		}

		ep := NewExecPlugin(config)
		plugins = append(plugins, ep)
		log.Info().Str("plugin", config.Name).Str("path", resolvedPath).Msg("Discovered plugin")
	}

	return plugins, nil
}
