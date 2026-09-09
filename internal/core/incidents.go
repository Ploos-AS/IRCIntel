package core

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/Ploos-AS/IRCIntel/internal/agent"
)

const (
	defaultIncidentLimit = 100
	maxIncidentLimit     = 500
	incidentScanLimit    = 5000
)

type IncidentEvent struct {
	Type               string    `json:"type"`
	AgentID            string    `json:"agent_id"`
	Host               string    `json:"host"`
	Port               string    `json:"port"`
	TLS                bool      `json:"tls"`
	ObservedAt         time.Time `json:"observed_at"`
	PreviousObservedAt time.Time `json:"previous_observed_at"`
}

type IncidentReader interface {
	RecentObservations(int) ([]agent.Observation, error)
}

type IncidentHandler struct {
	Token  string
	Reader IncidentReader
}

func (h IncidentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	observations, err := h.Reader.RecentObservations(incidentScanLimit)
	if err != nil {
		http.Error(w, "incident query failed", http.StatusServiceUnavailable)
		return
	}
	events := deriveIncidentEvents(observations)
	if len(events) > limit {
		events = events[:limit]
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Events []IncidentEvent `json:"events"`
	}{Events: events})
}

func deriveIncidentEvents(observations []agent.Observation) []IncidentEvent {
	type endpointKey struct {
		agentID string
		host    string
		port    string
		tls     bool
	}

	sort.Slice(observations, func(i, j int) bool {
		return observations[i].ObservedAt.Before(observations[j].ObservedAt)
	})

	previous := make(map[endpointKey]agent.Observation)
	events := make([]IncidentEvent, 0)
	for _, observation := range observations {
		key := endpointKey{
			agentID: observation.AgentID,
			host:    observation.Endpoint.Host,
			port:    observation.Endpoint.Port,
			tls:     observation.Endpoint.TLS,
		}
		prior, ok := previous[key]
		previous[key] = observation
		if !ok || prior.Result.Reachable == observation.Result.Reachable {
			continue
		}

		eventType := "down"
		if observation.Result.Reachable {
			eventType = "recovered"
		}
		events = append(events, IncidentEvent{
			Type:               eventType,
			AgentID:            observation.AgentID,
			Host:               observation.Endpoint.Host,
			Port:               observation.Endpoint.Port,
			TLS:                observation.Endpoint.TLS,
			ObservedAt:         observation.ObservedAt,
			PreviousObservedAt: prior.ObservedAt,
		})
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].ObservedAt.After(events[j].ObservedAt)
	})
	return events
}
