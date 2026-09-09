package core

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type NetworkIncidentStats struct {
	NetworkID                string     `json:"network_id,omitempty"`
	Since                    *time.Time `json:"since,omitempty"`
	Until                    *time.Time `json:"until,omitempty"`
	TotalIncidents           int        `json:"total_incidents"`
	OpenIncidents            int        `json:"open_incidents"`
	ClosedIncidents          int        `json:"closed_incidents"`
	DegradedIncidents        int        `json:"degraded_incidents"`
	DownIncidents            int        `json:"down_incidents"`
	TotalDowntimeSeconds     int64      `json:"total_downtime_seconds"`
	MeanRecoverySeconds      *int64     `json:"mean_recovery_seconds,omitempty"`
	MaxIncidentDurationSeconds *int64   `json:"max_incident_duration_seconds,omitempty"`
	LatestStartedAt          *time.Time `json:"latest_started_at,omitempty"`
}

type NetworkIncidentStatsReader interface {
	ListNetworkIncidentRecords(NetworkIncidentRecordQuery) ([]NetworkIncident, error)
}

type NetworkIncidentStatsHandler struct {
	Token  string
	Reader NetworkIncidentStatsReader
}

func (h NetworkIncidentStatsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.Reader == nil { http.Error(w, "network incident stats reader unavailable", http.StatusServiceUnavailable); return }
	if h.Token != "" && r.Header.Get("Authorization") != "Bearer "+h.Token { http.Error(w, "unauthorized", http.StatusUnauthorized); return }
	q := NetworkIncidentRecordQuery{NetworkID: strings.TrimSpace(r.URL.Query().Get("network_id")), Limit: 500}
	var err error
	if raw := strings.TrimSpace(r.URL.Query().Get("since")); raw != "" { q.Since, err = time.Parse(time.RFC3339Nano, raw); if err != nil { http.Error(w, "invalid since", http.StatusBadRequest); return } }
	if raw := strings.TrimSpace(r.URL.Query().Get("until")); raw != "" { q.Until, err = time.Parse(time.RFC3339Nano, raw); if err != nil { http.Error(w, "invalid until", http.StatusBadRequest); return } }
	if !q.Since.IsZero() && !q.Until.IsZero() && q.Since.After(q.Until) { http.Error(w, "invalid time range", http.StatusBadRequest); return }
	items, err := h.Reader.ListNetworkIncidentRecords(q)
	if err != nil { http.Error(w, "network incident stats query failed", http.StatusServiceUnavailable); return }
	stats := summarizeNetworkIncidents(q, items)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(stats)
}

func summarizeNetworkIncidents(q NetworkIncidentRecordQuery, items []NetworkIncident) NetworkIncidentStats {
	stats := NetworkIncidentStats{NetworkID: q.NetworkID}
	if !q.Since.IsZero() { v := q.Since; stats.Since = &v }
	if !q.Until.IsZero() { v := q.Until; stats.Until = &v }
	var closedDuration int64
	var closedWithDuration int64
	for _, item := range items {
		stats.TotalIncidents++
		if stats.LatestStartedAt == nil || item.StartedAt.After(*stats.LatestStartedAt) { v := item.StartedAt; stats.LatestStartedAt = &v }
		switch item.Status { case "open": stats.OpenIncidents++; case "closed": stats.ClosedIncidents++ }
		switch item.Severity { case "degraded": stats.DegradedIncidents++; case "down": stats.DownIncidents++ }
		if item.Status == "closed" && item.DurationSeconds != nil {
			d := *item.DurationSeconds
			stats.TotalDowntimeSeconds += d
			closedDuration += d
			closedWithDuration++
			if stats.MaxIncidentDurationSeconds == nil || d > *stats.MaxIncidentDurationSeconds { v := d; stats.MaxIncidentDurationSeconds = &v }
		}
	}
	if closedWithDuration > 0 { v := closedDuration / closedWithDuration; stats.MeanRecoverySeconds = &v }
	return stats
}
