package shipper

import (
	"context"

	"github.com/0x524A/metricsd/internal/collector"
)

// Shipper is the interface for shipping metrics to remote endpoints (Interface Segregation Principle)
type Shipper interface {
	Ship(ctx context.Context, metrics []collector.Metric) error
	Close() error
}
