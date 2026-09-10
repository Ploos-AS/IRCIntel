package core

import (
	"context"
	"errors"
	"testing"
	"time"
)

func resetPostgresDiscovery(t *testing.T, store *PostgresStore) {
	t.Helper()
	_, err := store.pool.Exec(context.Background(), `
TRUNCATE discovery_promotions, discovery_reviews, discovery_candidates,
         network_incident_records, network_endpoints, network_servers, networks
RESTART IDENTITY CASCADE`)
	if err != nil { t.Fatal(err) }
}

func TestPostgresDiscoveryReviewPromotionParity(t *testing.T) {
	store := openTestPostgresStore(t)
	resetPostgresDiscovery(t, store)
	base := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)

	candidate, err := store.UpsertDiscoveryCandidate(DiscoveryCandidateInput{
		Host: " IRC.Example ", Port: "6697", TLS: true, Source: " TelnetBBSGuide ", SourceRef: "ref-1",
	}, base)
	if err != nil { t.Fatal(err) }
	if candidate.Status != "pending" || candidate.Host != "irc.example" || candidate.Source != "telnetbbsguide" || candidate.SeenCount != 1 {
		t.Fatalf("candidate=%+v", candidate)
	}
	candidate2, err := store.UpsertDiscoveryCandidate(DiscoveryCandidateInput{
		Host: "irc.example", Port: "6697", TLS: true, Source: "telnetbbsguide", SourceRef: "ref-2",
	}, base.Add(time.Minute))
	if err != nil { t.Fatal(err) }
	if candidate2.ID != candidate.ID || candidate2.SeenCount != 2 || candidate2.SourceRef != "ref-2" || !candidate2.FirstSeen.Equal(base) || !candidate2.LastSeen.Equal(base.Add(time.Minute)) {
		t.Fatalf("repeat candidate=%+v", candidate2)
	}

	review, err := store.ReviewDiscoveryCandidate(DiscoveryReviewInput{
		CandidateID: candidate.ID, Status: "accepted", Reviewer: "admin", Note: "verified",
	}, base.Add(2*time.Minute))
	if err != nil { t.Fatal(err) }
	if review.Status != "accepted" || review.Reviewer != "admin" { t.Fatalf("review=%+v", review) }
	if _, err := store.ReviewDiscoveryCandidate(DiscoveryReviewInput{CandidateID: candidate.ID, Status: "rejected", Reviewer: "admin"}, base.Add(3*time.Minute)); !errors.Is(err, ErrDiscoveryCandidateAlreadyReviewed) {
		t.Fatalf("second review error=%v", err)
	}

	if err := store.UpsertNetwork(Network{ID: "net-1", Name: "ExampleNet"}); err != nil { t.Fatal(err) }
	if err := store.UpsertNetworkServer(NetworkServer{ID: "srv-1", NetworkID: "net-1", Name: "irc.example"}); err != nil { t.Fatal(err) }
	promotion, err := store.PromoteDiscoveryCandidate(DiscoveryPromotionInput{
		CandidateID: candidate.ID, EndpointID: "ep-1", ServerID: "srv-1", Promoter: "admin", Note: "curated",
	}, base.Add(4*time.Minute))
	if err != nil { t.Fatal(err) }
	if promotion.EndpointID != "ep-1" || promotion.ServerID != "srv-1" { t.Fatalf("promotion=%+v", promotion) }

	snapshot, err := store.RegistrySnapshot(); if err != nil { t.Fatal(err) }
	if len(snapshot.Endpoints) != 1 || snapshot.Endpoints[0].Host != "irc.example" || !snapshot.Endpoints[0].TLS {
		t.Fatalf("snapshot=%+v", snapshot)
	}
	promotions, err := store.ListDiscoveryPromotions(); if err != nil { t.Fatal(err) }
	if len(promotions) != 1 || promotions[0].CandidateID != candidate.ID { t.Fatalf("promotions=%+v", promotions) }
	reviews, err := store.ListDiscoveryReviews(); if err != nil { t.Fatal(err) }
	if len(reviews) != 1 || reviews[0].Status != "accepted" { t.Fatalf("reviews=%+v", reviews) }
	accepted, err := store.ListDiscoveryCandidates("accepted"); if err != nil { t.Fatal(err) }
	if len(accepted) != 1 || accepted[0].ID != candidate.ID { t.Fatalf("accepted=%+v", accepted) }

	if _, err := store.PromoteDiscoveryCandidate(DiscoveryPromotionInput{CandidateID:candidate.ID,EndpointID:"ep-2",ServerID:"srv-1",Promoter:"admin"}, base.Add(5*time.Minute)); !errors.Is(err, ErrDiscoveryCandidatePromoted) {
		t.Fatalf("second promotion error=%v", err)
	}
}

func TestPostgresDiscoveryTypedConflicts(t *testing.T) {
	store := openTestPostgresStore(t)
	resetPostgresDiscovery(t, store)
	base := time.Date(2026,9,10,11,0,0,0,time.UTC)
	if _,err:=store.ReviewDiscoveryCandidate(DiscoveryReviewInput{CandidateID:"missing",Status:"accepted",Reviewer:"admin"},base);!errors.Is(err,ErrDiscoveryCandidateNotFound){t.Fatalf("missing review error=%v",err)}
	candidate,err:=store.UpsertDiscoveryCandidate(DiscoveryCandidateInput{Host:"irc.example",Port:"6697",TLS:true,Source:"manual"},base);if err!=nil{t.Fatal(err)}
	if _,err=store.PromoteDiscoveryCandidate(DiscoveryPromotionInput{CandidateID:candidate.ID,EndpointID:"ep-1",ServerID:"srv-1",Promoter:"admin"},base);!errors.Is(err,ErrDiscoveryCandidateNotAccepted){t.Fatalf("pending promotion error=%v",err)}
	if _,err=store.ReviewDiscoveryCandidate(DiscoveryReviewInput{CandidateID:candidate.ID,Status:"accepted",Reviewer:"admin"},base.Add(time.Minute));err!=nil{t.Fatal(err)}
	if _,err=store.PromoteDiscoveryCandidate(DiscoveryPromotionInput{CandidateID:candidate.ID,EndpointID:"ep-1",ServerID:"missing",Promoter:"admin"},base.Add(2*time.Minute));!errors.Is(err,ErrDiscoveryPromotionServer){t.Fatalf("missing server error=%v",err)}
	if err=store.UpsertNetwork(Network{ID:"net-1",Name:"ExampleNet"});err!=nil{t.Fatal(err)}
	if err=store.UpsertNetworkServer(NetworkServer{ID:"srv-1",NetworkID:"net-1",Name:"server"});err!=nil{t.Fatal(err)}
	if err=store.UpsertNetworkEndpoint(NetworkEndpoint{ID:"existing",ServerID:"srv-1",Host:"irc.example",Port:"6697",TLS:true});err!=nil{t.Fatal(err)}
	if _,err=store.PromoteDiscoveryCandidate(DiscoveryPromotionInput{CandidateID:candidate.ID,EndpointID:"ep-1",ServerID:"srv-1",Promoter:"admin"},base.Add(3*time.Minute));!errors.Is(err,ErrDiscoveryPromotionEndpointConflict){t.Fatalf("endpoint conflict error=%v",err)}
}
