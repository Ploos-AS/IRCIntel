package core

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type fakeDiscoveryPromotionStore struct {
	err error
}

func (f fakeDiscoveryPromotionStore) PromoteDiscoveryCandidate(DiscoveryPromotionInput, time.Time) (DiscoveryPromotion, error) {
	return DiscoveryPromotion{}, f.err
}

func (f fakeDiscoveryPromotionStore) ListDiscoveryPromotions() ([]DiscoveryPromotion, error) {
	return nil, f.err
}

func TestDiscoveryPromotionHandlerReturnsConflictForEndpointConflict(t *testing.T) {
	h := DiscoveryPromotionHandler{Token: "secret", Store: fakeDiscoveryPromotionStore{err: ErrDiscoveryPromotionEndpointConflict}}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/discovery/candidates/promote", bytes.NewBufferString(`{"candidate_id":"candidate","endpoint_id":"endpoint","server_id":"server","promoter":"operator"}`))
	req.Header.Set("Authorization", "Bearer secret")
	res := httptest.NewRecorder()
	h.Promote(res, req)
	if res.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%q", res.Code, res.Body.String())
	}
}

func TestDiscoveryPromotionHandlerRecognizesWrappedEndpointConflict(t *testing.T) {
	h := DiscoveryPromotionHandler{Token: "secret", Store: fakeDiscoveryPromotionStore{err: errors.Join(errors.New("storage context"), ErrDiscoveryPromotionEndpointConflict)}}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/discovery/candidates/promote", bytes.NewBufferString(`{"candidate_id":"candidate","endpoint_id":"endpoint","server_id":"server","promoter":"operator"}`))
	req.Header.Set("Authorization", "Bearer secret")
	res := httptest.NewRecorder()
	h.Promote(res, req)
	if res.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%q", res.Code, res.Body.String())
	}
}
