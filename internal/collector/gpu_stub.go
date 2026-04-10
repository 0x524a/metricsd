//go:build !nvidia
// +build !nvidia

package collector

import (
	"context"
	"fmt"
)

// GPUCollector collects GPU metrics using NVML (Single Responsibility Principle)
type GPUCollector struct {
	initialized bool
}

// NewGPUCollector creates a new GPU metrics collector
func NewGPUCollector() *GPUCollector {
	return &GPUCollector{
		initialized: false,
	}
}

// Name returns the collector name
func (c *GPUCollector) Name() string {
	return "gpu"
}

// Collect gathers GPU metrics (stub - returns error when NVIDIA support not built)
func (c *GPUCollector) Collect(ctx context.Context) ([]Metric, error) {
	return nil, fmt.Errorf("GPU metrics not available: binary not built with NVIDIA support (use -tags nvidia)")
}

// Shutdown cleans up NVML resources (stub - no-op)
func (c *GPUCollector) Shutdown() error {
	return nil
}
