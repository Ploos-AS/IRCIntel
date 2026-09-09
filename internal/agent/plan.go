package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type ProbePlanClient struct {
	CoreURL string
	Token   string
	Client  *http.Client
}

type probePlanResponse struct {
	AgentID   string `json:"agent_id"`
	Endpoints []struct {
		ID   string `json:"id"`
		Host string `json:"host"`
		Port string `json:"port"`
		TLS  bool   `json:"tls"`
	} `json:"endpoints"`
}

func (c ProbePlanClient) Fetch(ctx context.Context, agentID string) ([]Endpoint, error) {
	base := strings.TrimRight(strings.TrimSpace(c.CoreURL), "/")
	if base == "" { return nil, errors.New("core URL is required") }
	if strings.TrimSpace(agentID) == "" { return nil, errors.New("agent id is required") }
	u, err := url.Parse(base + "/api/v1/probe-plan")
	if err != nil { return nil, err }
	q := u.Query()
	q.Set("agent_id", agentID)
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil { return nil, err }
	if c.Token != "" { req.Header.Set("Authorization", "Bearer "+c.Token) }
	client := c.Client
	if client == nil { client = &http.Client{Timeout: 10 * time.Second} }
	resp, err := client.Do(req)
	if err != nil { return nil, err }
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 { return nil, fmt.Errorf("probe plan request failed: %s", resp.Status) }
	var plan probePlanResponse
	dec := json.NewDecoder(resp.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&plan); err != nil { return nil, err }
	if plan.AgentID != agentID { return nil, errors.New("probe plan agent id mismatch") }
	endpoints := make([]Endpoint, 0, len(plan.Endpoints))
	for _, item := range plan.Endpoints {
		host := strings.TrimSpace(item.Host)
		if host == "" { return nil, errors.New("probe plan endpoint host is required") }
		endpoints = append(endpoints, Endpoint{Host: host, Port: strings.TrimSpace(item.Port), TLS: item.TLS})
	}
	if len(endpoints) == 0 { return nil, errors.New("probe plan contains no endpoints") }
	return endpoints, nil
}
