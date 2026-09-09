package probe

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
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

func serveOnce(t *testing.T, network, address string, handler func(net.Conn) error) (string, <-chan error) {
	t.Helper()
	ln, err := net.Listen(network, address)
	if err != nil { t.Skipf("%s unavailable: %v", network, err) }
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

func serveIRC(t *testing.T, network, address string) (string, <-chan error) {
	return serveOnce(t, network, address, func(conn net.Conn) error {
		r := bufio.NewReader(conn)
		if err := readUntil(r, "USER ircintel 0 * :IRCIntel test probe | contact: https://example.invalid/ircintel"); err != nil { return err }
		if _, err := fmt.Fprint(conn, ":test.example CAP IRCIntelProbe LS :multi-prefix sasl server-time\r\n"); err != nil { return err }
		if err := readUntil(r, "CAP END"); err != nil { return err }
		_, err := fmt.Fprint(conn, ":test.example 001 IRCIntelProbe :welcome\r\n")
		return err
	})
}

func readUntil(r *bufio.Reader, want string) error {
	for {
		line, err := r.ReadString('\n')
		if err != nil { return err }
		if strings.TrimSpace(line) == want { return nil }
	}
}

func TestRunPlainIRC(t *testing.T) {
	port, done := serveIRC(t, "tcp4", "127.0.0.1:0")
	result, err := testRunner(t, time.Millisecond).Run(context.Background(), Config{Host: "localhost", Port: port, Family: "ipv4", Timeout: 2 * time.Second})
	if err != nil { t.Fatal(err) }
	if result.Server != "test.example" { t.Fatalf("server=%q", result.Server) }
	if result.AddressFamily != "ipv4" { t.Fatalf("family=%q", result.AddressFamily) }
	if net.ParseIP(result.SelectedAddress).To4() == nil { t.Fatalf("selected=%q", result.SelectedAddress) }
	if len(result.Capabilities) != 3 { t.Fatalf("capabilities=%v", result.Capabilities) }
	if err := <-done; err != nil { t.Fatal(err) }
}

func TestRunExplicitIPv6(t *testing.T) {
	port, done := serveIRC(t, "tcp6", "[::1]:0")
	result, err := testRunner(t, time.Millisecond).Run(context.Background(), Config{Host: "localhost", Port: port, Family: "ipv6", Timeout: 2 * time.Second})
	if err != nil { t.Fatal(err) }
	ip := net.ParseIP(result.SelectedAddress)
	if result.AddressFamily != "ipv6" { t.Fatalf("family=%q", result.AddressFamily) }
	if ip == nil || ip.To4() != nil { t.Fatalf("selected=%q", result.SelectedAddress) }
	if err := <-done; err != nil { t.Fatal(err) }
}

func TestAddressFamilyHelpers(t *testing.T) {
	cases := []struct{ in, family, network string }{
		{"", "any", "tcp"}, {"auto", "any", "tcp"}, {"4", "ipv4", "tcp4"}, {"ipv4", "ipv4", "tcp4"}, {"6", "ipv6", "tcp6"}, {"tcp6", "ipv6", "tcp6"},
	}
	for _, tc := range cases {
		family, network, err := normalizeFamily(tc.in)
		if err != nil { t.Fatalf("%q: %v", tc.in, err) }
		if family != tc.family || network != tc.network { t.Fatalf("%q: got %q/%q", tc.in, family, network) }
	}
	if _, _, err := normalizeFamily("ipx"); err == nil { t.Fatal("expected invalid family error") }
	if got, err := selectAddress([]string{"2001:db8::1", "192.0.2.1"}, "ipv4"); err != nil || got != "192.0.2.1" { t.Fatalf("ipv4 got=%q err=%v", got, err) }
	if got, err := selectAddress([]string{"192.0.2.1", "2001:db8::1"}, "ipv6"); err != nil || got != "2001:db8::1" { t.Fatalf("ipv6 got=%q err=%v", got, err) }
	if _, err := selectAddress([]string{"192.0.2.1"}, "ipv6"); err == nil { t.Fatal("expected missing ipv6 error") }
}

func TestRunCAPEndRequiredBeforeWelcome(t *testing.T) {
	port, done := serveOnce(t, "tcp4", "127.0.0.1:0", func(conn net.Conn) error {
		r := bufio.NewReader(conn)
		if err := readUntil(r, "USER ircintel 0 * :IRCIntel test probe | contact: https://example.invalid/ircintel"); err != nil { return err }
		if _, err := fmt.Fprint(conn, ":test.example CAP IRCIntelProbe LS :multi-prefix sasl\r\n"); err != nil { return err }
		if err := readUntil(r, "CAP END"); err != nil { return err }
		_, err := fmt.Fprint(conn, ":test.example 001 IRCIntelProbe :welcome\r\n")
		return err
	})
	if _, err := testRunner(t, time.Millisecond).Run(context.Background(), Config{Host: "localhost", Port: port, Family: "ipv4", Timeout: 2 * time.Second}); err != nil { t.Fatal(err) }
	if err := <-done; err != nil { t.Fatal(err) }
}

func TestRunMultilineCAPLS(t *testing.T) {
	port, done := serveOnce(t, "tcp4", "127.0.0.1:0", func(conn net.Conn) error {
		r := bufio.NewReader(conn)
		if err := readUntil(r, "USER ircintel 0 * :IRCIntel test probe | contact: https://example.invalid/ircintel"); err != nil { return err }
		if _, err := fmt.Fprint(conn, ":test.example CAP IRCIntelProbe LS * :multi-prefix sasl\r\n"); err != nil { return err }
		_ = conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		if line, err := r.ReadString('\n'); err == nil && strings.TrimSpace(line) == "CAP END" { return fmt.Errorf("CAP END sent before final CAP LS line") }
		_ = conn.SetReadDeadline(time.Time{})
		if _, err := fmt.Fprint(conn, ":test.example CAP IRCIntelProbe LS :server-time account-tag\r\n"); err != nil { return err }
		if err := readUntil(r, "CAP END"); err != nil { return err }
		_, err := fmt.Fprint(conn, ":test.example 001 IRCIntelProbe :welcome\r\n")
		return err
	})
	result, err := testRunner(t, time.Millisecond).Run(context.Background(), Config{Host: "localhost", Port: port, Family: "ipv4", Timeout: 2 * time.Second})
	if err != nil { t.Fatal(err) }
	want := []string{"account-tag", "multi-prefix", "sasl", "server-time"}
	if fmt.Sprint(result.Capabilities) != fmt.Sprint(want) { t.Fatalf("capabilities=%v want=%v", result.Capabilities, want) }
	if err := <-done; err != nil { t.Fatal(err) }
}

func TestRunRespondsToPINGDuringRegistration(t *testing.T) {
	port, done := serveOnce(t, "tcp4", "127.0.0.1:0", func(conn net.Conn) error {
		r := bufio.NewReader(conn)
		if err := readUntil(r, "USER ircintel 0 * :IRCIntel test probe | contact: https://example.invalid/ircintel"); err != nil { return err }
		if _, err := fmt.Fprint(conn, "PING :probe-token\r\n"); err != nil { return err }
		if err := readUntil(r, "PONG :probe-token"); err != nil { return err }
		if _, err := fmt.Fprint(conn, ":test.example CAP IRCIntelProbe LS :server-time\r\n"); err != nil { return err }
		if err := readUntil(r, "CAP END"); err != nil { return err }
		_, err := fmt.Fprint(conn, ":test.example 001 IRCIntelProbe :welcome\r\n")
		return err
	})
	if _, err := testRunner(t, time.Millisecond).Run(context.Background(), Config{Host: "localhost", Port: port, Family: "ipv4", Timeout: 2 * time.Second}); err != nil { t.Fatal(err) }
	if err := <-done; err != nil { t.Fatal(err) }
}

func TestMetadataFromTLSState(t *testing.T) {
	cert := &x509.Certificate{Issuer: pkix.Name{CommonName: "Example Test CA", Organization: []string{"Example Org"}}, Subject: pkix.Name{CommonName: "irc.example.test"}, NotBefore: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), NotAfter: time.Date(2027, 1, 2, 3, 4, 5, 0, time.UTC), DNSNames: []string{"irc.example.test", "*.example.test"}, IPAddresses: []net.IP{net.ParseIP("192.0.2.1"), net.ParseIP("2001:db8::1")}}
	metadata := metadataFromTLSState(tls.ConnectionState{Version: tls.VersionTLS13, CipherSuite: tls.TLS_AES_128_GCM_SHA256, PeerCertificates: []*x509.Certificate{cert}})
	if metadata.Version != "TLS 1.3" { t.Fatalf("version=%q", metadata.Version) }
	if metadata.CipherSuite != "TLS_AES_128_GCM_SHA256" { t.Fatalf("cipher=%q", metadata.CipherSuite) }
	if metadata.Issuer == "" || !strings.Contains(metadata.Issuer, "Example Test CA") { t.Fatalf("issuer=%q", metadata.Issuer) }
	if metadata.Subject != "CN=irc.example.test" { t.Fatalf("subject=%q", metadata.Subject) }
	if metadata.NotBefore != "2026-01-02T03:04:05Z" || metadata.NotAfter != "2027-01-02T03:04:05Z" { t.Fatalf("validity=%q/%q", metadata.NotBefore, metadata.NotAfter) }
	if fmt.Sprint(metadata.DNSNames) != fmt.Sprint([]string{"*.example.test", "irc.example.test"}) { t.Fatalf("dns_names=%v", metadata.DNSNames) }
	if fmt.Sprint(metadata.IPAddresses) != fmt.Sprint([]string{"192.0.2.1", "2001:db8::1"}) { t.Fatalf("ip_addresses=%v", metadata.IPAddresses) }
}

func TestTLSVersionName(t *testing.T) {
	cases := map[uint16]string{tls.VersionTLS12: "TLS 1.2", tls.VersionTLS13: "TLS 1.3", 0x9999: "0x9999"}
	for version, want := range cases { if got := tlsVersionName(version); got != want { t.Fatalf("version %04x: got %q want %q", version, got, want) } }
}

func TestNewRunnerRequiresIdentityAndAllowlist(t *testing.T) {
	if _, err := NewRunner(Identity{}, Policy{}); err == nil { t.Fatal("expected identity validation error") }
	if _, err := NewRunner(Identity{Nick: "n", Username: "u", Contact: "c"}, Policy{}); err == nil { t.Fatal("expected allowlist validation error") }
}

func TestPolicyDenyAndDefaultDeny(t *testing.T) {
	r := testRunner(t, time.Minute)
	_, err := r.Run(context.Background(), Config{Host: "example.org"})
	if code, stage := ErrorInfo(err); code != CodeTargetNotAllowed || stage != "policy" { t.Fatalf("not allowed: code=%q stage=%q err=%v", code, stage, err) }
	_, err = r.Run(context.Background(), Config{Host: "blocked.localhost"})
	if code, stage := ErrorInfo(err); code != CodeTargetDenied || stage != "policy" { t.Fatalf("denied: code=%q stage=%q err=%v", code, stage, err) }
}

func TestRateLimit(t *testing.T) {
	r := testRunner(t, time.Hour)
	now := time.Unix(1000, 0)
	r.now = func() time.Time { return now }
	if err := r.authorize("localhost"); err != nil { t.Fatal(err) }
	err := r.authorize("localhost")
	if code, stage := ErrorInfo(err); code != CodeRateLimited || stage != "policy" { t.Fatalf("rate limit: code=%q stage=%q err=%v", code, stage, err) }
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
