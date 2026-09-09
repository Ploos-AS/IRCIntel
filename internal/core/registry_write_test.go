package core

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeRegistryWriter struct {
	network  Network
	server   NetworkServer
	endpoint NetworkEndpoint
	err      error
}

func (f *fakeRegistryWriter) UpsertNetwork(item Network) error {
	f.network = item
	return f.err
}

func (f *fakeRegistryWriter) UpsertNetworkServer(item NetworkServer) error {
	f.server = item
	return f.err
}

func (f *fakeRegistryWriter) UpsertNetworkEndpoint(item NetworkEndpoint) error {
	f.endpoint = item
	return f.err
}

func TestRegistryWriteNetworkRequiresConfiguredToken(t *testing.T) {
	h := RegistryWriteHandler{Writer: &fakeRegistryWriter{}}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/registry/networks", bytes.NewBufferString(`{"id":"libera","name":"Libera.Chat"}`))
	res := httptest.NewRecorder()
	h.Network(res, req)
	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%q", res.Code, res.Body.String())
	}
}

func TestRegistryWriteNetworkUpsertsAuthorizedPayload(t *testing.T) {
	writer := &fakeRegistryWriter{}
	h := RegistryWriteHandler{Token: "secret", Writer: writer}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/registry/networks", bytes.NewBufferString(`{"id":" libera ","name":" Libera.Chat ","website":" https://libera.chat "}`))
	req.Header.Set("Authorization", "Bearer secret")
	res := httptest.NewRecorder()
	h.Network(res, req)
	if res.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%q", res.Code, res.Body.String())
	}
	if writer.network.ID != "libera" || writer.network.Name != "Libera.Chat" || writer.network.Website != "https://libera.chat" {
		t.Fatalf("network=%+v", writer.network)
	}
}

func TestRegistryWriteRejectsUnknownFields(t *testing.T) {
	h := RegistryWriteHandler{Token: "secret", Writer: &fakeRegistryWriter{}}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/registry/networks", bytes.NewBufferString(`{"id":"libera","name":"Libera.Chat","unexpected":true}`))
	req.Header.Set("Authorization", "Bearer secret")
	res := httptest.NewRecorder()
	h.Network(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%q", res.Code, res.Body.String())
	}
}

func TestRegistryWriteServerReportsMissingParentFromTypedError(t *testing.T) {
	writer := &fakeRegistryWriter{err: fmt.Errorf("wrapped: %w", ErrRegistryNetworkNotFound)}
	h := RegistryWriteHandler{Token: "secret", Writer: writer}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/registry/servers", bytes.NewBufferString(`{"id":"server-1","network_id":"missing","name":"irc.example"}`))
	req.Header.Set("Authorization", "Bearer secret")
	res := httptest.NewRecorder()
	h.Server(res, req)
	if res.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%q", res.Code, res.Body.String())
	}
}

func TestRegistryWriteEndpointReportsMissingParentFromTypedError(t *testing.T) {
	writer := &fakeRegistryWriter{err: fmt.Errorf("wrapped: %w", ErrRegistryServerNotFound)}
	h := RegistryWriteHandler{Token: "secret", Writer: writer}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/registry/endpoints", bytes.NewBufferString(`{"id":"endpoint-1","server_id":"missing","host":"irc.example","port":"6697","tls":true}`))
	req.Header.Set("Authorization", "Bearer secret")
	res := httptest.NewRecorder()
	h.Endpoint(res, req)
	if res.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%q", res.Code, res.Body.String())
	}
}

func TestRegistryWriteDoesNotClassifyErrorTextAsMissingParent(t *testing.T) {
	writer := &fakeRegistryWriter{err: errors.New("network does not exist")}
	h := RegistryWriteHandler{Token: "secret", Writer: writer}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/registry/servers", bytes.NewBufferString(`{"id":"server-1","network_id":"n1","name":"irc.example"}`))
	req.Header.Set("Authorization", "Bearer secret")
	res := httptest.NewRecorder()
	h.Server(res, req)
	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%q", res.Code, res.Body.String())
	}
}

func TestRegistryWriteEndpointNormalizesHostname(t *testing.T) {
	writer := &fakeRegistryWriter{}
	h := RegistryWriteHandler{Token: "secret", Writer: writer}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/registry/endpoints", bytes.NewBufferString(`{"id":"libera-tls","server_id":"server-1","host":" IRC.Libera.Chat ","port":"6697","tls":true}`))
	req.Header.Set("Authorization", "Bearer secret")
	res := httptest.NewRecorder()
	h.Endpoint(res, req)
	if res.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%q", res.Code, res.Body.String())
	}
	if writer.endpoint.Host != "irc.libera.chat" || writer.endpoint.Port != "6697" || !writer.endpoint.TLS {
		t.Fatalf("endpoint=%+v", writer.endpoint)
	}
}

func TestRegistryWriteRejectsUnauthorizedRequest(t *testing.T) {
	h := RegistryWriteHandler{Token: "secret", Writer: &fakeRegistryWriter{}}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/registry/networks", bytes.NewBufferString(`{"id":"libera","name":"Libera.Chat"}`))
	res := httptest.NewRecorder()
	h.Network(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%q", res.Code, res.Body.String())
	}
}
