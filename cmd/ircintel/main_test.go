package main

import (
	"testing"
	"time"
)

func TestGetenvFallback(t *testing.T) {
	t.Setenv("IRCINTEL_TEST_VALUE", "")
	if got := getenv("IRCINTEL_TEST_VALUE", "fallback"); got != "fallback" {
		t.Fatalf("got %q, want fallback", got)
	}
}

func TestGetenvValue(t *testing.T) {
	t.Setenv("IRCINTEL_TEST_VALUE", "configured")
	if got := getenv("IRCINTEL_TEST_VALUE", "fallback"); got != "configured" {
		t.Fatalf("got %q, want configured", got)
	}
}

func TestGetenvDurationFallback(t *testing.T) {
	t.Setenv("IRCINTEL_TEST_DURATION", "")
	got, err := getenvDuration("IRCINTEL_TEST_DURATION", 15*time.Minute)
	if err != nil { t.Fatal(err) }
	if got != 15*time.Minute { t.Fatalf("got %s, want 15m", got) }
}

func TestGetenvDurationValue(t *testing.T) {
	t.Setenv("IRCINTEL_TEST_DURATION", "7m30s")
	got, err := getenvDuration("IRCINTEL_TEST_DURATION", 15*time.Minute)
	if err != nil { t.Fatal(err) }
	if got != 7*time.Minute+30*time.Second { t.Fatalf("got %s, want 7m30s", got) }
}

func TestGetenvDurationRejectsInvalid(t *testing.T) {
	for _, raw := range []string{"not-a-duration", "0s", "-1m"} {
		t.Run(raw, func(t *testing.T) {
			t.Setenv("IRCINTEL_TEST_DURATION", raw)
			if _, err := getenvDuration("IRCINTEL_TEST_DURATION", 15*time.Minute); err == nil {
				t.Fatalf("expected error for %q", raw)
			}
		})
	}
}
