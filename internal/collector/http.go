package collector

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// HTTPCollector scrapes metrics from HTTP endpoints (Single Responsibility Principle)
type HTTPCollector struct {
	endpoints []EndpointConfig
	client    *http.Client
}

// EndpointConfig represents an HTTP endpoint to scrape
type EndpointConfig struct {
	Name string
	URL  string
}

// NewHTTPCollector creates a new HTTP metrics collector
func NewHTTPCollector(endpoints []EndpointConfig, timeout time.Duration) *HTTPCollector {
	return &HTTPCollector{
		endpoints: endpoints,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Name returns the collector name
func (c *HTTPCollector) Name() string {
	return "http"
}

// Collect scrapes metrics from all configured HTTP endpoints
func (c *HTTPCollector) Collect(ctx context.Context) ([]Metric, error) {
	metrics := make([]Metric, 0)

	for _, endpoint := range c.endpoints {
		endpointMetrics, err := c.scrapeEndpoint(ctx, endpoint)
		if err != nil {
			log.Warn().
				Err(err).
				Str("endpoint", endpoint.Name).
				Str("url", endpoint.URL).
				Msg("Failed to scrape endpoint")
			continue
		}
		metrics = append(metrics, endpointMetrics...)
	}

	return metrics, nil
}

func (c *HTTPCollector) scrapeEndpoint(ctx context.Context, endpoint EndpointConfig) ([]Metric, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Auto-detect format and parse accordingly
	if isPrometheusFormat(body) {
		return c.parsePrometheusText(endpoint.Name, body), nil
	}

	// Try to parse as JSON metrics
	var rawMetrics map[string]interface{}
	if err := json.Unmarshal(body, &rawMetrics); err != nil {
		return nil, fmt.Errorf("failed to parse response (not valid JSON or Prometheus format): %w", err)
	}

	return c.parseMetrics(endpoint.Name, rawMetrics), nil
}

// isPrometheusFormat checks if the body is in Prometheus text format
func isPrometheusFormat(body []byte) bool {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return false
	}
	// JSON always starts with { or [
	if trimmed[0] == '{' || trimmed[0] == '[' {
		return false
	}
	// Prometheus format: # comment/HELP/TYPE lines
	if trimmed[0] == '#' {
		return true
	}
	// Check the first non-comment line for Prometheus metric pattern:
	// metric_name[{labels}] <numeric_value> [timestamp]
	scanner := bufio.NewScanner(bytes.NewReader(trimmed))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Extract the value part (after metric name and optional labels)
		valueStr := line
		if idx := strings.Index(line, "}"); idx != -1 {
			valueStr = strings.TrimSpace(line[idx+1:])
		} else {
			parts := strings.Fields(line)
			if len(parts) < 2 {
				return false
			}
			valueStr = parts[1]
		}
		// Must parse as a float to be a valid Prometheus metric line
		_, err := strconv.ParseFloat(strings.Fields(valueStr)[0], 64)
		return err == nil
	}
	return false
}

// parsePrometheusText parses Prometheus text exposition format
func (c *HTTPCollector) parsePrometheusText(endpointName string, body []byte) []Metric {
	metrics := make([]Metric, 0)
	scanner := bufio.NewScanner(bytes.NewReader(body))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments (including HELP and TYPE)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		metric := c.parsePrometheusLine(endpointName, line)
		if metric != nil {
			metrics = append(metrics, *metric)
		}
	}

	return metrics
}

// parsePrometheusLine parses a single Prometheus metric line
// Format: metric_name{label="value",...} value [timestamp]
// Or: metric_name value [timestamp]
func (c *HTTPCollector) parsePrometheusLine(endpointName, line string) *Metric {
	var metricName string
	var labelsStr string
	var valueStr string

	// Check if there are labels
	if idx := strings.Index(line, "{"); idx != -1 {
		metricName = line[:idx]
		endIdx := strings.Index(line, "}")
		if endIdx == -1 {
			return nil
		}
		labelsStr = line[idx+1 : endIdx]
		rest := strings.TrimSpace(line[endIdx+1:])
		parts := strings.Fields(rest)
		if len(parts) == 0 {
			return nil
		}
		valueStr = parts[0]
	} else {
		// No labels
		parts := strings.Fields(line)
		if len(parts) < 2 {
			return nil
		}
		metricName = parts[0]
		valueStr = parts[1]
	}

	// Parse value
	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return nil
	}

	// Parse labels
	labels := map[string]string{
		"endpoint": endpointName,
	}

	if labelsStr != "" {
		labelPairs := splitLabels(labelsStr)
		for _, pair := range labelPairs {
			kv := strings.SplitN(pair, "=", 2)
			if len(kv) == 2 {
				key := strings.TrimSpace(kv[0])
				val := strings.Trim(strings.TrimSpace(kv[1]), "\"")
				labels[key] = val
			}
		}
	}

	return &Metric{
		Name:   metricName,
		Labels: labels,
		Value:  value,
		Type:   "gauge",
	}
}

// splitLabels splits label pairs handling quoted values with commas
func splitLabels(labelsStr string) []string {
	var result []string
	var current strings.Builder
	inQuotes := false

	for i := 0; i < len(labelsStr); i++ {
		ch := labelsStr[i]
		switch ch {
		case '"':
			inQuotes = !inQuotes
			current.WriteByte(ch)
		case ',':
			if inQuotes {
				current.WriteByte(ch)
			} else {
				if current.Len() > 0 {
					result = append(result, current.String())
					current.Reset()
				}
			}
		default:
			current.WriteByte(ch)
		}
	}

	if current.Len() > 0 {
		result = append(result, current.String())
	}

	return result
}

func (c *HTTPCollector) parseMetrics(endpointName string, rawMetrics map[string]interface{}) []Metric {
	metrics := make([]Metric, 0)

	for key, value := range rawMetrics {
		// Convert value to float64
		var floatValue float64
		switch v := value.(type) {
		case float64:
			floatValue = v
		case float32:
			floatValue = float64(v)
		case int:
			floatValue = float64(v)
		case int64:
			floatValue = float64(v)
		case int32:
			floatValue = float64(v)
		default:
			// Skip non-numeric values
			continue
		}

		metrics = append(metrics, Metric{
			Name: fmt.Sprintf("app_%s", key),
			Labels: map[string]string{
				"endpoint": endpointName,
			},
			Value: floatValue,
			Type:  "gauge",
		})
	}

	return metrics
}
