package probe

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func testRunner(t *testing.T, interval time.Duration) *Runner {
	t.Helper()
	runner, err := NewRunner(Identity{Nick: "IRCIntelProbe", Username: "ircintel", Realname: "IRCIntel test probe", Contact: "https://example.invalid/ircintel"}, Policy{AllowHosts: []string{"localhost"}, DenyHosts: []string{"blocked.localhost"}, MinInterval: interval})
	if err != nil { t.Fatal(err) }
	return runner
}

func serveOnce(t *testing.T, handler func(net.Conn) error) (string, <-chan error) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil { t.Fatal(err) }
	done := make(chan error, 1)
	go func() {
		defer ln.Close()
		conn, err := ln.Accept()
		if err != nil { done <- err; return }
		defer conn.Close()
		done <- handler(conn)
	}()
	return fmtPort(ln.Addr().(*net.TCPAddr).Port), done
}

func readUntil(r *bufio.Reader, want string) error {
	for {
		line, err := r.ReadString('\n')
		if err != nil { return err }
		if strings.TrimSpace(line) == want { return nil }
	}
}

func TestRunPlainIRC(t *testing.T) {
	port, done := serveOnce(t, func(conn net.Conn) error {
		r := bufio.NewReader(conn)
		if err := readUntil(r, "USER ircintel 0 * :IRCIntel test probe | contact: https://example.invalid/ircintel"); err != nil { return err }
		if _, err := fmt.Fprint(conn, ":test.example CAP IRCIntelProbe LS :multi-prefix sasl server-time\r\n"); err != nil { return err }
		if err := readUntil(r, "CAP END"); err != nil { return err }
		_, err := fmt.Fprint(conn, ":test.example 001 IRCIntelProbe :welcome\r\n")
		return err
	})

	result, err := testRunner(t, time.Millisecond).Run(context.Background(), Config{Host: "localhost", Port: port, Timeout: 2 * time.Second})
	if err != nil { t.Fatal(err) }
	if result.Server != "test.example" { t.Fatalf("server=%q", result.Server) }
	if len(result.Capabilities) != 3 { t.Fatalf("capabilities=%v", result.Capabilities) }
	if err := <-done; err != nil { t.Fatal(err) }
}

func TestRunCAPEndRequiredBeforeWelcome(t *testing.T) {
	port, done := serveOnce(t, func(conn net.Conn) error {
		r := bufio.NewReader(conn)
		if err := readUntil(r, "USER ircintel 0 * :IRCIntel test probe | contact: https://example.invalid/ircintel"); err != nil { return err }
		if _, err := fmt.Fprint(conn, ":test.example CAP IRCIntelProbe LS :multi-prefix sasl\r\n"); err != nil { return err }
		if err := readUntil(r, "CAP END"); err != nil { return err }
		_, err := fmt.Fprint(conn, ":test.example 001 IRCIntelProbe :welcome\r\n")
		return err
	})

	if _, err := testRunner(t, time.Millisecond).Run(context.Background(), Config{Host: "localhost", Port: port, Timeout: 2 * time.Second}); err != nil { t.Fatal(err) }
	if err := <-done; err != nil { t.Fatal(err) }
}

func TestRunMultilineCAPLS(t *testing.T) {
	port, done := serveOnce(t, func(conn net.Conn) error {
		r := bufio.NewReader(conn)
		if err := readUntil(r, "USER ircintel 0 * :IRCIntel test probe | contact: https://example.invalid/ircintel"); err != nil { return err }
		if _, err := fmt.Fprint(conn, ":test.example CAP IRCIntelProbe LS * :multi-prefix sasl\r\n"); err != nil { return err }
		_ = conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		if line, err := r.ReadString('\n'); err == nil && strings.TrimSpace(line) == "CAP END" {
			return fmt.Errorf("CAP END sent before final CAP LS line")
		}
		_ = conn.SetReadDeadline(time.Time{})
		if _, err := fmt.Fprint(conn, ":test.example CAP IRCIntelProbe LS :server-time account-tag\r\n"); err != nil { return err }
		if err := readUntil(r, "CAP END"); err != nil { return err }
		_, err := fmt.Fprint(conn, ":test.example 001 IRCIntelProbe :welcome\r\n")
		return err
	})

	result, err := testRunner(t, time.Millisecond).Run(context.Background(), Config{Host: "localhost", Port: port, Timeout: 2 * time.Second})
	if err != nil { t.Fatal(err) }
	want := []string{"account-tag", "multi-prefix", "sasl", "server-time"}
	if fmt.Sprint(result.Capabilities) != fmt.Sprint(want) { t.Fatalf("capabilities=%v want=%v", result.Capabilities, want) }
	if err := <-done; err != nil { t.Fatal(err) }
}

func TestRunRespondsToPINGDuringRegistration(t *testing.T) {
	port, done := serveOnce(t, func(conn net.Conn) error {
		r := bufio.NewReader(conn)
		if err := readUntil(r, "USER ircintel 0 * :IRCIntel test probe | contact: https://example.invalid/ircintel"); err != nil { return err }
		if _, err := fmt.Fprint(conn, "PING :probe-token\r\n"); err != nil { return err }
		if err := readUntil(r, "PONG :probe-token"); err != nil { return err }
		if _, err := fmt.Fprint(conn, ":test.example CAP IRCIntelProbe LS :server-time\r\n"); err != nil { return err }
		if err := readUntil(r, "CAP END"); err != nil { return err }
		_, err := fmt.Fprint(conn, ":test.example 001 IRCIntelProbe :welcome\r\n")
		return err
	})

	if _, err := testRunner(t, time.Millisecond).Run(context.Background(), Config{Host: "localhost", Port: port, Timeout: 2 * time.Second}); err != nil { t.Fatal(err) }
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
