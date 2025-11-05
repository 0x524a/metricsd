package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

	// Try to parse as JSON metrics
	var rawMetrics map[string]interface{}
	if err := json.Unmarshal(body, &rawMetrics); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return c.parseMetrics(endpoint.Name, rawMetrics), nil
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
