package core

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/Ploos-AS/IRCIntel/internal/agent"
)

const maxObservationBytes = 1 << 20

type ObservationStore interface {
	Store(agent.Observation) error
}

type MemoryStore struct {
	mu           sync.Mutex
	observations []agent.Observation
}

func (s *MemoryStore) Store(observation agent.Observation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.observations = append(s.observations, observation)
	return nil
}

func (s *MemoryStore) Observations() []agent.Observation {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]agent.Observation(nil), s.observations...)
}

type IngestHandler struct {
	Token string
	Store ObservationStore
}

func (h IngestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		http.Error(w, "observation store unavailable", http.StatusServiceUnavailable)
		return
	}
	if h.Token != "" && r.Header.Get("Authorization") != "Bearer "+h.Token {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxObservationBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var observation agent.Observation
	if err := decoder.Decode(&observation); err != nil {
		http.Error(w, "invalid observation", http.StatusBadRequest)
		return
	}
	if err := ensureEOF(decoder); err != nil {
		http.Error(w, "invalid observation", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(observation.AgentID) == "" || observation.ObservedAt.IsZero() || strings.TrimSpace(observation.Endpoint.Host) == "" {
		http.Error(w, "invalid observation", http.StatusBadRequest)
		return
	}
	if err := h.Store.Store(observation); err != nil {
		http.Error(w, "observation store failed", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func ensureEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return err
	}
	return errors.New("multiple JSON values")
}
