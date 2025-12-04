//go:build linux && cgo

package collector

import (
	"context"
	"fmt"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
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

// Collect gathers GPU metrics
func (c *GPUCollector) Collect(ctx context.Context) ([]Metric, error) {
	// Initialize NVML if not already done
	if !c.initialized {
		ret := nvml.Init()
		if ret != nvml.SUCCESS {
			return nil, fmt.Errorf("failed to initialize NVML: %v", nvml.ErrorString(ret))
		}
		c.initialized = true
	}

	metrics := make([]Metric, 0)

	// Get device count
	count, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf("failed to get device count: %v", nvml.ErrorString(ret))
	}

	metrics = append(metrics, Metric{
		Name:   "system_gpu_count",
		Labels: map[string]string{},
		Value:  float64(count),
		Type:   "gauge",
	})

	// Collect metrics for each GPU
	for i := 0; i < count; i++ {
		device, ret := nvml.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			continue
		}

		deviceMetrics, err := c.collectDeviceMetrics(device, i)
		if err == nil {
			metrics = append(metrics, deviceMetrics...)
		}
	}

	return metrics, nil
}

func (c *GPUCollector) collectDeviceMetrics(device nvml.Device, index int) ([]Metric, error) {
	metrics := make([]Metric, 0)
	labels := map[string]string{"gpu": fmt.Sprintf("%d", index)}

	// Get device name
	name, ret := device.GetName()
	if ret == nvml.SUCCESS {
		labels["name"] = name
	}

	// GPU utilization
	utilization, ret := device.GetUtilizationRates()
	if ret == nvml.SUCCESS {
		metrics = append(metrics,
			Metric{
				Name:   "system_gpu_utilization_percent",
				Labels: labels,
				Value:  float64(utilization.Gpu),
				Type:   "gauge",
			},
			Metric{
				Name:   "system_gpu_memory_utilization_percent",
				Labels: labels,
				Value:  float64(utilization.Memory),
				Type:   "gauge",
			},
		)
	}

	// Memory info
	memInfo, ret := device.GetMemoryInfo()
	if ret == nvml.SUCCESS {
		metrics = append(metrics,
			Metric{
				Name:   "system_gpu_memory_total_bytes",
				Labels: labels,
				Value:  float64(memInfo.Total),
				Type:   "gauge",
			},
			Metric{
				Name:   "system_gpu_memory_used_bytes",
				Labels: labels,
				Value:  float64(memInfo.Used),
				Type:   "gauge",
			},
			Metric{
				Name:   "system_gpu_memory_free_bytes",
				Labels: labels,
				Value:  float64(memInfo.Free),
				Type:   "gauge",
			},
		)
	}

	// Temperature
	temp, ret := device.GetTemperature(nvml.TEMPERATURE_GPU)
	if ret == nvml.SUCCESS {
		metrics = append(metrics, Metric{
			Name:   "system_gpu_temperature_celsius",
			Labels: labels,
			Value:  float64(temp),
			Type:   "gauge",
		})
	}

	// Power usage
	power, ret := device.GetPowerUsage()
	if ret == nvml.SUCCESS {
		metrics = append(metrics, Metric{
			Name:   "system_gpu_power_usage_milliwatts",
			Labels: labels,
			Value:  float64(power),
			Type:   "gauge",
		})
	}

	// Fan speed
	fanSpeed, ret := device.GetFanSpeed()
	if ret == nvml.SUCCESS {
		metrics = append(metrics, Metric{
			Name:   "system_gpu_fan_speed_percent",
			Labels: labels,
			Value:  float64(fanSpeed),
			Type:   "gauge",
		})
	}

	// Clock speeds
	smClock, ret := device.GetClockInfo(nvml.CLOCK_SM)
	if ret == nvml.SUCCESS {
		metrics = append(metrics, Metric{
			Name:   "system_gpu_clock_sm_mhz",
			Labels: labels,
			Value:  float64(smClock),
			Type:   "gauge",
		})
	}

	memClock, ret := device.GetClockInfo(nvml.CLOCK_MEM)
	if ret == nvml.SUCCESS {
		metrics = append(metrics, Metric{
			Name:   "system_gpu_clock_memory_mhz",
			Labels: labels,
			Value:  float64(memClock),
			Type:   "gauge",
		})
	}

	return metrics, nil
}

// Shutdown cleans up NVML resources
func (c *GPUCollector) Shutdown() error {
	if c.initialized {
		ret := nvml.Shutdown()
		if ret != nvml.SUCCESS {
			return fmt.Errorf("failed to shutdown NVML: %v", nvml.ErrorString(ret))
		}
		c.initialized = false
	}
	return nil
}
