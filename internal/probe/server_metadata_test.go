package probe

import (
	"bufio"
	"context"
	"fmt"
	"testing"
	"time"
)

func TestServerMetadataParsers(t *testing.T) {
	metadata := ServerMetadata{}
	parseServerSoftware([]string{":irc.example", "004", "irc.example", "ircd-2.0", "io", "k", "beI"}, &metadata)
	parseISupport([]string{":irc.example", "005", "nick", "NETWORK=ExampleNet", "CHANTYPES=#&", "PREFIX=(ov)@+", ":are supported"}, &metadata)

	if metadata.Software != "irc.example" || metadata.SoftwareVersion != "ircd-2.0" {
		t.Fatalf("software=%q version=%q", metadata.Software, metadata.SoftwareVersion)
	}
	if metadata.Network != "ExampleNet" {
		t.Fatalf("network=%q", metadata.Network)
	}
	if metadata.ISupport["CHANTYPES"] != "#&" || metadata.ISupport["PREFIX"] != "(ov)@+" {
		t.Fatalf("isupport=%v", metadata.ISupport)
	}
	want := []string{"CHANTYPES", "NETWORK", "PREFIX"}
	got := sortedISupportKeys(metadata.ISupport)
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("keys=%v want=%v", got, want)
	}
}

func TestRunCapturesServerMetadata(t *testing.T) {
	port, done := serveOnce(t, "tcp4", "127.0.0.1:0", func(conn net.Conn) error {
		r := bufio.NewReader(conn)
		if err := readUntil(r, "USER ircintel 0 * :IRCIntel test probe | contact: https://example.invalid/ircintel"); err != nil { return err }
		if _, err := fmt.Fprint(conn, ":irc.example CAP IRCIntelProbe LS :multi-prefix sasl\r\n"); err != nil { return err }
		if err := readUntil(r, "CAP END"); err != nil { return err }
		if _, err := fmt.Fprint(conn, ":irc.example 004 IRCIntelProbe irc.example ircd-2.0 io k beI\r\n"); err != nil { return err }
		if _, err := fmt.Fprint(conn, ":irc.example 005 IRCIntelProbe NETWORK=ExampleNet CHANTYPES=#& PREFIX=(ov)@+ :are supported\r\n"); err != nil { return err }
		_, err := fmt.Fprint(conn, ":irc.example 001 IRCIntelProbe :welcome\r\n")
		return err
	})

	result, err := testRunner(t, time.Millisecond).Run(context.Background(), Config{Host: "localhost", Port: port, Family: "ipv4", Timeout: 2 * time.Second})
	if err != nil { t.Fatal(err) }
	if result.ServerMetadata == nil { t.Fatal("expected server metadata") }
	if result.ServerMetadata.Network != "ExampleNet" { t.Fatalf("network=%q", result.ServerMetadata.Network) }
	if result.ServerMetadata.Software != "irc.example" || result.ServerMetadata.SoftwareVersion != "ircd-2.0" {
		t.Fatalf("software=%q version=%q", result.ServerMetadata.Software, result.ServerMetadata.SoftwareVersion)
	}
	if result.ServerMetadata.ISupport["CHANTYPES"] != "#&" { t.Fatalf("isupport=%v", result.ServerMetadata.ISupport) }
	if err := <-done; err != nil { t.Fatal(err) }
}
