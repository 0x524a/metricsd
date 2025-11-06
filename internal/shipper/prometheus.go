package shipper

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/golang/snappy"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
	"github.com/rs/zerolog/log"

	"github.com/0x524A/metricsd/internal/collector"
)

// PrometheusRemoteWriteShipper ships metrics using Prometheus remote write protocol (Single Responsibility Principle)
type PrometheusRemoteWriteShipper struct {
	endpoint string
	client   *http.Client
}

// NewPrometheusRemoteWriteShipper creates a new Prometheus remote write shipper
func NewPrometheusRemoteWriteShipper(endpoint string, tlsEnabled bool, certFile, keyFile, caFile string, insecureSkipVerify bool, timeout time.Duration) (*PrometheusRemoteWriteShipper, error) {
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

	return &PrometheusRemoteWriteShipper{
		endpoint: endpoint,
		client:   client,
	}, nil
}

// Ship sends metrics to the Prometheus remote write endpoint
func (s *PrometheusRemoteWriteShipper) Ship(ctx context.Context, metrics []collector.Metric) error {
	if len(metrics) == 0 {
		return nil
	}

	// Convert to Prometheus TimeSeries
	timeseries := s.convertToTimeSeries(metrics)

	// Create WriteRequest
	writeRequest := &prompb.WriteRequest{
		Timeseries: timeseries,
	}

	// Marshal to protobuf
	data, err := writeRequest.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal write request: %w", err)
	}

	// Compress with Snappy
	compressed := snappy.Encode(nil, data)

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(compressed))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Encoding", "snappy")
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")

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
		Msg("Successfully shipped metrics via Prometheus remote write")

	return nil
}

func (s *PrometheusRemoteWriteShipper) convertToTimeSeries(metrics []collector.Metric) []prompb.TimeSeries {
	now := time.Now().UnixMilli()
	timeseries := make([]prompb.TimeSeries, 0, len(metrics))

	for _, metric := range metrics {
		labels := []prompb.Label{
			{Name: model.MetricNameLabel, Value: metric.Name},
		}

		for k, v := range metric.Labels {
			labels = append(labels, prompb.Label{
				Name:  k,
				Value: v,
			})
		}

		timeseries = append(timeseries, prompb.TimeSeries{
			Labels: labels,
			Samples: []prompb.Sample{
				{
					Value:     metric.Value,
					Timestamp: now,
				},
			},
		})
	}

	return timeseries
}

// Close cleans up resources
func (s *PrometheusRemoteWriteShipper) Close() error {
	s.client.CloseIdleConnections()
	return nil
}

// ConvertToPrometheusMetrics converts collected metrics to Prometheus format (utility function)
func ConvertToPrometheusMetrics(metrics []collector.Metric) []prometheus.Metric {
	return collector.ToPrometheusMetrics(metrics)
}
