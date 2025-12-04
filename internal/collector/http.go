package collector

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
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

	// Accept both Prometheus and JSON formats
	req.Header.Set("Accept", "text/plain, application/json")

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

	contentType := resp.Header.Get("Content-Type")

	// Try to detect format and parse accordingly
	if strings.Contains(contentType, "application/json") {
		// Parse as JSON
		var rawMetrics map[string]interface{}
		if err := json.Unmarshal(body, &rawMetrics); err != nil {
			return nil, fmt.Errorf("failed to parse JSON response: %w", err)
		}
		return c.parseJSONMetrics(endpoint.Name, rawMetrics), nil
	}

	// Try Prometheus text format first (most common for /metrics endpoints)
	if metrics := c.parsePrometheusMetrics(endpoint.Name, string(body)); len(metrics) > 0 {
		return metrics, nil
	}

	// Fallback to JSON parsing
	var rawMetrics map[string]interface{}
	if err := json.Unmarshal(body, &rawMetrics); err != nil {
		return nil, fmt.Errorf("failed to parse response (tried prometheus and JSON formats): %w", err)
	}
	return c.parseJSONMetrics(endpoint.Name, rawMetrics), nil
}

// parsePrometheusMetrics parses Prometheus exposition format text
func (c *HTTPCollector) parsePrometheusMetrics(endpointName string, body string) []Metric {
	metrics := make([]Metric, 0)

	// Regex to match Prometheus metric lines
	// Format: metric_name{label1="value1",label2="value2"} value [timestamp]
	// Or just: metric_name value [timestamp]
	metricLineRegex := regexp.MustCompile(`^([a-zA-Z_:][a-zA-Z0-9_:]*)\s*(\{[^}]*\})?\s+([+-]?[0-9]*\.?[0-9]+(?:[eE][+-]?[0-9]+)?)\s*(\d+)?$`)
	labelRegex := regexp.MustCompile(`([a-zA-Z_][a-zA-Z0-9_]*)="([^"]*)"`)

	// Track metric types from HELP/TYPE comments
	metricTypes := make(map[string]string)

	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines
		if line == "" {
			continue
		}

		// Parse TYPE comments to get metric types
		if strings.HasPrefix(line, "# TYPE ") {
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				metricName := parts[2]
				metricType := parts[3]
				metricTypes[metricName] = metricType
			}
			continue
		}

		// Skip HELP and other comments
		if strings.HasPrefix(line, "#") {
			continue
		}

		// Parse metric line
		matches := metricLineRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		metricName := matches[1]
		labelsStr := matches[2]
		valueStr := matches[3]

		// Parse value
		value, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			continue
		}

		// Parse labels
		labels := map[string]string{
			"endpoint": endpointName,
		}

		if labelsStr != "" {
			// Remove curly braces
			labelsStr = strings.Trim(labelsStr, "{}")
			labelMatches := labelRegex.FindAllStringSubmatch(labelsStr, -1)
			for _, lm := range labelMatches {
				if len(lm) == 3 {
					labels[lm[1]] = lm[2]
				}
			}
		}

		// Determine metric type
		metricType := "gauge"
		if t, ok := metricTypes[metricName]; ok {
			if t == "counter" {
				metricType = "counter"
			}
		} else if strings.HasSuffix(metricName, "_total") || strings.HasSuffix(metricName, "_count") {
			metricType = "counter"
		}

		metrics = append(metrics, Metric{
			Name:   metricName,
			Labels: labels,
			Value:  value,
			Type:   metricType,
		})
	}

	return metrics
}

// parseJSONMetrics parses JSON format metrics (legacy support)
func (c *HTTPCollector) parseJSONMetrics(endpointName string, rawMetrics map[string]interface{}) []Metric {
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
