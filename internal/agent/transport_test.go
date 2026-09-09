package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestHTTPSubmitterPostsObservation(t *testing.T) {
	var got Observation
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost { t.Fatalf("method=%s", r.Method) }
		if r.Header.Get("Content-Type") != "application/json" { t.Fatalf("content-type=%q", r.Header.Get("Content-Type")) }
		if r.Header.Get("Authorization") != "Bearer secret" { t.Fatalf("authorization=%q", r.Header.Get("Authorization")) }
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil { t.Fatal(err) }
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	observation := Observation{AgentID: "probe-oslo-1", ObservedAt: time.Date(2026, 9, 9, 2, 0, 0, 0, time.UTC)}
	s := HTTPSubmitter{URL: server.URL, Token: "secret", Retries: 0}
	if err := s.Submit(context.Background(), observation); err != nil { t.Fatal(err) }
	if got.AgentID != observation.AgentID || !got.ObservedAt.Equal(observation.ObservedAt) {
		t.Fatalf("got=%+v", got)
	}
}

func TestHTTPSubmitterRetriesServerFailure(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) < 3 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	s := HTTPSubmitter{
		URL:        server.URL,
		Retries:    2,
		RetryDelay: time.Millisecond,
		Sleep: func(context.Context, time.Duration) error { return nil },
	}
	if err := s.Submit(context.Background(), Observation{AgentID: "probe-oslo-1"}); err != nil { t.Fatal(err) }
	if got := attempts.Load(); got != 3 { t.Fatalf("attempts=%d", got) }
}

func TestHTTPSubmitterReturnsFinalFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no", http.StatusBadGateway)
	}))
	defer server.Close()

	s := HTTPSubmitter{
		URL:        server.URL,
		Retries:    1,
		RetryDelay: time.Millisecond,
		Sleep: func(context.Context, time.Duration) error { return nil },
	}
	if err := s.Submit(context.Background(), Observation{AgentID: "probe-oslo-1"}); err == nil {
		t.Fatal("expected final failure")
	}
}
