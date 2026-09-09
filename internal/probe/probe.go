package probe

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
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
	Family     string
}

type TLSMetadata struct {
	Version      string   `json:"version,omitempty"`
	CipherSuite  string   `json:"cipher_suite,omitempty"`
	Issuer       string   `json:"issuer,omitempty"`
	Subject      string   `json:"subject,omitempty"`
	NotBefore    string   `json:"not_before,omitempty"`
	NotAfter     string   `json:"not_after,omitempty"`
	DNSNames     []string `json:"dns_names,omitempty"`
	IPAddresses []string `json:"ip_addresses,omitempty"`
}

type Result struct {
	Host              string          `json:"host"`
	Port              string          `json:"port"`
	TLS               bool            `json:"tls"`
	AddressFamily     string          `json:"address_family,omitempty"`
	SelectedAddress   string          `json:"selected_address,omitempty"`
	ResolvedAddresses []string        `json:"resolved_addresses,omitempty"`
	DNSLatencyMS      int64           `json:"dns_latency_ms"`
	ConnectLatencyMS  int64           `json:"connect_latency_ms"`
	TLSLatencyMS      int64           `json:"tls_latency_ms,omitempty"`
	TLSMetadata       *TLSMetadata    `json:"tls_metadata,omitempty"`
	RegistrationMS    int64           `json:"registration_latency_ms"`
	Capabilities      []string        `json:"capabilities,omitempty"`
	Server            string          `json:"server,omitempty"`
	ServerMetadata    *ServerMetadata `json:"server_metadata,omitempty"`
}

func (r *Runner) Run(ctx context.Context, cfg Config) (Result, error) {
	if cfg.Host == "" { return Result{}, probeError(CodeInvalidConfig, "config", errors.New("probe host is required")) }
	if err := r.authorize(cfg.Host); err != nil { return Result{Host: cfg.Host}, err }
	if cfg.Port == "" {
		if cfg.TLS { cfg.Port = "6697" } else { cfg.Port = "6667" }
	}
	if cfg.Timeout <= 0 { cfg.Timeout = 10 * time.Second }
	family, network, err := normalizeFamily(cfg.Family)
	if err != nil { return Result{Host: cfg.Host, Port: cfg.Port, TLS: cfg.TLS}, probeError(CodeInvalidConfig, "config", err) }

	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()
	result := Result{Host: cfg.Host, Port: cfg.Port, TLS: cfg.TLS, AddressFamily: family}

	dnsStart := time.Now()
	ips, err := net.DefaultResolver.LookupHost(ctx, cfg.Host)
	result.DNSLatencyMS = time.Since(dnsStart).Milliseconds()
	if err != nil { return result, probeError(CodeDNSLookupFailed, "dns", err) }
	sort.Strings(ips)
	result.ResolvedAddresses = ips

	selected, err := selectAddress(ips, family)
	if err != nil {
		code := CodeNoAddress
		if family == "ipv4" { code = CodeNoIPv4Address }
		if family == "ipv6" { code = CodeNoIPv6Address }
		return result, probeError(code, "address_selection", err)
	}
	result.SelectedAddress = selected

	addr := net.JoinHostPort(selected, cfg.Port)
	connectStart := time.Now()
	conn, err := (&net.Dialer{}).DialContext(ctx, network, addr)
	result.ConnectLatencyMS = time.Since(connectStart).Milliseconds()
	if err != nil { return result, probeError(CodeTCPConnectFailed, "tcp", err) }
	defer conn.Close()

	if cfg.TLS {
		serverName := cfg.ServerName
		if serverName == "" { serverName = cfg.Host }
		tlsConn := tls.Client(conn, &tls.Config{ServerName: serverName, MinVersion: tls.VersionTLS12})
		tlsStart := time.Now()
		if err := tlsConn.HandshakeContext(ctx); err != nil { return result, probeError(CodeTLSHandshakeFailed, "tls", err) }
		result.TLSLatencyMS = time.Since(tlsStart).Milliseconds()
		metadata := metadataFromTLSState(tlsConn.ConnectionState())
		result.TLSMetadata = &metadata
		conn = tlsConn
	}

	deadline, ok := ctx.Deadline()
	if ok { _ = conn.SetDeadline(deadline) }
	realname := strings.TrimSpace(r.identity.Realname)
	if realname == "" { realname = "IRCIntel network probe" }
	realname += " | contact: " + r.identity.Contact

	registrationStart := time.Now()
	if _, err := fmt.Fprintf(conn, "CAP LS 302\r\nNICK %s\r\nUSER %s 0 * :%s\r\n", r.identity.Nick, r.identity.Username, realname); err != nil {
		return result, probeError(CodeIRCRegistrationWrite, "irc_registration", err)
	}

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 4096), 256*1024)
	caps := map[string]struct{}{}
	capEnded := false
	serverMetadata := ServerMetadata{}
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) > 0 && strings.HasPrefix(parts[0], ":") && result.Server == "" { result.Server = strings.TrimPrefix(parts[0], ":") }
		parseServerSoftware(parts, &serverMetadata)
		parseISupport(parts, &serverMetadata)

		if strings.HasPrefix(line, "PING ") {
			payload := strings.TrimSpace(strings.TrimPrefix(line, "PING"))
			if payload == "" { payload = ":" }
			if _, err := fmt.Fprintf(conn, "PONG %s\r\n", payload); err != nil { return result, probeError(CodeIRCPongWrite, "irc_registration", err) }
			continue
		}

		if capIdx, ok := capLSIndex(parts); ok {
			if idx := strings.Index(line, " :"); idx >= 0 {
				for _, capability := range strings.Fields(line[idx+2:]) { caps[capability] = struct{}{} }
			}
			if !capLSContinues(parts, capIdx) && !capEnded {
				if _, err := fmt.Fprint(conn, "CAP END\r\n"); err != nil { return result, probeError(CodeIRCCapEndWrite, "irc_registration", err) }
				capEnded = true
			}
		}

		if len(parts) >= 2 && parts[1] == "001" {
			result.RegistrationMS = time.Since(registrationStart).Milliseconds()
			_, _ = fmt.Fprint(conn, "QUIT :IRCIntel probe complete\r\n")
			for capability := range caps { result.Capabilities = append(result.Capabilities, capability) }
			sort.Strings(result.Capabilities)
			if serverMetadata.Network != "" || serverMetadata.Software != "" || serverMetadata.SoftwareVersion != "" || len(serverMetadata.ISupport) > 0 {
				result.ServerMetadata = &serverMetadata
			}
			return result, nil
		}
	}
	if err := scanner.Err(); err != nil { return result, probeError(CodeIRCReadFailed, "irc_registration", err) }
	return result, probeError(CodeIRCRegistrationFailed, "irc_registration", errors.New("connection closed before IRC registration completed"))
}

func normalizeFamily(family string) (string, string, error) {
	switch strings.ToLower(strings.TrimSpace(family)) {
	case "", "any", "auto":
		return "any", "tcp", nil
	case "4", "ipv4", "tcp4":
		return "ipv4", "tcp4", nil
	case "6", "ipv6", "tcp6":
		return "ipv6", "tcp6", nil
	default:
		return "", "", fmt.Errorf("unsupported address family %q", family)
	}
}

func selectAddress(addresses []string, family string) (string, error) {
	for _, address := range addresses {
		ip := net.ParseIP(address)
		if ip == nil { continue }
		switch family {
		case "any":
			return address, nil
		case "ipv4":
			if ip.To4() != nil { return address, nil }
		case "ipv6":
			if ip.To4() == nil { return address, nil }
		}
	}
	return "", fmt.Errorf("no %s address available", family)
}

func metadataFromTLSState(state tls.ConnectionState) TLSMetadata {
	metadata := TLSMetadata{Version: tlsVersionName(state.Version), CipherSuite: tls.CipherSuiteName(state.CipherSuite)}
	if len(state.PeerCertificates) == 0 { return metadata }
	populateCertificateMetadata(&metadata, state.PeerCertificates[0])
	return metadata
}

func populateCertificateMetadata(metadata *TLSMetadata, cert *x509.Certificate) {
	metadata.Issuer = cert.Issuer.String()
	metadata.Subject = cert.Subject.String()
	metadata.NotBefore = cert.NotBefore.UTC().Format(time.RFC3339)
	metadata.NotAfter = cert.NotAfter.UTC().Format(time.RFC3339)
	metadata.DNSNames = append([]string(nil), cert.DNSNames...)
	for _, ip := range cert.IPAddresses { metadata.IPAddresses = append(metadata.IPAddresses, ip.String()) }
	sort.Strings(metadata.DNSNames)
	sort.Strings(metadata.IPAddresses)
}

func tlsVersionName(version uint16) string {
	switch version {
	case tls.VersionTLS13: return "TLS 1.3"
	case tls.VersionTLS12: return "TLS 1.2"
	case tls.VersionTLS11: return "TLS 1.1"
	case tls.VersionTLS10: return "TLS 1.0"
	default: return fmt.Sprintf("0x%04x", version)
	}
}

func capLSIndex(parts []string) (int, bool) {
	for i := 0; i+2 < len(parts); i++ {
		if strings.EqualFold(parts[i], "CAP") && strings.EqualFold(parts[i+2], "LS") { return i, true }
	}
	return 0, false
}

func capLSContinues(parts []string, capIdx int) bool {
	return capIdx+3 < len(parts) && parts[capIdx+3] == "*"
}
