package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestRegistryPersistsNetworkServerAndEndpoint(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ircintel.db")
	store, err := OpenSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.UpsertNetwork(Network{ID: "libera", Name: "Libera.Chat", Website: "https://libera.chat", Description: "IRC network"}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertNetworkServer(NetworkServer{ID: "libera-rotation", NetworkID: "libera", Name: "Client rotation"}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertNetworkEndpoint(NetworkEndpoint{ID: "libera-6697", ServerID: "libera-rotation", Host: "IRC.LIBERA.CHAT", Port: "6697", TLS: true}); err != nil {
		t.Fatal(err)
	}

	snapshot, err := store.RegistrySnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Networks) != 1 || snapshot.Networks[0].ID != "libera" {
		t.Fatalf("networks=%+v", snapshot.Networks)
	}
	if len(snapshot.Servers) != 1 || snapshot.Servers[0].NetworkID != "libera" {
		t.Fatalf("servers=%+v", snapshot.Servers)
	}
	if len(snapshot.Endpoints) != 1 || snapshot.Endpoints[0].Host != "irc.libera.chat" || !snapshot.Endpoints[0].TLS {
		t.Fatalf("endpoints=%+v", snapshot.Endpoints)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = OpenSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	snapshot, err = store.RegistrySnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Networks) != 1 || len(snapshot.Servers) != 1 || len(snapshot.Endpoints) != 1 {
		t.Fatalf("reopened snapshot=%+v", snapshot)
	}
}

func TestRegistryRejectsMissingParents(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := store.UpsertNetworkServer(NetworkServer{ID: "server", NetworkID: "missing", Name: "Missing parent"}); err == nil {
		t.Fatal("expected missing network error")
	}
	if err := store.UpsertNetworkEndpoint(NetworkEndpoint{ID: "endpoint", ServerID: "missing", Host: "irc.example", Port: "6697", TLS: true}); err == nil {
		t.Fatal("expected missing server error")
	}
}

func TestRegistryHandlerReturnsSnapshotAndRequiresAuth(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.UpsertNetwork(Network{ID: "example", Name: "ExampleNet"}); err != nil {
		t.Fatal(err)
	}

	handler := RegistryHandler{Token: "secret", Reader: store}
	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/v1/registry", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status=%d", unauthorized.Code)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/v1/registry", nil)
	request.Header.Set("Authorization", "Bearer secret")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
	}
	var snapshot RegistrySnapshot
	if err := json.NewDecoder(response.Body).Decode(&snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Networks) != 1 || snapshot.Networks[0].Name != "ExampleNet" {
		t.Fatalf("snapshot=%+v", snapshot)
	}
}
