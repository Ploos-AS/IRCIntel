package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Ploos-AS/IRCIntel/internal/probe"
)

func main() {
	host := flag.String("host", "", "IRC host to probe (required)")
	port := flag.String("port", "6697", "IRC port")
	useTLS := flag.Bool("tls", true, "use TLS")
	serverName := flag.String("server-name", "", "TLS server name (defaults to host)")
	timeout := flag.Duration("timeout", 15*time.Second, "overall probe timeout")
	contact := flag.String("contact", "https://github.com/Ploos-AS/IRCIntel", "operator/contact URL or address")
	flag.Parse()

	if *host == "" {
		fmt.Fprintln(os.Stderr, "-host is required")
		os.Exit(2)
	}

	runner, err := probe.NewRunner(probe.Identity{
		Nick:     "IRCIntelProbe",
		Username: "ircintel",
		Realname: "IRCIntel public endpoint qualification",
		Contact:  *contact,
	}, probe.Policy{
		AllowHosts:  []string{*host},
		MinInterval: 5 * time.Minute,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	result, err := runner.Run(context.Background(), probe.Config{
		Host:       *host,
		Port:       *port,
		TLS:        *useTLS,
		ServerName: *serverName,
		Timeout:    *timeout,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
