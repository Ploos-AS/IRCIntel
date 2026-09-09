package agent

import (
	"testing"
	"time"
)

func TestConfigFromEnv(t *testing.T) {
	t.Setenv("IRCINTEL_AGENT_ID", "oslo-1")
	t.Setenv("IRCINTEL_AGENT_INTERVAL", "10m")
	t.Setenv("IRCINTEL_AGENT_CONTACT", "https://example.invalid/ircintel")
	t.Setenv("IRCINTEL_AGENT_TARGETS", `[{"host":"irc.example","port":"6697","tls":true},{"host":"irc.example","port":"7000","tls":true}]`)

	cfg, err := ConfigFromEnv()
	if err != nil { t.Fatal(err) }
	if cfg.ID != "oslo-1" || cfg.Interval != 10*time.Minute { t.Fatalf("cfg=%+v", cfg) }
	if len(cfg.Endpoints) != 2 { t.Fatalf("endpoints=%v", cfg.Endpoints) }
	if len(cfg.Policy.AllowHosts) != 1 || cfg.Policy.AllowHosts[0] != "irc.example" { t.Fatalf("allow=%v", cfg.Policy.AllowHosts) }
	if cfg.Identity.Nick != "IRCIntelProbe" || cfg.Identity.Username != "ircintel" { t.Fatalf("identity=%+v", cfg.Identity) }
}

func TestConfigFromEnvRequiresIdentityAndTargets(t *testing.T) {
	t.Setenv("IRCINTEL_AGENT_ID", "")
	if _, err := ConfigFromEnv(); err == nil { t.Fatal("expected agent id error") }

	t.Setenv("IRCINTEL_AGENT_ID", "oslo-1")
	t.Setenv("IRCINTEL_AGENT_CONTACT", "https://example.invalid/ircintel")
	t.Setenv("IRCINTEL_AGENT_TARGETS", "")
	if _, err := ConfigFromEnv(); err == nil { t.Fatal("expected target error") }
}
