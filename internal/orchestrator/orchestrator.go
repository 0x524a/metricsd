package orchestrator

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/0x524A/metricsd/internal/collector"
	"github.com/0x524A/metricsd/internal/shipper"
)

// Orchestrator coordinates the collection and shipping of metrics (Single Responsibility Principle)
type Orchestrator struct {
	registry         *collector.Registry
	shipper          shipper.Shipper
	interval         time.Duration
	stopChan         chan struct{}
	lastShipDuration time.Duration
	lastShipSuccess  bool
}

// NewOrchestrator creates a new orchestrator
func NewOrchestrator(registry *collector.Registry, shpr shipper.Shipper, interval time.Duration) *Orchestrator {
	return &Orchestrator{
		registry: registry,
		shipper:  shpr,
		interval: interval,
		stopChan: make(chan struct{}),
	}
}

// Start begins the periodic collection and shipping of metrics
func (o *Orchestrator) Start(ctx context.Context) error {
	ticker := time.NewTicker(o.interval)
	defer ticker.Stop()

	log.Info().
		Dur("interval", o.interval).
		Msg("Orchestrator started")

	// Collect and ship immediately on start
	o.collectAndShip(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Orchestrator stopping due to context cancellation")
			return ctx.Err()
		case <-o.stopChan:
			log.Info().Msg("Orchestrator stopped")
			return nil
		case <-ticker.C:
			o.collectAndShip(ctx)
		}
	}
}

// Stop stops the orchestrator
func (o *Orchestrator) Stop() {
	close(o.stopChan)
}

func (o *Orchestrator) collectAndShip(ctx context.Context) {
	startTime := time.Now()

	log.Debug().Msg("Starting metrics collection")

	// Collect metrics from all collectors in parallel
	metrics, err := o.registry.CollectAllParallel(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to collect metrics")
		return
	}

	collectDuration := time.Since(startTime)

	// Deadline warning: if collection took >80% of interval, warn
	threshold := time.Duration(float64(o.interval) * 0.8)
	if collectDuration > threshold {
		log.Warn().
			Dur("collection_duration", collectDuration).
			Dur("interval", o.interval).
			Msg("Collection duration exceeds 80% of interval — consider increasing interval or reducing collectors")
	}

	log.Debug().
		Int("metric_count", len(metrics)).
		Dur("duration", collectDuration).
		Msg("Metrics collected")

	// Append internal metrics about metricsd itself
	internalMetrics := []collector.Metric{
		{
			Name:   "metricsd_collection_duration_seconds",
			Value:  collectDuration.Seconds(),
			Type:   "gauge",
			Labels: map[string]string{},
		},
	}

	// Include last ship duration from previous cycle (avoids chicken-and-egg)
	if o.lastShipDuration > 0 {
		internalMetrics = append(internalMetrics, collector.Metric{
			Name:   "metricsd_ship_duration_seconds",
			Value:  o.lastShipDuration.Seconds(),
			Type:   "gauge",
			Labels: map[string]string{},
		})
	}

	metrics = append(metrics, internalMetrics...)

	// Ship metrics with one retry on failure
	shipStart := time.Now()
	if err := o.shipper.Ship(ctx, metrics); err != nil {
		log.Warn().Err(err).Msg("Ship failed, retrying in 1s")
		time.Sleep(1 * time.Second)

		if err := o.shipper.Ship(ctx, metrics); err != nil {
			log.Error().Err(err).Msg("Ship retry failed")
			o.lastShipDuration = time.Since(shipStart)
			return
		}
	}
	o.lastShipDuration = time.Since(shipStart)

	log.Info().
		Int("metric_count", len(metrics)).
		Dur("total_duration", time.Since(startTime)).
		Msg("Collection and shipping cycle completed successfully")
}
