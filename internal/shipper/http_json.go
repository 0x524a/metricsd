package shipper

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/0x524A/metricsd/internal/collector"
)

// HTTPJSONShipper ships metrics as JSON via HTTP POST (Single Responsibility Principle)
type HTTPJSONShipper struct {
	endpoint string
	client   *http.Client
}

// NewHTTPJSONShipper creates a new HTTP JSON shipper
func NewHTTPJSONShipper(endpoint string, tlsEnabled bool, certFile, keyFile, caFile string, insecureSkipVerify bool, timeout time.Duration) (*HTTPJSONShipper, error) {
	var tlsConfig *tls.Config

	if tlsEnabled {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS certificate: %w", err)
		}

		caCert, err := os.ReadFile(caFile)
		if err != nil && caFile != "" {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}

		caCertPool := x509.NewCertPool()
		if len(caCert) > 0 {
			caCertPool.AppendCertsFromPEM(caCert)
		}

		tlsConfig = &tls.Config{
			Certificates:       []tls.Certificate{cert},
			RootCAs:            caCertPool,
			InsecureSkipVerify: insecureSkipVerify,
		}
	}

	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	return &HTTPJSONShipper{
		endpoint: endpoint,
		client:   client,
	}, nil
}

// MetricPayload represents the JSON structure for shipping metrics
type MetricPayload struct {
	Timestamp int64             `json:"timestamp"`
	Metrics   []MetricData      `json:"metrics"`
}

// MetricData represents a single metric in JSON format
type MetricData struct {
	Name   string            `json:"name"`
	Value  float64           `json:"value"`
	Type   string            `json:"type"`
	Labels map[string]string `json:"labels"`
}

// Ship sends metrics to the HTTP JSON endpoint
func (s *HTTPJSONShipper) Ship(ctx context.Context, metrics []collector.Metric) error {
	if len(metrics) == 0 {
		return nil
	}

	// Convert to JSON payload
	payload := s.convertToPayload(metrics)

	// Marshal to JSON
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	log.Info().
		Int("metric_count", len(metrics)).
		Str("endpoint", s.endpoint).
		Msg("Successfully shipped metrics via HTTP JSON")

	return nil
}

func (s *HTTPJSONShipper) convertToPayload(metrics []collector.Metric) MetricPayload {
	metricData := make([]MetricData, 0, len(metrics))

	for _, metric := range metrics {
		metricData = append(metricData, MetricData{
			Name:   metric.Name,
			Value:  metric.Value,
			Type:   metric.Type,
			Labels: metric.Labels,
		})
	}

	return MetricPayload{
		Timestamp: time.Now().Unix(),
		Metrics:   metricData,
	}
}

// Close cleans up resources
func (s *HTTPJSONShipper) Close() error {
	s.client.CloseIdleConnections()
	return nil
}
