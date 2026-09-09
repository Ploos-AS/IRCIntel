package core

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestDiscoveryCandidateDeduplicatesByEndpointAndSource(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()

	first := time.Date(2026, 9, 9, 6, 40, 0, 0, time.UTC)
	second := first.Add(5 * time.Minute)
	input := DiscoveryCandidateInput{Host: "IRC.Example", Port: "6697", TLS: true, Source: "Agent", SourceRef: "obs-1"}
	one, err := store.UpsertDiscoveryCandidate(input, first)
	if err != nil { t.Fatal(err) }
	input.SourceRef = "obs-2"
	two, err := store.UpsertDiscoveryCandidate(input, second)
	if err != nil { t.Fatal(err) }
	if one.ID != two.ID { t.Fatalf("candidate ids differ: %q %q", one.ID, two.ID) }
	if two.Host != "irc.example" || two.Source != "agent" { t.Fatalf("candidate=%+v", two) }
	if two.Status != "pending" || two.SeenCount != 2 { t.Fatalf("candidate=%+v", two) }
	if !two.FirstSeen.Equal(first) || !two.LastSeen.Equal(second) || two.SourceRef != "obs-2" { t.Fatalf("candidate=%+v", two) }
}

func TestDiscoveryHandlerIntakeAndList(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()
	now := time.Date(2026, 9, 9, 6, 45, 0, 0, time.UTC)
	h := DiscoveryHandler{Token: "secret", Store: store, Now: func() time.Time { return now }}

	body := bytes.NewBufferString(`{"host":"IRC.Example","port":"6697","tls":true,"source":"agent-oslo","source_ref":"obs-42"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/discovery/candidates", body)
	req.Header.Set("Authorization", "Bearer secret")
	res := httptest.NewRecorder()
	h.Intake(res, req)
	if res.Code != http.StatusAccepted { t.Fatalf("status=%d body=%q", res.Code, res.Body.String()) }
	var candidate DiscoveryCandidate
	if err := json.NewDecoder(res.Body).Decode(&candidate); err != nil { t.Fatal(err) }
	if candidate.Status != "pending" || candidate.SeenCount != 1 || candidate.Host != "irc.example" { t.Fatalf("candidate=%+v", candidate) }

	req = httptest.NewRequest(http.MethodGet, "/api/v1/discovery/candidates?status=pending", nil)
	req.Header.Set("Authorization", "Bearer secret")
	res = httptest.NewRecorder()
	h.List(res, req)
	if res.Code != http.StatusOK { t.Fatalf("status=%d body=%q", res.Code, res.Body.String()) }
	var envelope struct { Candidates []DiscoveryCandidate `json:"candidates"` }
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil { t.Fatal(err) }
	if len(envelope.Candidates) != 1 || envelope.Candidates[0].ID != candidate.ID { t.Fatalf("candidates=%+v", envelope.Candidates) }
}

func TestDiscoveryHandlerRequiresTokenAndValidStatus(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()

	res := httptest.NewRecorder()
	DiscoveryHandler{Store: store}.Intake(res, httptest.NewRequest(http.MethodPost, "/api/v1/discovery/candidates", bytes.NewBufferString(`{}`)))
	if res.Code != http.StatusServiceUnavailable { t.Fatalf("status=%d", res.Code) }

	h := DiscoveryHandler{Token: "secret", Store: store}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/discovery/candidates?status=maybe", nil)
	req.Header.Set("Authorization", "Bearer secret")
	res = httptest.NewRecorder()
	h.List(res, req)
	if res.Code != http.StatusBadRequest { t.Fatalf("status=%d", res.Code) }
}
