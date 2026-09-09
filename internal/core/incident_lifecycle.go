package core

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

type IncidentLifecycle struct {
	Host            string     `json:"host"`
	Port            string     `json:"port"`
	TLS             bool       `json:"tls"`
	Status          string     `json:"status"`
	StartedAt       time.Time  `json:"started_at"`
	RecoveredAt     *time.Time `json:"recovered_at,omitempty"`
	DurationSeconds *int64     `json:"duration_seconds,omitempty"`
	DownAgents      []string   `json:"down_agents"`
	RecoveryAgents  []string   `json:"recovery_agents,omitempty"`
}

type IncidentLifecycleHandler struct {
	Token  string
	Reader IncidentReader
}

func (h IncidentLifecycleHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.Reader == nil {
		http.Error(w, "incident reader unavailable", http.StatusServiceUnavailable)
		return
	}
	if h.Token != "" && r.Header.Get("Authorization") != "Bearer "+h.Token {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	query, ok := parseIncidentRecordQuery(w, r)
	if !ok {
		return
	}

	rawWindow := strings.TrimSpace(r.URL.Query().Get("window"))
	if rawWindow == "" {
		if records, ok := h.Reader.(IncidentRecordReader); ok {
			lifecycles, err := records.ListIncidentRecords(query)
			if err != nil {
				http.Error(w, "incident record query failed", http.StatusServiceUnavailable)
				return
			}
			writeLifecycleEnvelope(w, lifecycles)
			return
		}
	}

	window := defaultCorrelationWindow
	if rawWindow != "" {
		parsed, err := time.ParseDuration(rawWindow)
		if err != nil || parsed < minCorrelationWindow || parsed > maxCorrelationWindow {
			http.Error(w, "invalid correlation window", http.StatusBadRequest)
			return
		}
		window = parsed
	}

	observations, err := h.Reader.RecentObservations(incidentScanLimit)
	if err != nil {
		http.Error(w, "incident query failed", http.StatusServiceUnavailable)
		return
	}
	events := deriveIncidentEvents(observations)
	correlated := correlateIncidentEvents(events, window)
	lifecycles := filterIncidentLifecycles(pairIncidentLifecycle(correlated), query)
	if len(lifecycles) > query.Limit {
		lifecycles = lifecycles[:query.Limit]
	}
	writeLifecycleEnvelope(w, lifecycles)
}

func parseIncidentRecordQuery(w http.ResponseWriter, r *http.Request) (IncidentRecordQuery, bool) {
	query := IncidentRecordQuery{Limit: defaultIncidentLimit}
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > maxIncidentLimit {
			http.Error(w, "invalid limit", http.StatusBadRequest)
			return IncidentRecordQuery{}, false
		}
		query.Limit = parsed
	}
	query.Status = strings.TrimSpace(r.URL.Query().Get("status"))
	if query.Status != "" && query.Status != "open" && query.Status != "closed" {
		http.Error(w, "invalid status", http.StatusBadRequest)
		return IncidentRecordQuery{}, false
	}
	query.Host = strings.TrimSpace(r.URL.Query().Get("host"))
	var err error
	if raw := strings.TrimSpace(r.URL.Query().Get("since")); raw != "" {
		query.Since, err = time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			http.Error(w, "invalid since", http.StatusBadRequest)
			return IncidentRecordQuery{}, false
		}
		query.Since = query.Since.UTC()
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("until")); raw != "" {
		query.Until, err = time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			http.Error(w, "invalid until", http.StatusBadRequest)
			return IncidentRecordQuery{}, false
		}
		query.Until = query.Until.UTC()
	}
	if !query.Since.IsZero() && !query.Until.IsZero() && query.Since.After(query.Until) {
		http.Error(w, "invalid time range", http.StatusBadRequest)
		return IncidentRecordQuery{}, false
	}
	return query, true
}

func filterIncidentLifecycles(items []IncidentLifecycle, query IncidentRecordQuery) []IncidentLifecycle {
	filtered := make([]IncidentLifecycle, 0, len(items))
	for _, item := range items {
		if query.Status != "" && item.Status != query.Status {
			continue
		}
		if query.Host != "" && item.Host != query.Host {
			continue
		}
		if !query.Since.IsZero() && item.StartedAt.Before(query.Since) {
			continue
		}
		if !query.Until.IsZero() && item.StartedAt.After(query.Until) {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func writeLifecycleEnvelope(w http.ResponseWriter, lifecycles []IncidentLifecycle) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Incidents []IncidentLifecycle `json:"incidents"`
	}{Incidents: lifecycles})
}

func pairIncidentLifecycle(incidents []DistributedIncident) []IncidentLifecycle {
	type endpointKey struct {
		host string
		port string
		tls  bool
	}

	ordered := append([]DistributedIncident(nil), incidents...)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].StartedAt.Before(ordered[j].StartedAt)
	})

	open := make(map[endpointKey]int)
	lifecycles := make([]IncidentLifecycle, 0)
	for _, incident := range ordered {
		key := endpointKey{host: incident.Host, port: incident.Port, tls: incident.TLS}
		switch incident.Type {
		case "down":
			lifecycles = append(lifecycles, IncidentLifecycle{
				Host:       incident.Host,
				Port:       incident.Port,
				TLS:        incident.TLS,
				Status:     "open",
				StartedAt:  incident.StartedAt,
				DownAgents: append([]string(nil), incident.Agents...),
			})
			open[key] = len(lifecycles) - 1
		case "recovered":
			index, ok := open[key]
			if !ok || incident.StartedAt.Before(lifecycles[index].StartedAt) {
				continue
			}
			recoveredAt := incident.LastEventAt
			durationSeconds := int64(recoveredAt.Sub(lifecycles[index].StartedAt).Seconds())
			lifecycles[index].Status = "closed"
			lifecycles[index].RecoveredAt = &recoveredAt
			lifecycles[index].DurationSeconds = &durationSeconds
			lifecycles[index].RecoveryAgents = append([]string(nil), incident.Agents...)
			delete(open, key)
		}
	}

	sort.Slice(lifecycles, func(i, j int) bool {
		return lifecycles[i].StartedAt.After(lifecycles[j].StartedAt)
	})
	return lifecycles
}
