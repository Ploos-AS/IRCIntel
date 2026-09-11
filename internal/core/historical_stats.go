package core

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

var ErrHistoricalStatsUnavailable = errors.New("historical statistics are unavailable for this storage backend")

type HistoricalStatsPoint struct {
	BucketStart      time.Time `json:"bucket_start"`
	ObservationCount int64     `json:"observation_count"`
	ReachableCount   int64     `json:"reachable_count"`
	DualStackCount   int64     `json:"dual_stack_count"`
	ReachablePercent float64   `json:"reachable_percent"`
	DualStackPercent float64   `json:"dual_stack_percent"`
}

type HistoricalStatsQuery struct {
	Period string
	Host   string
	Now    time.Time
}

type HistoricalStatsSeries struct {
	Period      string                 `json:"period"`
	Granularity string                 `json:"granularity"`
	Since       *time.Time             `json:"since,omitempty"`
	Until       time.Time              `json:"until"`
	Host        string                 `json:"host,omitempty"`
	Points      []HistoricalStatsPoint `json:"points"`
}

type HistoricalStatsReader interface {
	HistoricalObservationStats(HistoricalStatsQuery) (HistoricalStatsSeries, error)
}

type HistoricalStatsHandler struct {
	Token  string
	Reader HistoricalStatsReader
	Now    func() time.Time
}

func (h HistoricalStatsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.Reader == nil {
		http.Error(w, "historical statistics unavailable", http.StatusServiceUnavailable)
		return
	}
	if h.Token != "" && r.Header.Get("Authorization") != "Bearer "+h.Token {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	period := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("period")))
	if period == "" { period = "24h" }
	now := time.Now().UTC()
	if h.Now != nil { now = h.Now().UTC() }
	series, err := h.Reader.HistoricalObservationStats(HistoricalStatsQuery{
		Period: period,
		Host: strings.ToLower(strings.TrimSpace(r.URL.Query().Get("host"))),
		Now: now,
	})
	if err != nil {
		if errors.Is(err, ErrHistoricalStatsUnavailable) {
			http.Error(w, "historical statistics unavailable", http.StatusServiceUnavailable)
			return
		}
		http.Error(w, "invalid historical statistics query", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(series)
}

func historicalWindow(period string, now time.Time) (granularity string, since time.Time, err error) {
	now = now.UTC()
	switch period {
	case "24h":
		return "hour", now.Add(-24 * time.Hour), nil
	case "7d":
		return "hour", now.Add(-7 * 24 * time.Hour), nil
	case "30d":
		return "day", now.Add(-30 * 24 * time.Hour), nil
	case "1y":
		return "day", now.AddDate(-1, 0, 0), nil
	case "all":
		return "day", time.Time{}, nil
	default:
		return "", time.Time{}, errors.New("unsupported historical period")
	}
}
