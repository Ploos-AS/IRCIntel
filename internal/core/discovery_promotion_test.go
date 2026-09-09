package core

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestPromoteDiscoveryCandidateRequiresAcceptedAndPublishesEndpoint(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()

	if err := store.UpsertNetwork(Network{ID: "libera", Name: "Libera.Chat"}); err != nil { t.Fatal(err) }
	if err := store.UpsertNetworkServer(NetworkServer{ID: "libera-main", NetworkID: "libera", Name: "Libera main"}); err != nil { t.Fatal(err) }
	base := time.Date(2026, 9, 9, 10, 0, 0, 0, time.UTC)
	candidate, err := store.UpsertDiscoveryCandidate(DiscoveryCandidateInput{Host: "IRC.Libera.Chat", Port: "6697", TLS: true, Source: "manual"}, base)
	if err != nil { t.Fatal(err) }

	_, err = store.PromoteDiscoveryCandidate(DiscoveryPromotionInput{CandidateID: candidate.ID, EndpointID: "libera-6697", ServerID: "libera-main", Promoter: "operator"}, base.Add(time.Minute))
	if !errors.Is(err, ErrDiscoveryCandidateNotAccepted) { t.Fatalf("expected not accepted, got %v", err) }

	if _, err := store.ReviewDiscoveryCandidate(DiscoveryReviewInput{CandidateID: candidate.ID, Status: "accepted", Reviewer: "reviewer"}, base.Add(2*time.Minute)); err != nil { t.Fatal(err) }
	promotion, err := store.PromoteDiscoveryCandidate(DiscoveryPromotionInput{CandidateID: candidate.ID, EndpointID: "libera-6697", ServerID: "libera-main", Promoter: "operator", Note: "verified"}, base.Add(3*time.Minute))
	if err != nil { t.Fatal(err) }
	if promotion.EndpointID != "libera-6697" || promotion.ServerID != "libera-main" || promotion.Promoter != "operator" { t.Fatalf("promotion=%+v", promotion) }

	snapshot, err := store.RegistrySnapshot()
	if err != nil { t.Fatal(err) }
	if len(snapshot.Endpoints) != 1 { t.Fatalf("endpoints=%+v", snapshot.Endpoints) }
	endpoint := snapshot.Endpoints[0]
	if endpoint.ID != "libera-6697" || endpoint.Host != "irc.libera.chat" || endpoint.Port != "6697" || !endpoint.TLS { t.Fatalf("endpoint=%+v", endpoint) }

	_, err = store.PromoteDiscoveryCandidate(DiscoveryPromotionInput{CandidateID: candidate.ID, EndpointID: "another", ServerID: "libera-main", Promoter: "operator"}, base.Add(4*time.Minute))
	if !errors.Is(err, ErrDiscoveryCandidatePromoted) { t.Fatalf("expected already promoted, got %v", err) }
}

func TestPromoteDiscoveryCandidateRejectsMissingServer(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()
	base := time.Date(2026, 9, 9, 10, 0, 0, 0, time.UTC)
	candidate, err := store.UpsertDiscoveryCandidate(DiscoveryCandidateInput{Host: "irc.example", Port: "6697", TLS: true, Source: "manual"}, base)
	if err != nil { t.Fatal(err) }
	if _, err := store.ReviewDiscoveryCandidate(DiscoveryReviewInput{CandidateID: candidate.ID, Status: "accepted", Reviewer: "reviewer"}, base.Add(time.Minute)); err != nil { t.Fatal(err) }
	_, err = store.PromoteDiscoveryCandidate(DiscoveryPromotionInput{CandidateID: candidate.ID, EndpointID: "ep", ServerID: "missing", Promoter: "operator"}, base.Add(2*time.Minute))
	if !errors.Is(err, ErrDiscoveryPromotionServer) { t.Fatalf("expected missing server, got %v", err) }
}

func TestPromoteDiscoveryCandidateClassifiesEndpointIDConflict(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()
	if err := store.UpsertNetwork(Network{ID: "net", Name: "Network"}); err != nil { t.Fatal(err) }
	if err := store.UpsertNetworkServer(NetworkServer{ID: "srv", NetworkID: "net", Name: "Server"}); err != nil { t.Fatal(err) }
	if err := store.UpsertNetworkEndpoint(NetworkEndpoint{ID: "taken", ServerID: "srv", Host: "existing.example", Port: "6697", TLS: true}); err != nil { t.Fatal(err) }
	base := time.Date(2026, 9, 9, 10, 0, 0, 0, time.UTC)
	candidate, err := store.UpsertDiscoveryCandidate(DiscoveryCandidateInput{Host: "new.example", Port: "6697", TLS: true, Source: "manual"}, base)
	if err != nil { t.Fatal(err) }
	if _, err := store.ReviewDiscoveryCandidate(DiscoveryReviewInput{CandidateID: candidate.ID, Status: "accepted", Reviewer: "reviewer"}, base.Add(time.Minute)); err != nil { t.Fatal(err) }
	_, err = store.PromoteDiscoveryCandidate(DiscoveryPromotionInput{CandidateID: candidate.ID, EndpointID: "taken", ServerID: "srv", Promoter: "operator"}, base.Add(2*time.Minute))
	if !errors.Is(err, ErrDiscoveryPromotionEndpointConflict) { t.Fatalf("expected endpoint conflict, got %v", err) }
}

func TestPromoteDiscoveryCandidateClassifiesEndpointTupleConflict(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()
	if err := store.UpsertNetwork(Network{ID: "net", Name: "Network"}); err != nil { t.Fatal(err) }
	if err := store.UpsertNetworkServer(NetworkServer{ID: "srv", NetworkID: "net", Name: "Server"}); err != nil { t.Fatal(err) }
	if err := store.UpsertNetworkEndpoint(NetworkEndpoint{ID: "existing", ServerID: "srv", Host: "irc.example", Port: "6697", TLS: true}); err != nil { t.Fatal(err) }
	base := time.Date(2026, 9, 9, 10, 0, 0, 0, time.UTC)
	candidate, err := store.UpsertDiscoveryCandidate(DiscoveryCandidateInput{Host: "IRC.EXAMPLE", Port: "6697", TLS: true, Source: "manual"}, base)
	if err != nil { t.Fatal(err) }
	if _, err := store.ReviewDiscoveryCandidate(DiscoveryReviewInput{CandidateID: candidate.ID, Status: "accepted", Reviewer: "reviewer"}, base.Add(time.Minute)); err != nil { t.Fatal(err) }
	_, err = store.PromoteDiscoveryCandidate(DiscoveryPromotionInput{CandidateID: candidate.ID, EndpointID: "new-id", ServerID: "srv", Promoter: "operator"}, base.Add(2*time.Minute))
	if !errors.Is(err, ErrDiscoveryPromotionEndpointConflict) { t.Fatalf("expected endpoint tuple conflict, got %v", err) }
}

func TestDiscoveryPromotionAuditPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ircintel.db")
	store, err := OpenSQLiteStore(path)
	if err != nil { t.Fatal(err) }
	if err := store.UpsertNetwork(Network{ID: "net", Name: "Network"}); err != nil { t.Fatal(err) }
	if err := store.UpsertNetworkServer(NetworkServer{ID: "srv", NetworkID: "net", Name: "Server"}); err != nil { t.Fatal(err) }
	base := time.Date(2026, 9, 9, 10, 0, 0, 0, time.UTC)
	candidate, err := store.UpsertDiscoveryCandidate(DiscoveryCandidateInput{Host: "irc.example", Port: "6697", TLS: true, Source: "manual"}, base)
	if err != nil { t.Fatal(err) }
	if _, err := store.ReviewDiscoveryCandidate(DiscoveryReviewInput{CandidateID: candidate.ID, Status: "accepted", Reviewer: "reviewer"}, base.Add(time.Minute)); err != nil { t.Fatal(err) }
	if _, err := store.PromoteDiscoveryCandidate(DiscoveryPromotionInput{CandidateID: candidate.ID, EndpointID: "ep", ServerID: "srv", Promoter: "operator", Note: "approved"}, base.Add(2*time.Minute)); err != nil { t.Fatal(err) }
	if err := store.Close(); err != nil { t.Fatal(err) }

	store, err = OpenSQLiteStore(path)
	if err != nil { t.Fatal(err) }
	defer store.Close()
	items, err := store.ListDiscoveryPromotions()
	if err != nil { t.Fatal(err) }
	if len(items) != 1 || items[0].CandidateID != candidate.ID || items[0].Note != "approved" { t.Fatalf("promotions=%+v", items) }
}
