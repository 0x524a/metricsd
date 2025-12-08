//go:build !linux || !cgo

package collector

import (
	"context"
	"fmt"
)

// GPUCollector is a stub for non-Linux systems
type GPUCollector struct {
	initialized bool
}

// NewGPUCollector creates a new GPU metrics collector (stub)
func NewGPUCollector() *GPUCollector {
	return &GPUCollector{
		initialized: false,
	}
}

// Name returns the collector name
func (c *GPUCollector) Name() string {
	return "gpu"
}

// Collect returns an error on non-Linux systems
func (c *GPUCollector) Collect(ctx context.Context) ([]Metric, error) {
	return nil, fmt.Errorf("GPU collector is only supported on Linux with NVIDIA drivers")
}

// Shutdown is a no-op on non-Linux systems
func (c *GPUCollector) Shutdown() error {
	return nil
}
