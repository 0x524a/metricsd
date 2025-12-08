package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/0x524A/metricsd/internal/collector"
	"github.com/0x524A/metricsd/internal/config"
	"github.com/0x524A/metricsd/internal/orchestrator"
	"github.com/0x524A/metricsd/internal/server"
	"github.com/0x524A/metricsd/internal/shipper"
)

const (
	defaultConfigPath = "config.json"
)

func main() {
	// Parse command-line flags
	configPath := flag.String("config", defaultConfigPath, "Path to configuration file")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	flag.Parse()

	// Setup logging
	setupLogging(*logLevel)

	log.Info().Msg("Starting Metrics Collector Service")

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	log.Info().
		Str("config_file", *configPath).
		Msg("Configuration loaded successfully")

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize components
	collectorRegistry := setupCollectors(cfg)
	metricShipper := setupShipper(cfg)
	defer metricShipper.Close()

	// Create orchestrator with global labels support
	var orch *orchestrator.Orchestrator
	if len(cfg.GlobalLabels) > 0 {
		orch = orchestrator.NewOrchestratorWithLabels(
			collectorRegistry,
			metricShipper,
			cfg.GetCollectionInterval(),
			cfg.GlobalLabels,
		)
		log.Info().
			Interface("global_labels", cfg.GlobalLabels).
			Msg("Orchestrator initialized with global labels")
	} else {
		orch = orchestrator.NewOrchestrator(
			collectorRegistry,
			metricShipper,
			cfg.GetCollectionInterval(),
		)
	}

	// Create HTTP server for health checks
	httpServer := server.NewServer(cfg.Server.Host, cfg.Server.Port)

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start services
	errChan := make(chan error, 2)

	// Start HTTP server
	go func() {
		if err := httpServer.Start(ctx); err != nil {
			errChan <- fmt.Errorf("HTTP server error: %w", err)
		}
	}()

	// Start orchestrator
	go func() {
		if err := orch.Start(ctx); err != nil && err != context.Canceled {
			errChan <- fmt.Errorf("orchestrator error: %w", err)
		}
	}()

	// Wait for termination signal or error
	select {
	case sig := <-sigChan:
		log.Info().Str("signal", sig.String()).Msg("Received shutdown signal")
	case err := <-errChan:
		log.Error().Err(err).Msg("Service error occurred")
	}

	// Graceful shutdown
	log.Info().Msg("Initiating graceful shutdown")
	cancel()

	// Give services time to shutdown gracefully
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Error during server shutdown")
	}

	orch.Stop()

	// Cleanup GPU collector if enabled
	if cfg.Collector.EnableGPU {
		cleanupGPUCollector(collectorRegistry)
	}

	log.Info().Msg("Metrics Collector Service stopped")
}

func setupLogging(level string) {
	// Configure zerolog
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	// Set log level
	switch level {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	// Pretty console output
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
}

func setupCollectors(cfg *config.Config) *collector.Registry {
	registry := collector.NewRegistry()

	// Register system collector if any OS metrics are enabled
	if cfg.Collector.EnableCPU || cfg.Collector.EnableMemory || cfg.Collector.EnableDisk || cfg.Collector.EnableNetwork {
		systemCollector := collector.NewSystemCollector(
			cfg.Collector.EnableCPU,
			cfg.Collector.EnableMemory,
			cfg.Collector.EnableDisk,
			cfg.Collector.EnableNetwork,
		)
		registry.Register(systemCollector)
		log.Info().Msg("System collector registered")
	}

	// Register GPU collector if enabled
	if cfg.Collector.EnableGPU {
		gpuCollector := collector.NewGPUCollector()
		registry.Register(gpuCollector)
		log.Info().Msg("GPU collector registered")
	}

	// Register HTTP collectors for application endpoints
	if len(cfg.Endpoints) > 0 {
		endpoints := make([]collector.EndpointConfig, 0, len(cfg.Endpoints))
		for _, ep := range cfg.Endpoints {
			endpoints = append(endpoints, collector.EndpointConfig{
				Name: ep.Name,
				URL:  ep.URL,
			})
		}
		httpCollector := collector.NewHTTPCollector(endpoints, cfg.Shipper.Timeout)
		registry.Register(httpCollector)
		log.Info().Int("endpoint_count", len(endpoints)).Msg("HTTP collector registered")
	}

	// Register plugin collectors (per-plugin intervals handled internally)
	pluginDefs, err := collector.LoadPlugins(cfg.Plugins.Directory)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load plugin definitions")
	}
	if len(pluginDefs) > 0 {
		pluginCollector, err := collector.NewPluginCollector(cfg.Plugins.Prefix, cfg.Plugins.Directory, pluginDefs)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to initialize plugin collector")
		}
		registry.Register(pluginCollector)
		log.Info().
			Int("plugin_count", len(pluginDefs)).
			Str("plugins_dir", cfg.Plugins.Directory).
			Msg("Plugin collector registered")
	} else {
		log.Info().Str("plugins_dir", cfg.Plugins.Directory).Msg("No plugins discovered")
	}

	return registry
}

func setupShipper(cfg *config.Config) shipper.Shipper {
	var shpr shipper.Shipper
	var err error

	timeout := cfg.Shipper.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	switch cfg.Shipper.Type {
	case "prometheus_remote_write":
		shpr, err = shipper.NewPrometheusRemoteWriteShipper(
			cfg.Shipper.Endpoint,
			cfg.Shipper.TLS.Enabled,
			cfg.Shipper.TLS.CertFile,
			cfg.Shipper.TLS.KeyFile,
			cfg.Shipper.TLS.CAFile,
			cfg.Shipper.TLS.InsecureSkipVerify,
			timeout,
		)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to create Prometheus remote write shipper")
		}
		log.Info().
			Str("type", "prometheus_remote_write").
			Str("endpoint", cfg.Shipper.Endpoint).
			Msg("Shipper initialized")

	case "http_json":
		shpr, err = shipper.NewHTTPJSONShipper(
			cfg.Shipper.Endpoint,
			cfg.Shipper.TLS.Enabled,
			cfg.Shipper.TLS.CertFile,
			cfg.Shipper.TLS.KeyFile,
			cfg.Shipper.TLS.CAFile,
			cfg.Shipper.TLS.InsecureSkipVerify,
			timeout,
		)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to create HTTP JSON shipper")
		}
		log.Info().
			Str("type", "http_json").
			Str("endpoint", cfg.Shipper.Endpoint).
			Msg("Shipper initialized")

	default:
		log.Fatal().Str("type", cfg.Shipper.Type).Msg("Unknown shipper type")
	}

	return shpr
}

func cleanupGPUCollector(registry *collector.Registry) {
	// Note: In a production system, you would want to have a better way to manage
	// lifecycle of collectors. For now, this is a simple cleanup approach.
	log.Debug().Msg("Cleaning up GPU collector resources")
}
