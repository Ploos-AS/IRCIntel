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

func TestCorrelateIncidentEventsRequiresMultipleAgents(t *testing.T) {
	base := time.Date(2026, 9, 9, 5, 0, 0, 0, time.UTC)
	events := []IncidentEvent{
		{Type: "down", AgentID: "oslo-1", Host: "irc.example", Port: "6697", TLS: true, ObservedAt: base},
		{Type: "down", AgentID: "ams-1", Host: "irc.example", Port: "6697", TLS: true, ObservedAt: base.Add(time.Minute)},
		{Type: "down", AgentID: "oslo-1", Host: "irc.other", Port: "6697", TLS: true, ObservedAt: base.Add(2 * time.Minute)},
	}

	incidents := correlateIncidentEvents(events, 5*time.Minute)
	if len(incidents) != 1 { t.Fatalf("len=%d incidents=%+v", len(incidents), incidents) }
	incident := incidents[0]
	if incident.Type != "down" || incident.Host != "irc.example" || incident.AgentCount != 2 {
		t.Fatalf("incident=%+v", incident)
	}
	if len(incident.Agents) != 2 || incident.Agents[0] != "ams-1" || incident.Agents[1] != "oslo-1" {
		t.Fatalf("agents=%v", incident.Agents)
	}
}

func TestCorrelateIncidentEventsHonorsWindowAndType(t *testing.T) {
	base := time.Date(2026, 9, 9, 5, 0, 0, 0, time.UTC)
	events := []IncidentEvent{
		{Type: "down", AgentID: "oslo-1", Host: "irc.example", Port: "6697", TLS: true, ObservedAt: base},
		{Type: "down", AgentID: "ams-1", Host: "irc.example", Port: "6697", TLS: true, ObservedAt: base.Add(6 * time.Minute)},
		{Type: "recovered", AgentID: "oslo-1", Host: "irc.example", Port: "6697", TLS: true, ObservedAt: base.Add(7 * time.Minute)},
		{Type: "recovered", AgentID: "ams-1", Host: "irc.example", Port: "6697", TLS: true, ObservedAt: base.Add(8 * time.Minute)},
	}

	incidents := correlateIncidentEvents(events, 5*time.Minute)
	if len(incidents) != 1 || incidents[0].Type != "recovered" {
		t.Fatalf("incidents=%+v", incidents)
	}
}

func TestDistributedIncidentHandler(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()

	base := time.Date(2026, 9, 9, 5, 0, 0, 0, time.UTC)
	items := []agent.Observation{
		{AgentID: "oslo-1", ObservedAt: base, Endpoint: agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: true}},
		{AgentID: "ams-1", ObservedAt: base, Endpoint: agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: true}},
		{AgentID: "oslo-1", ObservedAt: base.Add(time.Minute), Endpoint: agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: false}},
		{AgentID: "ams-1", ObservedAt: base.Add(2 * time.Minute), Endpoint: agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: false}},
	}
	for _, item := range items {
		if err := store.Store(item); err != nil { t.Fatal(err) }
	}

	handler := DistributedIncidentHandler{Token: "secret", Reader: store}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/incidents/distributed", nil))
	if response.Code != http.StatusUnauthorized { t.Fatalf("unauthorized=%d", response.Code) }

	req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents/distributed?window=5m&limit=10", nil)
	req.Header.Set("Authorization", "Bearer secret")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Code != http.StatusOK { t.Fatalf("status=%d body=%q", response.Code, response.Body.String()) }
	var envelope struct { Incidents []DistributedIncident `json:"incidents"` }
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil { t.Fatal(err) }
	if len(envelope.Incidents) != 1 || envelope.Incidents[0].AgentCount != 2 {
		t.Fatalf("incidents=%+v", envelope.Incidents)
	}
}

func TestDistributedIncidentHandlerRejectsInvalidWindow(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()

	handler := DistributedIncidentHandler{Reader: store}
	for _, target := range []string{
		"/api/v1/incidents/distributed?window=10s",
		"/api/v1/incidents/distributed?window=20m",
		"/api/v1/incidents/distributed?window=wat",
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
		if response.Code != http.StatusBadRequest { t.Fatalf("target=%s status=%d", target, response.Code) }
	}
}
