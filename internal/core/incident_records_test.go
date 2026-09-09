package core

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/Ploos-AS/IRCIntel/internal/agent"
	"github.com/Ploos-AS/IRCIntel/internal/probe"
)

func TestIncidentRecordsPersistAndClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ircintel.db")
	store, err := OpenSQLiteStore(path)
	if err != nil { t.Fatal(err) }

	base := time.Date(2026, 9, 9, 7, 0, 0, 0, time.UTC)
	items := []agent.Observation{
		{AgentID: "oslo-1", ObservedAt: base, Endpoint: agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: true}},
		{AgentID: "ams-1", ObservedAt: base, Endpoint: agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: true}},
		{AgentID: "oslo-1", ObservedAt: base.Add(time.Minute), Endpoint: agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: false}},
		{AgentID: "ams-1", ObservedAt: base.Add(2 * time.Minute), Endpoint: agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: false}},
	}
	for _, item := range items {
		if err := store.Store(item); err != nil { t.Fatal(err) }
	}

	records, err := store.ListIncidentRecords(10)
	if err != nil { t.Fatal(err) }
	if len(records) != 1 || records[0].Status != "open" { t.Fatalf("records=%+v", records) }

	for _, item := range []agent.Observation{
		{AgentID: "oslo-1", ObservedAt: base.Add(10 * time.Minute), Endpoint: agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: true}},
		{AgentID: "ams-1", ObservedAt: base.Add(11 * time.Minute), Endpoint: agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: true}},
	} {
		if err := store.Store(item); err != nil { t.Fatal(err) }
	}

	records, err = store.ListIncidentRecords(10)
	if err != nil { t.Fatal(err) }
	if len(records) != 1 || records[0].Status != "closed" || records[0].DurationSeconds == nil {
		t.Fatalf("records=%+v", records)
	}
	if err := store.Close(); err != nil { t.Fatal(err) }

	store, err = OpenSQLiteStore(path)
	if err != nil { t.Fatal(err) }
	defer store.Close()
	records, err = store.ListIncidentRecords(10)
	if err != nil { t.Fatal(err) }
	if len(records) != 1 || records[0].Status != "closed" {
		t.Fatalf("reopened records=%+v", records)
	}
}
