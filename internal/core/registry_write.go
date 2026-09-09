package core

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

const maxRegistryWriteBytes int64 = 64 * 1024

type RegistryWriter interface {
	UpsertNetwork(Network) error
	UpsertNetworkServer(NetworkServer) error
	UpsertNetworkEndpoint(NetworkEndpoint) error
}

type RegistryWriteHandler struct {
	Token  string
	Writer RegistryWriter
}

func (h RegistryWriteHandler) Network(w http.ResponseWriter, r *http.Request) {
	var item Network
	if !h.authorize(w, r) || !decodeRegistryWrite(w, r, &item) {
		return
	}
	item.ID = strings.TrimSpace(item.ID)
	item.Name = strings.TrimSpace(item.Name)
	item.Website = strings.TrimSpace(item.Website)
	item.Description = strings.TrimSpace(item.Description)
	if item.ID == "" || item.Name == "" {
		http.Error(w, "network id and name are required", http.StatusBadRequest)
		return
	}
	if err := h.Writer.UpsertNetwork(item); err != nil {
		http.Error(w, "registry write failed", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h RegistryWriteHandler) Server(w http.ResponseWriter, r *http.Request) {
	var item NetworkServer
	if !h.authorize(w, r) || !decodeRegistryWrite(w, r, &item) {
		return
	}
	item.ID = strings.TrimSpace(item.ID)
	item.NetworkID = strings.TrimSpace(item.NetworkID)
	item.Name = strings.TrimSpace(item.Name)
	if item.ID == "" || item.NetworkID == "" || item.Name == "" {
		http.Error(w, "server id, network id, and name are required", http.StatusBadRequest)
		return
	}
	if err := h.Writer.UpsertNetworkServer(item); err != nil {
		if strings.Contains(err.Error(), "network does not exist") {
			http.Error(w, "network does not exist", http.StatusConflict)
			return
		}
		http.Error(w, "registry write failed", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h RegistryWriteHandler) Endpoint(w http.ResponseWriter, r *http.Request) {
	var item NetworkEndpoint
	if !h.authorize(w, r) || !decodeRegistryWrite(w, r, &item) {
		return
	}
	item.ID = strings.TrimSpace(item.ID)
	item.ServerID = strings.TrimSpace(item.ServerID)
	item.Host = strings.ToLower(strings.TrimSpace(item.Host))
	item.Port = strings.TrimSpace(item.Port)
	if item.ID == "" || item.ServerID == "" || item.Host == "" || item.Port == "" {
		http.Error(w, "endpoint id, server id, host, and port are required", http.StatusBadRequest)
		return
	}
	if err := h.Writer.UpsertNetworkEndpoint(item); err != nil {
		if strings.Contains(err.Error(), "server does not exist") {
			http.Error(w, "server does not exist", http.StatusConflict)
			return
		}
		http.Error(w, "registry write failed", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h RegistryWriteHandler) authorize(w http.ResponseWriter, r *http.Request) bool {
	if h.Writer == nil {
		http.Error(w, "registry writer unavailable", http.StatusServiceUnavailable)
		return false
	}
	// Registry mutation is an administrative operation. Unlike read APIs, it is
	// deliberately disabled when no Core token has been configured.
	if strings.TrimSpace(h.Token) == "" {
		http.Error(w, "registry writes require IRCINTEL_CORE_TOKEN", http.StatusServiceUnavailable)
		return false
	}
	if r.Header.Get("Authorization") != "Bearer "+h.Token {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

func decodeRegistryWrite(w http.ResponseWriter, r *http.Request, dst any) bool {
	body := http.MaxBytesReader(w, r.Body, maxRegistryWriteBytes)
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		http.Error(w, "invalid registry payload", http.StatusBadRequest)
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		http.Error(w, "registry payload must contain one JSON object", http.StatusBadRequest)
		return false
	}
	return true
}
