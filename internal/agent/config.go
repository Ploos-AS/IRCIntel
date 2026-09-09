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

	targets := strings.TrimSpace(os.Getenv("IRCINTEL_AGENT_TARGETS"))
	if targets == "" { return cfg, errors.New("IRCINTEL_AGENT_TARGETS is required") }
	if err := json.Unmarshal([]byte(targets), &cfg.Endpoints); err != nil { return cfg, err }
	if len(cfg.Endpoints) == 0 { return cfg, errors.New("IRCINTEL_AGENT_TARGETS must contain at least one endpoint") }

	contact := strings.TrimSpace(os.Getenv("IRCINTEL_AGENT_CONTACT"))
	if contact == "" { return cfg, errors.New("IRCINTEL_AGENT_CONTACT is required") }
	nick := strings.TrimSpace(os.Getenv("IRCINTEL_AGENT_NICK"))
	if nick == "" { nick = "IRCIntelProbe" }
	username := strings.TrimSpace(os.Getenv("IRCINTEL_AGENT_USERNAME"))
	if username == "" { username = "ircintel" }
	cfg.Identity = probe.Identity{Nick: nick, Username: username, Realname: "IRCIntel distributed probe", Contact: contact}

	cfg.CoreURL = strings.TrimSpace(os.Getenv("IRCINTEL_CORE_URL"))
	cfg.CoreToken = strings.TrimSpace(os.Getenv("IRCINTEL_CORE_TOKEN"))
	cfg.Retries = 3
	if raw := strings.TrimSpace(os.Getenv("IRCINTEL_AGENT_RETRIES")); raw != "" {
		retries, err := strconv.Atoi(raw)
		if err != nil || retries < 0 { return cfg, errors.New("IRCINTEL_AGENT_RETRIES must be a non-negative integer") }
		cfg.Retries = retries
	}

	allowHosts := make([]string, 0, len(cfg.Endpoints))
	seen := map[string]struct{}{}
	for _, endpoint := range cfg.Endpoints {
		host := strings.TrimSpace(endpoint.Host)
		if host == "" { return cfg, errors.New("agent endpoint host is required") }
		if _, ok := seen[host]; ok { continue }
		seen[host] = struct{}{}
		allowHosts = append(allowHosts, host)
	}
	cfg.Policy = probe.Policy{AllowHosts: allowHosts, MinInterval: cfg.Interval}
	return cfg, nil
}
