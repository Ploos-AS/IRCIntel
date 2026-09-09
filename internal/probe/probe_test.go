package probe

import (
	"context"
	"net"
	"testing"
	"time"
)

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
	result, err := Run(context.Background(), Config{Host: "localhost", Port: fmtPort(port), Nick: "IRCIntelProbe", Timeout: 2 * time.Second})
	if err != nil { t.Fatal(err) }
	if result.Server != "test.example" { t.Fatalf("server=%q", result.Server) }
	if len(result.Capabilities) != 3 { t.Fatalf("capabilities=%v", result.Capabilities) }
	if err := <-done; err != nil { t.Fatal(err) }
}

func TestRunRequiresHost(t *testing.T) {
	if _, err := Run(context.Background(), Config{}); err == nil { t.Fatal("expected host validation error") }
}

func fmtPort(port int) string {
	const digits = "0123456789"
	if port == 0 { return "0" }
	var b [20]byte
	i := len(b)
	for port > 0 { i--; b[i] = digits[port%10]; port /= 10 }
	return string(b[i:])
}
