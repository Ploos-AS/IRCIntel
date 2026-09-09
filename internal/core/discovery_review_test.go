package core

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestDiscoveryReviewAcceptsPendingCandidateAndAuditsDecision(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()

	seen := time.Date(2026, 9, 9, 8, 0, 0, 0, time.UTC)
	candidate, err := store.UpsertDiscoveryCandidate(DiscoveryCandidateInput{Host: "IRC.Example", Port: "6697", TLS: true, Source: "manual", SourceRef: "seed"}, seen)
	if err != nil { t.Fatal(err) }
	reviewed := seen.Add(time.Minute)
	review, err := store.ReviewDiscoveryCandidate(DiscoveryReviewInput{CandidateID: candidate.ID, Status: "accepted", Reviewer: "operator-1", Note: "verified"}, reviewed)
	if err != nil { t.Fatal(err) }
	if review.Status != "accepted" || review.Reviewer != "operator-1" || review.Note != "verified" || !review.ReviewedAt.Equal(reviewed) {
		t.Fatalf("review=%+v", review)
	}
	items, err := store.ListDiscoveryCandidates("accepted")
	if err != nil { t.Fatal(err) }
	if len(items) != 1 || items[0].ID != candidate.ID { t.Fatalf("items=%+v", items) }
	reviews, err := store.ListDiscoveryReviews()
	if err != nil { t.Fatal(err) }
	if len(reviews) != 1 || reviews[0].CandidateID != candidate.ID { t.Fatalf("reviews=%+v", reviews) }
}

func TestDiscoveryReviewRejectsSecondDecision(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()
	candidate, err := store.UpsertDiscoveryCandidate(DiscoveryCandidateInput{Host: "irc.example", Port: "6667", Source: "manual"}, time.Now().UTC())
	if err != nil { t.Fatal(err) }
	if _, err := store.ReviewDiscoveryCandidate(DiscoveryReviewInput{CandidateID: candidate.ID, Status: "rejected", Reviewer: "operator-1"}, time.Now().UTC()); err != nil { t.Fatal(err) }
	if _, err := store.ReviewDiscoveryCandidate(DiscoveryReviewInput{CandidateID: candidate.ID, Status: "accepted", Reviewer: "operator-2"}, time.Now().UTC()); err != ErrDiscoveryCandidateAlreadyReviewed {
		t.Fatalf("err=%v", err)
	}
}

func TestDiscoveryReviewHandlerAuthValidationAndConflict(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()
	candidate, err := store.UpsertDiscoveryCandidate(DiscoveryCandidateInput{Host: "irc.example", Port: "6697", TLS: true, Source: "manual"}, time.Now().UTC())
	if err != nil { t.Fatal(err) }
	handler := DiscoveryReviewHandler{Token: "secret", Store: store, Now: func() time.Time { return time.Date(2026, 9, 9, 8, 30, 0, 0, time.UTC) }}

	unauthorized := httptest.NewRecorder()
	handler.Review(unauthorized, httptest.NewRequest(http.MethodPost, "/api/v1/discovery/candidates/review", bytes.NewBufferString(`{"candidate_id":"x","status":"accepted","reviewer":"r"}`)))
	if unauthorized.Code != http.StatusUnauthorized { t.Fatalf("unauthorized=%d", unauthorized.Code) }

	request := httptest.NewRequest(http.MethodPost, "/api/v1/discovery/candidates/review", bytes.NewBufferString(`{"candidate_id":"`+candidate.ID+`","status":"accepted","reviewer":"operator"}`))
	request.Header.Set("Authorization", "Bearer secret")
	response := httptest.NewRecorder()
	handler.Review(response, request)
	if response.Code != http.StatusOK { t.Fatalf("status=%d body=%q", response.Code, response.Body.String()) }
	var review DiscoveryReview
	if err := json.NewDecoder(response.Body).Decode(&review); err != nil { t.Fatal(err) }
	if review.CandidateID != candidate.ID || review.Status != "accepted" { t.Fatalf("review=%+v", review) }

	request2 := httptest.NewRequest(http.MethodPost, "/api/v1/discovery/candidates/review", bytes.NewBufferString(`{"candidate_id":"`+candidate.ID+`","status":"rejected","reviewer":"operator"}`))
	request2.Header.Set("Authorization", "Bearer secret")
	response2 := httptest.NewRecorder()
	handler.Review(response2, request2)
	if response2.Code != http.StatusConflict { t.Fatalf("status=%d body=%q", response2.Code, response2.Body.String()) }
}

func TestDiscoveryReviewsPersistAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ircintel.db")
	store, err := OpenSQLiteStore(path)
	if err != nil { t.Fatal(err) }
	candidate, err := store.UpsertDiscoveryCandidate(DiscoveryCandidateInput{Host: "irc.example", Port: "6697", TLS: true, Source: "manual"}, time.Now().UTC())
	if err != nil { t.Fatal(err) }
	if _, err := store.ReviewDiscoveryCandidate(DiscoveryReviewInput{CandidateID: candidate.ID, Status: "accepted", Reviewer: "operator"}, time.Now().UTC()); err != nil { t.Fatal(err) }
	if err := store.Close(); err != nil { t.Fatal(err) }

	store, err = OpenSQLiteStore(path)
	if err != nil { t.Fatal(err) }
	defer store.Close()
	reviews, err := store.ListDiscoveryReviews()
	if err != nil { t.Fatal(err) }
	if len(reviews) != 1 || reviews[0].CandidateID != candidate.ID { t.Fatalf("reviews=%+v", reviews) }

	_ = context.Background()
}
