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
	runner, err := probe.NewRunner(cfg.Identity, cfg.Policy)
	if err != nil { return err }
	a := agent.Agent{
		ID:        cfg.ID,
		Interval:  cfg.Interval,
		Endpoints: cfg.Endpoints,
		Runner:    runner,
		Output:    os.Stdout,
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
