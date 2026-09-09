package core

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Ploos-AS/IRCIntel/internal/agent"
)

const (
	defaultObservationLimit = 50
	maxObservationLimit     = 500
)

type ObservationQuery struct {
	AgentID string
	Host    string
	Since   time.Time
	Until   time.Time
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

	since, err := parseObservationTime(r.URL.Query().Get("since"))
	if err != nil {
		http.Error(w, "invalid since", http.StatusBadRequest)
		return
	}
	until, err := parseObservationTime(r.URL.Query().Get("until"))
	if err != nil {
		http.Error(w, "invalid until", http.StatusBadRequest)
		return
	}
	if !since.IsZero() && !until.IsZero() && since.After(until) {
		http.Error(w, "invalid time range", http.StatusBadRequest)
		return
	}

	observations, err := h.Reader.List(ObservationQuery{
		AgentID: strings.TrimSpace(r.URL.Query().Get("agent_id")),
		Host:    strings.TrimSpace(r.URL.Query().Get("host")),
		Since:   since,
		Until:   until,
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

func parseObservationTime(raw string) (time.Time, error) {
	if strings.TrimSpace(raw) == "" {
		return time.Time{}, nil
	}
	value, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, err
	}
	return value.UTC(), nil
}
