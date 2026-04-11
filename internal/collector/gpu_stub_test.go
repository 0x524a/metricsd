//go:build !nvidia
// +build !nvidia

package collector

import (
	"context"
	"testing"
)

func TestGPUCollector_Name(t *testing.T) {
	c := NewGPUCollector()
	if got := c.Name(); got != "gpu" {
		t.Errorf("Name() = %q, want %q", got, "gpu")
	}
}

func TestGPUCollector_Collect(t *testing.T) {
	c := NewGPUCollector()
	metrics, err := c.Collect(context.Background())
	if err == nil {
		t.Error("Collect() expected an error for stub GPU collector, got nil")
	}
	if metrics != nil {
		t.Errorf("Collect() expected nil metrics for stub, got %v", metrics)
	}
}

func TestGPUCollector_Shutdown(t *testing.T) {
	c := NewGPUCollector()
	if err := c.Shutdown(); err != nil {
		t.Errorf("Shutdown() expected nil error, got %v", err)
	}
}
