package core

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"
)

type MaintenanceRunner interface {
	RunRetentionMaintenance(time.Time, RetentionPolicy) (MaintenanceResult, error)
}

type MaintenanceStatus struct {
	Enabled            bool               `json:"enabled"`
	IntervalSeconds    int64              `json:"interval_seconds,omitempty"`
	LastStartedAt      *time.Time         `json:"last_started_at,omitempty"`
	LastCompletedAt    *time.Time         `json:"last_completed_at,omitempty"`
	LastSuccessAt      *time.Time         `json:"last_success_at,omitempty"`
	LastDurationMillis int64              `json:"last_duration_millis,omitempty"`
	LastError          string             `json:"last_error,omitempty"`
	LastResult         *MaintenanceResult `json:"last_result,omitempty"`
	Runs               uint64             `json:"runs"`
	Failures           uint64             `json:"failures"`
}

type MaintenanceScheduler struct {
	runner   MaintenanceRunner
	policy   RetentionPolicy
	interval time.Duration
	now      func() time.Time

	mu     sync.RWMutex
	status MaintenanceStatus
}

func NewMaintenanceScheduler(runner MaintenanceRunner, policy RetentionPolicy, interval time.Duration) (*MaintenanceScheduler, error) {
	if runner == nil { return nil, errors.New("maintenance runner is required") }
	if err := policy.validate(); err != nil { return nil, err }
	if interval <= 0 { return nil, errors.New("maintenance interval must be greater than zero") }
	return &MaintenanceScheduler{
		runner: runner,
		policy: policy,
		interval: interval,
		now: func() time.Time { return time.Now().UTC() },
		status: MaintenanceStatus{Enabled: true, IntervalSeconds: int64(interval / time.Second)},
	}, nil
}

func DisabledMaintenanceStatus() MaintenanceStatus { return MaintenanceStatus{Enabled: false} }

func (s *MaintenanceScheduler) Status() MaintenanceStatus {
	if s == nil { return DisabledMaintenanceStatus() }
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

func (s *MaintenanceScheduler) RunOnce() {
	if s == nil { return }
	started := s.now().UTC()
	s.mu.Lock()
	s.status.LastStartedAt = &started
	s.status.Runs++
	s.mu.Unlock()

	wallStart := time.Now()
	result, err := s.runner.RunRetentionMaintenance(started, s.policy)
	completed := s.now().UTC()
	duration := time.Since(wallStart)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.LastCompletedAt = &completed
	s.status.LastDurationMillis = duration.Milliseconds()
	if err != nil {
		s.status.Failures++
		s.status.LastError = err.Error()
		return
	}
	s.status.LastError = ""
	s.status.LastSuccessAt = &completed
	s.status.LastResult = &result
}

// Run waits one full interval before the first maintenance pass. Starting Core
// therefore never causes immediate destructive maintenance.
func (s *MaintenanceScheduler) Run(ctx context.Context) {
	if s == nil { return }
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.RunOnce()
		}
	}
}

type MaintenanceStatusProvider interface { Status() MaintenanceStatus }

type StaticMaintenanceStatus MaintenanceStatus
func (s StaticMaintenanceStatus) Status() MaintenanceStatus { return MaintenanceStatus(s) }

type MaintenanceStatusHandler struct {
	Token string
	Provider MaintenanceStatusProvider
}

func (h MaintenanceStatusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.Token != "" && r.Header.Get("Authorization") != "Bearer "+h.Token {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	status := DisabledMaintenanceStatus()
	if h.Provider != nil { status = h.Provider.Status() }
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(status)
}
