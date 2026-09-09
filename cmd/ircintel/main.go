package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Ploos-AS/IRCIntel/internal/core"
)

var version = "dev"

type versionResponse struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func main() {
	listen := getenv("IRCINTEL_LISTEN", ":8080")
	dbPath := getenv("IRCINTEL_DB_PATH", "/data/ircintel.db")
	store, err := core.OpenSQLiteStore(dbPath)
	if err != nil {
		log.Fatalf("open observation store: %v", err)
	}
	defer store.Close()
	token := os.Getenv("IRCINTEL_CORE_TOKEN")
	ingest := core.IngestHandler{Token: token, Store: store}
	read := core.ReadHandler{Token: token, Reader: store}
	status := core.StatusHandler{Token: token, Reader: store}
	incidents := core.IncidentHandler{Token: token, Reader: store}
	distributedIncidents := core.DistributedIncidentHandler{Token: token, Reader: store}
	incidentLifecycle := core.IncidentLifecycleHandler{Token: token, Reader: store}
	registry := core.RegistryHandler{Token: token, Reader: store}
	registryWrite := core.RegistryWriteHandler{Token: token, Writer: store}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("GET /api/v1/version", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(versionResponse{Name: "IRCIntel", Version: version})
	})
	mux.Handle("GET /api/v1/observations", read)
	mux.Handle("POST /api/v1/observations", ingest)
	mux.Handle("GET /api/v1/endpoints/status", status)
	mux.Handle("GET /api/v1/incidents", incidents)
	mux.Handle("GET /api/v1/incidents/distributed", distributedIncidents)
	mux.Handle("GET /api/v1/incidents/lifecycle", incidentLifecycle)
	mux.Handle("GET /api/v1/registry", registry)
	mux.HandleFunc("POST /api/v1/registry/networks", registryWrite.Network)
	mux.HandleFunc("POST /api/v1/registry/servers", registryWrite.Server)
	mux.HandleFunc("POST /api/v1/registry/endpoints", registryWrite.Endpoint)

	srv := &http.Server{
		Addr:              listen,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("IRCIntel %s listening on %s", version, listen)
	log.Fatal(srv.ListenAndServe())
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
