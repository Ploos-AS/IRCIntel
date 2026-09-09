package core

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"
)

type NetworkIncidentStatsByNetworkItem struct {
	NetworkID   string               `json:"network_id"`
	NetworkName string               `json:"network_name"`
	Stats       NetworkIncidentStats `json:"stats"`
}

type NetworkIncidentStatsSnapshotReader interface {
	NetworkIncidentStatsSnapshot(since, until time.Time) (RegistrySnapshot, []NetworkIncident, error)
}

type NetworkIncidentStatsByNetworkHandler struct {
	Token  string
	Reader NetworkIncidentStatsSnapshotReader
}

func (h NetworkIncidentStatsByNetworkHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.Reader == nil {
		http.Error(w, "network incident stats reader unavailable", http.StatusServiceUnavailable)
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
		if err != nil {
			http.Error(w, "invalid since", http.StatusBadRequest)
			return
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("until")); raw != "" {
		until, err = time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			http.Error(w, "invalid until", http.StatusBadRequest)
			return
		}
	}
	if !since.IsZero() && !until.IsZero() && since.After(until) {
		http.Error(w, "invalid time range", http.StatusBadRequest)
		return
	}

	snapshot, incidents, err := h.Reader.NetworkIncidentStatsSnapshot(since, until)
	if err != nil {
		http.Error(w, "network incident stats query failed", http.StatusServiceUnavailable)
		return
	}

	byNetwork := make(map[string][]NetworkIncident)
	for _, incident := range incidents {
		byNetwork[incident.NetworkID] = append(byNetwork[incident.NetworkID], incident)
	}

	items := make([]NetworkIncidentStatsByNetworkItem, 0, len(snapshot.Networks))
	for _, network := range snapshot.Networks {
		q := NetworkIncidentRecordQuery{NetworkID: network.ID, Since: since, Until: until}
		stats := summarizeNetworkIncidents(q, byNetwork[network.ID])
		items = append(items, NetworkIncidentStatsByNetworkItem{NetworkID: network.ID, NetworkName: network.Name, Stats: stats})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].NetworkName != items[j].NetworkName {
			return items[i].NetworkName < items[j].NetworkName
		}
		return items[i].NetworkID < items[j].NetworkID
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Networks []NetworkIncidentStatsByNetworkItem `json:"networks"`
	}{Networks: items})
}

func (s *SQLiteStore) NetworkIncidentStatsSnapshot(since, until time.Time) (RegistrySnapshot, []NetworkIncident, error) {
	if s == nil || s.db == nil {
		return RegistrySnapshot{}, nil, errors.New("sqlite store is not open")
	}
	if !since.IsZero() && !until.IsZero() && since.After(until) {
		return RegistrySnapshot{}, nil, errors.New("invalid network incident time range")
	}
	snapshot, err := s.RegistrySnapshot()
	if err != nil {
		return RegistrySnapshot{}, nil, err
	}

	statement := "SELECT payload_json FROM network_incident_records"
	args := make([]any, 0, 2)
	where := make([]string, 0, 2)
	if !since.IsZero() {
		where = append(where, "started_at >= ?")
		args = append(args, formatObservationTime(since))
	}
	if !until.IsZero() {
		where = append(where, "started_at <= ?")
		args = append(args, formatObservationTime(until))
	}
	if len(where) > 0 {
		statement += " WHERE " + strings.Join(where, " AND ")
	}
	statement += " ORDER BY started_at DESC, id DESC"

	rows, err := s.db.Query(statement, args...)
	if err != nil {
		return RegistrySnapshot{}, nil, err
	}
	defer rows.Close()

	incidents := make([]NetworkIncident, 0)
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return RegistrySnapshot{}, nil, err
		}
		var incident NetworkIncident
		if err := json.Unmarshal(payload, &incident); err != nil {
			return RegistrySnapshot{}, nil, err
		}
		incidents = append(incidents, incident)
	}
	if err := rows.Err(); err != nil {
		return RegistrySnapshot{}, nil, err
	}
	return snapshot, incidents, nil
}
