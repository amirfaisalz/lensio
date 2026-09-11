package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/amirfaisalz/lensio/apps/api/internal/http/handlers"
)

type mockPinger struct {
	pingFunc func(ctx context.Context) error
}

func (m *mockPinger) PingContext(ctx context.Context) error {
	if m.pingFunc != nil {
		return m.pingFunc(ctx)
	}
	return nil
}

func TestReadyHandler_Connected(t *testing.T) {
	pinger := &mockPinger{
		pingFunc: func(ctx context.Context) error {
			return nil
		},
	}

	handler := handlers.ReadyHandler(pinger)
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp handlers.ReadyResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != "ready" || resp.Database != "connected" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestReadyHandler_Disconnected(t *testing.T) {
	pinger := &mockPinger{
		pingFunc: func(ctx context.Context) error {
			return errors.New("connection refused")
		},
	}

	handler := handlers.ReadyHandler(pinger)
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}

	var resp handlers.ReadyResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != "unready" || resp.Database != "disconnected" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestReadyHandler_NilPinger(t *testing.T) {
	handler := handlers.ReadyHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}

	var resp handlers.ReadyResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != "unready" || resp.Database != "not_configured" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}
