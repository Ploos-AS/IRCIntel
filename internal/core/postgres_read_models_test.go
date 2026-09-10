package core

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Ploos-AS/IRCIntel/internal/agent"
	"github.com/Ploos-AS/IRCIntel/internal/probe"
)

func TestPostgresReadModelParity(t *testing.T) {
	store := openTestPostgresStore(t)
	ctx := context.Background()
	if _, err := store.pool.Exec(ctx, `TRUNCATE discovery_promotions, discovery_reviews, discovery_candidates, network_incident_records, network_endpoints, network_servers, networks, endpoint_transition_events, agent_endpoint_state, incident_records, observations RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}

	if err := store.UpsertNetwork(Network{ID: "net-a", Name: "ExampleNet"}); err != nil { t.Fatal(err) }
	if err := store.UpsertNetworkServer(NetworkServer{ID: "srv-a", NetworkID: "net-a", Name: "irc.example"}); err != nil { t.Fatal(err) }
	if err := store.UpsertNetworkEndpoint(NetworkEndpoint{ID: "ep-a", ServerID: "srv-a", Host: "irc.example", Port: "6697", TLS: true}); err != nil { t.Fatal(err) }

	base := time.Date(2026, 9, 10, 8, 0, 0, 0, time.UTC)
	obs := func(agentID string, at time.Time, reachable bool) agent.Observation {
		return agent.Observation{
			AgentID: agentID,
			ObservedAt: at,
			Endpoint: agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true},
			Result: probe.EndpointResult{Reachable: reachable},
		}
	}
	for _, item := range []agent.Observation{
		obs("agent-a", base, true),
		obs("agent-b", base.Add(10*time.Second), true),
		obs("agent-a", base.Add(time.Minute), false),
		obs("agent-b", base.Add(2*time.Minute), false),
		obs("agent-a", base.Add(10*time.Minute), true),
		obs("agent-b", base.Add(11*time.Minute), true),
	} {
		if err := store.Store(item); err != nil { t.Fatal(err) }
	}

	snapshot, incidents, err := store.NetworkIncidentStatsSnapshot(time.Time{}, time.Time{})
	if err != nil { t.Fatal(err) }
	if len(snapshot.Networks) != 1 || len(snapshot.Endpoints) != 1 { t.Fatalf("snapshot=%+v", snapshot) }
	if len(incidents) != 1 || incidents[0].Status != "closed" { t.Fatalf("incidents=%+v", incidents) }
	if _, filtered, err := store.NetworkIncidentStatsSnapshot(base.Add(30*time.Minute), time.Time{}); err != nil || len(filtered) != 0 {
		t.Fatalf("filtered=%+v err=%v", filtered, err)
	}
	if _, _, err := store.NetworkIncidentStatsSnapshot(base.Add(time.Hour), base); err == nil {
		t.Fatal("expected invalid time range")
	}

	probeReq := httptest.NewRequest("GET", "/api/v1/probe-plan?agent_id=agent-a", nil)
	probeRec := httptest.NewRecorder()
	ProbePlanHandler{Reader: store}.ServeHTTP(probeRec, probeReq)
	if probeRec.Code != 200 { t.Fatalf("probe plan status=%d body=%s", probeRec.Code, probeRec.Body.String()) }
	var plan ProbePlan
	if err := json.Unmarshal(probeRec.Body.Bytes(), &plan); err != nil { t.Fatal(err) }
	if len(plan.Endpoints) != 1 || plan.Endpoints[0].ID != "ep-a" { t.Fatalf("plan=%+v", plan) }

	statusReq := httptest.NewRequest("GET", "/api/v1/networks/status", nil)
	statusRec := httptest.NewRecorder()
	NetworkStatusHandler{Reader: store, Freshness: time.Hour, Now: func() time.Time { return base.Add(12 * time.Minute) }}.ServeHTTP(statusRec, statusReq)
	if statusRec.Code != 200 { t.Fatalf("network status=%d body=%s", statusRec.Code, statusRec.Body.String()) }
	var statusPayload struct { Networks []NetworkStatus `json:"networks"` }
	if err := json.Unmarshal(statusRec.Body.Bytes(), &statusPayload); err != nil { t.Fatal(err) }
	if len(statusPayload.Networks) != 1 || statusPayload.Networks[0].Status != "up" { t.Fatalf("network status payload=%+v", statusPayload) }

	statsReq := httptest.NewRequest("GET", "/api/v1/networks/incidents/stats?network_id=net-a", nil)
	statsRec := httptest.NewRecorder()
	NetworkIncidentStatsHandler{Reader: store}.ServeHTTP(statsRec, statsReq)
	if statsRec.Code != 200 { t.Fatalf("stats status=%d body=%s", statsRec.Code, statsRec.Body.String()) }
	var stats NetworkIncidentStats
	if err := json.Unmarshal(statsRec.Body.Bytes(), &stats); err != nil { t.Fatal(err) }
	if stats.TotalIncidents != 1 || stats.ClosedIncidents != 1 { t.Fatalf("stats=%+v", stats) }

	byNetworkReq := httptest.NewRequest("GET", "/api/v1/networks/incidents/stats/by-network", nil)
	byNetworkRec := httptest.NewRecorder()
	NetworkIncidentStatsByNetworkHandler{Reader: store}.ServeHTTP(byNetworkRec, byNetworkReq)
	if byNetworkRec.Code != 200 { t.Fatalf("by-network status=%d body=%s", byNetworkRec.Code, byNetworkRec.Body.String()) }
	var byNetwork struct { Networks []NetworkIncidentStatsByNetworkItem `json:"networks"` }
	if err := json.Unmarshal(byNetworkRec.Body.Bytes(), &byNetwork); err != nil { t.Fatal(err) }
	if len(byNetwork.Networks) != 1 || byNetwork.Networks[0].Stats.TotalIncidents != 1 { t.Fatalf("by-network=%+v", byNetwork) }
}

var _ RegistryReader = (*PostgresStore)(nil)
var _ NetworkStatusReader = (*PostgresStore)(nil)
var _ NetworkIncidentStatsReader = (*PostgresStore)(nil)
