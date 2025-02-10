// Package health provides health check functionality.
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Status represents the health status.
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusDegraded  Status = "degraded"
	StatusUnhealthy Status = "unhealthy"
)

// Check represents a health check function.
type Check func(ctx context.Context) CheckResult

// CheckResult contains the result of a health check.
type CheckResult struct {
	Status  Status `json:"status"`
	Message string `json:"message,omitempty"`
	Latency string `json:"latency,omitempty"`
}

// Checker manages health checks.
type Checker struct {
	mu     sync.RWMutex
	checks map[string]Check
}

// NewChecker creates a new health checker.
func NewChecker() *Checker {
	return &Checker{
		checks: make(map[string]Check),
	}
}

// Register adds a named health check.
func (c *Checker) Register(name string, check Check) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks[name] = check
}

// HealthResponse is the JSON response for health endpoints.
type HealthResponse struct {
	Status    Status                 `json:"status"`
	Timestamp string                 `json:"timestamp"`
	Checks    map[string]CheckResult `json:"checks,omitempty"`
}

// Check runs all registered health checks.
func (c *Checker) Check(ctx context.Context) HealthResponse {
	c.mu.RLock()
	defer c.mu.RUnlock()

	resp := HealthResponse{
		Status:    StatusHealthy,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Checks:    make(map[string]CheckResult),
	}

	for name, check := range c.checks {
		start := time.Now()
		result := check(ctx)
		result.Latency = time.Since(start).String()
		resp.Checks[name] = result

		// Overall status is worst of all checks
		if result.Status == StatusUnhealthy {
			resp.Status = StatusUnhealthy
		} else if result.Status == StatusDegraded && resp.Status != StatusUnhealthy {
			resp.Status = StatusDegraded
		}
	}

	return resp
}

// LivenessHandler returns an HTTP handler for liveness probes.
// Liveness indicates the process is running (not deadlocked).
func (c *Checker) LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "alive",
		})
	}
}

// ReadinessHandler returns an HTTP handler for readiness probes.
// Readiness indicates the service can accept traffic.
func (c *Checker) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		resp := c.Check(ctx)

		w.Header().Set("Content-Type", "application/json")
		if resp.Status == StatusUnhealthy {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
		json.NewEncoder(w).Encode(resp)
	}
}

// HealthHandler returns an HTTP handler for detailed health checks.
func (c *Checker) HealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		resp := c.Check(ctx)

		w.Header().Set("Content-Type", "application/json")
		switch resp.Status {
		case StatusHealthy:
			w.WriteHeader(http.StatusOK)
		case StatusDegraded:
			w.WriteHeader(http.StatusOK) // Degraded is still serving
		case StatusUnhealthy:
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		json.NewEncoder(w).Encode(resp)
	}
}

// Common health check builders

// CollectorCheck creates a health check for the metrics collector.
func CollectorCheck(lastCollectionTime func() time.Time, maxAge time.Duration) Check {
	return func(ctx context.Context) CheckResult {
		last := lastCollectionTime()
		if last.IsZero() {
			return CheckResult{
				Status:  StatusDegraded,
				Message: "no metrics collected yet",
			}
		}

		age := time.Since(last)
		if age > maxAge {
			return CheckResult{
				Status:  StatusUnhealthy,
				Message: "metrics collection stale",
			}
		}

		return CheckResult{
			Status:  StatusHealthy,
			Message: "collecting metrics",
		}
	}
}

// ConsensusCheck creates a health check for consensus.
func ConsensusCheck(isPartitioned func() bool, partitionDuration func() time.Duration) Check {
	return func(ctx context.Context) CheckResult {
		if isPartitioned() {
			dur := partitionDuration()
			return CheckResult{
				Status:  StatusDegraded,
				Message: "partitioned for " + dur.String(),
			}
		}
		return CheckResult{
			Status:  StatusHealthy,
			Message: "connected to cluster",
		}
	}
}

// PredictorCheck creates a health check for the predictor.
func PredictorCheck(historySize func() int, minSamples int) Check {
	return func(ctx context.Context) CheckResult {
		size := historySize()
		if size < minSamples {
			return CheckResult{
				Status:  StatusDegraded,
				Message: "insufficient history for accurate predictions",
			}
		}
		return CheckResult{
			Status:  StatusHealthy,
			Message: "predictor ready",
		}
	}
}
