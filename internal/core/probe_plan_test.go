package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type probePlanRegistryReader struct {
	snapshot RegistrySnapshot
}

func (r probePlanRegistryReader) RegistrySnapshot() (RegistrySnapshot, error) { return r.snapshot, nil }

func TestProbePlanUsesOnlyRegistryEndpoints(t *testing.T) {
	h := ProbePlanHandler{Token: "secret", Reader: probePlanRegistryReader{snapshot: RegistrySnapshot{
		Endpoints: []NetworkEndpoint{
			{ID: "b", ServerID: "s", Host: "irc.example", Port: "6697", TLS: true},
			{ID: "a", ServerID: "s", Host: "irc.example", Port: "6667", TLS: false},
		},
	}}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/probe-plan?agent_id=oslo-1", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK { t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String()) }
	var plan ProbePlan
	if err := json.Unmarshal(rr.Body.Bytes(), &plan); err != nil { t.Fatal(err) }
	if plan.AgentID != "oslo-1" || len(plan.Endpoints) != 2 { t.Fatalf("plan=%+v", plan) }
	if plan.Endpoints[0].ID != "a" || plan.Endpoints[1].ID != "b" { t.Fatalf("endpoints=%+v", plan.Endpoints) }
}

func TestProbePlanRequiresAgentAndAuth(t *testing.T) {
	h := ProbePlanHandler{Token: "secret", Reader: probePlanRegistryReader{snapshot: RegistrySnapshot{}}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/probe-plan", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized { t.Fatalf("status=%d", rr.Code) }
	req = httptest.NewRequest(http.MethodGet, "/api/v1/probe-plan", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest { t.Fatalf("status=%d", rr.Code) }
}
