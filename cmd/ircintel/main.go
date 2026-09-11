package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Ploos-AS/IRCIntel/internal/core"
)

var version = "dev"

const defaultStatusFreshness = 15 * time.Minute
const defaultMaintenanceInterval = 24 * time.Hour

type versionResponse struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type storageConfig struct {
	Backend     string
	SQLitePath  string
	DatabaseURL string
}

type maintenanceConfig struct {
	Enabled       bool
	Interval      time.Duration
	RawRetention  time.Duration
	HourlyRetention time.Duration
}

func main() {
	listen := getenv("IRCINTEL_LISTEN", ":8080")
	statusFreshness, err := getenvDuration("IRCINTEL_STATUS_FRESHNESS", defaultStatusFreshness)
	if err != nil { log.Fatalf("configure status freshness: %v", err) }
	storage, err := storageConfigFromEnv()
	if err != nil { log.Fatalf("configure storage: %v", err) }
	maintenanceCfg, err := maintenanceConfigFromEnv()
	if err != nil { log.Fatalf("configure maintenance: %v", err) }
	store, err := openStorage(storage)
	if err != nil { log.Fatalf("open observation store: %v", err) }
	defer store.Close()
	token := os.Getenv("IRCINTEL_CORE_TOKEN")
	ingest := core.IngestHandler{Token: token, Store: store}
	read := core.ReadHandler{Token: token, Reader: store}
	status := core.StatusHandler{Token: token, Reader: store, Freshness: statusFreshness}
	networkStatus := core.NetworkStatusHandler{Token: token, Reader: store, Freshness: statusFreshness}
	networkIncidents := core.NetworkIncidentHandler{Token: token, Reader: store}
	networkIncidentStats := core.NetworkIncidentStatsHandler{Token: token, Reader: store}
	networkIncidentStatsByNetwork := core.NetworkIncidentStatsByNetworkHandler{Token: token, Reader: store}
	networkReliability := core.NetworkReliabilityHandler{Token: token, Reader: store}
	incidents := core.IncidentHandler{Token: token, Reader: store}
	distributedIncidents := core.DistributedIncidentHandler{Token: token, Reader: store}
	incidentLifecycle := core.IncidentLifecycleHandler{Token: token, Reader: store}
	registry := core.RegistryHandler{Token: token, Reader: store}
	registryWrite := core.RegistryWriteHandler{Token: token, Writer: store}
	discovery := core.DiscoveryHandler{Token: token, Store: store}
	discoveryReview := core.DiscoveryReviewHandler{Token: token, Store: store}
	discoveryPromotion := core.DiscoveryPromotionHandler{Token: token, Store: store}
	probePlan := core.ProbePlanHandler{Token: token, Reader: store}
	var historicalReader core.HistoricalStatsReader
	if reader, ok := store.(core.HistoricalStatsReader); ok { historicalReader = reader }
	historicalStats := core.HistoricalStatsHandler{Token: token, Reader: historicalReader}
	var networkHistoricalReader core.NetworkHistoricalStatsReader
	if reader, ok := store.(core.NetworkHistoricalStatsReader); ok { networkHistoricalReader = reader }
	networkHistoricalStats := core.NetworkHistoricalStatsHandler{Token: token, Reader: networkHistoricalReader}
	var networkRankingReader core.NetworkHistoryRankingReader
	if reader, ok := store.(core.NetworkHistoryRankingReader); ok { networkRankingReader = reader }
	networkHistoryRanking := core.NetworkHistoryRankingHandler{Token: token, Reader: networkRankingReader}

	var maintenanceProvider core.MaintenanceStatusProvider = core.StaticMaintenanceStatus(core.DisabledMaintenanceStatus())
	if maintenanceCfg.Enabled {
		runner, ok := store.(core.MaintenanceRunner)
		if !ok { log.Fatalf("maintenance requires a storage backend with retention maintenance support") }
		scheduler, err := core.NewMaintenanceScheduler(runner, core.RetentionPolicy{RawObservations: maintenanceCfg.RawRetention, HourlyRollups: maintenanceCfg.HourlyRetention}, maintenanceCfg.Interval)
		if err != nil { log.Fatalf("configure maintenance scheduler: %v", err) }
		maintenanceProvider = scheduler
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go scheduler.Run(ctx)
		log.Printf("maintenance enabled interval=%s raw_retention=%s hourly_retention=%s", maintenanceCfg.Interval, maintenanceCfg.RawRetention, maintenanceCfg.HourlyRetention)
	}
	maintenanceStatus := core.MaintenanceStatusHandler{Token: token, Provider: maintenanceProvider}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.Header().Set("Content-Type", "text/plain; charset=utf-8"); w.WriteHeader(http.StatusOK); _, _ = w.Write([]byte("ok\n")) })
	mux.HandleFunc("GET /api/v1/version", func(w http.ResponseWriter, _ *http.Request) { w.Header().Set("Content-Type", "application/json"); _ = json.NewEncoder(w).Encode(versionResponse{Name: "IRCIntel", Version: version}) })
	mux.Handle("GET /api/v1/maintenance/status", maintenanceStatus)
	mux.Handle("GET /api/v1/observations", read)
	mux.Handle("POST /api/v1/observations", ingest)
	mux.Handle("GET /api/v1/history/observations", historicalStats)
	mux.Handle("GET /api/v1/history/networks", networkHistoricalStats)
	mux.Handle("GET /api/v1/history/networks/ranking", networkHistoryRanking)
	mux.Handle("GET /api/v1/endpoints/status", status)
	mux.Handle("GET /api/v1/networks/status", networkStatus)
	mux.Handle("GET /api/v1/networks/incidents", networkIncidents)
	mux.Handle("GET /api/v1/networks/incidents/stats", networkIncidentStats)
	mux.Handle("GET /api/v1/networks/incidents/stats/by-network", networkIncidentStatsByNetwork)
	mux.Handle("GET /api/v1/networks/reliability", networkReliability)
	mux.Handle("GET /api/v1/incidents", incidents)
	mux.Handle("GET /api/v1/incidents/distributed", distributedIncidents)
	mux.Handle("GET /api/v1/incidents/lifecycle", incidentLifecycle)
	mux.Handle("GET /api/v1/registry", registry)
	mux.HandleFunc("POST /api/v1/registry/networks", registryWrite.Network)
	mux.HandleFunc("POST /api/v1/registry/servers", registryWrite.Server)
	mux.HandleFunc("POST /api/v1/registry/endpoints", registryWrite.Endpoint)
	mux.HandleFunc("GET /api/v1/discovery/candidates", discovery.List)
	mux.HandleFunc("POST /api/v1/discovery/candidates", discovery.Intake)
	mux.HandleFunc("POST /api/v1/discovery/candidates/review", discoveryReview.Review)
	mux.HandleFunc("GET /api/v1/discovery/reviews", discoveryReview.List)
	mux.HandleFunc("POST /api/v1/discovery/candidates/promote", discoveryPromotion.Promote)
	mux.HandleFunc("GET /api/v1/discovery/promotions", discoveryPromotion.List)
	mux.Handle("GET /api/v1/probe-plan", probePlan)

	srv := &http.Server{Addr: listen, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Printf("IRCIntel %s listening on %s (storage=%s)", version, listen, storage.Backend)
	log.Fatal(srv.ListenAndServe())
}

func storageConfigFromEnv() (storageConfig, error) {
	backend := strings.ToLower(strings.TrimSpace(getenv("IRCINTEL_STORAGE_BACKEND", "sqlite")))
	cfg := storageConfig{
		Backend:     backend,
		SQLitePath:  strings.TrimSpace(getenv("IRCINTEL_DB_PATH", "/data/ircintel.db")),
		DatabaseURL: strings.TrimSpace(os.Getenv("IRCINTEL_DATABASE_URL")),
	}
	switch backend {
	case "sqlite":
		if cfg.SQLitePath == "" { return storageConfig{}, errors.New("IRCINTEL_DB_PATH is required for sqlite storage") }
		if cfg.DatabaseURL != "" { return storageConfig{}, errors.New("IRCINTEL_DATABASE_URL is only valid with postgres storage") }
	case "postgres":
		if cfg.DatabaseURL == "" { return storageConfig{}, errors.New("IRCINTEL_DATABASE_URL is required for postgres storage") }
	default:
		return storageConfig{}, fmt.Errorf("unsupported IRCINTEL_STORAGE_BACKEND %q", backend)
	}
	return cfg, nil
}

func maintenanceConfigFromEnv() (maintenanceConfig, error) {
	enabled, err := getenvBool("IRCINTEL_MAINTENANCE_ENABLED", false)
	if err != nil { return maintenanceConfig{}, err }
	interval, err := getenvDuration("IRCINTEL_MAINTENANCE_INTERVAL", defaultMaintenanceInterval)
	if err != nil { return maintenanceConfig{}, err }
	rawRetention, err := getenvDuration("IRCINTEL_RAW_OBSERVATION_RETENTION", core.DefaultRawObservationRetention)
	if err != nil { return maintenanceConfig{}, err }
	hourlyRetention, err := getenvDuration("IRCINTEL_HOURLY_ROLLUP_RETENTION", core.DefaultHourlyRollupRetention)
	if err != nil { return maintenanceConfig{}, err }
	if hourlyRetention < rawRetention { return maintenanceConfig{}, errors.New("IRCINTEL_HOURLY_ROLLUP_RETENTION must not be shorter than IRCINTEL_RAW_OBSERVATION_RETENTION") }
	return maintenanceConfig{Enabled: enabled, Interval: interval, RawRetention: rawRetention, HourlyRetention: hourlyRetention}, nil
}

func openStorage(cfg storageConfig) (core.RuntimeStore, error) {
	switch cfg.Backend {
	case "sqlite":
		return core.OpenSQLiteStore(cfg.SQLitePath)
	case "postgres":
		return core.OpenPostgresStore(cfg.DatabaseURL)
	default:
		return nil, fmt.Errorf("unsupported storage backend %q", cfg.Backend)
	}
}

func getenv(key, fallback string) string { if value := os.Getenv(key); value != "" { return value }; return fallback }

func getenvBool(key string, fallback bool) (bool, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" { return fallback, nil }
	value, err := strconv.ParseBool(raw)
	if err != nil { return false, fmt.Errorf("%s must be a boolean: %w", key, err) }
	return value, nil
}

func getenvDuration(key string, fallback time.Duration) (time.Duration, error) {
	raw := os.Getenv(key)
	if raw == "" { return fallback, nil }
	value, err := time.ParseDuration(raw)
	if err != nil { return 0, fmt.Errorf("%s must be a Go duration: %w", key, err) }
	if value <= 0 { return 0, fmt.Errorf("%s must be greater than zero", key) }
	return value, nil
}
