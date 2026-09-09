package probe

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestErrorInfo(t *testing.T) {
	err := probeError(CodeTLSHandshakeFailed, "tls", errors.New("boom"))
	code, stage := ErrorInfo(err)
	if code != CodeTLSHandshakeFailed || stage != "tls" {
		t.Fatalf("got %q/%q", code, stage)
	}
	code, stage = ErrorInfo(errors.New("plain"))
	if code != "unknown" || stage != "unknown" {
		t.Fatalf("plain error got %q/%q", code, stage)
	}
}

func TestRunNoIPv6AddressCode(t *testing.T) {
	_, err := testRunner(t, time.Millisecond).Run(context.Background(), Config{Host: "127.0.0.1", Port: "1", Family: "ipv6", Timeout: time.Second})
	if err == nil {
		t.Fatal("expected error")
	}
	code, stage := ErrorInfo(err)
	if code != CodeNoIPv6Address || stage != "address_selection" {
		t.Fatalf("got %q/%q: %v", code, stage, err)
	}
}

func TestRunTCPConnectFailureCode(t *testing.T) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil { t.Fatal(err) }
	port := fmtPort(ln.Addr().(*net.TCPAddr).Port)
	_ = ln.Close()

	_, err = testRunner(t, time.Millisecond).Run(context.Background(), Config{Host: "127.0.0.1", Port: port, Family: "ipv4", Timeout: time.Second})
	if err == nil {
		t.Fatal("expected error")
	}
	code, stage := ErrorInfo(err)
	if code != CodeTCPConnectFailed || stage != "tcp" {
		t.Fatalf("got %q/%q: %v", code, stage, err)
	}
}
