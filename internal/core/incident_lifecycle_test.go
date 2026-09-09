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

func TestPairIncidentLifecycleClosesRecoveredIncident(t *testing.T) {
	base := time.Date(2026, 9, 9, 6, 0, 0, 0, time.UTC)
	incidents := []DistributedIncident{
		{Type: "down", Host: "irc.example", Port: "6697", TLS: true, StartedAt: base, LastEventAt: base.Add(time.Minute), Agents: []string{"ams-1", "oslo-1"}, AgentCount: 2},
		{Type: "recovered", Host: "irc.example", Port: "6697", TLS: true, StartedAt: base.Add(10 * time.Minute), LastEventAt: base.Add(11 * time.Minute), Agents: []string{"ams-1", "oslo-1"}, AgentCount: 2},
	}

	lifecycles := pairIncidentLifecycle(incidents)
	if len(lifecycles) != 1 { t.Fatalf("len=%d incidents=%+v", len(lifecycles), lifecycles) }
	incident := lifecycles[0]
	if incident.Status != "closed" || incident.RecoveredAt == nil || incident.DurationSeconds == nil {
		t.Fatalf("incident=%+v", incident)
	}
	if *incident.DurationSeconds != 660 { t.Fatalf("duration=%d", *incident.DurationSeconds) }
	if len(incident.DownAgents) != 2 || len(incident.RecoveryAgents) != 2 { t.Fatalf("incident=%+v", incident) }
}

func TestPairIncidentLifecycleKeepsOpenIncident(t *testing.T) {
	base := time.Date(2026, 9, 9, 6, 0, 0, 0, time.UTC)
	lifecycles := pairIncidentLifecycle([]DistributedIncident{
		{Type: "down", Host: "irc.example", Port: "6697", TLS: true, StartedAt: base, LastEventAt: base, Agents: []string{"ams-1", "oslo-1"}, AgentCount: 2},
	})
	if len(lifecycles) != 1 || lifecycles[0].Status != "open" || lifecycles[0].RecoveredAt != nil || lifecycles[0].DurationSeconds != nil {
		t.Fatalf("incidents=%+v", lifecycles)
	}
}

func TestIncidentLifecycleHandler(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()

	base := time.Date(2026, 9, 9, 6, 0, 0, 0, time.UTC)
	items := []agent.Observation{
		{AgentID: "oslo-1", ObservedAt: base, Endpoint: agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: true}},
		{AgentID: "ams-1", ObservedAt: base, Endpoint: agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: true}},
		{AgentID: "oslo-1", ObservedAt: base.Add(time.Minute), Endpoint: agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: false}},
		{AgentID: "ams-1", ObservedAt: base.Add(2 * time.Minute), Endpoint: agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: false}},
		{AgentID: "oslo-1", ObservedAt: base.Add(10 * time.Minute), Endpoint: agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: true}},
		{AgentID: "ams-1", ObservedAt: base.Add(11 * time.Minute), Endpoint: agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: true}},
	}
	for _, item := range items {
		if err := store.Store(item); err != nil { t.Fatal(err) }
	}

	handler := IncidentLifecycleHandler{Reader: store}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/incidents/lifecycle", nil))
	if response.Code != http.StatusOK { t.Fatalf("status=%d body=%q", response.Code, response.Body.String()) }
	var envelope struct { Incidents []IncidentLifecycle `json:"incidents"` }
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil { t.Fatal(err) }
	if len(envelope.Incidents) != 1 || envelope.Incidents[0].Status != "closed" {
		t.Fatalf("incidents=%+v", envelope.Incidents)
	}
}
