package core

import (
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"github.com/Ploos-AS/IRCIntel/internal/agent"
)

type NetworkStatus struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Status           string    `json:"status"`
	Endpoints        int       `json:"endpoints"`
	UpEndpoints      int       `json:"up_endpoints"`
	DegradedEndpoints int      `json:"degraded_endpoints"`
	DownEndpoints    int       `json:"down_endpoints"`
	StaleEndpoints   int       `json:"stale_endpoints"`
	LatestObservedAt time.Time `json:"latest_observed_at,omitempty"`
}

type NetworkStatusReader interface {
	RegistrySnapshot() (RegistrySnapshot, error)
	LatestEndpointObservations() ([]agent.Observation, error)
}

type NetworkStatusHandler struct {
	Token string
	Reader NetworkStatusReader
	Freshness time.Duration
	Now func() time.Time
}

func (h NetworkStatusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.Reader == nil { http.Error(w, "network status reader unavailable", http.StatusServiceUnavailable); return }
	if h.Token != "" && r.Header.Get("Authorization") != "Bearer "+h.Token { http.Error(w, "unauthorized", http.StatusUnauthorized); return }
	snapshot, err := h.Reader.RegistrySnapshot()
	if err != nil { http.Error(w, "registry query failed", http.StatusServiceUnavailable); return }
	observations, err := h.Reader.LatestEndpointObservations()
	if err != nil { http.Error(w, "endpoint status query failed", http.StatusServiceUnavailable); return }

	freshness := h.Freshness
	if freshness <= 0 { freshness = defaultStatusFreshness }
	now := time.Now().UTC()
	if h.Now != nil { now = h.Now().UTC() }
	cutoff := now.Add(-freshness)

	type key struct { host, port string; tls bool }
	type measured struct { fresh, reachable int; latest time.Time }
	measurements := map[key]*measured{}
	for _, o := range observations {
		k := key{o.Endpoint.Host, o.Endpoint.Port, o.Endpoint.TLS}
		m := measurements[k]
		if m == nil { m = &measured{}; measurements[k] = m }
		if o.ObservedAt.After(m.latest) { m.latest = o.ObservedAt }
		if o.ObservedAt.Before(cutoff) { continue }
		m.fresh++
		if o.Result.Reachable { m.reachable++ }
	}

	serverNetwork := map[string]string{}
	for _, s := range snapshot.Servers { serverNetwork[s.ID] = s.NetworkID }
	statuses := map[string]*NetworkStatus{}
	for _, n := range snapshot.Networks { statuses[n.ID] = &NetworkStatus{ID:n.ID, Name:n.Name} }
	for _, e := range snapshot.Endpoints {
		ns := statuses[serverNetwork[e.ServerID]]
		if ns == nil { continue }
		ns.Endpoints++
		m := measurements[key{e.Host,e.Port,e.TLS}]
		if m == nil || m.fresh == 0 { ns.StaleEndpoints++ } else if m.reachable == 0 { ns.DownEndpoints++ } else if m.reachable == m.fresh { ns.UpEndpoints++ } else { ns.DegradedEndpoints++ }
		if m != nil && m.latest.After(ns.LatestObservedAt) { ns.LatestObservedAt = m.latest }
	}

	out := make([]NetworkStatus,0,len(statuses))
	for _, ns := range statuses {
		switch {
		case ns.Endpoints == 0 || ns.StaleEndpoints == ns.Endpoints: ns.Status = "stale"
		case ns.DownEndpoints == ns.Endpoints: ns.Status = "down"
		case ns.UpEndpoints == ns.Endpoints: ns.Status = "up"
		default: ns.Status = "degraded"
		}
		out = append(out,*ns)
	}
	sort.Slice(out,func(i,j int) bool { if out[i].Name != out[j].Name { return out[i].Name < out[j].Name }; return out[i].ID < out[j].ID })
	w.Header().Set("Content-Type","application/json")
	_ = json.NewEncoder(w).Encode(struct{Networks []NetworkStatus `json:"networks"`}{out})
}
