package agent

import (
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Ploos-AS/IRCIntel/internal/probe"
)

type Config struct {
	ID        string
	Interval  time.Duration
	Endpoints []Endpoint
	Identity  probe.Identity
	Policy    probe.Policy
	CoreURL   string
	CoreToken string
	Retries   int
}

func ConfigFromEnv() (Config, error) {
	var cfg Config
	cfg.ID = strings.TrimSpace(os.Getenv("IRCINTEL_AGENT_ID"))
	if cfg.ID == "" { return cfg, errors.New("IRCINTEL_AGENT_ID is required") }

	interval := strings.TrimSpace(os.Getenv("IRCINTEL_AGENT_INTERVAL"))
	if interval == "" { interval = "5m" }
	parsedInterval, err := time.ParseDuration(interval)
	if err != nil || parsedInterval <= 0 { return cfg, errors.New("IRCINTEL_AGENT_INTERVAL must be a positive duration") }
	cfg.Interval = parsedInterval

	cfg.CoreURL = strings.TrimSpace(os.Getenv("IRCINTEL_CORE_URL"))
	cfg.CoreToken = strings.TrimSpace(os.Getenv("IRCINTEL_CORE_TOKEN"))

	targets := strings.TrimSpace(os.Getenv("IRCINTEL_AGENT_TARGETS"))
	if targets != "" {
		if err := json.Unmarshal([]byte(targets), &cfg.Endpoints); err != nil { return cfg, err }
		if len(cfg.Endpoints) == 0 { return cfg, errors.New("IRCINTEL_AGENT_TARGETS must contain at least one endpoint") }
	} else if cfg.CoreURL == "" {
		return cfg, errors.New("IRCINTEL_AGENT_TARGETS or IRCINTEL_CORE_URL is required")
	}

	contact := strings.TrimSpace(os.Getenv("IRCINTEL_AGENT_CONTACT"))
	if contact == "" { return cfg, errors.New("IRCINTEL_AGENT_CONTACT is required") }
	nick := strings.TrimSpace(os.Getenv("IRCINTEL_AGENT_NICK"))
	if nick == "" { nick = "IRCIntelProbe" }
	username := strings.TrimSpace(os.Getenv("IRCINTEL_AGENT_USERNAME"))
	if username == "" { username = "ircintel" }
	cfg.Identity = probe.Identity{Nick: nick, Username: username, Realname: "IRCIntel distributed probe", Contact: contact}

	cfg.Retries = 3
	if raw := strings.TrimSpace(os.Getenv("IRCINTEL_AGENT_RETRIES")); raw != "" {
		retries, err := strconv.Atoi(raw)
		if err != nil || retries < 0 { return cfg, errors.New("IRCINTEL_AGENT_RETRIES must be a non-negative integer") }
		cfg.Retries = retries
	}

	cfg.Policy = PolicyForEndpoints(cfg.Endpoints, cfg.Interval)
	return cfg, nil
}

func PolicyForEndpoints(endpoints []Endpoint, interval time.Duration) probe.Policy {
	allowHosts := make([]string, 0, len(endpoints))
	seen := map[string]struct{}{}
	for _, endpoint := range endpoints {
		host := strings.TrimSpace(endpoint.Host)
		if host == "" { continue }
		if _, ok := seen[host]; ok { continue }
		seen[host] = struct{}{}
		allowHosts = append(allowHosts, host)
	}
	return probe.Policy{AllowHosts: allowHosts, MinInterval: interval}
}
