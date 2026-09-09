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

func TestDeriveIncidentEvents(t *testing.T) {
	base := time.Date(2026, 9, 9, 4, 0, 0, 0, time.UTC)
	observations := []agent.Observation{
		{AgentID: "oslo-1", ObservedAt: base, Endpoint: agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: true}},
		{AgentID: "oslo-1", ObservedAt: base.Add(time.Minute), Endpoint: agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: false}},
		{AgentID: "ams-1", ObservedAt: base.Add(90 * time.Second), Endpoint: agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: true}},
		{AgentID: "oslo-1", ObservedAt: base.Add(2 * time.Minute), Endpoint: agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: true}},
	}

	events := deriveIncidentEvents(observations)
	if len(events) != 2 { t.Fatalf("len=%d events=%+v", len(events), events) }
	if events[0].Type != "recovered" || events[1].Type != "down" { t.Fatalf("events=%+v", events) }
	if events[0].PreviousObservedAt != base.Add(time.Minute) { t.Fatalf("previous=%s", events[0].PreviousObservedAt) }
}

func TestIncidentHandlerReturnsEventsAndAuth(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()

	base := time.Date(2026, 9, 9, 4, 0, 0, 0, time.UTC)
	items := []agent.Observation{
		{AgentID: "oslo-1", ObservedAt: base, Endpoint: agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: true}},
		{AgentID: "oslo-1", ObservedAt: base.Add(time.Minute), Endpoint: agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: false}},
	}
	for _, item := range items {
		if err := store.Store(item); err != nil { t.Fatal(err) }
	}

	handler := IncidentHandler{Token: "secret", Reader: store}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/incidents", nil))
	if response.Code != http.StatusUnauthorized { t.Fatalf("unauthorized status=%d", response.Code) }

	req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents?limit=10", nil)
	req.Header.Set("Authorization", "Bearer secret")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Code != http.StatusOK { t.Fatalf("status=%d body=%q", response.Code, response.Body.String()) }
	var envelope struct { Events []IncidentEvent `json:"events"` }
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil { t.Fatal(err) }
	if len(envelope.Events) != 1 || envelope.Events[0].Type != "down" { t.Fatalf("events=%+v", envelope.Events) }
}

func TestIncidentHandlerRejectsInvalidLimit(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()

	handler := IncidentHandler{Reader: store}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/incidents?limit=501", nil))
	if response.Code != http.StatusBadRequest { t.Fatalf("status=%d", response.Code) }
}
