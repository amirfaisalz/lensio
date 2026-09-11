package integration_test

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"
)

type healthResponse struct {
	Status string `json:"status"`
}

type readyResponse struct {
	Status   string `json:"status"`
	Database string `json:"database"`
}

func TestIntegration_Probes(t *testing.T) {
	apiURL := os.Getenv("API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:8080"
	}

	client := &http.Client{
		Timeout: 3 * time.Second,
	}

	// 1. Check if the server is accessible
	resp, err := client.Get(apiURL + "/health")
	if err != nil {
		t.Skipf("skipping live API integration test (server unreachable at %s): %v", apiURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /health, got %d", resp.StatusCode)
	}

	var health healthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("failed decoding /health response: %v", err)
	}
	if health.Status != "ok" {
		t.Fatalf("expected health status 'ok', got '%s'", health.Status)
	}

	// 2. Check /ready
	readyResp, err := client.Get(apiURL + "/ready")
	if err != nil {
		t.Fatalf("failed calling /ready: %v", err)
	}
	defer readyResp.Body.Close()

	if readyResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /ready, got %d", readyResp.StatusCode)
	}

	var ready readyResponse
	if err := json.NewDecoder(readyResp.Body).Decode(&ready); err != nil {
		t.Fatalf("failed decoding /ready response: %v", err)
	}
	if ready.Status != "ready" || ready.Database != "connected" {
		t.Fatalf("unexpected ready response: %+v", ready)
	}
}
