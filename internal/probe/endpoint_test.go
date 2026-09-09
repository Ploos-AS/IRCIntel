package probe

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

func TestRunEndpointMixedFamilyResult(t *testing.T) {
	port, done := serveIRC(t, "tcp4", "127.0.0.1:0")
	r := testRunner(t, time.Millisecond)
	result, err := r.RunEndpoint(context.Background(), Config{
		Host:    "localhost",
		Port:    port,
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Measurements) != 2 {
		t.Fatalf("measurements=%d", len(result.Measurements))
	}
	if result.Measurements[0].Family != "ipv4" || !result.Measurements[0].OK {
		t.Fatalf("ipv4=%+v", result.Measurements[0])
	}
	if result.Measurements[0].Result == nil || net.ParseIP(result.Measurements[0].Result.SelectedAddress).To4() == nil {
		t.Fatalf("ipv4 result=%+v", result.Measurements[0].Result)
	}
	if result.Measurements[1].Family != "ipv6" || result.Measurements[1].OK {
		t.Fatalf("ipv6=%+v", result.Measurements[1])
	}
	if result.Measurements[1].Error == "" {
		t.Fatal("expected structured ipv6 error")
	}
	if result.Measurements[1].ErrorCode != CodeTCPConnectFailed && result.Measurements[1].ErrorCode != CodeNoIPv6Address {
		t.Fatalf("unexpected ipv6 error code: %+v", result.Measurements[1])
	}
	if result.Measurements[1].ErrorStage == "" {
		t.Fatalf("missing ipv6 error stage: %+v", result.Measurements[1])
	}
	if !result.Reachable {
		t.Fatal("endpoint should be reachable when one family succeeds")
	}
	if result.DualStackOK {
		t.Fatal("dual stack must be false when one family fails")
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestRunEndpointConsumesOneParentRateLimitSlot(t *testing.T) {
	port, done := serveIRC(t, "tcp4", "127.0.0.1:0")
	r := testRunner(t, time.Hour)
	if _, err := r.RunEndpoint(context.Background(), Config{Host: "localhost", Port: port, Timeout: 2 * time.Second}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.RunEndpoint(context.Background(), Config{Host: "localhost", Port: port, Timeout: 100 * time.Millisecond}); err == nil || !strings.Contains(err.Error(), "rate limited") {
		t.Fatalf("expected parent rate limit, got %v", err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
