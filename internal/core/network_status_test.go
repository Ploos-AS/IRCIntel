package core

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Ploos-AS/IRCIntel/internal/agent"
	"github.com/Ploos-AS/IRCIntel/internal/probe"
)

type networkStatusFixture struct { snapshot RegistrySnapshot; observations []agent.Observation }
func (f networkStatusFixture) RegistrySnapshot() (RegistrySnapshot,error) { return f.snapshot,nil }
func (f networkStatusFixture) LatestEndpointObservations() ([]agent.Observation,error) { return f.observations,nil }

func TestNetworkStatusAggregation(t *testing.T) {
	now := time.Date(2026,9,9,12,0,0,0,time.UTC)
	fixture := networkStatusFixture{
		snapshot: RegistrySnapshot{
			Networks: []Network{{ID:"net-a",Name:"Alpha"},{ID:"net-b",Name:"Beta"}},
			Servers: []NetworkServer{{ID:"srv-a",NetworkID:"net-a",Name:"A"},{ID:"srv-b",NetworkID:"net-b",Name:"B"}},
			Endpoints: []NetworkEndpoint{{ID:"a1",ServerID:"srv-a",Host:"a.example",Port:"6697",TLS:true},{ID:"a2",ServerID:"srv-a",Host:"a2.example",Port:"6697",TLS:true},{ID:"b1",ServerID:"srv-b",Host:"b.example",Port:"6697",TLS:true}},
		},
		observations: []agent.Observation{
			{AgentID:"p1",ObservedAt:now.Add(-time.Minute),Endpoint:agent.Endpoint{Host:"a.example",Port:"6697",TLS:true},Result:probe.EndpointResult{Reachable:true}},
			{AgentID:"p2",ObservedAt:now.Add(-time.Minute),Endpoint:agent.Endpoint{Host:"a.example",Port:"6697",TLS:true},Result:probe.EndpointResult{Reachable:true}},
			{AgentID:"p1",ObservedAt:now.Add(-time.Minute),Endpoint:agent.Endpoint{Host:"a2.example",Port:"6697",TLS:true},Result:probe.EndpointResult{Reachable:false}},
			{AgentID:"p1",ObservedAt:now.Add(-time.Hour),Endpoint:agent.Endpoint{Host:"b.example",Port:"6697",TLS:true},Result:probe.EndpointResult{Reachable:true}},
		},
	}
	h := NetworkStatusHandler{Reader:fixture,Now:func()time.Time{return now}}
	rr := httptest.NewRecorder(); req := httptest.NewRequest(http.MethodGet,"/api/v1/networks/status",nil); h.ServeHTTP(rr,req)
	if rr.Code != http.StatusOK { t.Fatalf("status=%d body=%s",rr.Code,rr.Body.String()) }
	body := rr.Body.String()
	for _, want := range []string{`"id":"net-a"`,`"status":"degraded"`,`"up_endpoints":1`,`"down_endpoints":1`,`"id":"net-b"`,`"status":"stale"`,`"stale_endpoints":1`} { if !contains(body,want) { t.Fatalf("missing %s in %s",want,body) } }
}

func contains(s, sub string) bool { for i:=0; i+len(sub)<=len(s); i++ { if s[i:i+len(sub)]==sub { return true } }; return false }
