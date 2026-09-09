package main

import "testing"

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
