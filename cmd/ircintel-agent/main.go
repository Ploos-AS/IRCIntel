package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Ploos-AS/IRCIntel/internal/agent"
	"github.com/Ploos-AS/IRCIntel/internal/probe"
)

func run(ctx context.Context) error {
	cfg, err := agent.ConfigFromEnv()
	if err != nil { return err }
	if len(cfg.Endpoints) == 0 {
		cfg.Endpoints, err = (agent.ProbePlanClient{CoreURL: cfg.CoreURL, Token: cfg.CoreToken}).Fetch(ctx, cfg.ID)
		if err != nil { return err }
		cfg.Policy = agent.PolicyForEndpoints(cfg.Endpoints, cfg.Interval)
	}
	runner, err := probe.NewRunner(cfg.Identity, cfg.Policy)
	if err != nil { return err }

	var submitter agent.Submitter = agent.WriterSubmitter{Writer: os.Stdout}
	if cfg.CoreURL != "" {
		submitter = agent.HTTPSubmitter{
			URL:     cfg.CoreURL,
			Token:   cfg.CoreToken,
			Retries: cfg.Retries,
		}
	}

	a := agent.Agent{
		ID:        cfg.ID,
		Interval:  cfg.Interval,
		Endpoints: cfg.Endpoints,
		Runner:    runner,
		Submitter: submitter,
	}
	return a.Run(ctx)
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
