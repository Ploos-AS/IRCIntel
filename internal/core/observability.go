package core

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

type ReadinessChecker interface {
	Ping(context.Context) error
}

type ReadinessHandler struct {
	Checker ReadinessChecker
	Timeout time.Duration
}

func (h ReadinessHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	timeout := h.Timeout
	if timeout <= 0 { timeout = 2 * time.Second }
	if h.Checker == nil {
		http.Error(w, "storage readiness unavailable", http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := h.Checker.Ping(ctx); err != nil {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ready\n"))
}

type RuntimeMetrics struct {
	startedAt time.Time
	requests atomic.Uint64
}

func NewRuntimeMetrics() *RuntimeMetrics { return &RuntimeMetrics{startedAt: time.Now().UTC()} }

func (m *RuntimeMetrics) Wrap(next http.Handler) http.Handler {
	if m == nil { return next }
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.requests.Add(1)
		next.ServeHTTP(w, r)
	})
}

type MetricsHandler struct {
	Metrics *RuntimeMetrics
	Ready   ReadinessChecker
	Maintenance MaintenanceStatusProvider
	Timeout time.Duration
}

func (h MetricsHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	ready := 0
	if h.Ready != nil {
		timeout := h.Timeout
		if timeout <= 0 { timeout = 2 * time.Second }
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		if h.Ready.Ping(ctx) == nil { ready = 1 }
		cancel()
	}
	requests := uint64(0)
	uptime := float64(0)
	if h.Metrics != nil {
		requests = h.Metrics.requests.Load()
		uptime = time.Since(h.Metrics.startedAt).Seconds()
		if uptime < 0 { uptime = 0 }
	}
	status := DisabledMaintenanceStatus()
	if h.Maintenance != nil { status = h.Maintenance.Status() }
	maintenanceEnabled := 0
	if status.Enabled { maintenanceEnabled = 1 }
	lastSuccess := float64(0)
	if status.LastSuccessAt != nil { lastSuccess = float64(status.LastSuccessAt.Unix()) }
	lastDuration := float64(status.LastDurationMillis) / 1000
	var rawPruned, hourlyPruned int64
	if status.LastResult != nil {
		rawPruned = status.LastResult.RawObservationsPruned
		hourlyPruned = status.LastResult.HourlyRollupsPruned
	}

	fmt.Fprintf(w, "# HELP ircintel_ready Whether the storage backend is ready.\n# TYPE ircintel_ready gauge\nircintel_ready %d\n", ready)
	fmt.Fprintf(w, "# HELP ircintel_process_uptime_seconds Process uptime.\n# TYPE ircintel_process_uptime_seconds gauge\nircintel_process_uptime_seconds %.3f\n", uptime)
	fmt.Fprintf(w, "# HELP ircintel_http_requests_total HTTP requests observed by Core.\n# TYPE ircintel_http_requests_total counter\nircintel_http_requests_total %d\n", requests)
	fmt.Fprintf(w, "# HELP ircintel_maintenance_enabled Whether scheduled maintenance is enabled.\n# TYPE ircintel_maintenance_enabled gauge\nircintel_maintenance_enabled %d\n", maintenanceEnabled)
	fmt.Fprintf(w, "# HELP ircintel_maintenance_runs_total Maintenance runs attempted.\n# TYPE ircintel_maintenance_runs_total counter\nircintel_maintenance_runs_total %d\n", status.Runs)
	fmt.Fprintf(w, "# HELP ircintel_maintenance_failures_total Maintenance runs failed.\n# TYPE ircintel_maintenance_failures_total counter\nircintel_maintenance_failures_total %d\n", status.Failures)
	fmt.Fprintf(w, "# HELP ircintel_maintenance_last_success_timestamp_seconds Unix timestamp of the last successful maintenance run.\n# TYPE ircintel_maintenance_last_success_timestamp_seconds gauge\nircintel_maintenance_last_success_timestamp_seconds %.0f\n", lastSuccess)
	fmt.Fprintf(w, "# HELP ircintel_maintenance_last_duration_seconds Duration of the last maintenance run.\n# TYPE ircintel_maintenance_last_duration_seconds gauge\nircintel_maintenance_last_duration_seconds %.3f\n", lastDuration)
	fmt.Fprintf(w, "# HELP ircintel_maintenance_last_raw_observations_pruned Rows pruned in the last successful maintenance run.\n# TYPE ircintel_maintenance_last_raw_observations_pruned gauge\nircintel_maintenance_last_raw_observations_pruned %d\n", rawPruned)
	fmt.Fprintf(w, "# HELP ircintel_maintenance_last_hourly_rollups_pruned Hourly rollup rows pruned in the last successful maintenance run.\n# TYPE ircintel_maintenance_last_hourly_rollups_pruned gauge\nircintel_maintenance_last_hourly_rollups_pruned %d\n", hourlyPruned)
}
