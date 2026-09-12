package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/apikey"
	internalhttp "github.com/amirfaisalz/lensio/apps/api/internal/http"
	"github.com/amirfaisalz/lensio/apps/api/internal/http/response"
	"github.com/amirfaisalz/lensio/apps/api/internal/store"
	"github.com/amirfaisalz/lensio/services/ocr"
	"github.com/amirfaisalz/lensio/services/ocr/providers"
	"github.com/amirfaisalz/lensio/tests/fixtures/synthetic"
)

// mockFailingPinger simulates a failing database pinger.
type mockFailingPinger struct{}

func (m *mockFailingPinger) PingContext(ctx context.Context) error {
	return errors.New("connection to postgres closed unexpectedly")
}

// mockHealthyPinger simulates a successful database pinger.
type mockHealthyPinger struct{}

func (m *mockHealthyPinger) PingContext(ctx context.Context) error {
	return nil
}

// mockFailingKeyStore simulates a database store that fails all queries during an outage.
type mockFailingKeyStore struct{}

func (m *mockFailingKeyStore) CreateAPIKey(ctx context.Context, key *store.APIKey) error {
	return errors.New("database connection refused")
}

func (m *mockFailingKeyStore) GetAPIKeyByHash(ctx context.Context, keyHash string) (*store.APIKey, error) {
	return nil, errors.New("database connection refused")
}

func (m *mockFailingKeyStore) ListAPIKeysByOrg(ctx context.Context, orgID string) ([]*store.APIKey, error) {
	return nil, errors.New("database connection refused")
}

func (m *mockFailingKeyStore) RevokeAPIKey(ctx context.Context, orgID, keyID string) error {
	return errors.New("database connection refused")
}

func (m *mockFailingKeyStore) TouchAPIKeyLastUsed(ctx context.Context, keyID string, lastUsed time.Time) error {
	return errors.New("database connection refused")
}

// mockStaticKeyStore provides an in-memory key store with a single active test key.
type mockStaticKeyStore struct {
	key *store.APIKey
}

func (m *mockStaticKeyStore) CreateAPIKey(ctx context.Context, key *store.APIKey) error {
	return nil
}

func (m *mockStaticKeyStore) GetAPIKeyByHash(ctx context.Context, keyHash string) (*store.APIKey, error) {
	if m.key != nil && m.key.KeyHash == keyHash {
		return m.key, nil
	}
	return nil, store.ErrNotFound
}

func (m *mockStaticKeyStore) ListAPIKeysByOrg(ctx context.Context, orgID string) ([]*store.APIKey, error) {
	if m.key != nil {
		return []*store.APIKey{m.key}, nil
	}
	return nil, nil
}

func (m *mockStaticKeyStore) RevokeAPIKey(ctx context.Context, orgID, keyID string) error {
	return nil
}

func (m *mockStaticKeyStore) TouchAPIKeyLastUsed(ctx context.Context, keyID string, lastUsed time.Time) error {
	return nil
}

func createMultipartUpload(fieldName, filename string, content []byte) (*bytes.Buffer, string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(fieldName, filename)
	if err != nil {
		return nil, "", err
	}
	if _, err := part.Write(content); err != nil {
		return nil, "", err
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return &body, writer.FormDataContentType(), nil
}

// -----------------------------------------------------------------------------
// Scenario A: OCR Provider Failure / Timeout (PRD Section 26)
// -----------------------------------------------------------------------------
func TestDrill_ScenarioA_OCRProviderFailureAndTimeout(t *testing.T) {
	validImage := synthetic.GenerateValidKTPImage()
	rawKey := "lensio_live_testkey_scenario_a_12345678"
	testKeyHash := apikey.Hash(rawKey)
	keyStore := &mockStaticKeyStore{
		key: &store.APIKey{
			ID:          "key-scenario-a",
			OrgID:       "org-drill-a",
			KeyHash:     testKeyHash,
			Prefix:      "lensio_live_",
			Scopes:      []string{"ocr:write", "ocr:read", "usage:read"},
			Environment: "live",
			CreatedAt:   time.Now(),
		},
	}

	mockEngine := providers.NewMockEngine()
	router := internalhttp.NewRouterWithDeps(internalhttp.RouterDeps{
		KeyStore:  keyStore,
		OCREngine: mockEngine,
	})
	ts := httptest.NewServer(router)
	defer ts.Close()

	client := &http.Client{Timeout: 5 * time.Second}

	t.Run("Subscenario A1: Engine Timeout triggers HTTP 504 and zero credential leaks", func(t *testing.T) {
		// Inject timeout into mock OCR engine
		mockEngine.SetCustomError(context.DeadlineExceeded)
		defer mockEngine.Reset()

		body, contentType, err := createMultipartUpload("document", "ktp.png", validImage)
		if err != nil {
			t.Fatalf("failed preparing multipart upload: %v", err)
		}

		req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/ocr/ktp", body)
		if err != nil {
			t.Fatalf("failed creating request: %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+rawKey)
		req.Header.Set("Content-Type", contentType)

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusGatewayTimeout {
			t.Fatalf("expected HTTP 504 Gateway Timeout, got %d", resp.StatusCode)
		}

		respBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed reading response body: %v", err)
		}

		var errEnvelope response.ErrorEnvelope
		if err := json.Unmarshal(respBytes, &errEnvelope); err != nil {
			t.Fatalf("failed parsing error envelope: %v", err)
		}

		if errEnvelope.Error.Code != response.CodeOCRFailed {
			t.Errorf("expected error code %q, got %q", response.CodeOCRFailed, errEnvelope.Error.Code)
		}
		if errEnvelope.Error.Message != "OCR engine processing timeout" {
			t.Errorf("expected clean error message, got %q", errEnvelope.Error.Message)
		}

		// Verify zero credential / URL leakage
		bodyStr := string(respBytes)
		leaks := []string{"AIzaSy", "googleapis.com", "gemini", "key=", "secret"}
		for _, leak := range leaks {
			if strings.Contains(strings.ToLower(bodyStr), strings.ToLower(leak)) {
				t.Fatalf("security violation: response leaked provider secret or internal URL %q in body: %s", leak, bodyStr)
			}
		}
	})

	t.Run("Subscenario A2: Provider Crash / 500 triggers HTTP 502 with ocr_failed", func(t *testing.T) {
		// Inject provider crash error
		mockEngine.SetCustomError(ocr.ErrOCRFailed)
		defer mockEngine.Reset()

		body, contentType, err := createMultipartUpload("document", "ktp.png", validImage)
		if err != nil {
			t.Fatalf("failed preparing multipart upload: %v", err)
		}

		req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/ocr/ktp", body)
		if err != nil {
			t.Fatalf("failed creating request: %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+rawKey)
		req.Header.Set("Content-Type", contentType)

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadGateway {
			t.Fatalf("expected HTTP 502 Bad Gateway, got %d", resp.StatusCode)
		}

		var errEnvelope response.ErrorEnvelope
		if err := json.NewDecoder(resp.Body).Decode(&errEnvelope); err != nil {
			t.Fatalf("failed parsing error envelope: %v", err)
		}
		if errEnvelope.Error.Code != response.CodeOCRFailed {
			t.Errorf("expected error code %q, got %q", response.CodeOCRFailed, errEnvelope.Error.Code)
		}
	})
}

// -----------------------------------------------------------------------------
// Scenario B: Database Outage (PRD Section 26)
// -----------------------------------------------------------------------------
func TestDrill_ScenarioB_DatabaseOutage(t *testing.T) {
	// Router initialized with failing database pinger and store
	router := internalhttp.NewRouterWithDeps(internalhttp.RouterDeps{
		Pinger:   &mockFailingPinger{},
		KeyStore: &mockFailingKeyStore{},
	})
	ts := httptest.NewServer(router)
	defer ts.Close()

	client := &http.Client{Timeout: 3 * time.Second}

	t.Run("Subscenario B1: Liveness /health remains HTTP 200 OK during DB outage", func(t *testing.T) {
		resp, err := client.Get(ts.URL + "/health")
		if err != nil {
			t.Fatalf("failed calling /health: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected HTTP 200 from /health, got %d", resp.StatusCode)
		}

		var health struct {
			Status string `json:"status"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
			t.Fatalf("failed decoding /health json: %v", err)
		}
		if health.Status != "ok" {
			t.Fatalf("expected status 'ok', got %q", health.Status)
		}
	})

	t.Run("Subscenario B2: Readiness /ready returns HTTP 503 Service Unavailable", func(t *testing.T) {
		resp, err := client.Get(ts.URL + "/ready")
		if err != nil {
			t.Fatalf("failed calling /ready: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Fatalf("expected HTTP 503 Service Unavailable from /ready, got %d", resp.StatusCode)
		}

		var ready struct {
			Status   string `json:"status"`
			Database string `json:"database"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&ready); err != nil {
			t.Fatalf("failed decoding /ready json: %v", err)
		}
		if ready.Status != "unready" || ready.Database != "disconnected" {
			t.Fatalf("unexpected ready response during DB outage: %+v", ready)
		}
	})

	t.Run("Subscenario B3: Authenticated traffic safely rejected with controlled 500 internal_error", func(t *testing.T) {
		validImage := synthetic.GenerateValidKTPImage()
		body, contentType, err := createMultipartUpload("document", "ktp.png", validImage)
		if err != nil {
			t.Fatalf("failed preparing multipart upload: %v", err)
		}

		req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/ocr/ktp", body)
		if err != nil {
			t.Fatalf("failed creating request: %v", err)
		}
		req.Header.Set("Authorization", "Bearer lensio_live_anyvalidlookingkey12345678")
		req.Header.Set("Content-Type", contentType)

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusInternalServerError {
			t.Fatalf("expected HTTP 500 Internal Server Error when DB is down, got %d", resp.StatusCode)
		}

		var errEnvelope response.ErrorEnvelope
		if err := json.NewDecoder(resp.Body).Decode(&errEnvelope); err != nil {
			t.Fatalf("failed decoding error response: %v", err)
		}

		if errEnvelope.Error.Code != response.CodeInternalError {
			t.Errorf("expected error code %q, got %q", response.CodeInternalError, errEnvelope.Error.Code)
		}
		if !strings.Contains(errEnvelope.Error.Message, "verify API key") {
			t.Errorf("unexpected error message: %q", errEnvelope.Error.Message)
		}
	})
}

// -----------------------------------------------------------------------------
// Scenario D: Production Regression Drill Metrics Probe
// -----------------------------------------------------------------------------
func TestDrill_ScenarioD_ProductionRegressionMetrics(t *testing.T) {
	router := internalhttp.NewRouterWithDeps(internalhttp.RouterDeps{
		Pinger: &mockHealthyPinger{},
	})
	ts := httptest.NewServer(router)
	defer ts.Close()

	client := &http.Client{Timeout: 3 * time.Second}

	// Verify Prometheus metrics endpoint exposes metrics for anomaly detection
	metricsResp, err := client.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatalf("failed fetching /metrics: %v", err)
	}
	defer metricsResp.Body.Close()

	if metricsResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from /metrics, got %d", metricsResp.StatusCode)
	}

	metricsBytes, err := io.ReadAll(metricsResp.Body)
	if err != nil {
		t.Fatalf("failed reading /metrics: %v", err)
	}

	metricsStr := string(metricsBytes)
	if !strings.Contains(metricsStr, "promhttp_metric_handler_requests_total") && !strings.Contains(metricsStr, "lensio_") {
		t.Errorf("expected standard prometheus metrics in output, got: %s", metricsStr[:200])
	}
}

// -----------------------------------------------------------------------------
// Scenario E: OCR Provider Latency Cascade & Circuit Breaker Protection (PRD Phase 11.6)
// -----------------------------------------------------------------------------
func TestDrill_ScenarioE_CircuitBreakerTrippingAndFastFail(t *testing.T) {
	validImage := synthetic.GenerateValidKTPImage()
	rawKey := "lensio_live_testkey_scenario_e_12345678"
	testKeyHash := apikey.Hash(rawKey)
	keyStore := &mockStaticKeyStore{
		key: &store.APIKey{
			ID:          "key-scenario-e",
			OrgID:       "org-drill-e",
			KeyHash:     testKeyHash,
			Prefix:      "lensio_live_",
			Scopes:      []string{"ocr:write", "ocr:read"},
			Environment: "live",
			CreatedAt:   time.Now(),
		},
	}

	mockEngine := providers.NewMockEngine()
	cb := ocr.NewCircuitBreaker(mockEngine, ocr.CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Cooldown:         10 * time.Second,
	})

	router := internalhttp.NewRouterWithDeps(internalhttp.RouterDeps{
		KeyStore:  keyStore,
		OCREngine: cb,
	})
	ts := httptest.NewServer(router)
	defer ts.Close()

	client := &http.Client{Timeout: 5 * time.Second}

	// 1. Inject upstream timeout into mock engine
	mockEngine.SetCustomError(context.DeadlineExceeded)

	for i := 1; i <= 3; i++ {
		body, contentType, err := createMultipartUpload("document", "ktp.png", validImage)
		if err != nil {
			t.Fatalf("failed preparing multipart upload: %v", err)
		}
		req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/ocr/ktp", body)
		if err != nil {
			t.Fatalf("failed creating request %d: %v", i, err)
		}
		req.Header.Set("Authorization", "Bearer "+rawKey)
		req.Header.Set("Content-Type", contentType)

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusGatewayTimeout {
			t.Fatalf("request %d: expected 504 Gateway Timeout, got %d", i, resp.StatusCode)
		}
	}

	// 2. Verify breaker has tripped to StateOpen
	if cb.State() != ocr.StateOpen {
		t.Fatalf("expected breaker to be in StateOpen after 3 consecutive failures, got %v", cb.State())
	}

	// 3. 4th request must fast-fail in < 50ms without hitting mock engine
	initialEngineCalls := mockEngine.GetCallCount()
	fastFailStart := time.Now()

	body, contentType, err := createMultipartUpload("document", "ktp.png", validImage)
	if err != nil {
		t.Fatalf("failed preparing multipart upload: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/ocr/ktp", body)
	if err != nil {
		t.Fatalf("failed creating 4th request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+rawKey)
	req.Header.Set("Content-Type", contentType)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("fast-fail request failed: %v", err)
	}
	defer resp.Body.Close()
	fastFailDuration := time.Since(fastFailStart)

	if resp.StatusCode != http.StatusGatewayTimeout {
		t.Fatalf("expected HTTP 504 Gateway Timeout on fast-fail, got %d", resp.StatusCode)
	}
	if fastFailDuration > 50*time.Millisecond {
		t.Errorf("expected fast-fail in < 50ms, took %v", fastFailDuration)
	}

	// Verify underlying engine was NOT called
	if mockEngine.GetCallCount() != initialEngineCalls {
		t.Errorf("expected mock engine call count to remain %d, but changed to %d",
			initialEngineCalls, mockEngine.GetCallCount())
	}

	var errEnvelope response.ErrorEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&errEnvelope); err != nil {
		t.Fatalf("failed decoding error response: %v", err)
	}
	if errEnvelope.Error.Code != response.CodeOCRFailed {
		t.Errorf("expected error code %q, got %q", response.CodeOCRFailed, errEnvelope.Error.Code)
	}
	if !strings.Contains(errEnvelope.Error.Message, "circuit breaker is open") {
		t.Errorf("expected circuit breaker message, got %q", errEnvelope.Error.Message)
	}

	// 4. Verify Prometheus metrics expose circuit breaker state
	metricsResp, err := client.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatalf("failed fetching /metrics: %v", err)
	}
	defer metricsResp.Body.Close()

	metricsBytes, err := io.ReadAll(metricsResp.Body)
	if err != nil {
		t.Fatalf("failed reading /metrics: %v", err)
	}
	metricsStr := string(metricsBytes)
	if !strings.Contains(metricsStr, "lensio_ocr_circuit_breaker_state 2") {
		t.Errorf("expected metrics to contain 'lensio_ocr_circuit_breaker_state 2', got:\n%s", metricsStr)
	}
	if !strings.Contains(metricsStr, "lensio_ocr_circuit_breaker_tripped_total") {
		t.Errorf("expected metrics to contain 'lensio_ocr_circuit_breaker_tripped_total', got:\n%s", metricsStr)
	}

	// 5. Reset circuit breaker and restore mock engine
	mockEngine.Reset()
	cb.Reset()
	if cb.State() != ocr.StateClosed {
		t.Fatalf("expected breaker to be StateClosed after Reset, got %v", cb.State())
	}
}

