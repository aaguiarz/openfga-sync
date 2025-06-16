package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/aaguiarz/openfga-sync/config"
	"github.com/aaguiarz/openfga-sync/metrics"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

// Server represents the HTTP server for health checks and metrics
type Server struct {
	config  *config.Config
	logger  *logrus.Logger
	metrics *metrics.Metrics
	server  *http.Server

	// Service state
	startTime time.Time
	ready     bool
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status  string            `json:"status"`
	Service string            `json:"service"`
	Version string            `json:"version"`
	Uptime  string            `json:"uptime"`
	Details map[string]string `json:"details,omitempty"`
}

// ReadinessResponse represents the readiness check response
type ReadinessResponse struct {
	Status       string            `json:"status"`
	Service      string            `json:"service"`
	Dependencies map[string]string `json:"dependencies"`
}

// New creates a new HTTP server instance
func New(cfg *config.Config, logger *logrus.Logger, metrics *metrics.Metrics) *Server {
	return &Server{
		config:    cfg,
		logger:    logger,
		metrics:   metrics,
		startTime: time.Now(),
		ready:     false,
	}
}

// Start starts the HTTP server
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/healthz", s.healthHandler)

	// Readiness check endpoint
	mux.HandleFunc("/readyz", s.readinessHandler)

	// Metrics endpoint (if enabled)
	if s.config.Observability.Metrics.Enabled {
		metricsPath := s.config.Observability.Metrics.Path
		if metricsPath == "" {
			metricsPath = "/metrics"
		}
		mux.Handle(metricsPath, promhttp.Handler())
		s.logger.WithField("path", metricsPath).Info("Metrics endpoint enabled")
	}

	// Create HTTP server
	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.config.Server.Port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Record service start
	s.metrics.RecordServiceStart()

	// Start uptime counter in background
	go s.trackUptime(ctx)

	s.logger.WithField("port", s.config.Server.Port).Info("Starting HTTP server")

	// Start server
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.WithError(err).Error("HTTP server error")
		}
	}()

	return nil
}

// Stop gracefully stops the HTTP server
func (s *Server) Stop(ctx context.Context) error {
	if s.server == nil {
		return nil
	}

	s.logger.Info("Stopping HTTP server")
	return s.server.Shutdown(ctx)
}

// SetReady marks the service as ready
func (s *Server) SetReady(ready bool) {
	s.ready = ready
	if ready {
		s.logger.Info("Service marked as ready")
	} else {
		s.logger.Info("Service marked as not ready")
	}
}

// healthHandler handles the /healthz endpoint
func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(s.startTime)

	response := HealthResponse{
		Status:  "UP",
		Service: "openfga-sync",
		Version: "1.0.0",
		Uptime:  uptime.String(),
		Details: map[string]string{
			"backend_type":  s.config.Backend.Type,
			"storage_mode":  string(s.config.Backend.Mode),
			"poll_interval": s.config.Service.PollInterval.String(),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.WithError(err).Error("Failed to encode health response")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	s.logger.WithFields(logrus.Fields{
		"endpoint": "/healthz",
		"status":   response.Status,
		"uptime":   response.Uptime,
	}).Debug("Health check requested")
}

// readinessHandler handles the /readyz endpoint
func (s *Server) readinessHandler(w http.ResponseWriter, r *http.Request) {
	status := "READY"
	statusCode := http.StatusOK

	dependencies := map[string]string{
		"service_ready": "OK",
	}

	// Check if service is marked as ready
	if !s.ready {
		status = "NOT_READY"
		statusCode = http.StatusServiceUnavailable
		dependencies["service_ready"] = "NOT_READY"
	}

	// Additional dependency checks could be added here
	// For example, checking OpenFGA connectivity, database health, etc.

	response := ReadinessResponse{
		Status:       status,
		Service:      "openfga-sync",
		Dependencies: dependencies,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.WithError(err).Error("Failed to encode readiness response")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	s.logger.WithFields(logrus.Fields{
		"endpoint": "/readyz",
		"status":   response.Status,
		"ready":    s.ready,
	}).Debug("Readiness check requested")
}

// trackUptime runs in the background to track service uptime
func (s *Server) trackUptime(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.metrics.IncrementUptime()
		}
	}
}
