package core

import (
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"github.com/Ploos-AS/IRCIntel/internal/agent"
)

const defaultStatusFreshness = 15 * time.Minute

type EndpointStatus struct {
	Host             string    `json:"host"`
	Port             string    `json:"port"`
	TLS              bool      `json:"tls"`
	Status           string    `json:"status"`
	Agents           int       `json:"agents"`
	ReachableAgents  int       `json:"reachable_agents"`
	DualStackAgents  int       `json:"dual_stack_agents"`
	StaleAgents      int       `json:"stale_agents"`
	LatestObservedAt time.Time `json:"latest_observed_at"`
}

type EndpointStatusReader interface {
	LatestEndpointObservations() ([]agent.Observation, error)
}

type StatusHandler struct {
	Token     string
	Reader    EndpointStatusReader
	Freshness time.Duration
	Now       func() time.Time
}

func (h StatusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.Reader == nil {
		http.Error(w, "endpoint status reader unavailable", http.StatusServiceUnavailable)
		return
	}
	if h.Token != "" && r.Header.Get("Authorization") != "Bearer "+h.Token {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	observations, err := h.Reader.LatestEndpointObservations()
	if err != nil {
		http.Error(w, "endpoint status query failed", http.StatusServiceUnavailable)
		return
	}

	freshness := h.Freshness
	if freshness <= 0 { freshness = defaultStatusFreshness }
	now := time.Now().UTC()
	if h.Now != nil { now = h.Now().UTC() }
	cutoff := now.Add(-freshness)

	type endpointKey struct {
		host string
		port string
		tls  bool
	}
	statusByEndpoint := make(map[endpointKey]*EndpointStatus)
	for _, observation := range observations {
		key := endpointKey{host: observation.Endpoint.Host, port: observation.Endpoint.Port, tls: observation.Endpoint.TLS}
		status := statusByEndpoint[key]
		if status == nil {
			status = &EndpointStatus{Host: key.host, Port: key.port, TLS: key.tls}
			statusByEndpoint[key] = status
		}
		if observation.ObservedAt.After(status.LatestObservedAt) {
			status.LatestObservedAt = observation.ObservedAt
		}
		if observation.ObservedAt.Before(cutoff) {
			status.StaleAgents++
			continue
		}
		status.Agents++
		if observation.Result.Reachable { status.ReachableAgents++ }
		if observation.Result.DualStackOK { status.DualStackAgents++ }
	}

	statuses := make([]EndpointStatus, 0, len(statusByEndpoint))
	for _, status := range statusByEndpoint {
		switch {
		case status.Agents == 0:
			status.Status = "stale"
		case status.ReachableAgents == 0:
			status.Status = "down"
		case status.ReachableAgents == status.Agents:
			status.Status = "up"
		default:
			status.Status = "degraded"
		}
		statuses = append(statuses, *status)
	}
	sort.Slice(statuses, func(i, j int) bool {
		if statuses[i].Host != statuses[j].Host { return statuses[i].Host < statuses[j].Host }
		if statuses[i].Port != statuses[j].Port { return statuses[i].Port < statuses[j].Port }
		return !statuses[i].TLS && statuses[j].TLS
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Endpoints []EndpointStatus `json:"endpoints"`
	}{Endpoints: statuses})
}
