package core

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNetworkReliabilityOrdersByObservedIncidentBurden(t *testing.T) {
	t0 := time.Date(2026, 9, 9, 10, 0, 0, 0, time.UTC)
	short := int64(60)
	long := int64(600)
	fixture := statsByNetworkFixture{
		snapshot: RegistrySnapshot{Networks: []Network{{ID: "bad", Name: "BadNet"}, {ID: "clean", Name: "CleanNet"}, {ID: "mid", Name: "MidNet"}}},
		incidents: []NetworkIncident{
			{NetworkID: "bad", NetworkName: "BadNet", Status: "closed", Severity: "down", StartedAt: t0, DurationSeconds: &long},
			{NetworkID: "mid", NetworkName: "MidNet", Status: "closed", Severity: "degraded", StartedAt: t0, DurationSeconds: &short},
		},
	}
	h := NetworkReliabilityHandler{Token: "secret", Reader: fixture}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/networks/reliability", nil)
	req.Header.Set("Authorization", "Bearer secret")
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK { t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String()) }
	body := rr.Body.String()
	clean, mid, bad := `"network_id":"clean"`, `"network_id":"mid"`, `"network_id":"bad"`
	if !contains(body, clean) || !contains(body, mid) || !contains(body, bad) { t.Fatalf("body=%s", body) }
	if !(indexOf(body, clean) < indexOf(body, mid) && indexOf(body, mid) < indexOf(body, bad)) { t.Fatalf("unexpected ordering: %s", body) }
	if !contains(body, `"rank":1`) || !contains(body, `"rank":3`) { t.Fatalf("missing ranks: %s", body) }
}

func TestNetworkReliabilityRejectsInvalidRange(t *testing.T) {
	fixture := statsByNetworkFixture{}
	h := NetworkReliabilityHandler{Reader: fixture}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/networks/reliability?since=2026-09-10T00:00:00Z&until=2026-09-09T00:00:00Z", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest { t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String()) }
}
