package core

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Ploos-AS/IRCIntel/internal/agent"
	"github.com/Ploos-AS/IRCIntel/internal/probe"
)

func TestPostgresHistoricalNetworkRanking(t *testing.T) {
	store := openTestPostgresStore(t)
	if _, err := store.pool.Exec(context.Background(), `TRUNCATE discovery_promotions, discovery_reviews, discovery_candidates, network_incident_records, network_endpoints, network_servers, networks, observation_rollups_hourly, observation_rollups_daily, endpoint_transition_events, agent_endpoint_state, incident_records, observations RESTART IDENTITY CASCADE`); err != nil { t.Fatal(err) }
	for _, n := range []Network{{ID:"net-a",Name:"Network A"},{ID:"net-b",Name:"Network B"}} { if err := store.UpsertNetwork(n); err != nil { t.Fatal(err) } }
	for _, s := range []NetworkServer{{ID:"srv-a",NetworkID:"net-a",Name:"A"},{ID:"srv-b",NetworkID:"net-b",Name:"B"}} { if err := store.UpsertNetworkServer(s); err != nil { t.Fatal(err) } }
	for _, e := range []NetworkEndpoint{
		{ID:"ep-a",ServerID:"srv-a",Host:"irc.a.example",Port:"6697",TLS:true},
		{ID:"ep-b",ServerID:"srv-b",Host:"irc.b.example",Port:"6697",TLS:true},
		{ID:"shared-a",ServerID:"srv-a",Host:"shared.example",Port:"6697",TLS:true},
		{ID:"shared-b",ServerID:"srv-b",Host:"shared.example",Port:"6697",TLS:true},
	} { if err := store.UpsertNetworkEndpoint(e); err != nil { t.Fatal(err) } }

	now := time.Date(2026,9,11,4,0,0,0,time.UTC)
	obs := []agent.Observation{
		// Previous 24h: A=50%, B=100%.
		{AgentID:"a1",ObservedAt:now.Add(-30*time.Hour),Endpoint:agent.Endpoint{Host:"irc.a.example",Port:"6697",TLS:true},Result:probe.EndpointResult{Reachable:true}},
		{AgentID:"a2",ObservedAt:now.Add(-29*time.Hour),Endpoint:agent.Endpoint{Host:"irc.a.example",Port:"6697",TLS:true},Result:probe.EndpointResult{Reachable:false}},
		{AgentID:"b1",ObservedAt:now.Add(-30*time.Hour),Endpoint:agent.Endpoint{Host:"irc.b.example",Port:"6697",TLS:true},Result:probe.EndpointResult{Reachable:true}},
		// Current 24h: A=100%, B=50%; A must rank first.
		{AgentID:"a1",ObservedAt:now.Add(-2*time.Hour),Endpoint:agent.Endpoint{Host:"irc.a.example",Port:"6697",TLS:true},Result:probe.EndpointResult{Reachable:true,DualStackOK:true}},
		{AgentID:"a2",ObservedAt:now.Add(-time.Hour),Endpoint:agent.Endpoint{Host:"irc.a.example",Port:"6697",TLS:true},Result:probe.EndpointResult{Reachable:true,DualStackOK:false}},
		{AgentID:"b1",ObservedAt:now.Add(-2*time.Hour),Endpoint:agent.Endpoint{Host:"irc.b.example",Port:"6697",TLS:true},Result:probe.EndpointResult{Reachable:true}},
		{AgentID:"b2",ObservedAt:now.Add(-time.Hour),Endpoint:agent.Endpoint{Host:"irc.b.example",Port:"6697",TLS:true},Result:probe.EndpointResult{Reachable:false}},
		// Ambiguous endpoint must not affect either network.
		{AgentID:"s1",ObservedAt:now.Add(-time.Hour),Endpoint:agent.Endpoint{Host:"shared.example",Port:"6697",TLS:true},Result:probe.EndpointResult{Reachable:true}},
	}
	for _, o := range obs { if err := store.Store(o); err != nil { t.Fatal(err) } }

	ranking, err := store.HistoricalNetworkRanking(NetworkHistoryRankingQuery{Period:"24h",Now:now,Limit:10})
	if err != nil { t.Fatal(err) }
	if ranking.Granularity != "hour" || len(ranking.Items) != 2 { t.Fatalf("ranking=%+v", ranking) }
	if ranking.Items[0].NetworkID != "net-a" || ranking.Items[0].Rank != 1 || ranking.Items[0].ObservationCount != 2 || ranking.Items[0].ReachablePercent != 100 { t.Fatalf("first=%+v", ranking.Items[0]) }
	if ranking.Items[1].NetworkID != "net-b" || ranking.Items[1].Rank != 2 || ranking.Items[1].ObservationCount != 2 || ranking.Items[1].ReachablePercent != 50 { t.Fatalf("second=%+v", ranking.Items[1]) }
	if ranking.Items[0].PreviousReachablePercent == nil || math.Abs(*ranking.Items[0].PreviousReachablePercent-50) > 0.0001 { t.Fatalf("a previous=%+v", ranking.Items[0]) }
	if ranking.Items[0].ReachableTrendPoints == nil || math.Abs(*ranking.Items[0].ReachableTrendPoints-50) > 0.0001 { t.Fatalf("a trend=%+v", ranking.Items[0]) }
	if ranking.Items[1].PreviousReachablePercent == nil || math.Abs(*ranking.Items[1].PreviousReachablePercent-100) > 0.0001 { t.Fatalf("b previous=%+v", ranking.Items[1]) }
	if ranking.Items[1].ReachableTrendPoints == nil || math.Abs(*ranking.Items[1].ReachableTrendPoints+50) > 0.0001 { t.Fatalf("b trend=%+v", ranking.Items[1]) }
}

func TestNetworkHistoryRankingHandlerValidation(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/history/networks/ranking?period=24h&limit=0", nil)
	res := httptest.NewRecorder()
	NetworkHistoryRankingHandler{Reader:networkHistoryRankingReaderStub{}}.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest { t.Fatalf("status=%d", res.Code) }

	req = httptest.NewRequest(http.MethodGet, "/api/v1/history/networks/ranking?period=90d", nil)
	res = httptest.NewRecorder()
	NetworkHistoryRankingHandler{Reader:networkHistoryRankingReaderStub{}}.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest { t.Fatalf("status=%d", res.Code) }
}

type networkHistoryRankingReaderStub struct{}
func (networkHistoryRankingReaderStub) HistoricalNetworkRanking(q NetworkHistoryRankingQuery) (NetworkHistoryRanking,error) {
	_,_,err := historicalWindow(q.Period,q.Now)
	return NetworkHistoryRanking{},err
}
