package core

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/Ploos-AS/IRCIntel/internal/agent"
)

const (
	defaultObservationLimit = 50
	maxObservationLimit     = 500
)

type ObservationQuery struct {
	AgentID string
	Host    string
	Limit   int
}

type ObservationReader interface {
	List(ObservationQuery) ([]agent.Observation, error)
}

type ReadHandler struct {
	Token  string
	Reader ObservationReader
}

func (h ReadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.Reader == nil {
		http.Error(w, "observation reader unavailable", http.StatusServiceUnavailable)
		return
	}
	if h.Token != "" && r.Header.Get("Authorization") != "Bearer "+h.Token {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	limit := defaultObservationLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > maxObservationLimit {
			http.Error(w, "invalid limit", http.StatusBadRequest)
			return
		}
		limit = parsed
	}

	observations, err := h.Reader.List(ObservationQuery{
		AgentID: strings.TrimSpace(r.URL.Query().Get("agent_id")),
		Host:    strings.TrimSpace(r.URL.Query().Get("host")),
		Limit:   limit,
	})
	if err != nil {
		http.Error(w, "observation query failed", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Observations []agent.Observation `json:"observations"`
	}{Observations: observations})
}
