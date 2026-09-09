package core

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type statsByNetworkFixture struct {
	snapshot  RegistrySnapshot
	incidents []NetworkIncident
}

func (f statsByNetworkFixture) NetworkIncidentStatsSnapshot(time.Time, time.Time) (RegistrySnapshot, []NetworkIncident, error) {
	return f.snapshot, f.incidents, nil
}

func TestNetworkIncidentStatsByNetworkIncludesZeroIncidentNetworks(t *testing.T) {
	t0 := time.Date(2026, 9, 9, 10, 0, 0, 0, time.UTC)
	duration := int64(120)
	fixture := statsByNetworkFixture{
		snapshot: RegistrySnapshot{Networks: []Network{{ID: "net-b", Name: "Beta"}, {ID: "net-a", Name: "Alpha"}}},
		incidents: []NetworkIncident{{NetworkID: "net-a", NetworkName: "Alpha", Status: "closed", Severity: "down", StartedAt: t0, DurationSeconds: &duration}},
	}
	h := NetworkIncidentStatsByNetworkHandler{Token: "secret", Reader: fixture}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/networks/incidents/stats/by-network", nil)
	req.Header.Set("Authorization", "Bearer secret")
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK { t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String()) }
	body := rr.Body.String()
	alpha := `"network_id":"net-a"`
	beta := `"network_id":"net-b"`
	if !contains(body, alpha) || !contains(body, beta) { t.Fatalf("body=%s", body) }
	if indexOf(body, alpha) > indexOf(body, beta) { t.Fatalf("networks not sorted by name: %s", body) }
	if !contains(body, `"total_incidents":1`) || !contains(body, `"total_incidents":0`) { t.Fatalf("missing expected stats: %s", body) }
}

func indexOf(value, needle string) int {
	for i := 0; i+len(needle) <= len(value); i++ {
		if value[i:i+len(needle)] == needle { return i }
	}
	return -1
}
