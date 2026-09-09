package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Ploos-AS/IRCIntel/internal/probe"
)

type fakeRunner struct {
	result  probe.EndpointResult
	err     error
	results []probe.EndpointResult
	errors  []error
	calls   []probe.Config
}

func (f *fakeRunner) RunEndpoint(_ context.Context, cfg probe.Config) (probe.EndpointResult, error) {
	index := len(f.calls)
	f.calls = append(f.calls, cfg)
	if index < len(f.results) || index < len(f.errors) {
		var result probe.EndpointResult
		var err error
		if index < len(f.results) { result = f.results[index] }
		if index < len(f.errors) { err = f.errors[index] }
		return result, err
	}
	return f.result, f.err
}

func TestRunOnceEmitsNormalizedObservation(t *testing.T) {
	var out bytes.Buffer
	runner := &fakeRunner{result: probe.EndpointResult{Host: "irc.example", Port: "6697", TLS: true, Reachable: true}}
	observedAt := time.Date(2026, 9, 9, 1, 55, 0, 0, time.UTC)
	a := Agent{
		ID:        "probe-oslo-1",
		Endpoints: []Endpoint{{Host: "irc.example", Port: "6697", TLS: true}},
		Runner:    runner,
		Output:    &out,
		Now:       func() time.Time { return observedAt },
	}
	if err := a.RunOnce(context.Background()); err != nil { t.Fatal(err) }
	if len(runner.calls) != 1 { t.Fatalf("calls=%d", len(runner.calls)) }
	if runner.calls[0].Host != "irc.example" || runner.calls[0].Port != "6697" || !runner.calls[0].TLS {
		t.Fatalf("config=%+v", runner.calls[0])
	}
	var observation Observation
	if err := json.NewDecoder(&out).Decode(&observation); err != nil { t.Fatal(err) }
	if observation.AgentID != "probe-oslo-1" { t.Fatalf("agent_id=%q", observation.AgentID) }
	if !observation.ObservedAt.Equal(observedAt) { t.Fatalf("observed_at=%v", observation.ObservedAt) }
	if !observation.Result.Reachable { t.Fatalf("result=%+v", observation.Result) }
	if observation.Error != "" || observation.ErrorCode != "" || observation.ErrorStage != "" { t.Fatalf("unexpected error fields: %+v", observation) }
}

func TestRunOnceContinuesAfterEndpointProbeFailure(t *testing.T) {
	var out bytes.Buffer
	runner := &fakeRunner{
		results: []probe.EndpointResult{
			{Host: "bad.example", Port: "6697", TLS: true, Reachable: false},
			{Host: "good.example", Port: "6697", TLS: true, Reachable: true},
		},
		errors: []error{
			&probe.ProbeError{Code: probe.CodeDNSLookupFailed, Stage: "dns", Err: errors.New("lookup failed")},
			nil,
		},
	}
	a := Agent{
		ID: "probe-oslo-1",
		Endpoints: []Endpoint{
			{Host: "bad.example", Port: "6697", TLS: true},
			{Host: "good.example", Port: "6697", TLS: true},
		},
		Runner: runner,
		Output: &out,
		Now:    func() time.Time { return time.Date(2026, 9, 9, 8, 0, 0, 0, time.UTC) },
	}
	if err := a.RunOnce(context.Background()); err != nil { t.Fatal(err) }
	if len(runner.calls) != 2 { t.Fatalf("calls=%d", len(runner.calls)) }

	decoder := json.NewDecoder(&out)
	var failed Observation
	if err := decoder.Decode(&failed); err != nil { t.Fatal(err) }
	if failed.Endpoint.Host != "bad.example" || failed.ErrorCode != probe.CodeDNSLookupFailed || failed.ErrorStage != "dns" || failed.Error == "" {
		t.Fatalf("failed observation=%+v", failed)
	}
	var succeeded Observation
	if err := decoder.Decode(&succeeded); err != nil { t.Fatal(err) }
	if succeeded.Endpoint.Host != "good.example" || !succeeded.Result.Reachable || succeeded.Error != "" {
		t.Fatalf("successful observation=%+v", succeeded)
	}
}

func TestRunOnceStopsOnSubmissionFailure(t *testing.T) {
	runner := &fakeRunner{result: probe.EndpointResult{Reachable: true}}
	a := Agent{
		ID:        "probe-oslo-1",
		Endpoints: []Endpoint{{Host: "one.example"}, {Host: "two.example"}},
		Runner:    runner,
		Submitter: failingSubmitter{},
	}
	if err := a.RunOnce(context.Background()); err == nil { t.Fatal("expected submission error") }
	if len(runner.calls) != 1 { t.Fatalf("calls=%d", len(runner.calls)) }
}

type failingSubmitter struct{}

func (failingSubmitter) Submit(context.Context, Observation) error { return errors.New("submit failed") }

func TestRunOnceRequiresConfiguration(t *testing.T) {
	if err := (&Agent{}).RunOnce(context.Background()); err == nil { t.Fatal("expected validation error") }
}
