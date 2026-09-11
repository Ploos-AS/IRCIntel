package core

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type readinessStub struct{ err error }
func (s readinessStub) Ping(context.Context) error { return s.err }

type maintenanceStatusStub struct{ status MaintenanceStatus }
func (s maintenanceStatusStub) Status() MaintenanceStatus { return s.status }

func TestReadinessHandler(t *testing.T) {
	for _, tc := range []struct {
		name string
		checker ReadinessChecker
		want int
	}{
		{name:"ready", checker:readinessStub{}, want:http.StatusOK},
		{name:"not ready", checker:readinessStub{err:errors.New("db down")}, want:http.StatusServiceUnavailable},
		{name:"missing", checker:nil, want:http.StatusServiceUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := httptest.NewRecorder()
			ReadinessHandler{Checker:tc.checker}.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/readyz", nil))
			if res.Code != tc.want { t.Fatalf("status=%d want=%d", res.Code, tc.want) }
		})
	}
}

func TestMetricsHandlerExposesRuntimeAndMaintenanceMetrics(t *testing.T) {
	metrics := NewRuntimeMetrics()
	wrapped := metrics.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	wrapped.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))
	now := time.Date(2026, 9, 11, 4, 0, 0, 0, time.UTC)
	status := MaintenanceStatus{
		Enabled:true,
		Runs:3,
		Failures:1,
		LastSuccessAt:&now,
		LastDurationMillis:1250,
		LastResult:&MaintenanceResult{RawObservationsPruned:7, HourlyRollupsPruned:2},
	}
	res := httptest.NewRecorder()
	MetricsHandler{Metrics:metrics, Ready:readinessStub{}, Maintenance:maintenanceStatusStub{status:status}}.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if res.Code != http.StatusOK { t.Fatalf("status=%d", res.Code) }
	body := res.Body.String()
	for _, want := range []string{
		"ircintel_ready 1",
		"ircintel_http_requests_total 1",
		"ircintel_maintenance_enabled 1",
		"ircintel_maintenance_runs_total 3",
		"ircintel_maintenance_failures_total 1",
		"ircintel_maintenance_last_raw_observations_pruned 7",
		"ircintel_maintenance_last_hourly_rollups_pruned 2",
	} {
		if !strings.Contains(body, want) { t.Fatalf("metrics missing %q\n%s", want, body) }
	}
	if got := res.Header().Get("Content-Type"); !strings.Contains(got, "text/plain") { t.Fatalf("content-type=%q", got) }
}

func TestMetricsReportsReadinessFailure(t *testing.T) {
	res := httptest.NewRecorder()
	MetricsHandler{Ready:readinessStub{err:errors.New("down")}}.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if !strings.Contains(res.Body.String(), "ircintel_ready 0") { t.Fatalf("body=%s", res.Body.String()) }
}
