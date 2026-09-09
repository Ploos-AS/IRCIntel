package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/Ploos-AS/IRCIntel/internal/agent"
)

func TestSQLiteStoreListsNewestAndFilters(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()

	base := time.Date(2026, 9, 9, 3, 0, 0, 0, time.UTC)
	items := []agent.Observation{
		{AgentID: "oslo-1", ObservedAt: base, Endpoint: agent.Endpoint{Host: "irc.one", Port: "6697", TLS: true}},
		{AgentID: "ams-1", ObservedAt: base.Add(time.Minute), Endpoint: agent.Endpoint{Host: "irc.one", Port: "6697", TLS: true}},
		{AgentID: "oslo-1", ObservedAt: base.Add(2 * time.Minute), Endpoint: agent.Endpoint{Host: "irc.two", Port: "6697", TLS: true}},
	}
	for _, item := range items {
		if err := store.Store(item); err != nil { t.Fatal(err) }
	}

	got, err := store.List(ObservationQuery{AgentID: "oslo-1", Limit: 10})
	if err != nil { t.Fatal(err) }
	if len(got) != 2 { t.Fatalf("len=%d", len(got)) }
	if got[0].Endpoint.Host != "irc.two" || got[1].Endpoint.Host != "irc.one" {
		t.Fatalf("unexpected order: %+v", got)
	}

	got, err = store.List(ObservationQuery{Host: "irc.one", Limit: 1})
	if err != nil { t.Fatal(err) }
	if len(got) != 1 || got[0].AgentID != "ams-1" { t.Fatalf("unexpected host result: %+v", got) }
}

func TestReadHandlerValidatesLimitAndAuthentication(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()

	handler := ReadHandler{Token: "secret", Reader: store}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/observations?limit=0", nil))
	if response.Code != http.StatusUnauthorized { t.Fatalf("unauthorized status=%d", response.Code) }

	req := httptest.NewRequest(http.MethodGet, "/api/v1/observations?limit=501", nil)
	req.Header.Set("Authorization", "Bearer secret")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Code != http.StatusBadRequest { t.Fatalf("invalid limit status=%d", response.Code) }
}

func TestReadHandlerReturnsEnvelope(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()
	if err := store.Store(validObservation()); err != nil { t.Fatal(err) }

	handler := ReadHandler{Reader: store}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/observations", nil))
	if response.Code != http.StatusOK { t.Fatalf("status=%d body=%q", response.Code, response.Body.String()) }
	var envelope struct { Observations []agent.Observation `json:"observations"` }
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil { t.Fatal(err) }
	if len(envelope.Observations) != 1 || envelope.Observations[0].AgentID != "oslo-1" { t.Fatalf("envelope=%+v", envelope) }
}
