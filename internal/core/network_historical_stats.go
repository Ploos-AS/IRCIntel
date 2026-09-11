package core

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

type NetworkHistoricalStatsQuery struct {
	NetworkID string
	Period    string
	Now       time.Time
}

type NetworkHistoricalStatsSeries struct {
	NetworkID   string                 `json:"network_id"`
	NetworkName string                 `json:"network_name"`
	Period      string                 `json:"period"`
	Granularity string                 `json:"granularity"`
	Since       *time.Time             `json:"since,omitempty"`
	Until       time.Time              `json:"until"`
	Points      []HistoricalStatsPoint `json:"points"`
}

type NetworkHistoricalStatsReader interface {
	HistoricalNetworkStats(NetworkHistoricalStatsQuery) (NetworkHistoricalStatsSeries, error)
}

type NetworkHistoricalStatsHandler struct {
	Token  string
	Reader NetworkHistoricalStatsReader
	Now    func() time.Time
}

func (h NetworkHistoricalStatsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.Reader == nil {
		http.Error(w, "network historical statistics unavailable", http.StatusServiceUnavailable)
		return
	}
	if h.Token != "" && r.Header.Get("Authorization") != "Bearer "+h.Token {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	networkID := strings.TrimSpace(r.URL.Query().Get("network_id"))
	if networkID == "" {
		http.Error(w, "network_id is required", http.StatusBadRequest)
		return
	}
	period := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("period")))
	if period == "" { period = "24h" }
	now := time.Now().UTC()
	if h.Now != nil { now = h.Now().UTC() }
	series, err := h.Reader.HistoricalNetworkStats(NetworkHistoricalStatsQuery{NetworkID: networkID, Period: period, Now: now})
	if err != nil {
		switch {
		case errors.Is(err, ErrRegistryNetworkNotFound):
			http.Error(w, "network does not exist", http.StatusNotFound)
		case errors.Is(err, ErrHistoricalStatsUnavailable):
			http.Error(w, "network historical statistics unavailable", http.StatusServiceUnavailable)
		default:
			http.Error(w, "invalid network historical statistics query", http.StatusBadRequest)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(series)
}
