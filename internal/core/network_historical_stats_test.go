package core

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Ploos-AS/IRCIntel/internal/agent"
	"github.com/Ploos-AS/IRCIntel/internal/probe"
)

func TestPostgresHistoricalNetworkStats(t *testing.T) {
	store := openTestPostgresStore(t)
	if _, err := store.pool.Exec(context.Background(), `TRUNCATE discovery_promotions, discovery_reviews, discovery_candidates, network_incident_records, network_endpoints, network_servers, networks, observation_rollups_hourly, observation_rollups_daily, endpoint_transition_events, agent_endpoint_state, incident_records, observations RESTART IDENTITY CASCADE`); err != nil { t.Fatal(err) }
	for _, n := range []Network{{ID:"net-a",Name:"Network A"},{ID:"net-b",Name:"Network B"}} { if err := store.UpsertNetwork(n); err != nil { t.Fatal(err) } }
	for _, s := range []NetworkServer{{ID:"srv-a",NetworkID:"net-a",Name:"A"},{ID:"srv-b",NetworkID:"net-b",Name:"B"}} { if err := store.UpsertNetworkServer(s); err != nil { t.Fatal(err) } }
	for _, e := range []NetworkEndpoint{
		{ID:"ep-a",ServerID:"srv-a",Host:"irc.a.example",Port:"6697",TLS:true},
		{ID:"ep-b",ServerID:"srv-b",Host:"irc.b.example",Port:"6697",TLS:true},
		{ID:"ep-shared-a",ServerID:"srv-a",Host:"shared.example",Port:"6697",TLS:true},
		{ID:"ep-shared-b",ServerID:"srv-b",Host:"shared.example",Port:"6697",TLS:true},
	} { if err := store.UpsertNetworkEndpoint(e); err != nil { t.Fatal(err) } }

	now := time.Date(2026,9,11,4,0,0,0,time.UTC)
	obs := []agent.Observation{
		{AgentID:"a",ObservedAt:now.Add(-2*time.Hour),Endpoint:agent.Endpoint{Host:"irc.a.example",Port:"6697",TLS:true},Result:probe.EndpointResult{Reachable:true,DualStackOK:true}},
		{AgentID:"b",ObservedAt:now.Add(-2*time.Hour+time.Minute),Endpoint:agent.Endpoint{Host:"irc.b.example",Port:"6697",TLS:true},Result:probe.EndpointResult{Reachable:false}},
		{AgentID:"s",ObservedAt:now.Add(-2*time.Hour+2*time.Minute),Endpoint:agent.Endpoint{Host:"shared.example",Port:"6697",TLS:true},Result:probe.EndpointResult{Reachable:true}},
	}
	for _, o := range obs { if err := store.Store(o); err != nil { t.Fatal(err) } }

	a, err := store.HistoricalNetworkStats(NetworkHistoricalStatsQuery{NetworkID:"net-a",Period:"24h",Now:now})
	if err != nil { t.Fatal(err) }
	if a.NetworkName != "Network A" || len(a.Points) != 1 { t.Fatalf("series=%+v", a) }
	if a.Points[0].ObservationCount != 1 || a.Points[0].ReachablePercent != 100 { t.Fatalf("point=%+v", a.Points[0]) }

	b, err := store.HistoricalNetworkStats(NetworkHistoricalStatsQuery{NetworkID:"net-b",Period:"24h",Now:now})
	if err != nil { t.Fatal(err) }
	if len(b.Points) != 1 || b.Points[0].ObservationCount != 1 || b.Points[0].ReachableCount != 0 { t.Fatalf("series=%+v", b) }

	if _, err := store.HistoricalNetworkStats(NetworkHistoricalStatsQuery{NetworkID:"missing",Period:"24h",Now:now}); !errors.Is(err, ErrRegistryNetworkNotFound) { t.Fatalf("err=%v", err) }
}

func TestNetworkHistoricalStatsHandlerValidation(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/history/networks?period=24h", nil)
	res := httptest.NewRecorder()
	NetworkHistoricalStatsHandler{Reader:networkHistoricalStatsReaderStub{}}.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest { t.Fatalf("status=%d", res.Code) }

	req = httptest.NewRequest(http.MethodGet, "/api/v1/history/networks?network_id=missing&period=24h", nil)
	res = httptest.NewRecorder()
	NetworkHistoricalStatsHandler{Reader:networkHistoricalStatsReaderStub{notFound:true}}.ServeHTTP(res, req)
	if res.Code != http.StatusNotFound { t.Fatalf("status=%d", res.Code) }
}

type networkHistoricalStatsReaderStub struct{ notFound bool }
func (s networkHistoricalStatsReaderStub) HistoricalNetworkStats(q NetworkHistoricalStatsQuery) (NetworkHistoricalStatsSeries,error) {
	if s.notFound { return NetworkHistoricalStatsSeries{}, ErrRegistryNetworkNotFound }
	_,_,err := historicalWindow(q.Period,q.Now)
	return NetworkHistoricalStatsSeries{},err
}
