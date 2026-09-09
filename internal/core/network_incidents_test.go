package core

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type networkIncidentFixture struct { snapshot RegistrySnapshot; records []IncidentLifecycle }
func (f networkIncidentFixture) RegistrySnapshot() (RegistrySnapshot,error) { return f.snapshot,nil }
func (f networkIncidentFixture) ListIncidentRecords(IncidentRecordQuery) ([]IncidentLifecycle,error) { return f.records,nil }

func TestDeriveNetworkIncidents(t *testing.T) {
	t0 := time.Date(2026,9,9,10,0,0,0,time.UTC)
	t1 := t0.Add(5*time.Minute)
	t2 := t0.Add(10*time.Minute)
	t3 := t0.Add(15*time.Minute)
	snapshot := RegistrySnapshot{
		Networks: []Network{{ID:"net-a",Name:"Alpha"}},
		Servers: []NetworkServer{{ID:"srv-a",NetworkID:"net-a",Name:"A"}},
		Endpoints: []NetworkEndpoint{{ID:"e1",ServerID:"srv-a",Host:"a1.example",Port:"6697",TLS:true},{ID:"e2",ServerID:"srv-a",Host:"a2.example",Port:"6697",TLS:true}},
	}
	records := []IncidentLifecycle{
		{Host:"a1.example",Port:"6697",TLS:true,Status:"closed",StartedAt:t0,RecoveredAt:&t3},
		{Host:"a2.example",Port:"6697",TLS:true,Status:"closed",StartedAt:t1,RecoveredAt:&t2},
	}
	items := deriveNetworkIncidents(snapshot,records)
	if len(items)!=1 { t.Fatalf("incidents=%v",items) }
	got := items[0]
	if got.Status!="closed" || got.Severity!="down" || got.AffectedEndpoints!=2 || got.TotalEndpoints!=2 { t.Fatalf("incident=%+v",got) }
	if got.RecoveredAt==nil || !got.RecoveredAt.Equal(t3) { t.Fatalf("recovered=%v",got.RecoveredAt) }
	if got.DurationSeconds==nil || *got.DurationSeconds!=900 { t.Fatalf("duration=%v",got.DurationSeconds) }
}

func TestNetworkIncidentHandlerFilters(t *testing.T) {
	t0 := time.Date(2026,9,9,10,0,0,0,time.UTC)
	fixture := networkIncidentFixture{
		snapshot: RegistrySnapshot{Networks:[]Network{{ID:"net-a",Name:"Alpha"}},Servers:[]NetworkServer{{ID:"srv-a",NetworkID:"net-a",Name:"A"}},Endpoints:[]NetworkEndpoint{{ID:"e1",ServerID:"srv-a",Host:"a.example",Port:"6697",TLS:true}}},
		records: []IncidentLifecycle{{Host:"a.example",Port:"6697",TLS:true,Status:"open",StartedAt:t0}},
	}
	h := NetworkIncidentHandler{Token:"secret",Reader:fixture}
	rr := httptest.NewRecorder(); req := httptest.NewRequest(http.MethodGet,"/api/v1/networks/incidents?status=open&network_id=net-a",nil); req.Header.Set("Authorization","Bearer secret"); h.ServeHTTP(rr,req)
	if rr.Code!=http.StatusOK { t.Fatalf("status=%d body=%s",rr.Code,rr.Body.String()) }
	body := rr.Body.String()
	if !contains(body,`"network_id":"net-a"`) || !contains(body,`"status":"open"`) || !contains(body,`"severity":"down"`) { t.Fatalf("body=%s",body) }
}
