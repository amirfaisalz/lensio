package http_test

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/amirfaisalz/lensio/apps/api/internal/apikey"
	internalhttp "github.com/amirfaisalz/lensio/apps/api/internal/http"
	"github.com/amirfaisalz/lensio/apps/api/internal/http/handlers"
	"github.com/amirfaisalz/lensio/apps/api/internal/http/response"
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

func TestRouter_E2E_KeyLifecycleAndAuth(t *testing.T) {
	kStore := newDummyKeyStore()
	router := internalhttp.NewRouter(&dummyPinger{}, kStore, nil, nil)
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

func TestRouter_E2E_OCRPipeline(t *testing.T) {
	kStore := newDummyKeyStore()
	ocrStore := newDummyOCRStore()
	engine := providers.NewMockEngine()

	router := internalhttp.NewRouter(&dummyPinger{}, kStore, engine, ocrStore)
	server := httptest.NewServer(router)
	defer server.Close()

	client := server.Client()

	// 1. Create full OCR key (write + read)
	createPayload := map[string]any{
		"name":   "OCR Full Key",
		"scopes": []string{"ocr:write", "ocr:read"},
	}
	body, _ := json.Marshal(createPayload)

	resp, err := client.Post(server.URL+"/api/v1/auth/api-keys", "application/json", bytes.NewReader(body))
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
	roResp, _ := client.Post(server.URL+"/api/v1/auth/api-keys", "application/json", bytes.NewReader(bodyRO))
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
