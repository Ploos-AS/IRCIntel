package core

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

const maxDiscoveryPromotionBytes int64 = 64 * 1024

var (
	ErrDiscoveryCandidateNotAccepted = errors.New("discovery candidate is not accepted")
	ErrDiscoveryCandidatePromoted    = errors.New("discovery candidate already promoted")
	ErrDiscoveryPromotionServer      = errors.New("promotion server does not exist")
)

type DiscoveryPromotion struct {
	CandidateID string    `json:"candidate_id"`
	EndpointID  string    `json:"endpoint_id"`
	ServerID    string    `json:"server_id"`
	Promoter    string    `json:"promoter"`
	Note        string    `json:"note,omitempty"`
	PromotedAt  time.Time `json:"promoted_at"`
}

type DiscoveryPromotionInput struct {
	CandidateID string `json:"candidate_id"`
	EndpointID  string `json:"endpoint_id"`
	ServerID    string `json:"server_id"`
	Promoter    string `json:"promoter"`
	Note        string `json:"note,omitempty"`
}

type DiscoveryPromotionStore interface {
	PromoteDiscoveryCandidate(DiscoveryPromotionInput, time.Time) (DiscoveryPromotion, error)
	ListDiscoveryPromotions() ([]DiscoveryPromotion, error)
}

type DiscoveryPromotionHandler struct {
	Token string
	Store DiscoveryPromotionStore
	Now   func() time.Time
}

func (h DiscoveryPromotionHandler) Promote(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r) {
		return
	}
	var input DiscoveryPromotionInput
	body := http.MaxBytesReader(w, r.Body, maxDiscoveryPromotionBytes)
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		http.Error(w, "invalid discovery promotion payload", http.StatusBadRequest)
		return
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		http.Error(w, "discovery promotion payload must contain one JSON object", http.StatusBadRequest)
		return
	}
	input.CandidateID = strings.TrimSpace(input.CandidateID)
	input.EndpointID = strings.TrimSpace(input.EndpointID)
	input.ServerID = strings.TrimSpace(input.ServerID)
	input.Promoter = strings.TrimSpace(input.Promoter)
	input.Note = strings.TrimSpace(input.Note)
	if input.CandidateID == "" || input.EndpointID == "" || input.ServerID == "" || input.Promoter == "" {
		http.Error(w, "candidate_id, endpoint_id, server_id, and promoter are required", http.StatusBadRequest)
		return
	}
	now := time.Now().UTC()
	if h.Now != nil {
		now = h.Now().UTC()
	}
	promotion, err := h.Store.PromoteDiscoveryCandidate(input, now)
	if err != nil {
		switch {
		case errors.Is(err, ErrDiscoveryCandidateNotFound):
			http.Error(w, "discovery candidate not found", http.StatusNotFound)
		case errors.Is(err, ErrDiscoveryCandidateNotAccepted):
			http.Error(w, "discovery candidate is not accepted", http.StatusConflict)
		case errors.Is(err, ErrDiscoveryCandidatePromoted):
			http.Error(w, "discovery candidate already promoted", http.StatusConflict)
		case errors.Is(err, ErrDiscoveryPromotionServer):
			http.Error(w, "promotion server does not exist", http.StatusConflict)
		default:
			http.Error(w, "discovery promotion failed", http.StatusServiceUnavailable)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(promotion)
}

func (h DiscoveryPromotionHandler) List(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r) {
		return
	}
	items, err := h.Store.ListDiscoveryPromotions()
	if err != nil {
		http.Error(w, "discovery promotion query failed", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Promotions []DiscoveryPromotion `json:"promotions"`
	}{Promotions: items})
}

func (h DiscoveryPromotionHandler) authorize(w http.ResponseWriter, r *http.Request) bool {
	if h.Store == nil {
		http.Error(w, "discovery promotion store unavailable", http.StatusServiceUnavailable)
		return false
	}
	if strings.TrimSpace(h.Token) == "" {
		http.Error(w, "discovery promotion requires IRCINTEL_CORE_TOKEN", http.StatusServiceUnavailable)
		return false
	}
	if r.Header.Get("Authorization") != "Bearer "+h.Token {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

func (s *SQLiteStore) ensureDiscoveryPromotionSchema() error {
	if s == nil || s.db == nil {
		return errors.New("sqlite store is not open")
	}
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS discovery_promotions (
    candidate_id TEXT PRIMARY KEY,
    endpoint_id TEXT NOT NULL,
    server_id TEXT NOT NULL,
    promoter TEXT NOT NULL,
    note TEXT NOT NULL DEFAULT '',
    promoted_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_discovery_promotions_time
    ON discovery_promotions(promoted_at DESC);`)
	return err
}

func (s *SQLiteStore) PromoteDiscoveryCandidate(input DiscoveryPromotionInput, promotedAt time.Time) (DiscoveryPromotion, error) {
	if err := s.ensureDiscoveryPromotionSchema(); err != nil {
		return DiscoveryPromotion{}, err
	}
	input.CandidateID = strings.TrimSpace(input.CandidateID)
	input.EndpointID = strings.TrimSpace(input.EndpointID)
	input.ServerID = strings.TrimSpace(input.ServerID)
	input.Promoter = strings.TrimSpace(input.Promoter)
	input.Note = strings.TrimSpace(input.Note)
	if input.CandidateID == "" || input.EndpointID == "" || input.ServerID == "" || input.Promoter == "" {
		return DiscoveryPromotion{}, errors.New("invalid discovery promotion")
	}
	if promotedAt.IsZero() {
		promotedAt = time.Now().UTC()
	}

	tx, err := s.db.Begin()
	if err != nil {
		return DiscoveryPromotion{}, err
	}
	defer tx.Rollback()

	var host, port, status string
	var tls bool
	if err := tx.QueryRow(`SELECT host, port, tls, status FROM discovery_candidates WHERE id = ?`, input.CandidateID).Scan(&host, &port, &tls, &status); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no rows") {
			return DiscoveryPromotion{}, ErrDiscoveryCandidateNotFound
		}
		return DiscoveryPromotion{}, err
	}
	if status != "accepted" {
		return DiscoveryPromotion{}, ErrDiscoveryCandidateNotAccepted
	}
	var promoted int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM discovery_promotions WHERE candidate_id = ?`, input.CandidateID).Scan(&promoted); err != nil {
		return DiscoveryPromotion{}, err
	}
	if promoted != 0 {
		return DiscoveryPromotion{}, ErrDiscoveryCandidatePromoted
	}
	var serverExists int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM network_servers WHERE id = ?`, input.ServerID).Scan(&serverExists); err != nil {
		return DiscoveryPromotion{}, err
	}
	if serverExists == 0 {
		return DiscoveryPromotion{}, ErrDiscoveryPromotionServer
	}
	if _, err := tx.Exec(`
INSERT INTO network_endpoints (id, server_id, host, port, tls)
VALUES (?, ?, ?, ?, ?)`, input.EndpointID, input.ServerID, host, port, tls); err != nil {
		return DiscoveryPromotion{}, err
	}
	stamp := formatObservationTime(promotedAt)
	if _, err := tx.Exec(`
INSERT INTO discovery_promotions (candidate_id, endpoint_id, server_id, promoter, note, promoted_at)
VALUES (?, ?, ?, ?, ?, ?)`, input.CandidateID, input.EndpointID, input.ServerID, input.Promoter, input.Note, stamp); err != nil {
		return DiscoveryPromotion{}, err
	}
	if err := tx.Commit(); err != nil {
		return DiscoveryPromotion{}, err
	}
	return DiscoveryPromotion{CandidateID: input.CandidateID, EndpointID: input.EndpointID, ServerID: input.ServerID, Promoter: input.Promoter, Note: input.Note, PromotedAt: promotedAt.UTC()}, nil
}

func (s *SQLiteStore) ListDiscoveryPromotions() ([]DiscoveryPromotion, error) {
	if err := s.ensureDiscoveryPromotionSchema(); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT candidate_id, endpoint_id, server_id, promoter, note, promoted_at FROM discovery_promotions ORDER BY promoted_at DESC, candidate_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]DiscoveryPromotion, 0)
	for rows.Next() {
		var item DiscoveryPromotion
		var promotedAt string
		if err := rows.Scan(&item.CandidateID, &item.EndpointID, &item.ServerID, &item.Promoter, &item.Note, &promotedAt); err != nil {
			return nil, err
		}
		item.PromotedAt, err = time.Parse(observationTimeLayout, promotedAt)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
