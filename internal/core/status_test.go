package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/Ploos-AS/IRCIntel/internal/agent"
	"github.com/Ploos-AS/IRCIntel/internal/probe"
)

func TestStatusHandlerAggregatesLatestAgentMeasurements(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()

	base := time.Date(2026, 9, 9, 4, 0, 0, 0, time.UTC)
	items := []agent.Observation{
		{AgentID: "oslo-1", ObservedAt: base, Endpoint: agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: false}},
		{AgentID: "oslo-1", ObservedAt: base.Add(time.Minute), Endpoint: agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: true, DualStackOK: true}},
		{AgentID: "ams-1", ObservedAt: base.Add(2 * time.Minute), Endpoint: agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: false}},
	}
	for _, item := range items { if err := store.Store(item); err != nil { t.Fatal(err) } }

	handler := StatusHandler{Reader: store, Now: func() time.Time { return base.Add(5 * time.Minute) }}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/endpoints/status", nil))
	if response.Code != http.StatusOK { t.Fatalf("status=%d body=%q", response.Code, response.Body.String()) }

	var envelope struct { Endpoints []EndpointStatus `json:"endpoints"` }
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil { t.Fatal(err) }
	if len(envelope.Endpoints) != 1 { t.Fatalf("endpoints=%+v", envelope.Endpoints) }
	status := envelope.Endpoints[0]
	if status.Status != "degraded" || status.Agents != 2 || status.ReachableAgents != 1 || status.DualStackAgents != 1 || status.StaleAgents != 0 {
		t.Fatalf("status=%+v", status)
	}
	if !status.LatestObservedAt.Equal(base.Add(2 * time.Minute)) { t.Fatalf("latest=%v", status.LatestObservedAt) }
}

func TestStatusHandlerReportsUpAndDown(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()
	base := time.Date(2026, 9, 9, 4, 0, 0, 0, time.UTC)
	for _, item := range []agent.Observation{
		{AgentID: "a", ObservedAt: base, Endpoint: agent.Endpoint{Host: "down.example", Port: "6667"}, Result: probe.EndpointResult{Reachable: false}},
		{AgentID: "a", ObservedAt: base, Endpoint: agent.Endpoint{Host: "up.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: true}},
		{AgentID: "b", ObservedAt: base, Endpoint: agent.Endpoint{Host: "up.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: true}},
	} { if err := store.Store(item); err != nil { t.Fatal(err) } }
	handler := StatusHandler{Reader: store, Now: func() time.Time { return base.Add(time.Minute) }}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/endpoints/status", nil))
	var envelope struct { Endpoints []EndpointStatus `json:"endpoints"` }
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil { t.Fatal(err) }
	if len(envelope.Endpoints) != 2 || envelope.Endpoints[0].Status != "down" || envelope.Endpoints[1].Status != "up" {
		t.Fatalf("endpoints=%+v", envelope.Endpoints)
	}
}

func TestStatusHandlerExcludesStaleAgentsFromHealth(t *testing.T) {
	base := time.Date(2026, 9, 9, 6, 0, 0, 0, time.UTC)
	reader := &MemoryStatusReader{Observations: []agent.Observation{
		{AgentID: "fresh", ObservedAt: base.Add(-2 * time.Minute), Endpoint: agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: true}},
		{AgentID: "stale", ObservedAt: base.Add(-30 * time.Minute), Endpoint: agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: false}},
	}}
	handler := StatusHandler{Reader: reader, Now: func() time.Time { return base }, Freshness: 15 * time.Minute}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/endpoints/status", nil))
	var envelope struct { Endpoints []EndpointStatus `json:"endpoints"` }
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil { t.Fatal(err) }
	if len(envelope.Endpoints) != 1 { t.Fatalf("endpoints=%+v", envelope.Endpoints) }
	status := envelope.Endpoints[0]
	if status.Status != "up" || status.Agents != 1 || status.ReachableAgents != 1 || status.StaleAgents != 1 {
		t.Fatalf("status=%+v", status)
	}
}

func TestStatusHandlerReportsFullyStaleEndpoint(t *testing.T) {
	base := time.Date(2026, 9, 9, 6, 0, 0, 0, time.UTC)
	reader := &MemoryStatusReader{Observations: []agent.Observation{
		{AgentID: "old", ObservedAt: base.Add(-time.Hour), Endpoint: agent.Endpoint{Host: "stale.example", Port: "6667"}, Result: probe.EndpointResult{Reachable: true}},
	}}
	handler := StatusHandler{Reader: reader, Now: func() time.Time { return base }, Freshness: 15 * time.Minute}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/endpoints/status", nil))
	var envelope struct { Endpoints []EndpointStatus `json:"endpoints"` }
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil { t.Fatal(err) }
	if len(envelope.Endpoints) != 1 || envelope.Endpoints[0].Status != "stale" || envelope.Endpoints[0].Agents != 0 || envelope.Endpoints[0].StaleAgents != 1 {
		t.Fatalf("endpoints=%+v", envelope.Endpoints)
	}
}

func TestStatusHandlerRequiresAuthentication(t *testing.T) {
	handler := StatusHandler{Token: "secret", Reader: &MemoryStatusReader{}}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/endpoints/status", nil))
	if response.Code != http.StatusUnauthorized { t.Fatalf("status=%d", response.Code) }
}

type MemoryStatusReader struct { Observations []agent.Observation }
func (r *MemoryStatusReader) LatestEndpointObservations() ([]agent.Observation, error) { return r.Observations, nil }
