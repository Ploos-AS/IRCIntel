package probe

import (
	"testing"
	"time"
)

func TestAuthorizeEndpointSeparatesPortsAndTLS(t *testing.T) {
	r, err := NewRunner(Identity{
		Nick: "IRCIntelProbe", Username: "ircintel", Contact: "https://example.invalid/contact",
	}, Policy{AllowHosts: []string{"irc.example"}, MinInterval: time.Hour})
	if err != nil { t.Fatal(err) }

	if err := r.authorizeEndpoint("irc.example", "6697", true); err != nil { t.Fatal(err) }
	if err := r.authorizeEndpoint("irc.example", "6667", false); err != nil {
		t.Fatalf("different port/TLS endpoint should have an independent slot: %v", err)
	}
	if err := r.authorizeEndpoint("irc.example", "7000", true); err != nil {
		t.Fatalf("different port should have an independent slot: %v", err)
	}
	if err := r.authorizeEndpoint("irc.example", "6697", false); err != nil {
		t.Fatalf("different TLS mode should have an independent slot: %v", err)
	}
}

func TestAuthorizeEndpointLimitsSameEndpoint(t *testing.T) {
	r, err := NewRunner(Identity{
		Nick: "IRCIntelProbe", Username: "ircintel", Contact: "https://example.invalid/contact",
	}, Policy{AllowHosts: []string{"irc.example"}, MinInterval: time.Hour})
	if err != nil { t.Fatal(err) }

	if err := r.authorizeEndpoint("irc.example", "6697", true); err != nil { t.Fatal(err) }
	if err := r.authorizeEndpoint("IRC.EXAMPLE", "6697", true); err == nil {
		t.Fatal("expected same normalized endpoint to be rate limited")
	} else if code, _ := ErrorInfo(err); code != CodeRateLimited {
		t.Fatalf("error code=%q err=%v", code, err)
	}
}

func TestAuthorizeEndpointUsesEffectiveDefaultPort(t *testing.T) {
	r, err := NewRunner(Identity{
		Nick: "IRCIntelProbe", Username: "ircintel", Contact: "https://example.invalid/contact",
	}, Policy{AllowHosts: []string{"irc.example"}, MinInterval: time.Hour})
	if err != nil { t.Fatal(err) }

	if err := r.authorizeEndpoint("irc.example", "", true); err != nil { t.Fatal(err) }
	if err := r.authorizeEndpoint("irc.example", "6697", true); err == nil {
		t.Fatal("implicit and explicit TLS default port must share one slot")
	}
}
