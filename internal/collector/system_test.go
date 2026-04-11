package collector

import (
	"context"
	"strings"
	"testing"
)

func TestNewSystemCollector(t *testing.T) {
	t.Run("all enabled", func(t *testing.T) {
		c := NewSystemCollector(true, true, true, true)
		if c == nil {
			t.Fatal("expected non-nil collector")
		}
		if !c.enableCPU {
			t.Error("expected enableCPU to be true")
		}
		if !c.enableMemory {
			t.Error("expected enableMemory to be true")
		}
		if !c.enableDisk {
			t.Error("expected enableDisk to be true")
		}
		if !c.enableNetwork {
			t.Error("expected enableNetwork to be true")
		}
	})

	t.Run("all disabled", func(t *testing.T) {
		c := NewSystemCollector(false, false, false, false)
		if c == nil {
			t.Fatal("expected non-nil collector")
		}
		if c.enableCPU {
			t.Error("expected enableCPU to be false")
		}
		if c.enableMemory {
			t.Error("expected enableMemory to be false")
		}
		if c.enableDisk {
			t.Error("expected enableDisk to be false")
		}
		if c.enableNetwork {
			t.Error("expected enableNetwork to be false")
		}
	})

	t.Run("mixed flags", func(t *testing.T) {
		c := NewSystemCollector(true, false, true, false)
		if !c.enableCPU {
			t.Error("expected enableCPU to be true")
		}
		if c.enableMemory {
			t.Error("expected enableMemory to be false")
		}
		if !c.enableDisk {
			t.Error("expected enableDisk to be true")
		}
		if c.enableNetwork {
			t.Error("expected enableNetwork to be false")
		}
	})
}

func TestSystemCollector_Name(t *testing.T) {
	c := NewSystemCollector(false, false, false, false)
	if got := c.Name(); got != "system" {
		t.Errorf("Name() = %q, want %q", got, "system")
	}
}

func TestSystemCollector_Collect(t *testing.T) {
	c := NewSystemCollector(true, true, true, true)
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() returned unexpected error: %v", err)
	}
	if len(metrics) == 0 {
		t.Fatal("Collect() returned no metrics, expected > 0")
	}

	for _, m := range metrics {
		if !strings.HasPrefix(m.Name, "system_") {
			t.Errorf("metric %q does not have expected prefix 'system_'", m.Name)
		}
		if m.Type != "gauge" && m.Type != "counter" {
			t.Errorf("metric %q has invalid type %q, want 'gauge' or 'counter'", m.Name, m.Type)
		}
	}
}

func TestSystemCollector_CollectCPUOnly(t *testing.T) {
	c := NewSystemCollector(true, false, false, false)
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() returned unexpected error: %v", err)
	}

	// Every metric must be either a host metric or a CPU metric — no memory/disk/network.
	for _, m := range metrics {
		isHost := strings.HasPrefix(m.Name, "system_uptime_") ||
			strings.HasPrefix(m.Name, "system_boot_") ||
			strings.HasPrefix(m.Name, "system_load_") ||
			strings.HasPrefix(m.Name, "system_procs_")
		isCPU := strings.HasPrefix(m.Name, "system_cpu_")

		if !isHost && !isCPU {
			t.Errorf("unexpected metric %q when only CPU is enabled", m.Name)
		}
	}

	// At least one CPU metric should be present.
	found := false
	for _, m := range metrics {
		if strings.HasPrefix(m.Name, "system_cpu_") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected at least one system_cpu_ metric when CPU collection is enabled")
	}
}

func TestSystemCollector_CollectNothingEnabled(t *testing.T) {
	c := NewSystemCollector(false, false, false, false)
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() returned unexpected error: %v", err)
	}

	// Host metrics (uptime, boot time, load, procs) are always collected.
	if len(metrics) == 0 {
		t.Fatal("Collect() returned no metrics; host metrics should always be present")
	}

	for _, m := range metrics {
		isHost := strings.HasPrefix(m.Name, "system_uptime_") ||
			strings.HasPrefix(m.Name, "system_boot_") ||
			strings.HasPrefix(m.Name, "system_load_") ||
			strings.HasPrefix(m.Name, "system_procs_")
		if !isHost {
			t.Errorf("unexpected non-host metric %q when all subsystems are disabled", m.Name)
		}
	}
}
