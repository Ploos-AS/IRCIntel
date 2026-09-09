package agent

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/Ploos-AS/IRCIntel/internal/probe"
)

type Endpoint struct {
	Host string `json:"host"`
	Port string `json:"port,omitempty"`
	TLS  bool   `json:"tls"`
}

type Observation struct {
	AgentID    string               `json:"agent_id"`
	ObservedAt time.Time            `json:"observed_at"`
	Endpoint   Endpoint             `json:"endpoint"`
	Result     probe.EndpointResult `json:"result"`
	Error      string               `json:"error,omitempty"`
	ErrorCode  string               `json:"error_code,omitempty"`
	ErrorStage string               `json:"error_stage,omitempty"`
}

type EndpointRunner interface {
	RunEndpoint(context.Context, probe.Config) (probe.EndpointResult, error)
}

type Agent struct {
	ID        string
	Interval  time.Duration
	Endpoints []Endpoint
	Runner    EndpointRunner
	Submitter Submitter
	Output    io.Writer
	Now       func() time.Time
}

func (a *Agent) RunOnce(ctx context.Context) error {
	if a.ID == "" { return errors.New("agent id is required") }
	if a.Runner == nil { return errors.New("agent runner is required") }
	if len(a.Endpoints) == 0 { return errors.New("at least one endpoint is required") }
	if a.Now == nil { a.Now = time.Now }

	submitter := a.Submitter
	if submitter == nil {
		if a.Output == nil { return errors.New("agent submitter is required") }
		submitter = WriterSubmitter{Writer: a.Output}
	}

	for _, endpoint := range a.Endpoints {
		result, probeErr := a.Runner.RunEndpoint(ctx, probe.Config{Host: endpoint.Host, Port: endpoint.Port, TLS: endpoint.TLS})
		observation := Observation{
			AgentID:    a.ID,
			ObservedAt: a.Now().UTC(),
			Endpoint:   endpoint,
			Result:     result,
		}
		if probeErr != nil {
			observation.Error = probeErr.Error()
			observation.ErrorCode, observation.ErrorStage = probe.ErrorInfo(probeErr)
		}
		if err := submitter.Submit(ctx, observation); err != nil { return err }
	}
	return nil
}

func (a *Agent) Run(ctx context.Context) error {
	if a.Interval <= 0 { return errors.New("agent interval must be positive") }
	if err := a.RunOnce(ctx); err != nil { return err }

	ticker := time.NewTicker(a.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := a.RunOnce(ctx); err != nil { return err }
		}
	}
}
