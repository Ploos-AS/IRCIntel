package probe

import (
	"context"
	"errors"
	"net"
	"reflect"
	"testing"
)

func TestAddressesForFamily(t *testing.T) {
	addresses := []string{"192.0.2.10", "2001:db8::10", "192.0.2.20", "invalid"}
	if got, want := addressesForFamily(addresses, "ipv4"), []string{"192.0.2.10", "192.0.2.20"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ipv4=%v want=%v", got, want)
	}
	if got, want := addressesForFamily(addresses, "ipv6"), []string{"2001:db8::10"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ipv6=%v want=%v", got, want)
	}
}

func TestDialCandidatesFallsBackToNextAddress(t *testing.T) {
	client, peer := net.Pipe()
	defer peer.Close()

	var attempts []string
	dial := func(_ context.Context, network, address string) (net.Conn, error) {
		attempts = append(attempts, network+" "+address)
		if len(attempts) == 1 {
			return nil, errors.New("synthetic first-address failure")
		}
		return client, nil
	}

	conn, selected, err := dialCandidates(context.Background(), "tcp4", "6697", []string{"192.0.2.10", "192.0.2.20"}, dial)
	if err != nil { t.Fatal(err) }
	defer conn.Close()
	if selected != "192.0.2.20" { t.Fatalf("selected=%q", selected) }
	want := []string{"tcp4 192.0.2.10:6697", "tcp4 192.0.2.20:6697"}
	if !reflect.DeepEqual(attempts, want) { t.Fatalf("attempts=%v want=%v", attempts, want) }
}

func TestDialCandidatesReturnsLastFailure(t *testing.T) {
	first := errors.New("first")
	last := errors.New("last")
	attempt := 0
	dial := func(_ context.Context, _, _ string) (net.Conn, error) {
		attempt++
		if attempt == 1 { return nil, first }
		return nil, last
	}

	conn, selected, err := dialCandidates(context.Background(), "tcp6", "6697", []string{"2001:db8::10", "2001:db8::20"}, dial)
	if conn != nil || selected != "" { t.Fatalf("conn=%v selected=%q", conn, selected) }
	if !errors.Is(err, last) { t.Fatalf("err=%v want last failure", err) }
}
