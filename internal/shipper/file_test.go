package shipper

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/0x524A/metricsd/internal/collector"
)

func TestFileShipper_Ship(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "metricsd-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "metrics.json")

	// Create shipper
	shipper, err := NewFileShipper(filePath, 100, 5, "single")
	if err != nil {
		t.Fatalf("Failed to create file shipper: %v", err)
	}
	defer shipper.Close()

	// Create test metrics
	metrics := []collector.Metric{
		{
			Name:   "cpu_usage",
			Value:  45.5,
			Type:   "gauge",
			Labels: map[string]string{"core": "0"},
		},
		{
			Name:   "memory_used_bytes",
			Value:  8589934592,
			Type:   "gauge",
			Labels: map[string]string{"host": "localhost"},
		},
	}

	// Ship metrics
	ctx := context.Background()
	err = shipper.Ship(ctx, metrics)
	if err != nil {
		t.Fatalf("Ship failed: %v", err)
	}

	// Close to flush
	shipper.Close()

	// Read and verify file contents
	file, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("Failed to open output file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
		var event FileMetricEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			t.Errorf("Failed to parse JSON line %d: %v", lineCount, err)
			continue
		}

		if event.Timestamp <= 0 {
			t.Errorf("Line %d: Expected positive timestamp, got %d", lineCount, event.Timestamp)
		}
		if event.Source != "metricsd" {
			t.Errorf("Line %d: Expected source 'metricsd', got %q", lineCount, event.Source)
		}
		if event.Host == "" {
			t.Errorf("Line %d: Expected non-empty host", lineCount)
		}
	}

	if lineCount != 2 {
		t.Errorf("Expected 2 lines, got %d", lineCount)
	}
}

func TestFileShipper_EmptyMetrics(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "metricsd-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "metrics.json")

	shipper, err := NewFileShipper(filePath, 100, 5, "single")
	if err != nil {
		t.Fatalf("Failed to create file shipper: %v", err)
	}
	defer shipper.Close()

	// Ship empty metrics should succeed without writing
	err = shipper.Ship(context.Background(), []collector.Metric{})
	if err != nil {
		t.Errorf("Expected no error for empty metrics, got: %v", err)
	}

	// File should exist but be empty
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}
	if info.Size() != 0 {
		t.Errorf("Expected empty file, got size %d", info.Size())
	}
}

func TestFileShipper_CreateDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "metricsd-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Use a nested path that doesn't exist
	filePath := filepath.Join(tmpDir, "subdir", "nested", "metrics.json")

	shipper, err := NewFileShipper(filePath, 100, 5, "single")
	if err != nil {
		t.Fatalf("Failed to create file shipper with nested path: %v", err)
	}
	defer shipper.Close()

	// Verify directory was created
	dir := filepath.Dir(filePath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("Expected directory to be created")
	}
}

func TestFileShipper_Rotation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "metricsd-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "metrics.json")

	// Create shipper with very small max size to trigger rotation (1 byte)
	shipper := &FileShipper{
		filePath:     filePath,
		maxSizeBytes: 1, // 1 byte to force immediate rotation
		maxFiles:     3,
		format:       "single",
	}
	if err := shipper.openFile(); err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}

	metrics := []collector.Metric{
		{Name: "test_metric", Value: 1.0, Type: "gauge"},
	}

	// Ship multiple times to trigger rotation
	for i := 0; i < 5; i++ {
		if err := shipper.Ship(context.Background(), metrics); err != nil {
			t.Fatalf("Ship %d failed: %v", i, err)
		}
	}
	shipper.Close()

	// Check that rotated files exist
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("Expected main file to exist")
	}
	if _, err := os.Stat(filePath + ".1"); os.IsNotExist(err) {
		t.Error("Expected rotated file .1 to exist")
	}
	if _, err := os.Stat(filePath + ".2"); os.IsNotExist(err) {
		t.Error("Expected rotated file .2 to exist")
	}
	// .3 should not exist because maxFiles is 3
	if _, err := os.Stat(filePath + ".3"); !os.IsNotExist(err) {
		t.Error("Expected rotated file .3 to NOT exist (maxFiles=3)")
	}
}

func TestFileShipper_JSONFormat(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "metricsd-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "metrics.json")

	shipper, err := NewFileShipper(filePath, 100, 5, "single")
	if err != nil {
		t.Fatalf("Failed to create file shipper: %v", err)
	}

	metrics := []collector.Metric{
		{
			Name:   "system_cpu_usage",
			Value:  42.5,
			Type:   "gauge",
			Labels: map[string]string{"cpu": "0", "mode": "user"},
		},
	}

	err = shipper.Ship(context.Background(), metrics)
	if err != nil {
		t.Fatalf("Ship failed: %v", err)
	}
	shipper.Close()

	// Read file and parse JSON
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	var event FileMetricEvent
	if err := json.Unmarshal(data[:len(data)-1], &event); err != nil { // -1 to remove newline
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Verify all fields
	if event.MetricName != "system_cpu_usage" {
		t.Errorf("Expected metric_name 'system_cpu_usage', got %q", event.MetricName)
	}
	if event.Value != 42.5 {
		t.Errorf("Expected value 42.5, got %f", event.Value)
	}
	if event.MetricType != "gauge" {
		t.Errorf("Expected metric_type 'gauge', got %q", event.MetricType)
	}
	if event.Source != "metricsd" {
		t.Errorf("Expected source 'metricsd', got %q", event.Source)
	}
	if event.Labels["cpu"] != "0" {
		t.Errorf("Expected label cpu='0', got %q", event.Labels["cpu"])
	}
	if event.Labels["mode"] != "user" {
		t.Errorf("Expected label mode='user', got %q", event.Labels["mode"])
	}
}

func TestFileShipper_MultiMetricFormat(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "metricsd-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "metrics.json")

	// Create shipper with multi-metric format
	shipper, err := NewFileShipper(filePath, 100, 5, "multi")
	if err != nil {
		t.Fatalf("Failed to create file shipper: %v", err)
	}

	metrics := []collector.Metric{
		{
			Name:   "system_cpu_usage",
			Value:  42.5,
			Type:   "gauge",
			Labels: map[string]string{"cpu": "0"},
		},
		{
			Name:   "system_memory_used",
			Value:  8589934592,
			Type:   "gauge",
			Labels: map[string]string{"host": "localhost"},
		},
		{
			Name:   "system_disk_usage",
			Value:  75.2,
			Type:   "gauge",
			Labels: map[string]string{"mountpoint": "/"},
		},
	}

	err = shipper.Ship(context.Background(), metrics)
	if err != nil {
		t.Fatalf("Ship failed: %v", err)
	}
	shipper.Close()

	// Read file and parse JSON
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	var event SplunkMultiMetricEvent
	if err := json.Unmarshal(data[:len(data)-1], &event); err != nil { // -1 to remove newline
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Verify Splunk multi-metric format structure
	if event.Event != "metric" {
		t.Errorf("Expected event 'metric', got %q", event.Event)
	}
	if event.Source != "metricsd" {
		t.Errorf("Expected source 'metricsd', got %q", event.Source)
	}
	if event.Host == "" {
		t.Error("Expected non-empty host")
	}
	if event.Time <= 0 {
		t.Errorf("Expected positive time, got %d", event.Time)
	}

	// Verify metric values are in fields with metric_name:<name> format
	if cpuVal, ok := event.Fields["metric_name:system_cpu_usage"]; !ok {
		t.Error("Expected field 'metric_name:system_cpu_usage' not found")
	} else if cpuVal.(float64) != 42.5 {
		t.Errorf("Expected cpu value 42.5, got %v", cpuVal)
	}

	if memVal, ok := event.Fields["metric_name:system_memory_used"]; !ok {
		t.Error("Expected field 'metric_name:system_memory_used' not found")
	} else if memVal.(float64) != 8589934592 {
		t.Errorf("Expected memory value 8589934592, got %v", memVal)
	}

	if diskVal, ok := event.Fields["metric_name:system_disk_usage"]; !ok {
		t.Error("Expected field 'metric_name:system_disk_usage' not found")
	} else if diskVal.(float64) != 75.2 {
		t.Errorf("Expected disk value 75.2, got %v", diskVal)
	}

	// Verify dimensions are included as flat fields
	if cpu, ok := event.Fields["cpu"]; !ok || cpu != "0" {
		t.Errorf("Expected dimension cpu='0', got %v", event.Fields["cpu"])
	}
	if host, ok := event.Fields["host"]; !ok || host != "localhost" {
		t.Errorf("Expected dimension host='localhost', got %v", event.Fields["host"])
	}
	if mp, ok := event.Fields["mountpoint"]; !ok || mp != "/" {
		t.Errorf("Expected dimension mountpoint='/', got %v", event.Fields["mountpoint"])
	}
}

func TestFileShipper_MultiMetricSingleEvent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "metricsd-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "metrics.json")

	shipper, err := NewFileShipper(filePath, 100, 5, "multi")
	if err != nil {
		t.Fatalf("Failed to create file shipper: %v", err)
	}

	// Ship 10 metrics
	metrics := make([]collector.Metric, 10)
	for i := 0; i < 10; i++ {
		metrics[i] = collector.Metric{
			Name:   "test_metric",
			Value:  float64(i),
			Type:   "gauge",
			Labels: map[string]string{},
		}
	}

	err = shipper.Ship(context.Background(), metrics)
	if err != nil {
		t.Fatalf("Ship failed: %v", err)
	}
	shipper.Close()

	// Read file - should only have 1 line (all metrics in single event)
	file, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
	}

	if lineCount != 1 {
		t.Errorf("Expected 1 line for multi-metric format, got %d", lineCount)
	}
}

func TestFileShipper_DefaultFormat(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "metricsd-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "metrics.json")

	// Create shipper with empty format (should default to "single")
	shipper, err := NewFileShipper(filePath, 100, 5, "")
	if err != nil {
		t.Fatalf("Failed to create file shipper: %v", err)
	}

	metrics := []collector.Metric{
		{Name: "test1", Value: 1.0, Type: "gauge"},
		{Name: "test2", Value: 2.0, Type: "gauge"},
	}

	err = shipper.Ship(context.Background(), metrics)
	if err != nil {
		t.Fatalf("Ship failed: %v", err)
	}
	shipper.Close()

	// Read file - should have 2 lines (single format)
	file, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
	}

	if lineCount != 2 {
		t.Errorf("Expected 2 lines for single format, got %d", lineCount)
	}
}
