package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/apikey"
	internalhttp "github.com/amirfaisalz/lensio/apps/api/internal/http"
	"github.com/amirfaisalz/lensio/apps/api/internal/http/handlers"
	"github.com/amirfaisalz/lensio/apps/api/internal/http/response"
	"github.com/amirfaisalz/lensio/apps/api/internal/idempotency"
	"github.com/amirfaisalz/lensio/apps/api/internal/store"
	"github.com/amirfaisalz/lensio/apps/api/internal/usage"
	"github.com/amirfaisalz/lensio/services/ocr/providers"
	"github.com/amirfaisalz/lensio/tests/fixtures/synthetic"
)

func buildTestMultipart(fieldName, filename string, content []byte) (*bytes.Buffer, string) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile(fieldName, filename)
	_, _ = part.Write(content)
	_ = writer.Close()
	return body, writer.FormDataContentType()
}

func seedTestAPIKey(t *testing.T, kStore *dummyKeyStore, orgID string, scopes []string) string {
	t.Helper()
	gen, err := apikey.Generate(apikey.EnvLive)
	if err != nil {
		t.Fatalf("failed generating bootstrap key: %v", err)
	}
	k := &store.APIKey{
		ID:          "bootstrap-admin-key",
		OrgID:       orgID,
		KeyHash:     gen.KeyHash,
		Prefix:      gen.Prefix,
		Scopes:      scopes,
		Environment: apikey.EnvLive,
		CreatedAt:   time.Now(),
	}
	_ = kStore.CreateAPIKey(context.Background(), k)
	return gen.Plaintext
}

func TestRouter_E2E_KeyLifecycleAndAuth(t *testing.T) {
	kStore := newDummyKeyStore()
	router := internalhttp.NewRouter(&dummyPinger{}, kStore, nil, nil)
	server := httptest.NewServer(router)
	defer server.Close()

	client := server.Client()

	// 0. Verify unauthenticated requests return 401 Unauthorized
	unauthGet, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/auth/api-keys", nil)
	unauthGetResp, err := client.Do(unauthGet)
	if err != nil {
		t.Fatalf("failed unauthenticated GET: %v", err)
	}
	unauthGetResp.Body.Close()
	if unauthGetResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated GET /api/v1/auth/api-keys, got %d", unauthGetResp.StatusCode)
	}

	unauthPost, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/auth/api-keys", bytes.NewBufferString(`{"name":"Unauth"}`))
	unauthPost.Header.Set("Content-Type", "application/json")
	unauthPostResp, err := client.Do(unauthPost)
	if err != nil {
		t.Fatalf("failed unauthenticated POST: %v", err)
	}
	unauthPostResp.Body.Close()
	if unauthPostResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated POST /api/v1/auth/api-keys, got %d", unauthPostResp.StatusCode)
	}

	unauthDel, _ := http.NewRequest(http.MethodDelete, server.URL+"/api/v1/auth/api-keys/some-key", nil)
	unauthDelResp, err := client.Do(unauthDel)
	if err != nil {
		t.Fatalf("failed unauthenticated DELETE: %v", err)
	}
	unauthDelResp.Body.Close()
	if unauthDelResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated DELETE /api/v1/auth/api-keys/{id}, got %d", unauthDelResp.StatusCode)
	}

	// Seed bootstrap key to perform key management operations
	adminToken := seedTestAPIKey(t, kStore, handlers.DefaultOrgID, []string{"*"})

	// 1. Create Key with valid authentication
	createPayload := map[string]any{
		"name":   "E2E Key",
		"scopes": []string{"ocr:write"},
	}
	body, _ := json.Marshal(createPayload)

	createReq, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/auth/api-keys", bytes.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := client.Do(createReq)
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
	badReq.Header.Set("Authorization", "Bearer lensio_live_"+apikey.Hash("unknown")[:64])
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
	readReq, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/auth/api-keys", bytes.NewReader(readBody))
	readReq.Header.Set("Content-Type", "application/json")
	readReq.Header.Set("Authorization", "Bearer "+adminToken)
	readResp, err := client.Do(readReq)
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

func TestRouter_E2E_OCRPipeline(t *testing.T) {
	kStore := newDummyKeyStore()
	ocrStore := newDummyOCRStore()
	engine := providers.NewMockEngine()

	router := internalhttp.NewRouter(&dummyPinger{}, kStore, engine, ocrStore)
	server := httptest.NewServer(router)
	defer server.Close()

	client := server.Client()
	adminToken := seedTestAPIKey(t, kStore, handlers.DefaultOrgID, []string{"*"})

	// 1. Create full OCR key (write + read)
	createPayload := map[string]any{
		"name":   "OCR Full Key",
		"scopes": []string{"ocr:write", "ocr:read"},
	}
	body, _ := json.Marshal(createPayload)

	createReq, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/auth/api-keys", bytes.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+adminToken)
	resp, err := client.Do(createReq)
	if err != nil {
		t.Fatalf("failed creating key: %v", err)
	}
	var fullKey handlers.CreateKeyResponse
	_ = json.NewDecoder(resp.Body).Decode(&fullKey)
	resp.Body.Close()

	// 2. Create read-only key
	readOnlyPayload := map[string]any{
		"name":   "OCR Read Only Key",
		"scopes": []string{"ocr:read"},
	}
	bodyRO, _ := json.Marshal(readOnlyPayload)
	roReq, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/auth/api-keys", bytes.NewReader(bodyRO))
	roReq.Header.Set("Content-Type", "application/json")
	roReq.Header.Set("Authorization", "Bearer "+adminToken)
	roResp, err := client.Do(roReq)
	if err != nil {
		t.Fatalf("failed creating read-only key: %v", err)
	}
	var roKey handlers.CreateKeyResponse
	_ = json.NewDecoder(roResp.Body).Decode(&roKey)
	roResp.Body.Close()

	t.Run("successful KTP upload and metadata query", func(t *testing.T) {
		validImg := synthetic.GenerateValidKTPImage()
		reqBody, contentType := buildTestMultipart("document", "ktp.png", validImg)

		req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/ocr/ktp", reqBody)
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("Authorization", "Bearer "+fullKey.Key)

		ocrResp, err := client.Do(req)
		if err != nil {
			t.Fatalf("failed POST /api/v1/ocr/ktp: %v", err)
		}
		defer ocrResp.Body.Close()

		if ocrResp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", ocrResp.StatusCode)
		}

		var ktpResp handlers.KTPResponse
		if err := json.NewDecoder(ocrResp.Body).Decode(&ktpResp); err != nil {
			t.Fatalf("failed decoding KTP response: %v", err)
		}

		if ktpResp.ID == "" || ktpResp.DocumentType != "ktp" {
			t.Errorf("unexpected KTP response: %+v", ktpResp)
		}
		if ktpResp.Data.NIK != "3171010101900001" {
			t.Errorf("expected NIK 3171010101900001, got %s", ktpResp.Data.NIK)
		}

		// Query metadata via GET /api/v1/ocr/{id}
		getReq, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/ocr/"+ktpResp.ID, nil)
		getReq.Header.Set("Authorization", "Bearer "+fullKey.Key)

		getResp, err := client.Do(getReq)
		if err != nil {
			t.Fatalf("failed GET /api/v1/ocr/:id: %v", err)
		}
		defer getResp.Body.Close()

		if getResp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 from GET metadata, got %d", getResp.StatusCode)
		}

		var meta handlers.OCRRequestMetadata
		if err := json.NewDecoder(getResp.Body).Decode(&meta); err != nil {
			t.Fatalf("failed decoding metadata: %v", err)
		}
		if meta.ID != ktpResp.ID || meta.Status != "completed" {
			t.Errorf("unexpected metadata: %+v", meta)
		}
	})

	t.Run("POST /api/v1/ocr/sim execution and retrieval", func(t *testing.T) {
		validImg := synthetic.GenerateValidSIMImage()
		reqBody, contentType := buildTestMultipart("document", "sim.png", validImg)

		req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/ocr/sim", reqBody)
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("Authorization", "Bearer "+fullKey.Key)

		ocrResp, err := client.Do(req)
		if err != nil {
			t.Fatalf("failed POST /api/v1/ocr/sim: %v", err)
		}
		defer ocrResp.Body.Close()

		if ocrResp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", ocrResp.StatusCode)
		}

		var simResp handlers.SIMResponse
		if err := json.NewDecoder(ocrResp.Body).Decode(&simResp); err != nil {
			t.Fatalf("failed decoding SIM response: %v", err)
		}

		if simResp.ID == "" || simResp.DocumentType != "sim" {
			t.Errorf("unexpected SIM response: %+v", simResp)
		}
		if simResp.Data == nil || simResp.Data.NomorSIM != "123456789012" {
			t.Errorf("expected NomorSIM 123456789012, got %+v", simResp.Data)
		}

		// Query metadata via GET /api/v1/ocr/{id}
		getReq, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/ocr/"+simResp.ID, nil)
		getReq.Header.Set("Authorization", "Bearer "+fullKey.Key)

		getResp, err := client.Do(getReq)
		if err != nil {
			t.Fatalf("failed GET /api/v1/ocr/:id: %v", err)
		}
		defer getResp.Body.Close()

		if getResp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 from GET metadata, got %d", getResp.StatusCode)
		}

		var meta handlers.OCRRequestMetadata
		if err := json.NewDecoder(getResp.Body).Decode(&meta); err != nil {
			t.Fatalf("failed decoding metadata: %v", err)
		}
		if meta.ID != simResp.ID || meta.Status != "completed" || meta.DocType != "sim" {
			t.Errorf("unexpected metadata: %+v", meta)
		}
	})

	t.Run("unsupported document returns 422", func(t *testing.T) {
		unsupportedImg := synthetic.GenerateUnsupportedDocImage()
		reqBody, contentType := buildTestMultipart("document", "receipt.png", unsupportedImg)

		req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/ocr/ktp", reqBody)
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("Authorization", "Bearer "+fullKey.Key)

		ocrResp, err := client.Do(req)
		if err != nil {
			t.Fatalf("failed request: %v", err)
		}
		defer ocrResp.Body.Close()

		if ocrResp.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("expected 422, got %d", ocrResp.StatusCode)
		}
	})

	t.Run("corrupt image returns 400", func(t *testing.T) {
		corruptImg := synthetic.GenerateCorruptedImage()
		reqBody, contentType := buildTestMultipart("document", "corrupt.jpg", corruptImg)

		req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/ocr/ktp", reqBody)
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("Authorization", "Bearer "+fullKey.Key)

		ocrResp, err := client.Do(req)
		if err != nil {
			t.Fatalf("failed request: %v", err)
		}
		defer ocrResp.Body.Close()

		if ocrResp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", ocrResp.StatusCode)
		}
	})

	t.Run("read-only key receives 403 on post ktp", func(t *testing.T) {
		validImg := synthetic.GenerateValidKTPImage()
		reqBody, contentType := buildTestMultipart("document", "ktp.png", validImg)

		req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/ocr/ktp", reqBody)
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("Authorization", "Bearer "+roKey.Key)

		ocrResp, err := client.Do(req)
		if err != nil {
			t.Fatalf("failed request: %v", err)
		}
		defer ocrResp.Body.Close()

		if ocrResp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", ocrResp.StatusCode)
		}
	})

	t.Run("invalid key receives 401", func(t *testing.T) {
		validImg := synthetic.GenerateValidKTPImage()
		reqBody, contentType := buildTestMultipart("document", "ktp.png", validImg)

		req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/ocr/ktp", reqBody)
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("Authorization", "Bearer lensio_live_fakekeytoken12345678901234567890123456789012")

		ocrResp, err := client.Do(req)
		if err != nil {
			t.Fatalf("failed request: %v", err)
		}
		defer ocrResp.Body.Close()

		if ocrResp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", ocrResp.StatusCode)
		}
	})

	t.Run("metadata not found returns 404", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/ocr/non-existent-id", nil)
		req.Header.Set("Authorization", "Bearer "+fullKey.Key)

		getResp, err := client.Do(req)
		if err != nil {
			t.Fatalf("failed request: %v", err)
		}
		defer getResp.Body.Close()

		if getResp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", getResp.StatusCode)
		}
	})
}

func TestRouter_E2E_IdempotencyPipeline(t *testing.T) {
	kStore := newDummyKeyStore()
	ocrStore := newDummyOCRStore()
	memIdempStore := idempotency.NewMemoryStore(time.Hour)
	engine := providers.NewMockEngine()

	router := internalhttp.NewRouterWithDeps(internalhttp.RouterDeps{
		KeyStore:         kStore,
		OCREngine:        engine,
		OCRStore:         ocrStore,
		IdempotencyStore: memIdempStore,
	})
	server := httptest.NewServer(router)
	defer server.Close()

	client := server.Client()
	adminToken := seedTestAPIKey(t, kStore, handlers.DefaultOrgID, []string{"*"})

	// 1. Create full OCR key
	createPayload := map[string]any{
		"name":   "Idempotency Test Key",
		"scopes": []string{"ocr:write", "ocr:read"},
	}
	body, _ := json.Marshal(createPayload)
	createReq, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/auth/api-keys", bytes.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+adminToken)
	resp, err := client.Do(createReq)
	if err != nil {
		t.Fatalf("failed creating key: %v", err)
	}
	var apiKey handlers.CreateKeyResponse
	_ = json.NewDecoder(resp.Body).Decode(&apiKey)
	resp.Body.Close()

	validImg := synthetic.GenerateValidKTPImage()
	altImg := synthetic.GenerateCorruptedImage()

	idempKey := "idemp-ktp-test-001"

	// 2. Initial POST /api/v1/ocr/ktp with Idempotency-Key
	reqBody1, contentType1 := buildTestMultipart("document", "ktp.png", validImg)
	rawBody1 := reqBody1.Bytes()
	req1, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/ocr/ktp", bytes.NewReader(rawBody1))
	req1.Header.Set("Content-Type", contentType1)
	req1.Header.Set("Authorization", "Bearer "+apiKey.Key)
	req1.Header.Set("Idempotency-Key", idempKey)

	resp1, err := client.Do(req1)
	if err != nil {
		t.Fatalf("initial request failed: %v", err)
	}
	defer resp1.Body.Close()

	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp1.StatusCode)
	}
	if resp1.Header.Get("Idempotent-Replayed") == "true" {
		t.Fatal("expected Idempotent-Replayed to be empty/false on initial request")
	}

	var ktpResp1 handlers.KTPResponse
	if err := json.NewDecoder(resp1.Body).Decode(&ktpResp1); err != nil {
		t.Fatalf("failed decoding response: %v", err)
	}
	if ktpResp1.ID == "" || ktpResp1.Data == nil {
		t.Fatalf("invalid ktp response: %+v", ktpResp1)
	}
	initialRecordCount := len(ocrStore.records)

	// 3. Replay with identical key and identical payload -> 200 OK with Idempotent-Replayed: true
	req2, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/ocr/ktp", bytes.NewReader(rawBody1))
	req2.Header.Set("Content-Type", contentType1)
	req2.Header.Set("Authorization", "Bearer "+apiKey.Key)
	req2.Header.Set("Idempotency-Key", idempKey)

	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("replay request failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on replay, got %d", resp2.StatusCode)
	}
	if resp2.Header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("expected Idempotent-Replayed: true, got %s", resp2.Header.Get("Idempotent-Replayed"))
	}

	var ktpResp2 handlers.KTPResponse
	if err := json.NewDecoder(resp2.Body).Decode(&ktpResp2); err != nil {
		t.Fatalf("failed decoding replayed response: %v", err)
	}
	if ktpResp2.ID != ktpResp1.ID {
		t.Fatalf("expected replayed ID %s, got %s", ktpResp1.ID, ktpResp2.ID)
	}
	// Verify OCR engine / store was NOT called a second time
	if len(ocrStore.records) != initialRecordCount {
		t.Fatalf("expected ocrStore record count to remain %d, got %d", initialRecordCount, len(ocrStore.records))
	}

	// 4. Replay with identical key but ALTERED payload -> 422 Unprocessable Entity
	reqBody3, contentType3 := buildTestMultipart("document", "ktp_alt.png", altImg)
	req3, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/ocr/ktp", reqBody3)
	req3.Header.Set("Content-Type", contentType3)
	req3.Header.Set("Authorization", "Bearer "+apiKey.Key)
	req3.Header.Set("Idempotency-Key", idempKey)

	resp3, err := client.Do(req3)
	if err != nil {
		t.Fatalf("altered payload request failed: %v", err)
	}
	defer resp3.Body.Close()

	if resp3.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for altered payload, got %d", resp3.StatusCode)
	}

	var errEnv response.ErrorEnvelope
	_ = json.NewDecoder(resp3.Body).Decode(&errEnv)
	if errEnv.Error.Code != response.CodeIdempotencyMismatch {
		t.Errorf("expected code %s, got %s", response.CodeIdempotencyMismatch, errEnv.Error.Code)
	}

	// 5. Test concurrent / in-progress request returns 409 Conflict
	concurrentKey := "idemp-concurrent-in-progress"
	// Pre-lock in memory store to simulate concurrent request
	reqBody4, contentType4 := buildTestMultipart("document", "ktp.png", validImg)
	hash4, _ := idempotency.ComputePayloadHash(httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(reqBody4.Bytes())), 0)
	_, _, _ = memIdempStore.LockOrGet(context.Background(), handlers.DefaultOrgID, concurrentKey, hash4, time.Hour)

	req4, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/ocr/ktp", reqBody4)
	req4.Header.Set("Content-Type", contentType4)
	req4.Header.Set("Authorization", "Bearer "+apiKey.Key)
	req4.Header.Set("Idempotency-Key", concurrentKey)

	resp4, err := client.Do(req4)
	if err != nil {
		t.Fatalf("concurrent request failed: %v", err)
	}
	defer resp4.Body.Close()

	if resp4.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 Conflict for in-progress request, got %d", resp4.StatusCode)
	}

	var errEnv4 response.ErrorEnvelope
	_ = json.NewDecoder(resp4.Body).Decode(&errEnv4)
	if errEnv4.Error.Code != response.CodeRequestInProgress {
		t.Errorf("expected code %s, got %s", response.CodeRequestInProgress, errEnv4.Error.Code)
	}
}

func TestRouter_E2E_Idempotency_ZeroDuplicateQuotaAndBilling(t *testing.T) {
	kStore := newDummyKeyStore()
	ocrStore := newDummyOCRStore()
	uStore := &dummyUsageStore{}
	recorder := usage.NewRecorder(uStore, 100)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = recorder.Close(ctx)
	}()

	memIdempStore := idempotency.NewMemoryStore(time.Hour)
	engine := providers.NewMockEngine()

	router := internalhttp.NewRouterWithDeps(internalhttp.RouterDeps{
		KeyStore:         kStore,
		OCREngine:        engine,
		OCRStore:         ocrStore,
		UsageRecorder:    recorder,
		IdempotencyStore: memIdempStore,
	})
	server := httptest.NewServer(router)
	defer server.Close()

	client := server.Client()
	adminToken := seedTestAPIKey(t, kStore, handlers.DefaultOrgID, []string{"*"})

	createPayload := map[string]any{
		"name":   "Quota Safe Key",
		"scopes": []string{"ocr:write", "ocr:read"},
	}
	body, _ := json.Marshal(createPayload)
	createReq, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/auth/api-keys", bytes.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+adminToken)
	resp, err := client.Do(createReq)
	if err != nil {
		t.Fatalf("failed creating key: %v", err)
	}
	var apiKey handlers.CreateKeyResponse
	_ = json.NewDecoder(resp.Body).Decode(&apiKey)
	resp.Body.Close()

	validImg := synthetic.GenerateValidKTPImage()
	idempKey := "quota-safe-idemp-key"

	// Flush recorder queue after API key creation
	time.Sleep(50 * time.Millisecond)
	uStore.mu.Lock()
	usageBeforeKTP := len(uStore.records)
	uStore.mu.Unlock()

	// 1. Initial request
	reqBody1, contentType1 := buildTestMultipart("document", "ktp.png", validImg)
	rawBytes := reqBody1.Bytes()
	req1, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/ocr/ktp", bytes.NewReader(rawBytes))
	req1.Header.Set("Content-Type", contentType1)
	req1.Header.Set("Authorization", "Bearer "+apiKey.Key)
	req1.Header.Set("Idempotency-Key", idempKey)

	resp1, err := client.Do(req1)
	if err != nil {
		t.Fatalf("first request failed: %v", err)
	}
	resp1.Body.Close()
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp1.StatusCode)
	}

	// Allow recorder worker pool to process channel
	time.Sleep(50 * time.Millisecond)

	uStore.mu.Lock()
	usageAfterFirst := len(uStore.records)
	uStore.mu.Unlock()

	if usageAfterFirst-usageBeforeKTP != 1 {
		t.Fatalf("expected exactly 1 usage record added for first OCR request, got delta %d", usageAfterFirst-usageBeforeKTP)
	}

	// 2. Replay request with same key
	req2, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/ocr/ktp", bytes.NewReader(rawBytes))
	req2.Header.Set("Content-Type", contentType1)
	req2.Header.Set("Authorization", "Bearer "+apiKey.Key)
	req2.Header.Set("Idempotency-Key", idempKey)

	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("replay request failed: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on replay, got %d", resp2.StatusCode)
	}
	if resp2.Header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("expected Idempotent-Replayed: true, got %s", resp2.Header.Get("Idempotent-Replayed"))
	}

	// Allow any asynchronous processing
	time.Sleep(50 * time.Millisecond)

	uStore.mu.Lock()
	usageAfterReplay := len(uStore.records)
	uStore.mu.Unlock()

	// Zero duplicate billing verified: delta must be exactly 0!
	if usageAfterReplay != usageAfterFirst {
		t.Fatalf("quota violation: replayed request recorded duplicate usage record! before=%d, after=%d", usageAfterFirst, usageAfterReplay)
	}
}

func TestRouter_E2E_SIMOCR_CompleteWorkflow(t *testing.T) {
	kStore := newDummyKeyStore()
	ocrStore := newDummyOCRStore()
	memIdempStore := idempotency.NewMemoryStore(time.Hour)
	engine := providers.NewMockEngine()

	router := internalhttp.NewRouterWithDeps(internalhttp.RouterDeps{
		KeyStore:         kStore,
		OCREngine:        engine,
		OCRStore:         ocrStore,
		IdempotencyStore: memIdempStore,
	})
	server := httptest.NewServer(router)
	defer server.Close()

	client := server.Client()
	adminToken := seedTestAPIKey(t, kStore, handlers.DefaultOrgID, []string{"*"})

	// 1. Create API key with ocr:write and ocr:read
	createPayload := map[string]any{
		"name":   "SIM OCR Key",
		"scopes": []string{"ocr:write", "ocr:read"},
	}
	body, _ := json.Marshal(createPayload)
	createReq, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/auth/api-keys", bytes.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+adminToken)
	resp, err := client.Do(createReq)
	if err != nil {
		t.Fatalf("failed creating key: %v", err)
	}
	var apiKey handlers.CreateKeyResponse
	_ = json.NewDecoder(resp.Body).Decode(&apiKey)
	resp.Body.Close()

	validSIM := synthetic.GenerateValidSIMImage()
	idempKey := "sim-e2e-idemp-key"

	// 2. Initial POST /api/v1/ocr/sim request
	reqBody1, contentType1 := buildTestMultipart("document", "sim.png", validSIM)
	rawBytes := reqBody1.Bytes()
	req1, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/ocr/sim", bytes.NewReader(rawBytes))
	req1.Header.Set("Content-Type", contentType1)
	req1.Header.Set("Authorization", "Bearer "+apiKey.Key)
	req1.Header.Set("Idempotency-Key", idempKey)

	resp1, err := client.Do(req1)
	if err != nil {
		t.Fatalf("initial SIM request failed: %v", err)
	}
	defer resp1.Body.Close()

	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for SIM OCR, got %d", resp1.StatusCode)
	}

	var simResp handlers.SIMResponse
	if err := json.NewDecoder(resp1.Body).Decode(&simResp); err != nil {
		t.Fatalf("failed decoding SIMResponse: %v", err)
	}
	if simResp.DocumentType != "sim" {
		t.Errorf("expected document_type sim, got %s", simResp.DocumentType)
	}
	if simResp.Data == nil || simResp.Data.NomorSIM == "" {
		t.Errorf("expected valid data.nomor_sim, got %v", simResp.Data)
	}

	// 3. Replay with same Idempotency-Key
	req2, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/ocr/sim", bytes.NewReader(rawBytes))
	req2.Header.Set("Content-Type", contentType1)
	req2.Header.Set("Authorization", "Bearer "+apiKey.Key)
	req2.Header.Set("Idempotency-Key", idempKey)

	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("replay SIM request failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK on replay, got %d", resp2.StatusCode)
	}
	if resp2.Header.Get("Idempotent-Replayed") != "true" {
		t.Errorf("expected Idempotent-Replayed header: true")
	}

	// 4. Retrieve saved metadata via GET /api/v1/ocr/{id}
	getReq, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/ocr/"+simResp.ID, nil)
	getReq.Header.Set("Authorization", "Bearer "+apiKey.Key)

	getResp, err := client.Do(getReq)
	if err != nil {
		t.Fatalf("failed GET /api/v1/ocr/{id}: %v", err)
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for GET /api/v1/ocr/{id}, got %d", getResp.StatusCode)
	}

	var meta handlers.OCRRequestMetadata
	if err := json.NewDecoder(getResp.Body).Decode(&meta); err != nil {
		t.Fatalf("failed decoding metadata: %v", err)
	}
	if meta.DocType != "sim" {
		t.Errorf("expected metadata.doc_type sim, got %s", meta.DocType)
	}
	if meta.Status != "completed" {
		t.Errorf("expected metadata.status completed, got %s", meta.Status)
	}

	// 5. Test unsupported document on /api/v1/ocr/sim returns 422
	unsupportedImg := synthetic.GenerateUnsupportedDocImage()
	reqBodyUnsupp, contentTypeUnsupp := buildTestMultipart("document", "unsupp.png", unsupportedImg)
	reqUnsupp, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/ocr/sim", reqBodyUnsupp)
	reqUnsupp.Header.Set("Content-Type", contentTypeUnsupp)
	reqUnsupp.Header.Set("Authorization", "Bearer "+apiKey.Key)

	respUnsupp, err := client.Do(reqUnsupp)
	if err != nil {
		t.Fatalf("unsupported doc request failed: %v", err)
	}
	defer respUnsupp.Body.Close()

	if respUnsupp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for unsupported document on /api/v1/ocr/sim, got %d", respUnsupp.StatusCode)
	}
}
