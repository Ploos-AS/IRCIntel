package core

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/Ploos-AS/IRCIntel/internal/agent"
	"github.com/Ploos-AS/IRCIntel/internal/probe"
)

func TestIncidentLifecycleHandlerFiltersPersistedRecords(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()

	base := time.Date(2026, 9, 9, 8, 0, 0, 0, time.UTC)
	items := []agent.Observation{
		{AgentID: "oslo-1", ObservedAt: base, Endpoint: agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: true}},
		{AgentID: "ams-1", ObservedAt: base, Endpoint: agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: true}},
		{AgentID: "oslo-1", ObservedAt: base.Add(time.Minute), Endpoint: agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: false}},
		{AgentID: "ams-1", ObservedAt: base.Add(2 * time.Minute), Endpoint: agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: false}},
	}
	for _, item := range items {
		if err := store.Store(item); err != nil { t.Fatal(err) }
	}

	handler := IncidentLifecycleHandler{Reader: store}
	target := "/api/v1/incidents/lifecycle?status=open&host=irc.example&since=2026-09-09T08:00:00Z&until=2026-09-09T08:05:00Z&limit=10"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
	if response.Code != http.StatusOK { t.Fatalf("status=%d body=%q", response.Code, response.Body.String()) }
	if body := response.Body.String(); body == "" || body == "{\"incidents\":[]}\n" { t.Fatalf("body=%q", body) }
}

func TestIncidentLifecycleHandlerRejectsInvalidFilters(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()
	handler := IncidentLifecycleHandler{Reader: store}

	for _, target := range []string{
		"/api/v1/incidents/lifecycle?status=wat",
		"/api/v1/incidents/lifecycle?since=wat",
		"/api/v1/incidents/lifecycle?until=wat",
		"/api/v1/incidents/lifecycle?since=2026-09-09T09:00:00Z&until=2026-09-09T08:00:00Z",
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
		if response.Code != http.StatusBadRequest { t.Fatalf("target=%s status=%d", target, response.Code) }
	}
}
