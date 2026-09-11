package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/amirfaisalz/nusaid/apps/api/internal/apikey"
	internalhttp "github.com/amirfaisalz/nusaid/apps/api/internal/http"
	"github.com/amirfaisalz/nusaid/apps/api/internal/http/handlers"
	"github.com/amirfaisalz/nusaid/apps/api/internal/http/response"
)

func TestRouter_E2E_KeyLifecycleAndAuth(t *testing.T) {
	kStore := newDummyKeyStore()
	router := internalhttp.NewRouter(&dummyPinger{}, kStore)
	server := httptest.NewServer(router)
	defer server.Close()

	client := server.Client()

	// 1. Create Key
	createPayload := map[string]any{
		"name":   "E2E Key",
		"scopes": []string{"ocr:write"},
	}
	body, _ := json.Marshal(createPayload)

	resp, err := client.Post(server.URL+"/api/v1/auth/api-keys", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed creating key: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", resp.StatusCode)
	}

	var created handlers.CreateKeyResponse
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("failed decoding create key response: %v", err)
	}
	if created.Key == "" {
		t.Fatal("expected plaintext key in response, got empty")
	}

	// 2. Call protected endpoint with valid key
	req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/auth/verify", nil)
	req.Header.Set("Authorization", "Bearer "+created.Key)

	authResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed calling verify: %v", err)
	}
	defer authResp.Body.Close()

	if authResp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", authResp.StatusCode)
	}

	var verifyData map[string]any
	_ = json.NewDecoder(authResp.Body).Decode(&verifyData)
	if verifyData["status"] != "authenticated" {
		t.Errorf("expected status 'authenticated', got %v", verifyData["status"])
	}

	// 3. Call protected endpoint with invalid key
	badReq, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/auth/verify", nil)
	badReq.Header.Set("Authorization", "Bearer nusa_live_"+apikey.Hash("unknown")[:64])
	badResp, err := client.Do(badReq)
	if err != nil {
		t.Fatalf("failed calling verify with bad key: %v", err)
	}
	defer badResp.Body.Close()

	if badResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", badResp.StatusCode)
	}

	// 4. Call protected endpoint with insufficient scope (create read-only key)
	readBody, _ := json.Marshal(map[string]any{
		"name":   "Read Only",
		"scopes": []string{"ocr:read"},
	})
	readResp, err := client.Post(server.URL+"/api/v1/auth/api-keys", "application/json", bytes.NewReader(readBody))
	if err != nil {
		t.Fatalf("failed creating read-only key: %v", err)
	}
	var readKey handlers.CreateKeyResponse
	_ = json.NewDecoder(readResp.Body).Decode(&readKey)
	readResp.Body.Close()

	insufficientReq, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/auth/verify", nil)
	insufficientReq.Header.Set("Authorization", "Bearer "+readKey.Key)
	insufficientResp, err := client.Do(insufficientReq)
	if err != nil {
		t.Fatalf("failed calling verify with insufficient scope: %v", err)
	}
	defer insufficientResp.Body.Close()

	if insufficientResp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden, got %d", insufficientResp.StatusCode)
	}

	var errEnv response.ErrorEnvelope
	_ = json.NewDecoder(insufficientResp.Body).Decode(&errEnv)
	if errEnv.Error.Code != response.CodeInsufficientScope {
		t.Errorf("expected code %q, got %q", response.CodeInsufficientScope, errEnv.Error.Code)
	}
}
