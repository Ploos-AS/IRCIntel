package core

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

type NetworkIncident struct {
	NetworkID         string     `json:"network_id"`
	NetworkName       string     `json:"network_name"`
	Status            string     `json:"status"`
	Severity          string     `json:"severity"`
	StartedAt         time.Time  `json:"started_at"`
	RecoveredAt       *time.Time `json:"recovered_at,omitempty"`
	DurationSeconds   *int64     `json:"duration_seconds,omitempty"`
	AffectedEndpoints int        `json:"affected_endpoints"`
	TotalEndpoints    int        `json:"total_endpoints"`
}

type NetworkIncidentReader interface {
	RegistrySnapshot() (RegistrySnapshot, error)
	ListIncidentRecords(IncidentRecordQuery) ([]IncidentLifecycle, error)
}

type NetworkIncidentHandler struct {
	Token  string
	Reader NetworkIncidentReader
}

func (h NetworkIncidentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.Reader == nil { http.Error(w, "network incident reader unavailable", http.StatusServiceUnavailable); return }
	if h.Token != "" && r.Header.Get("Authorization") != "Bearer "+h.Token { http.Error(w, "unauthorized", http.StatusUnauthorized); return }

	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		v, err := strconv.Atoi(raw); if err != nil || v < 1 || v > 500 { http.Error(w, "invalid limit", http.StatusBadRequest); return }; limit = v
	}
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	if status != "" && status != "open" && status != "closed" { http.Error(w, "invalid status", http.StatusBadRequest); return }
	networkID := strings.TrimSpace(r.URL.Query().Get("network_id"))

	snapshot, err := h.Reader.RegistrySnapshot()
	if err != nil { http.Error(w, "registry query failed", http.StatusServiceUnavailable); return }
	records, err := h.Reader.ListIncidentRecords(IncidentRecordQuery{Limit: 500})
	if err != nil { http.Error(w, "incident record query failed", http.StatusServiceUnavailable); return }
	items := deriveNetworkIncidents(snapshot, records)
	filtered := make([]NetworkIncident, 0, len(items))
	for _, item := range items {
		if status != "" && item.Status != status { continue }
		if networkID != "" && item.NetworkID != networkID { continue }
		filtered = append(filtered, item)
		if len(filtered) == limit { break }
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct{ Incidents []NetworkIncident `json:"incidents"` }{filtered})
}

func deriveNetworkIncidents(snapshot RegistrySnapshot, records []IncidentLifecycle) []NetworkIncident {
	type endpointKey struct { host, port string; tls bool }
	type event struct { at time.Time; start bool; endpoint endpointKey }
	serverNetwork := map[string]string{}
	for _, server := range snapshot.Servers { serverNetwork[server.ID] = server.NetworkID }
	networkName := map[string]string{}
	for _, network := range snapshot.Networks { networkName[network.ID] = network.Name }
	endpointNetwork := map[endpointKey]string{}
	totals := map[string]int{}
	for _, endpoint := range snapshot.Endpoints {
		nid := serverNetwork[endpoint.ServerID]
		if nid == "" { continue }
		endpointNetwork[endpointKey{endpoint.Host, endpoint.Port, endpoint.TLS}] = nid
		totals[nid]++
	}
	events := map[string][]event{}
	for _, record := range records {
		key := endpointKey{record.Host, record.Port, record.TLS}
		nid := endpointNetwork[key]
		if nid == "" { continue }
		events[nid] = append(events[nid], event{at: record.StartedAt, start: true, endpoint: key})
		if record.RecoveredAt != nil { events[nid] = append(events[nid], event{at: *record.RecoveredAt, start: false, endpoint: key}) }
	}

	out := make([]NetworkIncident, 0)
	for nid, evs := range events {
		sort.SliceStable(evs, func(i, j int) bool {
			if !evs[i].at.Equal(evs[j].at) { return evs[i].at.Before(evs[j].at) }
			return !evs[i].start && evs[j].start
		})
		active := map[endpointKey]bool{}
		var current *NetworkIncident
		for _, ev := range evs {
			before := len(active)
			if ev.start { active[ev.endpoint] = true } else { delete(active, ev.endpoint) }
			after := len(active)
			if before == 0 && after > 0 {
				current = &NetworkIncident{NetworkID: nid, NetworkName: networkName[nid], Status: "open", Severity: "degraded", StartedAt: ev.at, AffectedEndpoints: after, TotalEndpoints: totals[nid]}
			}
			if current != nil && after > current.AffectedEndpoints { current.AffectedEndpoints = after }
			if current != nil && totals[nid] > 0 && after == totals[nid] { current.Severity = "down" }
			if current != nil && before > 0 && after == 0 {
				recovered := ev.at
				duration := int64(recovered.Sub(current.StartedAt).Seconds())
				current.Status = "closed"; current.RecoveredAt = &recovered; current.DurationSeconds = &duration
				out = append(out, *current); current = nil
			}
		}
		if current != nil { out = append(out, *current) }
	}
	sort.Slice(out, func(i, j int) bool { if !out[i].StartedAt.Equal(out[j].StartedAt) { return out[i].StartedAt.After(out[j].StartedAt) }; return out[i].NetworkID < out[j].NetworkID })
	return out
}
