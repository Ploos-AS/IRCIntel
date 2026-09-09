package probe

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"
)

type Config struct {
	Host       string
	Port       string
	TLS        bool
	ServerName string
	Timeout    time.Duration
}

type Result struct {
	Host              string   `json:"host"`
	Port              string   `json:"port"`
	TLS               bool     `json:"tls"`
	ResolvedAddresses []string `json:"resolved_addresses,omitempty"`
	DNSLatencyMS      int64    `json:"dns_latency_ms"`
	ConnectLatencyMS  int64    `json:"connect_latency_ms"`
	TLSLatencyMS      int64    `json:"tls_latency_ms,omitempty"`
	RegistrationMS    int64    `json:"registration_latency_ms"`
	Capabilities      []string `json:"capabilities,omitempty"`
	Server            string   `json:"server,omitempty"`
}

func (r *Runner) Run(ctx context.Context, cfg Config) (Result, error) {
	if cfg.Host == "" { return Result{}, errors.New("probe host is required") }
	if err := r.authorize(cfg.Host); err != nil { return Result{Host: cfg.Host}, err }
	if cfg.Port == "" {
		if cfg.TLS { cfg.Port = "6697" } else { cfg.Port = "6667" }
	}
	if cfg.Timeout <= 0 { cfg.Timeout = 10 * time.Second }

	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()
	result := Result{Host: cfg.Host, Port: cfg.Port, TLS: cfg.TLS}

	dnsStart := time.Now()
	ips, err := net.DefaultResolver.LookupHost(ctx, cfg.Host)
	result.DNSLatencyMS = time.Since(dnsStart).Milliseconds()
	if err != nil { return result, fmt.Errorf("dns lookup: %w", err) }
	sort.Strings(ips)
	result.ResolvedAddresses = ips

	addr := net.JoinHostPort(cfg.Host, cfg.Port)
	connectStart := time.Now()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	result.ConnectLatencyMS = time.Since(connectStart).Milliseconds()
	if err != nil { return result, fmt.Errorf("tcp connect: %w", err) }
	defer conn.Close()

	if cfg.TLS {
		serverName := cfg.ServerName
		if serverName == "" { serverName = cfg.Host }
		tlsConn := tls.Client(conn, &tls.Config{ServerName: serverName, MinVersion: tls.VersionTLS12})
		tlsStart := time.Now()
		if err := tlsConn.HandshakeContext(ctx); err != nil { return result, fmt.Errorf("tls handshake: %w", err) }
		result.TLSLatencyMS = time.Since(tlsStart).Milliseconds()
		conn = tlsConn
	}

	deadline, ok := ctx.Deadline()
	if ok { _ = conn.SetDeadline(deadline) }
	realname := strings.TrimSpace(r.identity.Realname)
	if realname == "" { realname = "IRCIntel network probe" }
	realname += " | contact: " + r.identity.Contact

	registrationStart := time.Now()
	if _, err := fmt.Fprintf(conn, "CAP LS 302\r\nNICK %s\r\nUSER %s 0 * :%s\r\n", r.identity.Nick, r.identity.Username, realname); err != nil {
		return result, fmt.Errorf("irc registration write: %w", err)
	}

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 4096), 256*1024)
	caps := map[string]struct{}{}
	capEnded := false
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) > 0 && strings.HasPrefix(parts[0], ":") && result.Server == "" { result.Server = strings.TrimPrefix(parts[0], ":") }

		if strings.HasPrefix(line, "PING ") {
			payload := strings.TrimSpace(strings.TrimPrefix(line, "PING"))
			if payload == "" { payload = ":" }
			if _, err := fmt.Fprintf(conn, "PONG %s\r\n", payload); err != nil { return result, fmt.Errorf("irc pong write: %w", err) }
			continue
		}

		if isCAPLS(parts) {
			if idx := strings.Index(line, " :"); idx >= 0 {
				for _, capability := range strings.Fields(line[idx+2:]) { caps[capability] = struct{}{} }
			}
			if !capLSContinues(parts) && !capEnded {
				if _, err := fmt.Fprint(conn, "CAP END\r\n"); err != nil { return result, fmt.Errorf("irc cap end write: %w", err) }
				capEnded = true
			}
		}

		if len(parts) >= 2 && parts[1] == "001" {
			result.RegistrationMS = time.Since(registrationStart).Milliseconds()
			_, _ = fmt.Fprint(conn, "QUIT :IRCIntel probe complete\r\n")
			for capability := range caps { result.Capabilities = append(result.Capabilities, capability) }
			sort.Strings(result.Capabilities)
			return result, nil
		}
	}
	if err := scanner.Err(); err != nil { return result, fmt.Errorf("irc read: %w", err) }
	return result, errors.New("connection closed before IRC registration completed")
}

func isCAPLS(parts []string) bool {
	for i := 0; i+1 < len(parts); i++ {
		if strings.EqualFold(parts[i], "CAP") && strings.EqualFold(parts[i+1], "LS") { return true }
	}
	return false
}

func capLSContinues(parts []string) bool {
	for i := 0; i+2 < len(parts); i++ {
		if strings.EqualFold(parts[i], "CAP") && strings.EqualFold(parts[i+1], "LS") {
			return parts[i+2] == "*"
		}
	}
	return false
}
