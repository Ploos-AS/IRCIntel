package probe

import (
	"context"
	"net"
	"testing"
	"time"
)

func testRunner(t *testing.T, interval time.Duration) *Runner {
	t.Helper()
	runner, err := NewRunner(Identity{Nick: "IRCIntelProbe", Username: "ircintel", Realname: "IRCIntel test probe", Contact: "https://example.invalid/ircintel"}, Policy{AllowHosts: []string{"localhost"}, DenyHosts: []string{"blocked.localhost"}, MinInterval: interval})
	if err != nil { t.Fatal(err) }
	return runner
}

func TestRunPlainIRC(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil { t.Fatal(err) }
	defer ln.Close()

	done := make(chan error, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil { done <- err; return }
		defer conn.Close()
		buf := make([]byte, 1024)
		_, _ = conn.Read(buf)
		_, err = conn.Write([]byte(":test.example CAP IRCIntelProbe LS :multi-prefix sasl server-time\r\n:test.example 001 IRCIntelProbe :welcome\r\n"))
		done <- err
	}()

	port := ln.Addr().(*net.TCPAddr).Port
	result, err := testRunner(t, time.Millisecond).Run(context.Background(), Config{Host: "localhost", Port: fmtPort(port), Timeout: 2 * time.Second})
	if err != nil { t.Fatal(err) }
	if result.Server != "test.example" { t.Fatalf("server=%q", result.Server) }
	if len(result.Capabilities) != 3 { t.Fatalf("capabilities=%v", result.Capabilities) }
	if err := <-done; err != nil { t.Fatal(err) }
}

func TestNewRunnerRequiresIdentityAndAllowlist(t *testing.T) {
	if _, err := NewRunner(Identity{}, Policy{}); err == nil { t.Fatal("expected identity validation error") }
	if _, err := NewRunner(Identity{Nick: "n", Username: "u", Contact: "c"}, Policy{}); err == nil { t.Fatal("expected allowlist validation error") }
}

func TestPolicyDenyAndDefaultDeny(t *testing.T) {
	r := testRunner(t, time.Minute)
	if _, err := r.Run(context.Background(), Config{Host: "example.org"}); err == nil { t.Fatal("expected non-allowlisted target rejection") }
	if _, err := r.Run(context.Background(), Config{Host: "blocked.localhost"}); err == nil { t.Fatal("expected denylist rejection") }
}

func TestRateLimit(t *testing.T) {
	r := testRunner(t, time.Hour)
	now := time.Unix(1000, 0)
	r.now = func() time.Time { return now }
	if err := r.authorize("localhost"); err != nil { t.Fatal(err) }
	if err := r.authorize("localhost"); err == nil { t.Fatal("expected rate limit") }
	now = now.Add(time.Hour)
	if err := r.authorize("localhost"); err != nil { t.Fatal(err) }
}

func fmtPort(port int) string {
	const digits = "0123456789"
	if port == 0 { return "0" }
	var b [20]byte
	i := len(b)
	for port > 0 { i--; b[i] = digits[port%10]; port /= 10 }
	return string(b[i:])
}
