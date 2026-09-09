package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Ploos-AS/IRCIntel/internal/probe"
)

type fakeRunner struct {
	result probe.EndpointResult
	err    error
	calls  []probe.Config
}

func (f *fakeRunner) RunEndpoint(_ context.Context, cfg probe.Config) (probe.EndpointResult, error) {
	f.calls = append(f.calls, cfg)
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
}

func TestRunOnceRequiresConfiguration(t *testing.T) {
	if err := (&Agent{}).RunOnce(context.Background()); err == nil { t.Fatal("expected validation error") }
}
