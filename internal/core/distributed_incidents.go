package core

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	defaultCorrelationWindow = 5 * time.Minute
	minCorrelationWindow     = 30 * time.Second
	maxCorrelationWindow     = 15 * time.Minute
)

type DistributedIncident struct {
	Type        string    `json:"type"`
	Host        string    `json:"host"`
	Port        string    `json:"port"`
	TLS         bool      `json:"tls"`
	StartedAt   time.Time `json:"started_at"`
	LastEventAt time.Time `json:"last_event_at"`
	Agents      []string  `json:"agents"`
	AgentCount  int       `json:"agent_count"`
}

type DistributedIncidentHandler struct {
	Token  string
	Reader IncidentReader
}

func (h DistributedIncidentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.Reader == nil {
		http.Error(w, "incident reader unavailable", http.StatusServiceUnavailable)
		return
	}
	if h.Token != "" && r.Header.Get("Authorization") != "Bearer "+h.Token {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	limit := defaultIncidentLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > maxIncidentLimit {
			http.Error(w, "invalid limit", http.StatusBadRequest)
			return
		}
		limit = parsed
	}

	window := defaultCorrelationWindow
	if raw := strings.TrimSpace(r.URL.Query().Get("window")); raw != "" {
		parsed, err := time.ParseDuration(raw)
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
	incidents := correlateIncidentEvents(events, window)
	if len(incidents) > limit {
		incidents = incidents[:limit]
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Incidents []DistributedIncident `json:"incidents"`
	}{Incidents: incidents})
}

func correlateIncidentEvents(events []IncidentEvent, window time.Duration) []DistributedIncident {
	type endpointKey struct {
		typeName string
		host     string
		port     string
		tls      bool
	}
	type cluster struct {
		incident DistributedIncident
		agents   map[string]struct{}
	}

	ordered := append([]IncidentEvent(nil), events...)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].ObservedAt.Before(ordered[j].ObservedAt)
	})

	active := make(map[endpointKey]*cluster)
	completed := make([]*cluster, 0)
	for _, event := range ordered {
		key := endpointKey{typeName: event.Type, host: event.Host, port: event.Port, tls: event.TLS}
		current := active[key]
		if current == nil || event.ObservedAt.Sub(current.incident.LastEventAt) > window {
			if current != nil {
				completed = append(completed, current)
			}
			current = &cluster{
				incident: DistributedIncident{
					Type:        event.Type,
					Host:        event.Host,
					Port:        event.Port,
					TLS:         event.TLS,
					StartedAt:   event.ObservedAt,
					LastEventAt: event.ObservedAt,
				},
				agents: make(map[string]struct{}),
			}
			active[key] = current
		}
		current.agents[event.AgentID] = struct{}{}
		if event.ObservedAt.After(current.incident.LastEventAt) {
			current.incident.LastEventAt = event.ObservedAt
		}
	}
	for _, current := range active {
		completed = append(completed, current)
	}

	incidents := make([]DistributedIncident, 0)
	for _, current := range completed {
		if len(current.agents) < 2 {
			continue
		}
		for agentID := range current.agents {
			current.incident.Agents = append(current.incident.Agents, agentID)
		}
		sort.Strings(current.incident.Agents)
		current.incident.AgentCount = len(current.incident.Agents)
		incidents = append(incidents, current.incident)
	}
	sort.Slice(incidents, func(i, j int) bool {
		return incidents[i].LastEventAt.After(incidents[j].LastEventAt)
	})
	return incidents
}
