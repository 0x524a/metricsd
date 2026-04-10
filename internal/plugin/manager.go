package plugin

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/0x524A/metricsd/internal/collector"
)

type pluginEntry struct {
	name      string
	collector collector.Collector
}

// Manager coordinates all plugin collectors with parallel execution,
// circuit breaker, and health tracking. Implements collector.Collector.
type Manager struct {
	mu      sync.RWMutex
	plugins []pluginEntry
	health  map[string]*PluginHealth
}

func NewManager() *Manager {
	return &Manager{
		health: make(map[string]*PluginHealth),
	}
}

func (m *Manager) Name() string {
	return "plugins"
}

func (m *Manager) AddExecPlugin(ep *ExecPlugin) {
	m.mu.Lock()
	defer m.mu.Unlock()
	name := ep.config.Name
	m.plugins = append(m.plugins, pluginEntry{name: name, collector: ep})
	m.health[name] = &PluginHealth{Name: name, Status: "ok"}
}

func (m *Manager) AddGoPlugin(name string, c collector.Collector) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.plugins = append(m.plugins, pluginEntry{name: name, collector: c})
	m.health[name] = &PluginHealth{Name: name, Status: "ok"}
}

func (m *Manager) Collect(ctx context.Context) ([]collector.Metric, error) {
	m.mu.RLock()
	entries := make([]pluginEntry, len(m.plugins))
	copy(entries, m.plugins)
	m.mu.RUnlock()

	type result struct {
		name    string
		metrics []collector.Metric
		err     error
	}

	results := make(chan result, len(entries))
	var wg sync.WaitGroup

	for _, entry := range entries {
		m.mu.RLock()
		h := m.health[entry.name]
		m.mu.RUnlock()

		if h != nil && !h.CircuitOpenUntil.IsZero() && time.Now().Before(h.CircuitOpenUntil) {
			log.Debug().Str("plugin", entry.name).Time("circuit_open_until", h.CircuitOpenUntil).Msg("Skipping plugin — circuit open")
			continue
		}

		wg.Add(1)
		go func(e pluginEntry) {
			defer wg.Done()
			metrics, err := e.collector.Collect(ctx)
			results <- result{name: e.name, metrics: metrics, err: err}
		}(entry)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var allMetrics []collector.Metric
	for r := range results {
		m.mu.Lock()
		h := m.health[r.name]
		h.LastCollect = time.Now()

		if r.err != nil {
			h.ConsecutiveFails++
			h.LastError = r.err.Error()
			log.Warn().Str("plugin", r.name).Int("consecutive_fails", h.ConsecutiveFails).Err(r.err).Msg("Plugin collection failed")

			if h.ConsecutiveFails >= MaxConsecutiveFailures {
				backoff := time.Duration(1<<uint(h.ConsecutiveFails-MaxConsecutiveFailures)) * time.Minute
				if backoff > MaxCircuitOpenDuration {
					backoff = MaxCircuitOpenDuration
				}
				h.CircuitOpenUntil = time.Now().Add(backoff)
				h.Status = "circuit_open"
				log.Warn().Str("plugin", r.name).Dur("backoff", backoff).Msg("Circuit breaker opened")
			} else {
				h.Status = "failing"
			}
		} else {
			h.ConsecutiveFails = 0
			h.CircuitOpenUntil = time.Time{}
			h.Status = "ok"
			h.LastSuccess = time.Now()
			h.LastMetricCount = len(r.metrics)
			h.LastError = ""
			allMetrics = append(allMetrics, r.metrics...)
		}
		m.mu.Unlock()
	}

	return allMetrics, nil
}

func (m *Manager) GetHealth() map[string]PluginHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()
	snapshot := make(map[string]PluginHealth, len(m.health))
	for k, v := range m.health {
		snapshot[k] = *v
	}
	return snapshot
}

func (m *Manager) PluginCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.plugins)
}
