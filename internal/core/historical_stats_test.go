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

func TestHistoricalWindowResolution(t *testing.T) {
	now := time.Date(2026, 9, 11, 3, 0, 0, 0, time.UTC)
	cases := []struct{ period, granularity string }{
		{"24h", "hour"}, {"7d", "hour"}, {"30d", "day"}, {"1y", "day"}, {"all", "day"},
	}
	for _, tc := range cases {
		t.Run(tc.period, func(t *testing.T) {
			granularity, _, err := historicalWindow(tc.period, now)
			if err != nil { t.Fatal(err) }
			if granularity != tc.granularity { t.Fatalf("granularity=%q want=%q", granularity, tc.granularity) }
		})
	}
	if _, _, err := historicalWindow("90d", now); err == nil { t.Fatal("expected unsupported period error") }
}

func TestPostgresHistoricalObservationStats(t *testing.T) {
	store := openTestPostgresStore(t)
	if _, err := store.pool.Exec(context.Background(), `TRUNCATE observation_rollups_hourly, observation_rollups_daily, observations RESTART IDENTITY`); err != nil { t.Fatal(err) }
	now := time.Date(2026, 9, 11, 3, 30, 0, 0, time.UTC)
	observations := []agent.Observation{
		{AgentID:"a", ObservedAt:now.Add(-2*time.Hour+5*time.Minute), Endpoint:agent.Endpoint{Host:"irc.one.example",Port:"6697",TLS:true}, Result:probe.EndpointResult{Reachable:true,DualStackOK:true}},
		{AgentID:"b", ObservedAt:now.Add(-2*time.Hour+15*time.Minute), Endpoint:agent.Endpoint{Host:"irc.two.example",Port:"6697",TLS:true}, Result:probe.EndpointResult{Reachable:false,DualStackOK:false}},
		{AgentID:"a", ObservedAt:now.Add(-time.Hour+5*time.Minute), Endpoint:agent.Endpoint{Host:"irc.one.example",Port:"6697",TLS:true}, Result:probe.EndpointResult{Reachable:true,DualStackOK:false}},
	}
	for _, observation := range observations { if err := store.Store(observation); err != nil { t.Fatal(err) } }

	series, err := store.HistoricalObservationStats(HistoricalStatsQuery{Period:"24h", Now:now})
	if err != nil { t.Fatal(err) }
	if series.Granularity != "hour" { t.Fatalf("granularity=%q", series.Granularity) }
	if len(series.Points) != 2 { t.Fatalf("points=%d want=2", len(series.Points)) }
	if series.Points[0].ObservationCount != 2 || series.Points[0].ReachableCount != 1 || series.Points[0].DualStackCount != 1 { t.Fatalf("first point=%+v", series.Points[0]) }
	if math.Abs(series.Points[0].ReachablePercent-50) > 0.0001 { t.Fatalf("reachable_percent=%v", series.Points[0].ReachablePercent) }
	if series.Points[1].ObservationCount != 1 || series.Points[1].ReachablePercent != 100 { t.Fatalf("second point=%+v", series.Points[1]) }

	hostSeries, err := store.HistoricalObservationStats(HistoricalStatsQuery{Period:"24h", Host:"IRC.ONE.EXAMPLE", Now:now})
	if err != nil { t.Fatal(err) }
	if len(hostSeries.Points) != 2 { t.Fatalf("host points=%d want=2", len(hostSeries.Points)) }
	for _, point := range hostSeries.Points { if point.ObservationCount != 1 { t.Fatalf("host point=%+v", point) } }

	allSeries, err := store.HistoricalObservationStats(HistoricalStatsQuery{Period:"all", Now:now})
	if err != nil { t.Fatal(err) }
	if allSeries.Granularity != "day" || allSeries.Since != nil { t.Fatalf("all series=%+v", allSeries) }
	if len(allSeries.Points) != 1 || allSeries.Points[0].ObservationCount != 3 { t.Fatalf("all points=%+v", allSeries.Points) }
}

func TestHistoricalStatsHandlerValidationAndUnavailable(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/history/observations?period=24h", nil)
	res := httptest.NewRecorder()
	HistoricalStatsHandler{}.ServeHTTP(res, req)
	if res.Code != http.StatusServiceUnavailable { t.Fatalf("status=%d", res.Code) }

	reader := historicalStatsReaderStub{}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/history/observations?period=90d", nil)
	res = httptest.NewRecorder()
	HistoricalStatsHandler{Reader:reader, Now:func() time.Time { return time.Date(2026,9,11,3,0,0,0,time.UTC) }}.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest { t.Fatalf("status=%d", res.Code) }
}

type historicalStatsReaderStub struct{}
func (historicalStatsReaderStub) HistoricalObservationStats(query HistoricalStatsQuery) (HistoricalStatsSeries, error) {
	_, _, err := historicalWindow(query.Period, query.Now)
	return HistoricalStatsSeries{}, err
}
