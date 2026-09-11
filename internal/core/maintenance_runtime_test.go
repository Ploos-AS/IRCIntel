package core

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type maintenanceRunnerStub struct {
	result MaintenanceResult
	err error
	calls int
}

func (s *maintenanceRunnerStub) RunRetentionMaintenance(now time.Time, policy RetentionPolicy) (MaintenanceResult, error) {
	s.calls++
	return s.result, s.err
}

func TestMaintenanceSchedulerRunOnceStatus(t *testing.T) {
	runner := &maintenanceRunnerStub{result: MaintenanceResult{RawObservationsPruned: 12, HourlyRollupsPruned: 3}}
	scheduler, err := NewMaintenanceScheduler(runner, DefaultRetentionPolicy(), time.Hour)
	if err != nil { t.Fatal(err) }
	now := time.Date(2026,9,11,4,0,0,0,time.UTC)
	scheduler.now = func() time.Time { return now }
	scheduler.RunOnce()
	status := scheduler.Status()
	if runner.calls != 1 || status.Runs != 1 || status.Failures != 0 { t.Fatalf("status=%+v calls=%d", status, runner.calls) }
	if status.LastSuccessAt == nil || status.LastResult == nil { t.Fatalf("status=%+v", status) }
	if status.LastResult.RawObservationsPruned != 12 || status.LastResult.HourlyRollupsPruned != 3 { t.Fatalf("result=%+v", status.LastResult) }
}

func TestMaintenanceSchedulerFailureStatus(t *testing.T) {
	runner := &maintenanceRunnerStub{err: errors.New("maintenance failed")}
	scheduler, err := NewMaintenanceScheduler(runner, DefaultRetentionPolicy(), time.Hour)
	if err != nil { t.Fatal(err) }
	scheduler.RunOnce()
	status := scheduler.Status()
	if status.Failures != 1 || status.LastError != "maintenance failed" || status.LastSuccessAt != nil { t.Fatalf("status=%+v", status) }
}

func TestMaintenanceSchedulerDoesNotRunImmediately(t *testing.T) {
	runner := &maintenanceRunnerStub{}
	scheduler, err := NewMaintenanceScheduler(runner, DefaultRetentionPolicy(), time.Hour)
	if err != nil { t.Fatal(err) }
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { scheduler.Run(ctx); close(done) }()
	time.Sleep(20 * time.Millisecond)
	cancel()
	<-done
	if runner.calls != 0 { t.Fatalf("calls=%d want=0", runner.calls) }
}

func TestMaintenanceStatusHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/maintenance/status", nil)
	res := httptest.NewRecorder()
	MaintenanceStatusHandler{Provider: StaticMaintenanceStatus(DisabledMaintenanceStatus())}.ServeHTTP(res, req)
	if res.Code != http.StatusOK { t.Fatalf("status=%d", res.Code) }
	if body := res.Body.String(); body == "" { t.Fatal("empty response") }
}
