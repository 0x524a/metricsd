package collector

import (
	"context"
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

// SystemCollector collects OS-level metrics using gopsutil (Single Responsibility Principle)
type SystemCollector struct {
	enableCPU     bool
	enableMemory  bool
	enableDisk    bool
	enableNetwork bool
}

// NewSystemCollector creates a new system metrics collector
func NewSystemCollector(enableCPU, enableMemory, enableDisk, enableNetwork bool) *SystemCollector {
	return &SystemCollector{
		enableCPU:     enableCPU,
		enableMemory:  enableMemory,
		enableDisk:    enableDisk,
		enableNetwork: enableNetwork,
	}
}

// Name returns the collector name
func (c *SystemCollector) Name() string {
	return "system"
}

// Collect gathers system metrics
func (c *SystemCollector) Collect(ctx context.Context) ([]Metric, error) {
	metrics := make([]Metric, 0)

	if c.enableCPU {
		cpuMetrics, err := c.collectCPU(ctx)
		if err == nil {
			metrics = append(metrics, cpuMetrics...)
		}
	}

	if c.enableMemory {
		memMetrics, err := c.collectMemory(ctx)
		if err == nil {
			metrics = append(metrics, memMetrics...)
		}
	}

	if c.enableDisk {
		diskMetrics, err := c.collectDisk(ctx)
		if err == nil {
			metrics = append(metrics, diskMetrics...)
		}
	}

	if c.enableNetwork {
		netMetrics, err := c.collectNetwork(ctx)
		if err == nil {
			metrics = append(metrics, netMetrics...)
		}
	}

	return metrics, nil
}

func (c *SystemCollector) collectCPU(ctx context.Context) ([]Metric, error) {
	metrics := make([]Metric, 0)

	// CPU usage per core
	percentages, err := cpu.PercentWithContext(ctx, 1*time.Second, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get CPU percentages: %w", err)
	}

	for i, percent := range percentages {
		metrics = append(metrics, Metric{
			Name: "system_cpu_usage_percent",
			Labels: map[string]string{
				"core": fmt.Sprintf("%d", i),
			},
			Value: percent,
			Type:  "gauge",
		})
	}

	// Overall CPU usage
	totalPercent, err := cpu.PercentWithContext(ctx, 1*time.Second, false)
	if err == nil && len(totalPercent) > 0 {
		metrics = append(metrics, Metric{
			Name:   "system_cpu_usage_total_percent",
			Labels: map[string]string{},
			Value:  totalPercent[0],
			Type:   "gauge",
		})
	}

	// CPU count
	count, err := cpu.CountsWithContext(ctx, true)
	if err == nil {
		metrics = append(metrics, Metric{
			Name:   "system_cpu_count",
			Labels: map[string]string{},
			Value:  float64(count),
			Type:   "gauge",
		})
	}

	return metrics, nil
}

func (c *SystemCollector) collectMemory(ctx context.Context) ([]Metric, error) {
	metrics := make([]Metric, 0)

	vmStat, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get memory stats: %w", err)
	}

	metrics = append(metrics,
		Metric{
			Name:   "system_memory_total_bytes",
			Labels: map[string]string{},
			Value:  float64(vmStat.Total),
			Type:   "gauge",
		},
		Metric{
			Name:   "system_memory_used_bytes",
			Labels: map[string]string{},
			Value:  float64(vmStat.Used),
			Type:   "gauge",
		},
		Metric{
			Name:   "system_memory_available_bytes",
			Labels: map[string]string{},
			Value:  float64(vmStat.Available),
			Type:   "gauge",
		},
		Metric{
			Name:   "system_memory_usage_percent",
			Labels: map[string]string{},
			Value:  vmStat.UsedPercent,
			Type:   "gauge",
		},
	)

	// Swap memory
	swapStat, err := mem.SwapMemoryWithContext(ctx)
	if err == nil {
		metrics = append(metrics,
			Metric{
				Name:   "system_swap_total_bytes",
				Labels: map[string]string{},
				Value:  float64(swapStat.Total),
				Type:   "gauge",
			},
			Metric{
				Name:   "system_swap_used_bytes",
				Labels: map[string]string{},
				Value:  float64(swapStat.Used),
				Type:   "gauge",
			},
			Metric{
				Name:   "system_swap_usage_percent",
				Labels: map[string]string{},
				Value:  swapStat.UsedPercent,
				Type:   "gauge",
			},
		)
	}

	return metrics, nil
}

func (c *SystemCollector) collectDisk(ctx context.Context) ([]Metric, error) {
	metrics := make([]Metric, 0)

	// Disk usage per partition
	partitions, err := disk.PartitionsWithContext(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get disk partitions: %w", err)
	}

	for _, partition := range partitions {
		usage, err := disk.UsageWithContext(ctx, partition.Mountpoint)
		if err != nil {
			continue
		}

		labels := map[string]string{
			"device":     partition.Device,
			"mountpoint": partition.Mountpoint,
			"fstype":     partition.Fstype,
		}

		metrics = append(metrics,
			Metric{
				Name:   "system_disk_total_bytes",
				Labels: labels,
				Value:  float64(usage.Total),
				Type:   "gauge",
			},
			Metric{
				Name:   "system_disk_used_bytes",
				Labels: labels,
				Value:  float64(usage.Used),
				Type:   "gauge",
			},
			Metric{
				Name:   "system_disk_free_bytes",
				Labels: labels,
				Value:  float64(usage.Free),
				Type:   "gauge",
			},
			Metric{
				Name:   "system_disk_usage_percent",
				Labels: labels,
				Value:  usage.UsedPercent,
				Type:   "gauge",
			},
		)
	}

	// Disk I/O stats
	ioStats, err := disk.IOCountersWithContext(ctx)
	if err == nil {
		for device, stats := range ioStats {
			labels := map[string]string{"device": device}

			metrics = append(metrics,
				Metric{
					Name:   "system_disk_read_bytes_total",
					Labels: labels,
					Value:  float64(stats.ReadBytes),
					Type:   "counter",
				},
				Metric{
					Name:   "system_disk_write_bytes_total",
					Labels: labels,
					Value:  float64(stats.WriteBytes),
					Type:   "counter",
				},
				Metric{
					Name:   "system_disk_read_count_total",
					Labels: labels,
					Value:  float64(stats.ReadCount),
					Type:   "counter",
				},
				Metric{
					Name:   "system_disk_write_count_total",
					Labels: labels,
					Value:  float64(stats.WriteCount),
					Type:   "counter",
				},
			)
		}
	}

	return metrics, nil
}

func (c *SystemCollector) collectNetwork(ctx context.Context) ([]Metric, error) {
	metrics := make([]Metric, 0)

	ioStats, err := net.IOCountersWithContext(ctx, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get network stats: %w", err)
	}

	for _, stats := range ioStats {
		labels := map[string]string{"interface": stats.Name}

		metrics = append(metrics,
			Metric{
				Name:   "system_network_bytes_sent_total",
				Labels: labels,
				Value:  float64(stats.BytesSent),
				Type:   "counter",
			},
			Metric{
				Name:   "system_network_bytes_recv_total",
				Labels: labels,
				Value:  float64(stats.BytesRecv),
				Type:   "counter",
			},
			Metric{
				Name:   "system_network_packets_sent_total",
				Labels: labels,
				Value:  float64(stats.PacketsSent),
				Type:   "counter",
			},
			Metric{
				Name:   "system_network_packets_recv_total",
				Labels: labels,
				Value:  float64(stats.PacketsRecv),
				Type:   "counter",
			},
			Metric{
				Name:   "system_network_errors_in_total",
				Labels: labels,
				Value:  float64(stats.Errin),
				Type:   "counter",
			},
			Metric{
				Name:   "system_network_errors_out_total",
				Labels: labels,
				Value:  float64(stats.Errout),
				Type:   "counter",
			},
			Metric{
				Name:   "system_network_drop_in_total",
				Labels: labels,
				Value:  float64(stats.Dropin),
				Type:   "counter",
			},
			Metric{
				Name:   "system_network_drop_out_total",
				Labels: labels,
				Value:  float64(stats.Dropout),
				Type:   "counter",
			},
		)
	}

	return metrics, nil
}
