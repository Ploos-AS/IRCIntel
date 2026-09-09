package probe

import (
	"context"
	"errors"
	"strings"
	"sync"
)

// FamilyMeasurement is one address-family observation for an IRC endpoint.
// Probe failures are data: a failed IPv6 path must not erase a successful IPv4 path.
type FamilyMeasurement struct {
	Family     string  `json:"family"`
	OK         bool    `json:"ok"`
	Result     *Result `json:"result,omitempty"`
	Error      string  `json:"error,omitempty"`
	ErrorCode  string  `json:"error_code,omitempty"`
	ErrorStage string  `json:"error_stage,omitempty"`
}

// EndpointResult groups the IPv4 and IPv6 observations that belong to one
// scheduled endpoint measurement.
type EndpointResult struct {
	Host         string              `json:"host"`
	Port         string              `json:"port"`
	TLS          bool                `json:"tls"`
	Reachable    bool                `json:"reachable"`
	DualStackOK  bool                `json:"dual_stack_ok"`
	Measurements []FamilyMeasurement `json:"measurements"`
}

// RunEndpoint performs one policy/rate-limit decision for the endpoint and
// then runs explicit IPv4 and IPv6 sub-probes. Individual family failures are
// returned as structured observations rather than as an endpoint-level error.
func (r *Runner) RunEndpoint(ctx context.Context, cfg Config) (EndpointResult, error) {
	if strings.TrimSpace(cfg.Host) == "" {
		return EndpointResult{}, probeError(CodeInvalidConfig, "config", errors.New("probe host is required"))
	}

	port := cfg.Port
	if port == "" {
		if cfg.TLS { port = "6697" } else { port = "6667" }
	}
	if err := r.authorizeEndpoint(cfg.Host, port, cfg.TLS); err != nil {
		return EndpointResult{Host: cfg.Host, Port: port, TLS: cfg.TLS}, err
	}
	out := EndpointResult{Host: cfg.Host, Port: port, TLS: cfg.TLS}

	type indexedMeasurement struct {
		index int
		value FamilyMeasurement
	}
	results := make(chan indexedMeasurement, 2)
	families := []string{"ipv4", "ipv6"}
	var wg sync.WaitGroup
	for i, family := range families {
		wg.Add(1)
		go func(index int, family string) {
			defer wg.Done()
			child, err := NewRunner(r.identity, r.policy)
			if err != nil {
				code, stage := ErrorInfo(err)
				results <- indexedMeasurement{index: index, value: FamilyMeasurement{Family: family, Error: err.Error(), ErrorCode: code, ErrorStage: stage}}
				return
			}
			childCfg := cfg
			childCfg.Port = port
			childCfg.Family = family
			result, err := child.Run(ctx, childCfg)
			measurement := FamilyMeasurement{Family: family}
			if err != nil {
				measurement.Error = err.Error()
				measurement.ErrorCode, measurement.ErrorStage = ErrorInfo(err)
				if result.Host != "" { measurement.Result = &result }
			} else {
				measurement.OK = true
				measurement.Result = &result
			}
			results <- indexedMeasurement{index: index, value: measurement}
		}(i, family)
	}
	go func() { wg.Wait(); close(results) }()

	out.Measurements = make([]FamilyMeasurement, 2)
	successes := 0
	for item := range results {
		out.Measurements[item.index] = item.value
		if item.value.OK { successes++ }
	}
	out.Reachable = successes > 0
	out.DualStackOK = successes == 2
	return out, nil
}
