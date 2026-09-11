package core

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

type NetworkHistoryRankingQuery struct {
	Period string
	Now    time.Time
	Limit  int
}

type NetworkHistoryRankingItem struct {
	Rank                     int      `json:"rank"`
	NetworkID                string   `json:"network_id"`
	NetworkName              string   `json:"network_name"`
	ObservationCount         int64    `json:"observation_count"`
	ReachableCount           int64    `json:"reachable_count"`
	DualStackCount           int64    `json:"dual_stack_count"`
	ReachablePercent         float64  `json:"reachable_percent"`
	DualStackPercent         float64  `json:"dual_stack_percent"`
	PreviousObservationCount int64    `json:"previous_observation_count"`
	PreviousReachablePercent *float64 `json:"previous_reachable_percent,omitempty"`
	ReachableTrendPoints     *float64 `json:"reachable_trend_points,omitempty"`
}

type NetworkHistoryRanking struct {
	Period      string                      `json:"period"`
	Granularity string                      `json:"granularity"`
	Since       *time.Time                  `json:"since,omitempty"`
	Until       time.Time                   `json:"until"`
	Items       []NetworkHistoryRankingItem `json:"items"`
}

type NetworkHistoryRankingReader interface {
	HistoricalNetworkRanking(NetworkHistoryRankingQuery) (NetworkHistoryRanking, error)
}

type NetworkHistoryRankingHandler struct {
	Token  string
	Reader NetworkHistoryRankingReader
	Now    func() time.Time
}

func (h NetworkHistoryRankingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.Reader == nil {
		http.Error(w, "network historical ranking unavailable", http.StatusServiceUnavailable)
		return
	}
	if h.Token != "" && r.Header.Get("Authorization") != "Bearer "+h.Token {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	period := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("period")))
	if period == "" { period = "24h" }
	limit := 100
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		var parsed int
		if _, err := fmtSscanfInt(raw, &parsed); err != nil || parsed < 1 || parsed > 500 {
			http.Error(w, "invalid ranking limit", http.StatusBadRequest)
			return
		}
		limit = parsed
	}
	now := time.Now().UTC()
	if h.Now != nil { now = h.Now().UTC() }
	ranking, err := h.Reader.HistoricalNetworkRanking(NetworkHistoryRankingQuery{Period: period, Now: now, Limit: limit})
	if err != nil {
		if errors.Is(err, ErrHistoricalStatsUnavailable) {
			http.Error(w, "network historical ranking unavailable", http.StatusServiceUnavailable)
			return
		}
		http.Error(w, "invalid network historical ranking query", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ranking)
}

func fmtSscanfInt(raw string, out *int) (int, error) {
	value := 0
	for _, r := range raw {
		if r < '0' || r > '9' { return 0, errors.New("not an integer") }
		value = value*10 + int(r-'0')
	}
	if raw == "" { return 0, errors.New("empty integer") }
	*out = value
	return 1, nil
}
