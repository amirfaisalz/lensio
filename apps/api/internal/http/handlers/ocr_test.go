package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/http/handlers"
	"github.com/amirfaisalz/lensio/apps/api/internal/http/middleware"
	"github.com/amirfaisalz/lensio/apps/api/internal/http/response"
	"github.com/amirfaisalz/lensio/apps/api/internal/store"
	"github.com/amirfaisalz/lensio/services/ocr"
	"github.com/amirfaisalz/lensio/services/ocr/providers"
	"github.com/amirfaisalz/lensio/tests/fixtures/synthetic"
)

type dummyOCRStore struct {
	records map[string]*store.OCRRequest
	err     error
}

func newDummyOCRStore() *dummyOCRStore {
	return &dummyOCRStore{
		records: make(map[string]*store.OCRRequest),
	}
}

func (d *dummyOCRStore) CreateOCRRequest(ctx context.Context, req *store.OCRRequest) error {
	if d.err != nil {
		return d.err
	}
	req.CreatedAt = time.Now()
	d.records[req.ID] = req
	return nil
}

func (d *dummyOCRStore) GetOCRRequestByID(ctx context.Context, orgID string, id string) (*store.OCRRequest, error) {
	if d.err != nil {
		return nil, d.err
	}
	rec, ok := d.records[id]
	if !ok || rec.OrgID != orgID {
		return nil, store.ErrNotFound
	}
	return rec, nil
}

func createMultipartRequest(t *testing.T, fieldName string, filename string, content []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if fieldName != "" {
		part, err := writer.CreateFormFile(fieldName, filename)
		if err != nil {
			t.Fatalf("failed creating form file: %v", err)
		}
		if _, err := part.Write(content); err != nil {
			t.Fatalf("failed writing part: %v", err)
		}
	}
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ocr/ktp", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func withAuth(req *http.Request, orgID, keyID string) *http.Request {
	key := &store.APIKey{
		ID:     keyID,
		OrgID:  orgID,
		Scopes: []string{"ocr:read", "ocr:write"},
	}
	return req.WithContext(middleware.WithAPIKey(req.Context(), key))
}

func TestGenerateUUIDv4(t *testing.T) {
	uuid := handlers.GenerateUUIDv4()
	if len(uuid) != 36 {
		t.Fatalf("expected 36 chars, got %s", uuid)
	}
	if uuid[14] != '4' {
		t.Fatalf("expected version 4, got %c", uuid[14])
	}
}

type mockQuotaChecker struct {
	allowed   bool
	remaining int
	limit     int
	err       error
}

func (m *mockQuotaChecker) CheckQuota(ctx context.Context, orgID string) (bool, int, int, error) {
	if m.err != nil {
		return false, 0, 0, m.err
	}
	return m.allowed, m.remaining, m.limit, nil
}

func TestKTPOCRHandler(t *testing.T) {
	validImage := synthetic.GenerateValidKTPImage()
	engine := providers.NewMockEngine()
	ocrStore := newDummyOCRStore()
	handler := handlers.KTPOCRHandler(engine, ocrStore, nil)

	t.Run("unauthenticated request returns 401", func(t *testing.T) {
		req := createMultipartRequest(t, "document", "ktp.png", validImage)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("bad body returns 400 invalid_document", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/ocr/ktp", bytes.NewReader([]byte("not multipart")))
		req.Header.Set("Content-Type", "multipart/form-data; boundary=missing")
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("missing document form file returns 400", func(t *testing.T) {
		req := createMultipartRequest(t, "other_file", "doc.png", validImage)
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("corrupt image returns 400 invalid_document", func(t *testing.T) {
		corruptImg := synthetic.GenerateCorruptedImage()
		req := createMultipartRequest(t, "document", "corrupt.jpg", corruptImg)
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("nil engine returns 502", func(t *testing.T) {
		nilHandler := handlers.KTPOCRHandler(nil, ocrStore, nil)
		req := createMultipartRequest(t, "document", "ktp.png", validImage)
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		nilHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadGateway {
			t.Fatalf("expected 502, got %d", rec.Code)
		}
	})

	t.Run("unsupported document returns 422", func(t *testing.T) {
		unsupportedImg := synthetic.GenerateUnsupportedDocImage()
		req := createMultipartRequest(t, "document", "unsupported.png", unsupportedImg)
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected 422, got %d", rec.Code)
		}
	})

	t.Run("engine failure returns 502", func(t *testing.T) {
		failImg := synthetic.GenerateOCRFailureImage()
		req := createMultipartRequest(t, "document", "fail.png", failImg)
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadGateway {
			t.Fatalf("expected 502, got %d", rec.Code)
		}
	})

	t.Run("engine timeout returns 504 and leaks no credentials", func(t *testing.T) {
		engine.SetCustomError(context.DeadlineExceeded)
		defer engine.Reset()

		req := createMultipartRequest(t, "document", "ktp.png", validImage)
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusGatewayTimeout {
			t.Fatalf("expected 504 Gateway Timeout, got %d", rec.Code)
		}

		var errEnv response.ErrorEnvelope
		if err := json.NewDecoder(rec.Body).Decode(&errEnv); err != nil {
			t.Fatalf("failed decoding error envelope: %v", err)
		}
		if errEnv.Error.Code != response.CodeOCRFailed {
			t.Errorf("expected code %s, got %s", response.CodeOCRFailed, errEnv.Error.Code)
		}
		if bytes.Contains(rec.Body.Bytes(), []byte("AIzaSy")) || bytes.Contains(rec.Body.Bytes(), []byte("googleapis.com")) {
			t.Errorf("error body leaked provider details or credentials: %s", rec.Body.String())
		}
	})

	t.Run("arbitrary engine internal error returns 500", func(t *testing.T) {
		engine.SetCustomError(errors.New("unexpected crash"))
		defer engine.Reset()

		req := createMultipartRequest(t, "document", "ktp.png", validImage)
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
	})

	t.Run("engine returns non-ktp document_type returns 422", func(t *testing.T) {
		engine.SetCustomResult(&ocr.OCRResult{
			DocumentType: "passport",
		})
		defer engine.Reset()

		req := createMultipartRequest(t, "document", "ktp.png", validImage)
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected 422, got %d", rec.Code)
		}
	})

	t.Run("successful extraction returns 200 with KTPResponse and persists metadata", func(t *testing.T) {
		req := createMultipartRequest(t, "document", "ktp.png", validImage)
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
		}

		var ktpResp handlers.KTPResponse
		if err := json.NewDecoder(rec.Body).Decode(&ktpResp); err != nil {
			t.Fatalf("failed decoding json: %v", err)
		}

		if ktpResp.ID == "" {
			t.Error("expected non-empty ID")
		}
		if ktpResp.DocumentType != "ktp" {
			t.Errorf("expected doc_type ktp, got %s", ktpResp.DocumentType)
		}
		if ktpResp.Data == nil || ktpResp.Data.NIK != "3171010101900001" {
			t.Errorf("expected NIK 3171010101900001, got %v", ktpResp.Data)
		}
		if ktpResp.Confidence < 0.90 {
			t.Errorf("expected high confidence, got %f", ktpResp.Confidence)
		}

		// Verify record in store
		recMeta, ok := ocrStore.records[ktpResp.ID]
		if !ok {
			t.Fatal("expected record to be saved in ocrStore")
		}
		if recMeta.OrgID != "org-1" || recMeta.Status != "completed" {
			t.Errorf("saved record mismatch: %+v", recMeta)
		}
	})

	t.Run("store failure does not block successful 200 response", func(t *testing.T) {
		failingStore := &dummyOCRStore{err: errors.New("database down")}
		failStoreHandler := handlers.KTPOCRHandler(engine, failingStore, nil)

		req := createMultipartRequest(t, "document", "ktp.png", validImage)
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		failStoreHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 even if store failed, got %d", rec.Code)
		}
	})

	t.Run("quota exhausted returns 429 quota_exceeded", func(t *testing.T) {
		quotaChecker := &mockQuotaChecker{allowed: false}
		quotaHandler := handlers.KTPOCRHandler(engine, ocrStore, quotaChecker)

		req := createMultipartRequest(t, "document", "ktp.png", validImage)
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		quotaHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("expected 429, got %d", rec.Code)
		}

		var envelope response.ErrorEnvelope
		_ = json.NewDecoder(rec.Body).Decode(&envelope)
		if envelope.Error.Code != response.CodeQuotaExceeded {
			t.Errorf("expected code quota_exceeded, got %s", envelope.Error.Code)
		}
	})

	t.Run("quota check error returns 500 internal_error", func(t *testing.T) {
		quotaChecker := &mockQuotaChecker{err: errors.New("quota db timeout")}
		quotaHandler := handlers.KTPOCRHandler(engine, ocrStore, quotaChecker)

		req := createMultipartRequest(t, "document", "ktp.png", validImage)
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		quotaHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
	})
}

func TestGetOCRRequestHandler(t *testing.T) {
	ocrStore := newDummyOCRStore()
	handler := handlers.GetOCRRequestHandler(ocrStore)

	testReq := &store.OCRRequest{
		ID:         "req-12345",
		OrgID:      "org-1",
		Status:     "completed",
		Confidence: 0.98,
		LatencyMS:  150,
		DocType:    "ktp",
	}
	_ = ocrStore.CreateOCRRequest(context.Background(), testReq)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/ocr/req-12345", nil)
		req.SetPathValue("id", "req-12345")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("missing id parameter returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/ocr/", nil)
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("nil store returns 500", func(t *testing.T) {
		nilHandler := handlers.GetOCRRequestHandler(nil)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/ocr/req-12345", nil)
		req.SetPathValue("id", "req-12345")
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		nilHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
	})

	t.Run("not found returns 404 invalid_request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/ocr/req-unknown", nil)
		req.SetPathValue("id", "req-unknown")
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}

		var errEnv response.ErrorEnvelope
		_ = json.NewDecoder(rec.Body).Decode(&errEnv)
		if errEnv.Error.Code != response.CodeInvalidRequest {
			t.Errorf("expected code invalid_request, got %s", errEnv.Error.Code)
		}
	})

	t.Run("store query error returns 500", func(t *testing.T) {
		failingStore := &dummyOCRStore{err: errors.New("db broken")}
		errHandler := handlers.GetOCRRequestHandler(failingStore)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/ocr/req-12345", nil)
		req.SetPathValue("id", "req-12345")
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		errHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
	})

	t.Run("successful retrieval returns 200 with metadata", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/ocr/req-12345", nil)
		req.SetPathValue("id", "req-12345")
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var meta handlers.OCRRequestMetadata
		if err := json.NewDecoder(rec.Body).Decode(&meta); err != nil {
			t.Fatalf("failed decoding json: %v", err)
		}
		if meta.ID != "req-12345" || meta.Status != "completed" || meta.DocType != "ktp" {
			t.Errorf("unexpected metadata: %+v", meta)
		}
		if meta.Confidence != 0.98 || meta.LatencyMS != 150 {
			t.Errorf("unexpected confidence or latency: %+v", meta)
		}
	})
}

func BenchmarkKTPOCRHandler(b *testing.B) {
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer slog.SetDefault(oldLogger)

	validImage := synthetic.GenerateValidKTPImage()
	engine := providers.NewMockEngine()
	ocrStore := newDummyOCRStore()
	quotaChecker := &mockQuotaChecker{allowed: true, remaining: 100, limit: 100}
	handler := handlers.KTPOCRHandler(engine, ocrStore, quotaChecker)

	var bodyBuf bytes.Buffer
	writer := multipart.NewWriter(&bodyBuf)
	part, err := writer.CreateFormFile("document", "ktp.png")
	if err != nil {
		b.Fatalf("failed creating form file: %v", err)
	}
	if _, err := part.Write(validImage); err != nil {
		b.Fatalf("failed writing image: %v", err)
	}
	writer.Close()
	rawBody := bodyBuf.Bytes()
	contentType := writer.FormDataContentType()

	key := &store.APIKey{
		ID:     "bench-key",
		OrgID:  "bench-org",
		Scopes: []string{"ocr:read", "ocr:write"},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/ocr/ktp", bytes.NewReader(rawBody))
		req.Header.Set("Content-Type", contentType)
		req = req.WithContext(middleware.WithAPIKey(req.Context(), key))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			b.Fatalf("expected 200, got %d", rec.Code)
		}
	}
}

func TestSIMOCRHandler(t *testing.T) {
	validImage := synthetic.GenerateValidSIMImage()
	engine := providers.NewMockEngine()
	ocrStore := newDummyOCRStore()
	handler := handlers.SIMOCRHandler(engine, ocrStore, nil)

	t.Run("unauthenticated request returns 401", func(t *testing.T) {
		req := createMultipartRequest(t, "document", "sim.png", validImage)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("bad body returns 400 invalid_document", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/ocr/sim", bytes.NewReader([]byte("not multipart")))
		req.Header.Set("Content-Type", "multipart/form-data; boundary=missing")
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("missing document form file returns 400", func(t *testing.T) {
		req := createMultipartRequest(t, "other_file", "doc.png", validImage)
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("corrupt image returns 400 invalid_document", func(t *testing.T) {
		corruptImg := synthetic.GenerateCorruptedImage()
		req := createMultipartRequest(t, "document", "corrupt.jpg", corruptImg)
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("nil engine returns 502", func(t *testing.T) {
		nilHandler := handlers.SIMOCRHandler(nil, ocrStore, nil)
		req := createMultipartRequest(t, "document", "sim.png", validImage)
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		nilHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadGateway {
			t.Fatalf("expected 502, got %d", rec.Code)
		}
	})

	t.Run("unsupported document returns 422", func(t *testing.T) {
		unsupportedImg := synthetic.GenerateUnsupportedDocImage()
		req := createMultipartRequest(t, "document", "unsupported.png", unsupportedImg)
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected 422, got %d", rec.Code)
		}
	})

	t.Run("circuit breaker open returns 504", func(t *testing.T) {
		engine.SetCustomError(ocr.ErrCircuitOpen)
		defer engine.Reset()

		req := createMultipartRequest(t, "document", "sim.png", validImage)
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusGatewayTimeout {
			t.Fatalf("expected 504 Gateway Timeout, got %d", rec.Code)
		}
	})

	t.Run("engine failure returns 502", func(t *testing.T) {
		failImg := synthetic.GenerateOCRFailureImage()
		req := createMultipartRequest(t, "document", "fail.png", failImg)
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadGateway {
			t.Fatalf("expected 502, got %d", rec.Code)
		}
	})

	t.Run("engine timeout returns 504", func(t *testing.T) {
		engine.SetCustomError(context.DeadlineExceeded)
		defer engine.Reset()

		req := createMultipartRequest(t, "document", "sim.png", validImage)
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusGatewayTimeout {
			t.Fatalf("expected 504 Gateway Timeout, got %d", rec.Code)
		}
	})

	t.Run("arbitrary engine internal error returns 500", func(t *testing.T) {
		engine.SetCustomError(errors.New("unexpected crash"))
		defer engine.Reset()

		req := createMultipartRequest(t, "document", "sim.png", validImage)
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
	})

	t.Run("engine returns non-sim document_type returns 422", func(t *testing.T) {
		engine.SetCustomResult(&ocr.OCRResult{
			DocumentType: "ktp",
		})
		defer engine.Reset()

		req := createMultipartRequest(t, "document", "sim.png", validImage)
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected 422, got %d", rec.Code)
		}
	})

	t.Run("successful extraction returns 200 with SIMResponse and persists metadata", func(t *testing.T) {
		req := createMultipartRequest(t, "document", "sim.png", validImage)
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
		}

		var simResp handlers.SIMResponse
		if err := json.NewDecoder(rec.Body).Decode(&simResp); err != nil {
			t.Fatalf("failed decoding json response: %v", err)
		}

		if simResp.DocumentType != "sim" {
			t.Errorf("expected doc_type sim, got %s", simResp.DocumentType)
		}
		if simResp.Data == nil || simResp.Data.NomorSIM != "123456789012" {
			t.Errorf("expected NomorSIM 123456789012, got %v", simResp.Data)
		}
		if simResp.Confidence < 0.90 {
			t.Errorf("expected high confidence >= 0.90, got %f", simResp.Confidence)
		}
	})

	t.Run("low confidence extraction succeeds with low_confidence status", func(t *testing.T) {
		lowConfImg := synthetic.GenerateSIMLowConfidenceImage()
		req := createMultipartRequest(t, "document", "sim_low.png", lowConfImg)
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var simResp handlers.SIMResponse
		if err := json.NewDecoder(rec.Body).Decode(&simResp); err != nil {
			t.Fatalf("failed decoding json: %v", err)
		}
		if simResp.Confidence >= 0.70 {
			t.Errorf("expected low confidence < 0.70, got %f", simResp.Confidence)
		}
	})

	t.Run("store error is logged but request succeeds", func(t *testing.T) {
		brokenStore := &dummyOCRStore{
			records: make(map[string]*store.OCRRequest),
			err:     errors.New("db disk failure"),
		}
		brokenHandler := handlers.SIMOCRHandler(engine, brokenStore, nil)

		req := createMultipartRequest(t, "document", "sim.png", validImage)
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		brokenHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 despite store error, got %d", rec.Code)
		}
	})

	t.Run("quota exceeded returns 429", func(t *testing.T) {
		quotaChecker := &mockQuotaChecker{allowed: false, remaining: 0, limit: 100}
		quotaHandler := handlers.SIMOCRHandler(engine, ocrStore, quotaChecker)

		req := createMultipartRequest(t, "document", "sim.png", validImage)
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		quotaHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("expected 429, got %d", rec.Code)
		}
	})

	t.Run("quota checker returns error returns 500", func(t *testing.T) {
		quotaChecker := &mockQuotaChecker{err: errors.New("redis timeout")}
		quotaHandler := handlers.SIMOCRHandler(engine, ocrStore, quotaChecker)

		req := createMultipartRequest(t, "document", "sim.png", validImage)
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		quotaHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
	})
}

func BenchmarkSIMOCRHandler(b *testing.B) {
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer slog.SetDefault(oldLogger)

	validImage := synthetic.GenerateValidSIMImage()
	engine := providers.NewMockEngine()
	ocrStore := newDummyOCRStore()
	quotaChecker := &mockQuotaChecker{allowed: true, remaining: 100, limit: 100}
	handler := handlers.SIMOCRHandler(engine, ocrStore, quotaChecker)

	var bodyBuf bytes.Buffer
	writer := multipart.NewWriter(&bodyBuf)
	part, err := writer.CreateFormFile("document", "sim.png")
	if err != nil {
		b.Fatalf("failed creating form file: %v", err)
	}
	if _, err := part.Write(validImage); err != nil {
		b.Fatalf("failed writing image: %v", err)
	}
	writer.Close()
	rawBody := bodyBuf.Bytes()
	contentType := writer.FormDataContentType()

	key := &store.APIKey{
		ID:     "bench-key",
		OrgID:  "bench-org",
		Scopes: []string{"ocr:read", "ocr:write"},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/ocr/sim", bytes.NewReader(rawBody))
		req.Header.Set("Content-Type", contentType)
		req = req.WithContext(middleware.WithAPIKey(req.Context(), key))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			b.Fatalf("expected 200, got %d", rec.Code)
		}
	}
}

func TestPassportOCRHandler(t *testing.T) {
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer slog.SetDefault(oldLogger)

	engine := providers.NewMockEngine()
	ocrStore := newDummyOCRStore()
	quotaChecker := &mockQuotaChecker{allowed: true, remaining: 100, limit: 100}
	handler := handlers.PassportOCRHandler(engine, ocrStore, quotaChecker)

	validImage := synthetic.GenerateValidPassportImage()

	t.Run("successful passport extraction 200 OK", func(t *testing.T) {
		req := createMultipartRequest(t, "document", "passport.png", validImage)
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp handlers.PassportResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed unmarshaling response: %v", err)
		}
		if resp.DocumentType != "passport" {
			t.Errorf("expected doc_type passport, got %s", resp.DocumentType)
		}
		if resp.Data == nil || resp.Data.PassportNumber != "X1234567" {
			t.Errorf("expected PassportNumber X1234567, got %v", resp.Data)
		}
		if resp.Confidence < 0.90 {
			t.Errorf("expected confidence >= 0.90, got %f", resp.Confidence)
		}
		if resp.ID == "" {
			t.Error("expected non-empty record ID")
		}

		// Verify record persisted in store
		saved, err := ocrStore.GetOCRRequestByID(context.Background(), "org-1", resp.ID)
		if err != nil || saved.DocType != "passport" {
			t.Fatalf("expected saved record with doc_type passport, got %v, err=%v", saved, err)
		}
	})

	t.Run("low confidence extraction", func(t *testing.T) {
		lowConfImg := synthetic.GeneratePassportLowConfidenceImage()
		req := createMultipartRequest(t, "document", "passport_low.png", lowConfImg)
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("unauthenticated request returns 401", func(t *testing.T) {
		req := createMultipartRequest(t, "document", "passport.png", validImage)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("quota checker error returns 500", func(t *testing.T) {
		errChecker := &mockQuotaChecker{err: errors.New("db failure")}
		errHandler := handlers.PassportOCRHandler(engine, ocrStore, errChecker)

		req := createMultipartRequest(t, "document", "passport.png", validImage)
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		errHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
	})

	t.Run("quota exceeded returns 429", func(t *testing.T) {
		exceededChecker := &mockQuotaChecker{allowed: false, remaining: 0, limit: 100}
		exceededHandler := handlers.PassportOCRHandler(engine, ocrStore, exceededChecker)

		req := createMultipartRequest(t, "document", "passport.png", validImage)
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		exceededHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("expected 429, got %d", rec.Code)
		}
	})

	t.Run("bad body returns 400 invalid_document", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/ocr/passport", bytes.NewReader([]byte("not multipart")))
		req.Header.Set("Content-Type", "multipart/form-data; boundary=missing")
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("missing document form file returns 400", func(t *testing.T) {
		req := createMultipartRequest(t, "other_file", "doc.png", validImage)
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("corrupt image returns 400 invalid_document", func(t *testing.T) {
		corruptImg := synthetic.GenerateCorruptedImage()
		req := createMultipartRequest(t, "document", "corrupt.jpg", corruptImg)
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("nil engine returns 502", func(t *testing.T) {
		nilHandler := handlers.PassportOCRHandler(nil, ocrStore, nil)
		req := createMultipartRequest(t, "document", "passport.png", validImage)
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		nilHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadGateway {
			t.Fatalf("expected 502, got %d", rec.Code)
		}
	})

	t.Run("engine returns ErrUnsupportedPassportDocument 422", func(t *testing.T) {
		engine.SetCustomError(ocr.ErrUnsupportedPassportDocument)
		defer engine.Reset()

		req := createMultipartRequest(t, "document", "passport.png", validImage)
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected 422, got %d", rec.Code)
		}
	})

	t.Run("engine returns ErrCircuitOpen 504", func(t *testing.T) {
		engine.SetCustomError(ocr.ErrCircuitOpen)
		defer engine.Reset()

		req := createMultipartRequest(t, "document", "passport.png", validImage)
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusGatewayTimeout {
			t.Fatalf("expected 504, got %d", rec.Code)
		}
	})

	t.Run("engine returns context deadline exceeded 504", func(t *testing.T) {
		engine.SetCustomError(context.DeadlineExceeded)
		defer engine.Reset()

		req := createMultipartRequest(t, "document", "passport.png", validImage)
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusGatewayTimeout {
			t.Fatalf("expected 504, got %d", rec.Code)
		}
	})

	t.Run("engine returns ErrOCRFailed 502", func(t *testing.T) {
		engine.SetCustomError(ocr.ErrOCRFailed)
		defer engine.Reset()

		req := createMultipartRequest(t, "document", "passport.png", validImage)
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadGateway {
			t.Fatalf("expected 502, got %d", rec.Code)
		}
	})

	t.Run("engine returns generic error 500", func(t *testing.T) {
		engine.SetCustomError(errors.New("something catastrophic"))
		defer engine.Reset()

		req := createMultipartRequest(t, "document", "passport.png", validImage)
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
	})

	t.Run("engine returns wrong document type 422", func(t *testing.T) {
		engine.SetCustomResult(&ocr.OCRResult{
			DocumentType: "ktp",
		})
		defer engine.Reset()

		req := createMultipartRequest(t, "document", "passport.png", validImage)
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected 422, got %d", rec.Code)
		}
	})

	t.Run("ocrStore error does not fail request", func(t *testing.T) {
		brokenStore := &dummyOCRStore{err: errors.New("db connection failure")}
		storeHandler := handlers.PassportOCRHandler(engine, brokenStore, quotaChecker)

		req := createMultipartRequest(t, "document", "passport.png", validImage)
		req = withAuth(req, "org-1", "key-1")
		rec := httptest.NewRecorder()

		storeHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 even if store failed, got %d", rec.Code)
		}
	})
}

func BenchmarkPassportOCRHandler(b *testing.B) {
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer slog.SetDefault(oldLogger)

	validImage := synthetic.GenerateValidPassportImage()
	engine := providers.NewMockEngine()
	ocrStore := newDummyOCRStore()
	quotaChecker := &mockQuotaChecker{allowed: true, remaining: 100, limit: 100}
	handler := handlers.PassportOCRHandler(engine, ocrStore, quotaChecker)

	var bodyBuf bytes.Buffer
	writer := multipart.NewWriter(&bodyBuf)
	part, err := writer.CreateFormFile("document", "passport.png")
	if err != nil {
		b.Fatalf("failed creating form file: %v", err)
	}
	if _, err := part.Write(validImage); err != nil {
		b.Fatalf("failed writing image: %v", err)
	}
	writer.Close()
	rawBody := bodyBuf.Bytes()
	contentType := writer.FormDataContentType()

	key := &store.APIKey{
		ID:     "bench-key",
		OrgID:  "bench-org",
		Scopes: []string{"ocr:read", "ocr:write"},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/ocr/passport", bytes.NewReader(rawBody))
		req.Header.Set("Content-Type", contentType)
		req = req.WithContext(middleware.WithAPIKey(req.Context(), key))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			b.Fatalf("expected 200, got %d", rec.Code)
		}
	}
}
