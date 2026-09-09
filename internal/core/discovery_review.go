package core

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

const maxDiscoveryReviewBytes int64 = 64 * 1024

var (
	ErrDiscoveryCandidateNotFound         = errors.New("discovery candidate not found")
	ErrDiscoveryCandidateAlreadyReviewed = errors.New("discovery candidate already reviewed")
)

type DiscoveryReview struct {
	CandidateID string    `json:"candidate_id"`
	Status      string    `json:"status"`
	Reviewer    string    `json:"reviewer"`
	Note        string    `json:"note,omitempty"`
	ReviewedAt  time.Time `json:"reviewed_at"`
}

type DiscoveryReviewInput struct {
	CandidateID string `json:"candidate_id"`
	Status      string `json:"status"`
	Reviewer    string `json:"reviewer"`
	Note        string `json:"note,omitempty"`
}

type DiscoveryReviewStore interface {
	ReviewDiscoveryCandidate(DiscoveryReviewInput, time.Time) (DiscoveryReview, error)
	ListDiscoveryReviews() ([]DiscoveryReview, error)
}

type DiscoveryReviewHandler struct {
	Token string
	Store DiscoveryReviewStore
	Now   func() time.Time
}

func (h DiscoveryReviewHandler) Review(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r) {
		return
	}
	var input DiscoveryReviewInput
	body := http.MaxBytesReader(w, r.Body, maxDiscoveryReviewBytes)
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		http.Error(w, "invalid discovery review payload", http.StatusBadRequest)
		return
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		http.Error(w, "discovery review payload must contain one JSON object", http.StatusBadRequest)
		return
	}
	input.CandidateID = strings.TrimSpace(input.CandidateID)
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	input.Reviewer = strings.TrimSpace(input.Reviewer)
	input.Note = strings.TrimSpace(input.Note)
	if input.CandidateID == "" || input.Reviewer == "" || (input.Status != "accepted" && input.Status != "rejected") {
		http.Error(w, "candidate_id, reviewer, and status accepted|rejected are required", http.StatusBadRequest)
		return
	}
	now := time.Now().UTC()
	if h.Now != nil {
		now = h.Now().UTC()
	}
	review, err := h.Store.ReviewDiscoveryCandidate(input, now)
	if err != nil {
		switch {
		case errors.Is(err, ErrDiscoveryCandidateNotFound):
			http.Error(w, "discovery candidate not found", http.StatusNotFound)
		case errors.Is(err, ErrDiscoveryCandidateAlreadyReviewed):
			http.Error(w, "discovery candidate already reviewed", http.StatusConflict)
		default:
			http.Error(w, "discovery review failed", http.StatusServiceUnavailable)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(review)
}

func (h DiscoveryReviewHandler) List(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r) {
		return
	}
	reviews, err := h.Store.ListDiscoveryReviews()
	if err != nil {
		http.Error(w, "discovery review query failed", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Reviews []DiscoveryReview `json:"reviews"`
	}{Reviews: reviews})
}

func (h DiscoveryReviewHandler) authorize(w http.ResponseWriter, r *http.Request) bool {
	if h.Store == nil {
		http.Error(w, "discovery review store unavailable", http.StatusServiceUnavailable)
		return false
	}
	if strings.TrimSpace(h.Token) == "" {
		http.Error(w, "discovery review requires IRCINTEL_CORE_TOKEN", http.StatusServiceUnavailable)
		return false
	}
	if r.Header.Get("Authorization") != "Bearer "+h.Token {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

func (s *SQLiteStore) ReviewDiscoveryCandidate(input DiscoveryReviewInput, reviewedAt time.Time) (DiscoveryReview, error) {
	if s == nil || s.db == nil {
		return DiscoveryReview{}, errors.New("sqlite store is not open")
	}
	input.CandidateID = strings.TrimSpace(input.CandidateID)
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	input.Reviewer = strings.TrimSpace(input.Reviewer)
	input.Note = strings.TrimSpace(input.Note)
	if input.CandidateID == "" || input.Reviewer == "" || (input.Status != "accepted" && input.Status != "rejected") {
		return DiscoveryReview{}, errors.New("invalid discovery review")
	}
	if reviewedAt.IsZero() {
		reviewedAt = time.Now().UTC()
	}

	tx, err := s.db.Begin()
	if err != nil {
		return DiscoveryReview{}, err
	}
	defer tx.Rollback()

	var currentStatus string
	if err := tx.QueryRow(`SELECT status FROM discovery_candidates WHERE id = ?`, input.CandidateID).Scan(&currentStatus); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no rows") {
			return DiscoveryReview{}, ErrDiscoveryCandidateNotFound
		}
		return DiscoveryReview{}, err
	}
	if currentStatus != "pending" {
		return DiscoveryReview{}, ErrDiscoveryCandidateAlreadyReviewed
	}
	stamp := formatObservationTime(reviewedAt)
	if _, err := tx.Exec(`
INSERT INTO discovery_reviews (candidate_id, status, reviewer, note, reviewed_at)
VALUES (?, ?, ?, ?, ?)`, input.CandidateID, input.Status, input.Reviewer, input.Note, stamp); err != nil {
		return DiscoveryReview{}, err
	}
	result, err := tx.Exec(`UPDATE discovery_candidates SET status = ? WHERE id = ? AND status = 'pending'`, input.Status, input.CandidateID)
	if err != nil {
		return DiscoveryReview{}, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return DiscoveryReview{}, err
	}
	if rows != 1 {
		return DiscoveryReview{}, ErrDiscoveryCandidateAlreadyReviewed
	}
	if err := tx.Commit(); err != nil {
		return DiscoveryReview{}, err
	}
	return DiscoveryReview{CandidateID: input.CandidateID, Status: input.Status, Reviewer: input.Reviewer, Note: input.Note, ReviewedAt: reviewedAt.UTC()}, nil
}

func (s *SQLiteStore) ListDiscoveryReviews() ([]DiscoveryReview, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("sqlite store is not open")
	}
	rows, err := s.db.Query(`SELECT candidate_id, status, reviewer, note, reviewed_at FROM discovery_reviews ORDER BY reviewed_at DESC, candidate_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	reviews := make([]DiscoveryReview, 0)
	for rows.Next() {
		var item DiscoveryReview
		var reviewedAt string
		if err := rows.Scan(&item.CandidateID, &item.Status, &item.Reviewer, &item.Note, &reviewedAt); err != nil {
			return nil, err
		}
		item.ReviewedAt, err = time.Parse(observationTimeLayout, reviewedAt)
		if err != nil {
			return nil, err
		}
		reviews = append(reviews, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return reviews, nil
}
