package integration_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"
)

type createKeyResponse struct {
	ID          string   `json:"id"`
	OrgID       string   `json:"org_id"`
	Name        string   `json:"name"`
	Key         string   `json:"key"`
	Prefix      string   `json:"prefix"`
	Scopes      []string `json:"scopes"`
	Environment string   `json:"environment"`
}

type listKeysResponse struct {
	Data []struct {
		ID        string   `json:"id"`
		Name      string   `json:"name"`
		Prefix    string   `json:"prefix"`
		MaskedKey string   `json:"masked_key"`
		Scopes    []string `json:"scopes"`
	} `json:"data"`
}

type errorEnvelope struct {
	Error struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"request_id"`
	} `json:"error"`
}

func TestIntegration_APIKeyLifecycleAndAuth(t *testing.T) {
	apiURL := os.Getenv("API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:8080"
	}

	client := &http.Client{Timeout: 5 * time.Second}

	// 0. Verify server is reachable
	healthResp, err := client.Get(apiURL + "/health")
	if err != nil {
		t.Skipf("skipping live API integration test (server unreachable at %s): %v", apiURL, err)
	}
	healthResp.Body.Close()

	// 1. Create a key with ocr:write scope
	createBody, _ := json.Marshal(map[string]any{
		"name":        "Integration Test Key",
		"environment": "live",
		"scopes":      []string{"ocr:write"},
	})

	resp, err := client.Post(apiURL+"/api/v1/auth/api-keys", "application/json", bytes.NewReader(createBody))
	if err != nil {
		t.Fatalf("failed to create API key: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201 from create key, got %d", resp.StatusCode)
	}

	var key1 createKeyResponse
	if err := json.NewDecoder(resp.Body).Decode(&key1); err != nil {
		t.Fatalf("failed decoding created key response: %v", err)
	}
	if key1.Key == "" || key1.ID == "" {
		t.Fatalf("expected non-empty key and ID, got key=%q, id=%q", key1.Key, key1.ID)
	}

	// 2. List API keys and verify masked key
	listResp, err := client.Get(apiURL + "/api/v1/auth/api-keys")
	if err != nil {
		t.Fatalf("failed to list API keys: %v", err)
	}
	defer listResp.Body.Close()

	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 from list keys, got %d", listResp.StatusCode)
	}

	var listData listKeysResponse
	if err := json.NewDecoder(listResp.Body).Decode(&listData); err != nil {
		t.Fatalf("failed decoding list keys: %v", err)
	}

	found := false
	for _, k := range listData.Data {
		if k.ID == key1.ID {
			found = true
			if k.MaskedKey == "" || k.MaskedKey == key1.Key {
				t.Errorf("expected masked key representation, got %q", k.MaskedKey)
			}
			break
		}
	}
	if !found {
		t.Fatalf("created key %s not found in list response", key1.ID)
	}

	// 3. Test protected verification endpoint with valid key (ocr:write)
	verifyReq, _ := http.NewRequest(http.MethodGet, apiURL+"/api/v1/auth/verify", nil)
	verifyReq.Header.Set("Authorization", "Bearer "+key1.Key)

	verifyResp, err := client.Do(verifyReq)
	if err != nil {
		t.Fatalf("failed calling /api/v1/auth/verify: %v", err)
	}
	defer verifyResp.Body.Close()

	if verifyResp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 for valid key, got %d", verifyResp.StatusCode)
	}

	// 4. Test protected endpoint without Authorization header
	noAuthReq, _ := http.NewRequest(http.MethodGet, apiURL+"/api/v1/auth/verify", nil)
	noAuthResp, err := client.Do(noAuthReq)
	if err != nil {
		t.Fatalf("failed calling verify without auth: %v", err)
	}
	defer noAuthResp.Body.Close()

	if noAuthResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for missing auth header, got %d", noAuthResp.StatusCode)
	}

	var errEnv errorEnvelope
	_ = json.NewDecoder(noAuthResp.Body).Decode(&errEnv)
	if errEnv.Error.Code != "invalid_api_key" {
		t.Errorf("expected code 'invalid_api_key', got %q", errEnv.Error.Code)
	}

	// 5. Test protected endpoint with insufficient scope
	createScopeBody, _ := json.Marshal(map[string]any{
		"name":        "Read Only Key",
		"environment": "live",
		"scopes":      []string{"ocr:read"},
	})
	scopeResp, err := client.Post(apiURL+"/api/v1/auth/api-keys", "application/json", bytes.NewReader(createScopeBody))
	if err != nil {
		t.Fatalf("failed creating read-only key: %v", err)
	}
	var readOnlyKey createKeyResponse
	_ = json.NewDecoder(scopeResp.Body).Decode(&readOnlyKey)
	scopeResp.Body.Close()

	scopeTestReq, _ := http.NewRequest(http.MethodGet, apiURL+"/api/v1/auth/verify", nil)
	scopeTestReq.Header.Set("Authorization", "Bearer "+readOnlyKey.Key)
	scopeTestResp, err := client.Do(scopeTestReq)
	if err != nil {
		t.Fatalf("failed calling verify with insufficient scope: %v", err)
	}
	defer scopeTestResp.Body.Close()

	if scopeTestResp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for insufficient scope, got %d", scopeTestResp.StatusCode)
	}

	var scopeErrEnv errorEnvelope
	_ = json.NewDecoder(scopeTestResp.Body).Decode(&scopeErrEnv)
	if scopeErrEnv.Error.Code != "insufficient_scope" {
		t.Errorf("expected code 'insufficient_scope', got %q", scopeErrEnv.Error.Code)
	}

	// 6. Revoke key
	revokeReq, _ := http.NewRequest(http.MethodDelete, apiURL+"/api/v1/auth/api-keys/"+key1.ID, nil)
	revokeResp, err := client.Do(revokeReq)
	if err != nil {
		t.Fatalf("failed calling DELETE /api/v1/auth/api-keys/%s: %v", key1.ID, err)
	}
	defer revokeResp.Body.Close()

	if revokeResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for revoking key, got %d", revokeResp.StatusCode)
	}

	// 7. Verify revoked key now returns 401
	revokedVerifyReq, _ := http.NewRequest(http.MethodGet, apiURL+"/api/v1/auth/verify", nil)
	revokedVerifyReq.Header.Set("Authorization", "Bearer "+key1.Key)
	revokedVerifyResp, err := client.Do(revokedVerifyReq)
	if err != nil {
		t.Fatalf("failed calling verify with revoked key: %v", err)
	}
	defer revokedVerifyResp.Body.Close()

	if revokedVerifyResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for revoked key, got %d", revokedVerifyResp.StatusCode)
	}

	// 8. Verify OpenAPI and Docs routes respond with 200
	docsResp, err := client.Get(apiURL + "/docs")
	if err != nil {
		t.Fatalf("failed calling /docs: %v", err)
	}
	docsResp.Body.Close()
	if docsResp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 from /docs, got %d", docsResp.StatusCode)
	}

	openapiResp, err := client.Get(apiURL + "/openapi.yaml")
	if err != nil {
		t.Fatalf("failed calling /openapi.yaml: %v", err)
	}
	openapiResp.Body.Close()
	if openapiResp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 from /openapi.yaml, got %d", openapiResp.StatusCode)
	}
}
