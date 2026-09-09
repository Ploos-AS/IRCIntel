package core

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
)

type ProbePlanEndpoint struct {
	ID   string `json:"id"`
	Host string `json:"host"`
	Port string `json:"port"`
	TLS  bool   `json:"tls"`
}

type ProbePlan struct {
	AgentID   string              `json:"agent_id"`
	Endpoints []ProbePlanEndpoint `json:"endpoints"`
}

type ProbePlanHandler struct {
	Token  string
	Reader RegistryReader
}

func (h ProbePlanHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.Reader == nil {
		http.Error(w, "probe plan reader unavailable", http.StatusServiceUnavailable)
		return
	}
	if h.Token != "" && r.Header.Get("Authorization") != "Bearer "+h.Token {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	agentID := strings.TrimSpace(r.URL.Query().Get("agent_id"))
	if agentID == "" {
		http.Error(w, "agent_id is required", http.StatusBadRequest)
		return
	}
	snapshot, err := h.Reader.RegistrySnapshot()
	if err != nil {
		http.Error(w, "probe plan query failed", http.StatusServiceUnavailable)
		return
	}
	endpoints := make([]ProbePlanEndpoint, 0, len(snapshot.Endpoints))
	for _, endpoint := range snapshot.Endpoints {
		endpoints = append(endpoints, ProbePlanEndpoint{ID: endpoint.ID, Host: endpoint.Host, Port: endpoint.Port, TLS: endpoint.TLS})
	}
	sort.Slice(endpoints, func(i, j int) bool {
		if endpoints[i].Host != endpoints[j].Host { return endpoints[i].Host < endpoints[j].Host }
		if endpoints[i].Port != endpoints[j].Port { return endpoints[i].Port < endpoints[j].Port }
		if endpoints[i].TLS != endpoints[j].TLS { return !endpoints[i].TLS && endpoints[j].TLS }
		return endpoints[i].ID < endpoints[j].ID
	})
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ProbePlan{AgentID: agentID, Endpoints: endpoints})
}
