package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

// CollectorHealth is the health info for a single collector.
type CollectorHealth struct {
	Status           string `json:"status"`
	LastCollect      string `json:"last_collect,omitempty"`
	MetricCount      int    `json:"metric_count,omitempty"`
	ConsecutiveFails int    `json:"consecutive_failures,omitempty"`
	LastError        string `json:"last_error,omitempty"`
}

// DetailedHealthStatus is the full health response.
type DetailedHealthStatus struct {
	Status     string                     `json:"status"`
	Uptime     float64                    `json:"uptime_seconds"`
	Collectors map[string]CollectorHealth `json:"collectors,omitempty"`
}

// HealthProvider supplies plugin health data to the server.
type HealthProvider interface {
	GetHealthData() map[string]CollectorHealth
}

// Server provides HTTP endpoints for health checks.
type Server struct {
	host           string
	port           int
	server         *http.Server
	startTime      time.Time
	healthProvider HealthProvider
}

// NewServer creates a new HTTP server.
// healthProvider may be nil if no detailed health is available.
func NewServer(host string, port int, healthProvider HealthProvider) *Server {
	return &Server{
		host:           host,
		port:           port,
		startTime:      time.Now(),
		healthProvider: healthProvider,
	}
}

// Start starts the HTTP server.
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)

	s.server = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", s.host, s.port),
		Handler: mux,
	}

	log.Info().Str("host", s.host).Int("port", s.port).Msg("Starting HTTP server")

	errChan := make(chan error, 1)
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	select {
	case <-ctx.Done():
		return s.Shutdown(context.Background())
	case err := <-errChan:
		return err
	}
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.server != nil {
		log.Info().Msg("Shutting down HTTP server")
		return s.server.Shutdown(ctx)
	}
	return nil
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	uptime := time.Since(s.startTime).Seconds()
	status := DetailedHealthStatus{
		Status: "healthy",
		Uptime: uptime,
	}

	if s.healthProvider != nil {
		status.Collectors = s.healthProvider.GetHealthData()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(status); err != nil {
		log.Error().Err(err).Msg("Failed to encode health status")
	}
}
