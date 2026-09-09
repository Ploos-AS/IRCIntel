package core

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Ploos-AS/IRCIntel/internal/agent"
)

func validObservation() agent.Observation {
	return agent.Observation{
		AgentID: "oslo-1",
		ObservedAt: time.Date(2026, 9, 9, 2, 0, 0, 0, time.UTC),
		Endpoint: agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true},
	}
}

func TestIngestAcceptsAuthenticatedObservation(t *testing.T) {
	store := &MemoryStore{}
	handler := IngestHandler{Token: "secret", Store: store}
	payload, err := json.Marshal(validObservation())
	if err != nil { t.Fatal(err) }
	req := httptest.NewRequest(http.MethodPost, "/api/v1/observations", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer secret")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Code != http.StatusAccepted { t.Fatalf("status=%d body=%q", response.Code, response.Body.String()) }
	if got := store.Observations(); len(got) != 1 || got[0].AgentID != "oslo-1" { t.Fatalf("observations=%+v", got) }
}

func TestIngestRejectsBadAuthentication(t *testing.T) {
	handler := IngestHandler{Token: "secret", Store: &MemoryStore{}}
	payload, _ := json.Marshal(validObservation())
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/observations", bytes.NewReader(payload)))
	if response.Code != http.StatusUnauthorized { t.Fatalf("status=%d", response.Code) }
}

func TestIngestRejectsInvalidObservation(t *testing.T) {
	store := &MemoryStore{}
	handler := IngestHandler{Store: store}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/observations", bytes.NewBufferString(`{"agent_id":""}`)))
	if response.Code != http.StatusBadRequest { t.Fatalf("status=%d", response.Code) }
	if len(store.Observations()) != 0 { t.Fatal("invalid observation was stored") }
}

func TestIngestRejectsUnknownFields(t *testing.T) {
	store := &MemoryStore{}
	handler := IngestHandler{Store: store}
	payload := `{"agent_id":"oslo-1","observed_at":"2026-09-09T02:00:00Z","endpoint":{"host":"irc.example"},"unexpected":true}`
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/observations", bytes.NewBufferString(payload)))
	if response.Code != http.StatusBadRequest { t.Fatalf("status=%d", response.Code) }
}
