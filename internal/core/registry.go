package core

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
)

var (
	ErrRegistryNetworkNotFound = errors.New("registry network does not exist")
	ErrRegistryServerNotFound  = errors.New("registry server does not exist")
)

type Network struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Website     string `json:"website,omitempty"`
	Description string `json:"description,omitempty"`
}

type NetworkServer struct {
	ID        string `json:"id"`
	NetworkID string `json:"network_id"`
	Name      string `json:"name"`
}

type NetworkEndpoint struct {
	ID       string `json:"id"`
	ServerID string `json:"server_id"`
	Host     string `json:"host"`
	Port     string `json:"port"`
	TLS      bool   `json:"tls"`
}

type RegistrySnapshot struct {
	Networks  []Network         `json:"networks"`
	Servers   []NetworkServer   `json:"servers"`
	Endpoints []NetworkEndpoint `json:"endpoints"`
}

type RegistryReader interface {
	RegistrySnapshot() (RegistrySnapshot, error)
}

type RegistryHandler struct {
	Token  string
	Reader RegistryReader
}

func (h RegistryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.Reader == nil {
		http.Error(w, "registry reader unavailable", http.StatusServiceUnavailable)
		return
	}
	if h.Token != "" && r.Header.Get("Authorization") != "Bearer "+h.Token {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	snapshot, err := h.Reader.RegistrySnapshot()
	if err != nil {
		http.Error(w, "registry query failed", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(snapshot)
}

func (s *SQLiteStore) UpsertNetwork(network Network) error {
	if s == nil || s.db == nil {
		return errors.New("sqlite store is not open")
	}
	network.ID = strings.TrimSpace(network.ID)
	network.Name = strings.TrimSpace(network.Name)
	if network.ID == "" || network.Name == "" {
		return errors.New("network id and name are required")
	}
	_, err := s.db.Exec(`
INSERT INTO networks (id, name, website, description)
VALUES (?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    name = excluded.name,
    website = excluded.website,
    description = excluded.description`,
		network.ID, network.Name, strings.TrimSpace(network.Website), strings.TrimSpace(network.Description))
	return err
}

func (s *SQLiteStore) UpsertNetworkServer(server NetworkServer) error {
	if s == nil || s.db == nil {
		return errors.New("sqlite store is not open")
	}
	server.ID = strings.TrimSpace(server.ID)
	server.NetworkID = strings.TrimSpace(server.NetworkID)
	server.Name = strings.TrimSpace(server.Name)
	if server.ID == "" || server.NetworkID == "" || server.Name == "" {
		return errors.New("server id, network id, and name are required")
	}
	var exists int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM networks WHERE id = ?`, server.NetworkID).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return ErrRegistryNetworkNotFound
	}
	_, err := s.db.Exec(`
INSERT INTO network_servers (id, network_id, name)
VALUES (?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    network_id = excluded.network_id,
    name = excluded.name`, server.ID, server.NetworkID, server.Name)
	return err
}

func (s *SQLiteStore) UpsertNetworkEndpoint(endpoint NetworkEndpoint) error {
	if s == nil || s.db == nil {
		return errors.New("sqlite store is not open")
	}
	endpoint.ID = strings.TrimSpace(endpoint.ID)
	endpoint.ServerID = strings.TrimSpace(endpoint.ServerID)
	endpoint.Host = strings.ToLower(strings.TrimSpace(endpoint.Host))
	endpoint.Port = strings.TrimSpace(endpoint.Port)
	if endpoint.ID == "" || endpoint.ServerID == "" || endpoint.Host == "" || endpoint.Port == "" {
		return errors.New("endpoint id, server id, host, and port are required")
	}
	var exists int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM network_servers WHERE id = ?`, endpoint.ServerID).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return ErrRegistryServerNotFound
	}
	_, err := s.db.Exec(`
INSERT INTO network_endpoints (id, server_id, host, port, tls)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    server_id = excluded.server_id,
    host = excluded.host,
    port = excluded.port,
    tls = excluded.tls`, endpoint.ID, endpoint.ServerID, endpoint.Host, endpoint.Port, endpoint.TLS)
	return err
}

func (s *SQLiteStore) RegistrySnapshot() (RegistrySnapshot, error) {
	if s == nil || s.db == nil {
		return RegistrySnapshot{}, errors.New("sqlite store is not open")
	}
	snapshot := RegistrySnapshot{
		Networks:  make([]Network, 0),
		Servers:   make([]NetworkServer, 0),
		Endpoints: make([]NetworkEndpoint, 0),
	}

	rows, err := s.db.Query(`SELECT id, name, website, description FROM networks ORDER BY name COLLATE NOCASE, id`)
	if err != nil {
		return RegistrySnapshot{}, err
	}
	for rows.Next() {
		var item Network
		if err := rows.Scan(&item.ID, &item.Name, &item.Website, &item.Description); err != nil {
			_ = rows.Close()
			return RegistrySnapshot{}, err
		}
		snapshot.Networks = append(snapshot.Networks, item)
	}
	if err := rows.Close(); err != nil {
		return RegistrySnapshot{}, err
	}

	rows, err = s.db.Query(`SELECT id, network_id, name FROM network_servers ORDER BY network_id, name COLLATE NOCASE, id`)
	if err != nil {
		return RegistrySnapshot{}, err
	}
	for rows.Next() {
		var item NetworkServer
		if err := rows.Scan(&item.ID, &item.NetworkID, &item.Name); err != nil {
			_ = rows.Close()
			return RegistrySnapshot{}, err
		}
		snapshot.Servers = append(snapshot.Servers, item)
	}
	if err := rows.Close(); err != nil {
		return RegistrySnapshot{}, err
	}

	rows, err = s.db.Query(`SELECT id, server_id, host, port, tls FROM network_endpoints ORDER BY server_id, host, port, tls, id`)
	if err != nil {
		return RegistrySnapshot{}, err
	}
	for rows.Next() {
		var item NetworkEndpoint
		if err := rows.Scan(&item.ID, &item.ServerID, &item.Host, &item.Port, &item.TLS); err != nil {
			_ = rows.Close()
			return RegistrySnapshot{}, err
		}
		snapshot.Endpoints = append(snapshot.Endpoints, item)
	}
	if err := rows.Close(); err != nil {
		return RegistrySnapshot{}, err
	}

	sort.SliceStable(snapshot.Endpoints, func(i, j int) bool {
		if snapshot.Endpoints[i].ServerID != snapshot.Endpoints[j].ServerID {
			return snapshot.Endpoints[i].ServerID < snapshot.Endpoints[j].ServerID
		}
		if snapshot.Endpoints[i].Host != snapshot.Endpoints[j].Host {
			return snapshot.Endpoints[i].Host < snapshot.Endpoints[j].Host
		}
		if snapshot.Endpoints[i].Port != snapshot.Endpoints[j].Port {
			return snapshot.Endpoints[i].Port < snapshot.Endpoints[j].Port
		}
		return !snapshot.Endpoints[i].TLS && snapshot.Endpoints[j].TLS
	})
	return snapshot, nil
}
