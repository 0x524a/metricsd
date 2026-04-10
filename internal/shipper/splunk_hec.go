package shipper

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/0x524A/metricsd/internal/collector"
)

// SplunkHECShipper ships metrics to Splunk HTTP Event Collector
type SplunkHECShipper struct {
	endpoint     string
	token        string
	client       *http.Client
	debugLogFile string // Optional file path to log payloads for debugging
}

// NewSplunkHECShipper creates a new Splunk HEC shipper
func NewSplunkHECShipper(endpoint, token string, tlsEnabled bool, certFile, keyFile, caFile string, insecureSkipVerify bool, timeout time.Duration, debugLogFile string) (*SplunkHECShipper, error) {
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
	} else {
		// For HTTPS without client cert (common for Splunk HEC)
		tlsConfig = &tls.Config{
			InsecureSkipVerify: insecureSkipVerify,
		}
	}

	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	// Ensure endpoint has the correct path for HEC
	if endpoint == "" {
		return nil, fmt.Errorf("endpoint URL is required")
	}
	if endpoint[len(endpoint)-1] != '/' {
		endpoint += "/"
	}
	endpoint += "services/collector/event"

	return &SplunkHECShipper{
		endpoint:     endpoint,
		token:        token,
		client:       client,
		debugLogFile: debugLogFile,
	}, nil
}

// SplunkHECEvent represents a single event for Splunk HEC
type SplunkHECEvent struct {
	Time       float64                `json:"time"`
	Host       string                 `json:"host,omitempty"`
	Source     string                 `json:"source,omitempty"`
	SourceType string                 `json:"sourcetype,omitempty"`
	Event      map[string]interface{} `json:"event"`
}

// Ship sends metrics to Splunk HEC endpoint
func (s *SplunkHECShipper) Ship(ctx context.Context, metrics []collector.Metric) error {
	if len(metrics) == 0 {
		return nil
	}

	// Get hostname for event metadata
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	// Convert metrics to Splunk HEC format
	// For metrics, we'll create one event per metric
	var buffer bytes.Buffer
	skippedCount := 0
	for _, metric := range metrics {
		// Skip metrics with NaN or Inf values as they cannot be marshaled to JSON
		if math.IsNaN(metric.Value) || math.IsInf(metric.Value, 0) {
			log.Warn().
				Str("metric_name", metric.Name).
				Float64("value", metric.Value).
				Msg("Skipping metric with invalid value (NaN or Inf)")
			skippedCount++
			continue
		}

		event := SplunkHECEvent{
			Time:       float64(time.Now().Unix()),
			Host:       hostname,
			Source:     "metricsd",
			SourceType: "metrics",
			Event: map[string]interface{}{
				"metric_name": metric.Name,
				"_value":      metric.Value,
				"metric_type": metric.Type,
			},
		}

		// Add labels to the event
		for k, v := range metric.Labels {
			event.Event[k] = v
		}

		// Marshal event to JSON
		data, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("failed to marshal metric: %w", err)
		}

		buffer.Write(data)
		buffer.WriteString("\n")
	}

	// Debug: Log payload to file if configured
	if s.debugLogFile != "" {
		payloadCopy := buffer.String()
		s.logPayloadToFile(payloadCopy)
	}

	// Get payload size before sending
	payloadSize := buffer.Len()

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, &buffer)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers required by Splunk HEC
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Splunk %s", s.token))

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

	successCount := len(metrics) - skippedCount
	logEvent := log.Info().
		Int("metric_count", successCount).
		Int("payload_size_bytes", payloadSize).
		Str("endpoint", s.endpoint)

	if skippedCount > 0 {
		logEvent = logEvent.Int("skipped_count", skippedCount)
	}

	logEvent.Msg("Successfully shipped metrics to Splunk HEC")

	return nil
}

// Close cleans up resources
func (s *SplunkHECShipper) Close() error {
	s.client.CloseIdleConnections()
	return nil
}

// logPayloadToFile writes the payload to a debug log file
func (s *SplunkHECShipper) logPayloadToFile(payload string) {
	f, err := os.OpenFile(s.debugLogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Error().Err(err).Str("file", s.debugLogFile).Msg("Failed to open debug log file")
		return
	}
	defer f.Close()

	timestamp := time.Now().Format(time.RFC3339)
	header := fmt.Sprintf("\n=== Splunk HEC Payload at %s ===\n", timestamp)
	if _, err := f.WriteString(header + payload + "\n"); err != nil {
		log.Error().Err(err).Msg("Failed to write to debug log file")
	}
}

