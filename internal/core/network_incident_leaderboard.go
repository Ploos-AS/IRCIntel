package core

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"
)

type NetworkReliabilityItem struct {
	Rank        int                  `json:"rank"`
	NetworkID   string               `json:"network_id"`
	NetworkName string               `json:"network_name"`
	Stats       NetworkIncidentStats `json:"stats"`
}

type NetworkReliabilityHandler struct {
	Token  string
	Reader NetworkIncidentStatsSnapshotReader
}

func (h NetworkReliabilityHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.Reader == nil {
		http.Error(w, "network reliability reader unavailable", http.StatusServiceUnavailable)
		return
	}
	if h.Token != "" && r.Header.Get("Authorization") != "Bearer "+h.Token {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var since, until time.Time
	var err error
	if raw := strings.TrimSpace(r.URL.Query().Get("since")); raw != "" {
		since, err = time.Parse(time.RFC3339Nano, raw)
		if err != nil { http.Error(w, "invalid since", http.StatusBadRequest); return }
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("until")); raw != "" {
		until, err = time.Parse(time.RFC3339Nano, raw)
		if err != nil { http.Error(w, "invalid until", http.StatusBadRequest); return }
	}
	if !since.IsZero() && !until.IsZero() && since.After(until) {
		http.Error(w, "invalid time range", http.StatusBadRequest)
		return
	}
	snapshot, incidents, err := h.Reader.NetworkIncidentStatsSnapshot(since, until)
	if err != nil { http.Error(w, "network reliability query failed", http.StatusServiceUnavailable); return }
	byNetwork := make(map[string][]NetworkIncident)
	for _, incident := range incidents { byNetwork[incident.NetworkID] = append(byNetwork[incident.NetworkID], incident) }
	items := make([]NetworkReliabilityItem, 0, len(snapshot.Networks))
	for _, network := range snapshot.Networks {
		q := NetworkIncidentRecordQuery{NetworkID: network.ID, Since: since, Until: until}
		items = append(items, NetworkReliabilityItem{NetworkID: network.ID, NetworkName: network.Name, Stats: summarizeNetworkIncidents(q, byNetwork[network.ID])})
	}
	// Reliability ordering is deliberately based on observed incident burden only.
	// Lower downtime wins, then fewer down incidents, then fewer total incidents.
	// Names/IDs provide deterministic ties. No claim is made for networks with
	// different observation coverage; coverage-aware scoring is a later milestone.
	sort.Slice(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if a.Stats.TotalDowntimeSeconds != b.Stats.TotalDowntimeSeconds { return a.Stats.TotalDowntimeSeconds < b.Stats.TotalDowntimeSeconds }
		if a.Stats.DownIncidents != b.Stats.DownIncidents { return a.Stats.DownIncidents < b.Stats.DownIncidents }
		if a.Stats.TotalIncidents != b.Stats.TotalIncidents { return a.Stats.TotalIncidents < b.Stats.TotalIncidents }
		if a.NetworkName != b.NetworkName { return a.NetworkName < b.NetworkName }
		return a.NetworkID < b.NetworkID
	})
	for i := range items { items[i].Rank = i + 1 }
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Since    *time.Time               `json:"since,omitempty"`
		Until    *time.Time               `json:"until,omitempty"`
		Networks []NetworkReliabilityItem `json:"networks"`
	}{Since: optionalTime(since), Until: optionalTime(until), Networks: items})
}

func optionalTime(v time.Time) *time.Time {
	if v.IsZero() { return nil }
	value := v
	return &value
}
