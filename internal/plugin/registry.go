package plugin

import (
	"github.com/0x524A/metricsd/internal/collector"
)

// GoPluginFactory creates a collector.Collector from a config map.
type GoPluginFactory func(config map[string]interface{}) (collector.Collector, error)

var goPluginRegistry = make(map[string]GoPluginFactory)

// RegisterGoPlugin registers a Go plugin factory by name.
func RegisterGoPlugin(name string, factory GoPluginFactory) {
	goPluginRegistry[name] = factory
}

// GetRegisteredGoPlugins returns a copy of all registered Go plugin factories.
func GetRegisteredGoPlugins() map[string]GoPluginFactory {
	copy := make(map[string]GoPluginFactory, len(goPluginRegistry))
	for k, v := range goPluginRegistry {
		copy[k] = v
	}
	return copy
}
