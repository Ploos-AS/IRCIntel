package core

import (
	"context"
	"errors"
	"time"
)

const (
	DefaultRawObservationRetention = 30 * 24 * time.Hour
	DefaultHourlyRollupRetention    = 400 * 24 * time.Hour
)

type RetentionPolicy struct {
	RawObservations time.Duration
	HourlyRollups   time.Duration
}

type MaintenanceResult struct {
	RawCutoff             time.Time `json:"raw_cutoff"`
	HourlyCutoff          time.Time `json:"hourly_cutoff"`
	RawObservationsPruned int64     `json:"raw_observations_pruned"`
	HourlyRollupsPruned   int64     `json:"hourly_rollups_pruned"`
}

func DefaultRetentionPolicy() RetentionPolicy {
	return RetentionPolicy{RawObservations: DefaultRawObservationRetention, HourlyRollups: DefaultHourlyRollupRetention}
}

func (p RetentionPolicy) validate() error {
	if p.RawObservations <= 0 { return errors.New("raw observation retention must be greater than zero") }
	if p.HourlyRollups <= 0 { return errors.New("hourly rollup retention must be greater than zero") }
	if p.HourlyRollups < p.RawObservations { return errors.New("hourly rollup retention must not be shorter than raw observation retention") }
	return nil
}

// RunRetentionMaintenance performs one explicit maintenance pass. Daily rollups
// are intentionally retained indefinitely: they are the long-term public
// statistics substrate. Before raw observations are deleted, both rollup
// granularities are rebuilt for the raw-retention window in the same
// transaction. Hourly rollups are then bounded independently.
func (s *PostgresStore) RunRetentionMaintenance(now time.Time, policy RetentionPolicy) (MaintenanceResult, error) {
	if s == nil || s.pool == nil { return MaintenanceResult{}, errors.New("postgres store is not open") }
	if now.IsZero() { return MaintenanceResult{}, errors.New("maintenance time is required") }
	if err := policy.validate(); err != nil { return MaintenanceResult{}, err }

	now = now.UTC()
	rawCutoff := now.Add(-policy.RawObservations)
	hourlyCutoff := dateBucket(now.Add(-policy.HourlyRollups), "hour")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil { return MaintenanceResult{}, err }
	defer tx.Rollback(ctx)

	if err := rebuildPostgresRollupsTx(ctx, tx, time.Time{}, rawCutoff); err != nil { return MaintenanceResult{}, err }
	rawResult, err := tx.Exec(ctx, `DELETE FROM observations WHERE observed_at < $1`, rawCutoff)
	if err != nil { return MaintenanceResult{}, err }
	hourlyResult, err := tx.Exec(ctx, `DELETE FROM observation_rollups_hourly WHERE bucket_start < $1`, hourlyCutoff)
	if err != nil { return MaintenanceResult{}, err }
	if err := tx.Commit(ctx); err != nil { return MaintenanceResult{}, err }

	return MaintenanceResult{
		RawCutoff: rawCutoff,
		HourlyCutoff: hourlyCutoff,
		RawObservationsPruned: rawResult.RowsAffected(),
		HourlyRollupsPruned: hourlyResult.RowsAffected(),
	}, nil
}
