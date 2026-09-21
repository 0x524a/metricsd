package main

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/0x524A/metricsd/internal/config"
)

func minimalConfig(t *testing.T, port int) *config.Config {
	tmpDir := t.TempDir()
	filePath := tmpDir + "/metrics.json"

	return &config.Config{
		Server: config.ServerConfig{
			Host: "127.0.0.1",
			Port: port,
		},
		Collector: config.CollectorConfig{
			IntervalSeconds: 1,
			EnableCPU:       false,
			EnableMemory:    false,
			EnableDisk:      false,
			EnableNetwork:   false,
			EnableGPU:       false,
			Plugins: config.PluginSystemConfig{
				Enabled: false,
			},
		},
		Shipper: config.ShipperConfig{
			Type:    "json_file",
			Timeout: 5 * time.Second,
			File: config.FileShipperConfig{
				Path:      filePath,
				MaxSizeMB: 100,
				MaxFiles:  5,
				Format:    "single",
			},
		},
		Endpoints: []config.EndpointConfig{},
	}
}

func TestRun_HappyPathShutdownViaSignal(t *testing.T) {
	cfg := minimalConfig(t, 0)
	sigChan := make(chan os.Signal, 1)

	done := make(chan error, 1)
	go func() {
		done <- run(context.Background(), cfg, sigChan)
	}()

	select {
	case <-time.After(2 * time.Second):
		sigChan <- syscall.SIGTERM
	case err := <-done:
		t.Fatalf("run() returned too early: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run() returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run() did not return within timeout after signal")
	}
}

func TestRun_ShutdownViaParentContextCancellation(t *testing.T) {
	cfg := minimalConfig(t, 0)
	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)

	done := make(chan error, 1)
	go func() {
		done <- run(ctx, cfg, sigChan)
	}()

	select {
	case <-time.After(1 * time.Second):
		cancel()
	case err := <-done:
		t.Fatalf("run() returned too early: %v", err)
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		sigChan <- syscall.SIGTERM
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run() returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run() did not return within timeout")
	}
}

func TestRun_DisabledCollectorsConfig(t *testing.T) {
	cfg := minimalConfig(t, 0)
	cfg.Collector.EnableCPU = false
	cfg.Collector.EnableMemory = false
	cfg.Collector.EnableDisk = false
	cfg.Collector.EnableNetwork = false
	cfg.Collector.EnableGPU = false
	cfg.Endpoints = []config.EndpointConfig{}

	sigChan := make(chan os.Signal, 1)

	done := make(chan error, 1)
	go func() {
		done <- run(context.Background(), cfg, sigChan)
	}()

	select {
	case <-time.After(2 * time.Second):
		sigChan <- syscall.SIGTERM
	case err := <-done:
		t.Fatalf("run() returned too early: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run() returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run() did not return within timeout after signal")
	}
}
