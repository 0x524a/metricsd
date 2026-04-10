package shipper

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/0x524A/metricsd/internal/collector"
)

// FileShipper writes metrics to a local file in JSON format for Splunk Universal Forwarder
type FileShipper struct {
	filePath     string
	maxSizeBytes int64
	maxFiles     int
	format       string // "single" or "multi"
	file         *os.File
	mu           sync.Mutex
}

// FileMetricEvent represents a single metric event in JSON Lines format (format: "single")
type FileMetricEvent struct {
	Timestamp  int64             `json:"timestamp"`
	MetricName string            `json:"metric_name"`
	Value      float64           `json:"value"`
	MetricType string            `json:"metric_type"`
	Host       string            `json:"host"`
	Source     string            `json:"source"`
	Labels     map[string]string `json:"labels,omitempty"`
}

// SplunkMultiMetricEvent represents multiple metrics in a single event for Splunk HEC (format: "multi")
// See: https://docs.splunk.com/Documentation/Splunk/latest/Metrics/GetMetricsInOther#The_multiple-metric_JSON_format
type SplunkMultiMetricEvent struct {
	Time   int64                  `json:"time"`
	Event  string                 `json:"event"`
	Host   string                 `json:"host"`
	Source string                 `json:"source"`
	Fields map[string]interface{} `json:"fields"`
}

// NewFileShipper creates a new file shipper for JSON output
// format can be "single" (one metric per line) or "multi" (Splunk multi-metric JSON)
func NewFileShipper(filePath string, maxSizeMB int, maxFiles int, format string) (*FileShipper, error) {
	// Ensure the directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Set defaults
	if maxSizeMB <= 0 {
		maxSizeMB = 100 // Default 100MB
	}
	if maxFiles <= 0 {
		maxFiles = 5 // Default keep 5 rotated files
	}
	if format == "" {
		format = "single" // Default to single metric per line
	}

	shipper := &FileShipper{
		filePath:     filePath,
		maxSizeBytes: int64(maxSizeMB) * 1024 * 1024,
		maxFiles:     maxFiles,
		format:       format,
	}

	// Open the file
	if err := shipper.openFile(); err != nil {
		return nil, err
	}

	return shipper, nil
}

func (s *FileShipper) openFile() error {
	file, err := os.OpenFile(s.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", s.filePath, err)
	}
	s.file = file
	return nil
}

// Ship writes metrics to the file in the configured JSON format
func (s *FileShipper) Ship(ctx context.Context, metrics []collector.Metric) error {
	if len(metrics) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if rotation is needed
	if err := s.checkRotation(); err != nil {
		log.Warn().Err(err).Msg("Failed to check/perform file rotation")
	}

	var err error
	var bytesWritten int
	if s.format == "multi" {
		bytesWritten, err = s.shipMultiMetric(metrics)
	} else {
		bytesWritten, err = s.shipSingleMetric(metrics)
	}

	if err != nil {
		return err
	}

	// Sync to ensure data is written to disk
	if err := s.file.Sync(); err != nil {
		log.Warn().Err(err).Msg("Failed to sync file")
	}

	log.Info().
		Int("metric_count", len(metrics)).
		Int("payload_size_bytes", bytesWritten).
		Str("file", s.filePath).
		Str("format", s.format).
		Msg("Successfully wrote metrics to file")

	return nil
}

// shipSingleMetric writes each metric as a separate JSON line
func (s *FileShipper) shipSingleMetric(metrics []collector.Metric) (int, error) {
	hostname, _ := os.Hostname()
	timestamp := time.Now().Unix()
	totalBytes := 0

	for _, metric := range metrics {
		event := FileMetricEvent{
			Timestamp:  timestamp,
			MetricName: metric.Name,
			Value:      metric.Value,
			MetricType: metric.Type,
			Host:       hostname,
			Source:     "metricsd",
			Labels:     metric.Labels,
		}

		data, err := json.Marshal(event)
		if err != nil {
			log.Warn().Err(err).Str("metric", metric.Name).Msg("Failed to marshal metric")
			continue
		}

		n, err := s.file.Write(append(data, '\n'))
		if err != nil {
			return totalBytes, fmt.Errorf("failed to write metric to file: %w", err)
		}
		totalBytes += n
	}

	return totalBytes, nil
}

// shipMultiMetric writes all metrics as a single Splunk multi-metric JSON event
func (s *FileShipper) shipMultiMetric(metrics []collector.Metric) (int, error) {
	hostname, _ := os.Hostname()
	timestamp := time.Now().Unix()

	// Build the fields map with metric_name:<name> keys for values
	fields := make(map[string]interface{})

	// Collect all unique dimension keys and their values
	dimensionValues := make(map[string]string)

	for _, metric := range metrics {
		// Add metric value with metric_name:<name> key format
		metricKey := fmt.Sprintf("metric_name:%s", metric.Name)
		fields[metricKey] = metric.Value

		// Collect dimension labels (will use last value if duplicates exist)
		for k, v := range metric.Labels {
			dimensionValues[k] = v
		}
	}

	// Add all dimensions as flat fields
	for k, v := range dimensionValues {
		fields[k] = v
	}

	event := SplunkMultiMetricEvent{
		Time:   timestamp,
		Event:  "metric",
		Host:   hostname,
		Source: "metricsd",
		Fields: fields,
	}

	data, err := json.Marshal(event)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal multi-metric event: %w", err)
	}

	n, err := s.file.Write(append(data, '\n'))
	if err != nil {
		return 0, fmt.Errorf("failed to write multi-metric event to file: %w", err)
	}

	return n, nil
}

func (s *FileShipper) checkRotation() error {
	info, err := s.file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	if info.Size() < s.maxSizeBytes {
		return nil
	}

	log.Info().
		Int64("size_bytes", info.Size()).
		Int64("max_bytes", s.maxSizeBytes).
		Msg("Rotating log file")

	return s.rotate()
}

func (s *FileShipper) rotate() error {
	// Close current file
	if err := s.file.Close(); err != nil {
		return fmt.Errorf("failed to close file for rotation: %w", err)
	}

	// Rotate existing files (delete oldest, rename others)
	for i := s.maxFiles - 1; i >= 1; i-- {
		oldPath := fmt.Sprintf("%s.%d", s.filePath, i)
		newPath := fmt.Sprintf("%s.%d", s.filePath, i+1)

		if i == s.maxFiles-1 {
			// Delete the oldest file
			os.Remove(oldPath)
		} else {
			// Rename to next number
			os.Rename(oldPath, newPath)
		}
	}

	// Rename current file to .1
	if err := os.Rename(s.filePath, s.filePath+".1"); err != nil {
		// If rename fails, try to reopen original file
		if openErr := s.openFile(); openErr != nil {
			return fmt.Errorf("failed to rename and reopen file: rename=%w, open=%v", err, openErr)
		}
		return fmt.Errorf("failed to rename file: %w", err)
	}

	// Open new file
	return s.openFile()
}

// Close closes the file
func (s *FileShipper) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.file != nil {
		return s.file.Close()
	}
	return nil
}
