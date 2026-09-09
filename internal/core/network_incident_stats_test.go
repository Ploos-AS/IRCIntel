package core

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type networkIncidentStatsFixture struct { items []NetworkIncident; query NetworkIncidentRecordQuery }
func (f *networkIncidentStatsFixture) ListNetworkIncidentRecords(q NetworkIncidentRecordQuery) ([]NetworkIncident,error) { f.query=q; return f.items,nil }

func TestSummarizeNetworkIncidents(t *testing.T) {
	t0:=time.Date(2026,9,9,10,0,0,0,time.UTC); d1:=int64(120); d2:=int64(360)
	items:=[]NetworkIncident{
		{NetworkID:"n1",Status:"closed",Severity:"degraded",StartedAt:t0,DurationSeconds:&d1},
		{NetworkID:"n1",Status:"closed",Severity:"down",StartedAt:t0.Add(time.Hour),DurationSeconds:&d2},
		{NetworkID:"n1",Status:"open",Severity:"down",StartedAt:t0.Add(2*time.Hour)},
	}
	got:=summarizeNetworkIncidents(NetworkIncidentRecordQuery{NetworkID:"n1"},items)
	if got.TotalIncidents!=3||got.OpenIncidents!=1||got.ClosedIncidents!=2||got.DegradedIncidents!=1||got.DownIncidents!=2{t.Fatalf("counts=%+v",got)}
	if got.TotalDowntimeSeconds!=480{t.Fatalf("downtime=%d",got.TotalDowntimeSeconds)}
	if got.MeanRecoverySeconds==nil||*got.MeanRecoverySeconds!=240{t.Fatalf("mean=%v",got.MeanRecoverySeconds)}
	if got.MaxIncidentDurationSeconds==nil||*got.MaxIncidentDurationSeconds!=360{t.Fatalf("max=%v",got.MaxIncidentDurationSeconds)}
	if got.LatestStartedAt==nil||!got.LatestStartedAt.Equal(t0.Add(2*time.Hour)){t.Fatalf("latest=%v",got.LatestStartedAt)}
}

func TestNetworkIncidentStatsHandler(t *testing.T) {
	t0:=time.Date(2026,9,9,10,0,0,0,time.UTC); d:=int64(90); fixture:=&networkIncidentStatsFixture{items:[]NetworkIncident{{NetworkID:"n1",Status:"closed",Severity:"down",StartedAt:t0,DurationSeconds:&d}}}
	h:=NetworkIncidentStatsHandler{Token:"secret",Reader:fixture}
	rr:=httptest.NewRecorder(); req:=httptest.NewRequest(http.MethodGet,"/api/v1/networks/incidents/stats?network_id=n1&since=2026-09-09T09:00:00Z&until=2026-09-09T11:00:00Z",nil); req.Header.Set("Authorization","Bearer secret"); h.ServeHTTP(rr,req)
	if rr.Code!=http.StatusOK{t.Fatalf("status=%d body=%s",rr.Code,rr.Body.String())}
	if fixture.query.NetworkID!="n1"||fixture.query.Limit!=500||fixture.query.Since.IsZero()||fixture.query.Until.IsZero(){t.Fatalf("query=%+v",fixture.query)}
	body:=rr.Body.String(); if !strings.Contains(body,`"total_incidents":1`)||!strings.Contains(body,`"total_downtime_seconds":90`)||!strings.Contains(body,`"mean_recovery_seconds":90`){t.Fatalf("body=%s",body)}
}
