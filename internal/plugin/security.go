// internal/plugin/security.go
package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

var metricNameRegex = regexp.MustCompile(`^[a-zA-Z_:][a-zA-Z0-9_:]*$`)

// ValidatePluginPath resolves symlinks and verifies the path stays within pluginsDir.
// Returns the resolved absolute path, or an error if the path escapes or is invalid.
func ValidatePluginPath(pluginPath, pluginsDir string) (string, error) {
	absPluginsDir, err := filepath.Abs(pluginsDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve plugins dir: %w", err)
	}
	resolvedDir, err := filepath.EvalSymlinks(absPluginsDir)
	if err != nil {
		return "", fmt.Errorf("failed to eval plugins dir symlinks: %w", err)
	}

	absPluginPath, err := filepath.Abs(pluginPath)
	if err != nil {
		return "", fmt.Errorf("failed to make plugin path absolute %s: %w", pluginPath, err)
	}
	resolvedPath, err := filepath.EvalSymlinks(absPluginPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve plugin path %s: %w", pluginPath, err)
	}

	// Check the resolved path is inside the resolved plugins dir
	if !strings.HasPrefix(resolvedPath, resolvedDir+string(filepath.Separator)) {
		return "", fmt.Errorf("plugin %s resolves to %s which is outside plugins dir %s", pluginPath, resolvedPath, resolvedDir)
	}

	// Warn if world-writable
	info, err := os.Stat(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("failed to stat plugin: %w", err)
	}
	if info.Mode()&0002 != 0 {
		log.Warn().
			Str("plugin", resolvedPath).
			Msg("Plugin is world-writable — this is a security risk")
	}

	return resolvedPath, nil
}

// BuildSafeEnv constructs a minimal environment for plugin execution.
// Does NOT inherit os.Environ(). Only includes safe defaults + explicit extras.
func BuildSafeEnv(extraEnv []string) []string {
	env := []string{
		"PATH=/usr/local/bin:/usr/bin:/bin",
		"HOME=/nonexistent",
		"LANG=C.UTF-8",
	}
	env = append(env, extraEnv...)
	return env
}

// ValidateMetricOutput filters and sanitizes plugin output metrics.
// Rejects: empty names, invalid names, labels starting with __.
// Truncates: label values over 1024 chars.
func ValidateMetricOutput(metrics []PluginMetric, pluginName string) []PluginMetric {
	valid := make([]PluginMetric, 0, len(metrics))

	for _, pm := range metrics {
		if pm.Name == "" {
			log.Warn().Str("plugin", pluginName).Msg("Skipping metric with empty name")
			continue
		}
		if !metricNameRegex.MatchString(pm.Name) {
			log.Warn().Str("plugin", pluginName).Str("name", pm.Name).Msg("Skipping metric with invalid name")
			continue
		}

		// Check for reserved label prefixes and truncate long values
		hasReserved := false
		sanitizedLabels := make(map[string]string, len(pm.Labels))
		for k, v := range pm.Labels {
			if strings.HasPrefix(k, "__") {
				log.Warn().Str("plugin", pluginName).Str("label", k).Msg("Rejecting metric with reserved label prefix __")
				hasReserved = true
				break
			}
			if len(v) > 1024 {
				v = v[:1024]
			}
			sanitizedLabels[k] = v
		}
		if hasReserved {
			continue
		}

		pm.Labels = sanitizedLabels
		valid = append(valid, pm)
	}

	return valid
}
