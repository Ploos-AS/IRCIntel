package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

const maxDiscoveryWriteBytes int64 = 64 * 1024

type DiscoveryCandidate struct {
	ID        string    `json:"id"`
	Host      string    `json:"host"`
	Port      string    `json:"port"`
	TLS       bool      `json:"tls"`
	Source    string    `json:"source"`
	SourceRef string    `json:"source_ref,omitempty"`
	Status    string    `json:"status"`
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
	SeenCount int       `json:"seen_count"`
}

type DiscoveryCandidateInput struct {
	Host      string `json:"host"`
	Port      string `json:"port"`
	TLS       bool   `json:"tls"`
	Source    string `json:"source"`
	SourceRef string `json:"source_ref,omitempty"`
}

type DiscoveryCandidateStore interface {
	UpsertDiscoveryCandidate(DiscoveryCandidateInput, time.Time) (DiscoveryCandidate, error)
	ListDiscoveryCandidates(string) ([]DiscoveryCandidate, error)
}

type DiscoveryHandler struct {
	Token string
	Store DiscoveryCandidateStore
	Now   func() time.Time
}

func (h DiscoveryHandler) List(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r) {
		return
	}
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	if status != "" && status != "pending" && status != "accepted" && status != "rejected" {
		http.Error(w, "invalid discovery status", http.StatusBadRequest)
		return
	}
	items, err := h.Store.ListDiscoveryCandidates(status)
	if err != nil {
		http.Error(w, "discovery query failed", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Candidates []DiscoveryCandidate `json:"candidates"`
	}{Candidates: items})
}

func (h DiscoveryHandler) Intake(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r) {
		return
	}
	var input DiscoveryCandidateInput
	body := http.MaxBytesReader(w, r.Body, maxDiscoveryWriteBytes)
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		http.Error(w, "invalid discovery payload", http.StatusBadRequest)
		return
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		http.Error(w, "discovery payload must contain one JSON object", http.StatusBadRequest)
		return
	}
	input.Host = strings.ToLower(strings.TrimSpace(input.Host))
	input.Port = strings.TrimSpace(input.Port)
	input.Source = strings.ToLower(strings.TrimSpace(input.Source))
	input.SourceRef = strings.TrimSpace(input.SourceRef)
	if input.Host == "" || input.Port == "" || input.Source == "" {
		http.Error(w, "host, port, and source are required", http.StatusBadRequest)
		return
	}
	now := time.Now().UTC()
	if h.Now != nil {
		now = h.Now().UTC()
	}
	candidate, err := h.Store.UpsertDiscoveryCandidate(input, now)
	if err != nil {
		http.Error(w, "discovery intake failed", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(candidate)
}

func (h DiscoveryHandler) authorize(w http.ResponseWriter, r *http.Request) bool {
	if h.Store == nil {
		http.Error(w, "discovery store unavailable", http.StatusServiceUnavailable)
		return false
	}
	if strings.TrimSpace(h.Token) == "" {
		http.Error(w, "discovery intake requires IRCINTEL_CORE_TOKEN", http.StatusServiceUnavailable)
		return false
	}
	if r.Header.Get("Authorization") != "Bearer "+h.Token {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

func discoveryCandidateID(input DiscoveryCandidateInput) string {
	canonical := strings.ToLower(strings.TrimSpace(input.Host)) + "\x00" + strings.TrimSpace(input.Port) + "\x00"
	if input.TLS {
		canonical += "1"
	} else {
		canonical += "0"
	}
	canonical += "\x00" + strings.ToLower(strings.TrimSpace(input.Source))
	sum := sha256.Sum256([]byte(canonical))
	return "dc_" + hex.EncodeToString(sum[:12])
}

func (s *SQLiteStore) ensureDiscoverySchema() error {
	if s == nil || s.db == nil {
		return errors.New("sqlite store is not open")
	}
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS discovery_candidates (
    id TEXT PRIMARY KEY,
    host TEXT NOT NULL,
    port TEXT NOT NULL,
    tls INTEGER NOT NULL,
    source TEXT NOT NULL,
    source_ref TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    first_seen TEXT NOT NULL,
    last_seen TEXT NOT NULL,
    seen_count INTEGER NOT NULL DEFAULT 1,
    CHECK(status IN ('pending','accepted','rejected'))
);
CREATE INDEX IF NOT EXISTS idx_discovery_candidates_status_seen
    ON discovery_candidates(status, last_seen DESC);
CREATE INDEX IF NOT EXISTS idx_discovery_candidates_endpoint
    ON discovery_candidates(host, port, tls);
`)
	return err
}

func (s *SQLiteStore) UpsertDiscoveryCandidate(input DiscoveryCandidateInput, observedAt time.Time) (DiscoveryCandidate, error) {
	if err := s.ensureDiscoverySchema(); err != nil {
		return DiscoveryCandidate{}, err
	}
	input.Host = strings.ToLower(strings.TrimSpace(input.Host))
	input.Port = strings.TrimSpace(input.Port)
	input.Source = strings.ToLower(strings.TrimSpace(input.Source))
	input.SourceRef = strings.TrimSpace(input.SourceRef)
	if input.Host == "" || input.Port == "" || input.Source == "" {
		return DiscoveryCandidate{}, errors.New("host, port, and source are required")
	}
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	id := discoveryCandidateID(input)
	stamp := formatObservationTime(observedAt)
	_, err := s.db.Exec(`
INSERT INTO discovery_candidates (
    id, host, port, tls, source, source_ref, status, first_seen, last_seen, seen_count
) VALUES (?, ?, ?, ?, ?, ?, 'pending', ?, ?, 1)
ON CONFLICT(id) DO UPDATE SET
    source_ref = excluded.source_ref,
    last_seen = excluded.last_seen,
    seen_count = discovery_candidates.seen_count + 1`,
		id, input.Host, input.Port, input.TLS, input.Source, input.SourceRef, stamp, stamp)
	if err != nil {
		return DiscoveryCandidate{}, err
	}
	return s.discoveryCandidateByID(id)
}

func (s *SQLiteStore) discoveryCandidateByID(id string) (DiscoveryCandidate, error) {
	var item DiscoveryCandidate
	var firstSeen, lastSeen string
	err := s.db.QueryRow(`
SELECT id, host, port, tls, source, source_ref, status, first_seen, last_seen, seen_count
FROM discovery_candidates WHERE id = ?`, id).Scan(
		&item.ID, &item.Host, &item.Port, &item.TLS, &item.Source, &item.SourceRef,
		&item.Status, &firstSeen, &lastSeen, &item.SeenCount)
	if err != nil {
		return DiscoveryCandidate{}, err
	}
	item.FirstSeen, err = time.Parse(observationTimeLayout, firstSeen)
	if err != nil {
		return DiscoveryCandidate{}, err
	}
	item.LastSeen, err = time.Parse(observationTimeLayout, lastSeen)
	if err != nil {
		return DiscoveryCandidate{}, err
	}
	return item, nil
}

func (s *SQLiteStore) ListDiscoveryCandidates(status string) ([]DiscoveryCandidate, error) {
	if err := s.ensureDiscoverySchema(); err != nil {
		return nil, err
	}
	status = strings.ToLower(strings.TrimSpace(status))
	statement := `SELECT id, host, port, tls, source, source_ref, status, first_seen, last_seen, seen_count FROM discovery_candidates`
	args := make([]any, 0, 1)
	if status != "" {
		statement += ` WHERE status = ?`
		args = append(args, status)
	}
	statement += ` ORDER BY last_seen DESC, id`
	rows, err := s.db.Query(statement, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]DiscoveryCandidate, 0)
	for rows.Next() {
		var item DiscoveryCandidate
		var firstSeen, lastSeen string
		if err := rows.Scan(&item.ID, &item.Host, &item.Port, &item.TLS, &item.Source, &item.SourceRef, &item.Status, &firstSeen, &lastSeen, &item.SeenCount); err != nil {
			return nil, err
		}
		item.FirstSeen, err = time.Parse(observationTimeLayout, firstSeen)
		if err != nil {
			return nil, err
		}
		item.LastSeen, err = time.Parse(observationTimeLayout, lastSeen)
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
