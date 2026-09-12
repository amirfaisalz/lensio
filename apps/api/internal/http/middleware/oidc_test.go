package middleware_test

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/apikey"
	"github.com/amirfaisalz/lensio/apps/api/internal/http/middleware"
	"github.com/amirfaisalz/lensio/apps/api/internal/http/response"
	"github.com/amirfaisalz/lensio/apps/api/internal/store"
)

// Helper to generate an RSA 2048 key pair
func generateRSAKey(t *testing.T) (*rsa.PrivateKey, *rsa.PublicKey) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed generating rsa key: %v", err)
	}
	return priv, &priv.PublicKey
}

// Helper to encode to unpadded base64url
func b64url(data []byte) string {
	return strings.TrimRight(base64.RawURLEncoding.EncodeToString(data), "=")
}

// Helper to generate a signed JWT
func signJWT(t *testing.T, priv *rsa.PrivateKey, kid string, alg string, claims map[string]any) string {
	t.Helper()
	header := map[string]any{
		"alg": alg,
		"kid": kid,
		"typ": "JWT",
	}
	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(claims)

	headerB64 := b64url(headerJSON)
	claimsB64 := b64url(claimsJSON)
	signingInput := headerB64 + "." + claimsB64

	hashed := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, hashed[:])
	if err != nil {
		t.Fatalf("failed signing jwt: %v", err)
	}
	sigB64 := b64url(sig)

	return signingInput + "." + sigB64
}

func TestOIDCUserContext(t *testing.T) {
	// Nil context
	if user := middleware.GetOIDCUser(nil); user != nil {
		t.Errorf("expected nil for nil context, got %v", user)
	}

	// Empty context
	if user := middleware.GetOIDCUser(context.Background()); user != nil {
		t.Errorf("expected nil for empty context, got %v", user)
	}

	// Injected user
	expected := &middleware.OIDCUser{
		Subject:           "sub-123",
		Email:             "admin@lensio.dev",
		PreferredUsername: "admin",
	}
	ctx := middleware.WithOIDCUser(context.Background(), expected)
	actual := middleware.GetOIDCUser(ctx)
	if actual == nil || actual.Subject != expected.Subject || actual.Email != expected.Email {
		t.Errorf("expected %v, got %v", expected, actual)
	}
}

func TestOIDCValidator_ValidateToken_Success(t *testing.T) {
	priv, pub := generateRSAKey(t)
	validator := middleware.NewStaticOIDCValidator(map[string]*rsa.PublicKey{
		"key-1": pub,
	})

	claims := map[string]any{
		"sub":                "user-uuid-1",
		"email":              "admin@lensio.dev",
		"email_verified":     true,
		"preferred_username": "admin",
		"name":               "Admin User",
		"iss":                "http://localhost:8082/realms/lensio",
		"roles":              []string{"admin"},
		"realm_access": map[string]any{
			"roles": []string{"developer", "admin"},
		},
		"exp": time.Now().Add(1 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}

	token := signJWT(t, priv, "key-1", "RS256", claims)

	user, err := validator.ValidateToken(context.Background(), token)
	if err != nil {
		t.Fatalf("expected valid token, got error: %v", err)
	}

	if user.Subject != "user-uuid-1" {
		t.Errorf("expected sub 'user-uuid-1', got '%s'", user.Subject)
	}
	if user.Email != "admin@lensio.dev" {
		t.Errorf("expected email 'admin@lensio.dev', got '%s'", user.Email)
	}
	if user.PreferredUsername != "admin" {
		t.Errorf("expected preferred_username 'admin', got '%s'", user.PreferredUsername)
	}
	if !user.EmailVerified {
		t.Errorf("expected email_verified true")
	}
	if len(user.Roles) != 2 {
		t.Errorf("expected 2 deduplicated roles, got %v", user.Roles)
	}
}

func TestOIDCValidator_ValidateToken_Errors(t *testing.T) {
	priv, pub := generateRSAKey(t)
	validator := middleware.NewStaticOIDCValidator(map[string]*rsa.PublicKey{
		"key-1": pub,
	})

	t.Run("malformed token structure", func(t *testing.T) {
		_, err := validator.ValidateToken(context.Background(), "invalid.token")
		if err == nil || !strings.Contains(err.Error(), "malformed") {
			t.Errorf("expected malformed token error, got %v", err)
		}
	})

	t.Run("invalid header base64", func(t *testing.T) {
		_, err := validator.ValidateToken(context.Background(), "@@@.payload.sig")
		if err == nil {
			t.Errorf("expected error for invalid header base64")
		}
	})

	t.Run("invalid header json", func(t *testing.T) {
		badHeader := b64url([]byte("not a json"))
		_, err := validator.ValidateToken(context.Background(), badHeader+".payload.sig")
		if err == nil {
			t.Errorf("expected error for invalid header json")
		}
	})

	t.Run("unsupported algorithm", func(t *testing.T) {
		token := signJWT(t, priv, "key-1", "HS256", map[string]any{"sub": "123"})
		_, err := validator.ValidateToken(context.Background(), token)
		if err == nil || !strings.Contains(err.Error(), "unsupported") {
			t.Errorf("expected unsupported algorithm error, got %v", err)
		}
	})

	t.Run("key not found", func(t *testing.T) {
		token := signJWT(t, priv, "unknown-kid", "RS256", map[string]any{"sub": "123"})
		_, err := validator.ValidateToken(context.Background(), token)
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected key not found error, got %v", err)
		}
	})

	t.Run("invalid signature base64", func(t *testing.T) {
		token := signJWT(t, priv, "key-1", "RS256", map[string]any{"sub": "123"})
		parts := strings.Split(token, ".")
		_, err := validator.ValidateToken(context.Background(), parts[0]+"."+parts[1]+".@@@")
		if err == nil {
			t.Errorf("expected error for invalid signature base64")
		}
	})

	t.Run("invalid signature content", func(t *testing.T) {
		otherPriv, _ := generateRSAKey(t)
		token := signJWT(t, otherPriv, "key-1", "RS256", map[string]any{"sub": "123"})
		_, err := validator.ValidateToken(context.Background(), token)
		if err == nil || !strings.Contains(err.Error(), "invalid jwt signature") {
			t.Errorf("expected invalid signature error, got %v", err)
		}
	})

	t.Run("invalid payload base64", func(t *testing.T) {
		token := signJWT(t, priv, "key-1", "RS256", map[string]any{"sub": "123"})
		parts := strings.Split(token, ".")
		_, err := validator.ValidateToken(context.Background(), parts[0]+".@@@."+parts[2])
		if err == nil {
			t.Errorf("expected error for invalid payload base64")
		}
	})

	t.Run("invalid payload json", func(t *testing.T) {
		header := b64url([]byte(`{"alg":"RS256","kid":"key-1"}`))
		payload := b64url([]byte(`not json`))
		signingInput := header + "." + payload
		hashed := sha256.Sum256([]byte(signingInput))
		sig, _ := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, hashed[:])
		token := signingInput + "." + b64url(sig)

		_, err := validator.ValidateToken(context.Background(), token)
		if err == nil {
			t.Errorf("expected error for invalid payload json")
		}
	})

	t.Run("expired token", func(t *testing.T) {
		claims := map[string]any{
			"sub": "user-uuid-1",
			"exp": time.Now().Add(-10 * time.Minute).Unix(),
		}
		token := signJWT(t, priv, "key-1", "RS256", claims)
		_, err := validator.ValidateToken(context.Background(), token)
		if err == nil || !strings.Contains(err.Error(), "expired") {
			t.Errorf("expected expired error, got %v", err)
		}
	})
}

func TestOIDCValidator_DynamicJWKS(t *testing.T) {
	priv, pub := generateRSAKey(t)

	nB64 := b64url(pub.N.Bytes())
	eBytes := []byte{1, 0, 1}
	eB64 := b64url(eBytes)

	// Mock JWKS Server
	serverStatus := http.StatusOK
	responseBody := fmt.Sprintf(`{
		"keys": [
			{
				"kty": "RSA",
				"alg": "RS256",
				"use": "sig",
				"kid": "dynamic-key-1",
				"n": "%s",
				"e": "%s"
			},
			{
				"kty": "EC",
				"kid": "ec-key"
			}
		]
	}`, nB64, eB64)

	jwksServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(serverStatus)
		_, _ = w.Write([]byte(responseBody))
	}))
	defer jwksServer.Close()

	validator := middleware.NewOIDCValidator(jwksServer.URL, jwksServer.Client())

	claims := map[string]any{
		"sub":   "dynamic-sub",
		"email": "test@dynamic.com",
		"exp":   time.Now().Add(1 * time.Hour).Unix(),
	}
	token := signJWT(t, priv, "dynamic-key-1", "RS256", claims)

	// First fetch should query server and populate cache
	user, err := validator.ValidateToken(context.Background(), token)
	if err != nil {
		t.Fatalf("expected successful dynamic key fetch, got error: %v", err)
	}
	if user.Subject != "dynamic-sub" {
		t.Errorf("expected sub 'dynamic-sub', got '%s'", user.Subject)
	}

	// Test SetKey manually
	priv2, pub2 := generateRSAKey(t)
	validator.SetKey("manual-key", pub2)
	tokenManual := signJWT(t, priv2, "manual-key", "RS256", claims)
	userManual, err := validator.ValidateToken(context.Background(), tokenManual)
	if err != nil || userManual.Subject != "dynamic-sub" {
		t.Fatalf("expected successful SetKey token verification, got error: %v", err)
	}

	// Test JWKS server error
	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer errorServer.Close()

	failValidator := middleware.NewOIDCValidator(errorServer.URL, errorServer.Client())
	token2 := signJWT(t, priv, "key-fail", "RS256", claims)
	_, err = failValidator.ValidateToken(context.Background(), token2)
	if err == nil {
		t.Errorf("expected error when jwks server returns 500")
	}

	// Test JWKS invalid JSON
	badJSONServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("invalid json"))
	}))
	defer badJSONServer.Close()

	badJSONValidator := middleware.NewOIDCValidator(badJSONServer.URL, badJSONServer.Client())
	_, err = badJSONValidator.ValidateToken(context.Background(), token2)
	if err == nil {
		t.Errorf("expected error when jwks response is malformed json")
	}

	// Test JWKS with malformed modulus/exponent
	malformedKeyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"keys":[{"kty":"RSA","kid":"key-fail","n":"@@@","e":"AQAB"}]}`))
	}))
	defer malformedKeyServer.Close()

	malformedValidator := middleware.NewOIDCValidator(malformedKeyServer.URL, malformedKeyServer.Client())
	_, err = malformedValidator.ValidateToken(context.Background(), token2)
	if err == nil {
		t.Errorf("expected error when jwks modulus is malformed base64")
	}
}

func TestRequireOIDCMiddleware(t *testing.T) {
	priv, pub := generateRSAKey(t)
	validator := middleware.NewStaticOIDCValidator(map[string]*rsa.PublicKey{
		"key-1": pub,
	})

	handler := middleware.RequireOIDC(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := middleware.GetOIDCUser(r.Context())
		if user == nil {
			t.Errorf("expected user in context")
		}
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("missing authorization header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("empty bearer token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer ")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("nil validator", func(t *testing.T) {
		nilHandler := middleware.RequireOIDC(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer something")
		rec := httptest.NewRecorder()
		nilHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", rec.Code)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer not.a.valid.jwt")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("valid token", func(t *testing.T) {
		token := signJWT(t, priv, "key-1", "RS256", map[string]any{
			"sub": "test-sub",
			"exp": time.Now().Add(1 * time.Hour).Unix(),
		})
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})
}

func TestDualAuthMiddleware(t *testing.T) {
	priv, pub := generateRSAKey(t)
	oidcValidator := middleware.NewStaticOIDCValidator(map[string]*rsa.PublicKey{
		"key-1": pub,
	})

	validRawKey := "lensio_live_testkey12345678901234567890"
	validKeyHash := apikey.Hash(validRawKey)

	revokedRawKey := "lensio_live_revokedkey123456789012345678"
	revokedKeyHash := apikey.Hash(revokedRawKey)
	now := time.Now()

	expiredRawKey := "lensio_live_expiredkey123456789012345678"
	expiredKeyHash := apikey.Hash(expiredRawKey)
	past := time.Now().Add(-1 * time.Hour)

	keyStore := newMockKeyStore()
	_ = keyStore.CreateAPIKey(context.Background(), &store.APIKey{
		ID:      "key-valid",
		OrgID:   "org-1",
		KeyHash: validKeyHash,
		Scopes:  []string{"ocr:write"},
	})
	_ = keyStore.CreateAPIKey(context.Background(), &store.APIKey{
		ID:        "key-revoked",
		OrgID:     "org-1",
		KeyHash:   revokedKeyHash,
		RevokedAt: &now,
	})
	_ = keyStore.CreateAPIKey(context.Background(), &store.APIKey{
		ID:        "key-expired",
		OrgID:     "org-1",
		KeyHash:   expiredKeyHash,
		ExpiresAt: &past,
	})

	dualHandler := middleware.DualAuth(keyStore, oidcValidator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user := middleware.GetOIDCUser(r.Context()); user != nil {
			response.JSON(w, http.StatusOK, map[string]any{"type": "oidc", "sub": user.Subject})
			return
		}
		if key := middleware.GetAPIKey(r.Context()); key != nil {
			response.JSON(w, http.StatusOK, map[string]any{"type": "apikey", "id": key.ID})
			return
		}
		w.WriteHeader(http.StatusTeapot)
	}))

	t.Run("missing authorization header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		dualHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("empty token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer ")
		rec := httptest.NewRecorder()
		dualHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("valid OIDC JWT token", func(t *testing.T) {
		token := signJWT(t, priv, "key-1", "RS256", map[string]any{
			"sub": "operator-123",
			"exp": time.Now().Add(1 * time.Hour).Unix(),
		})
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		dualHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "operator-123") {
			t.Errorf("expected response to contain operator-123, got %s", rec.Body.String())
		}
	})

	t.Run("valid machine API Key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer "+validRawKey)
		rec := httptest.NewRecorder()
		dualHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "key-valid") {
			t.Errorf("expected response to contain key-valid, got %s", rec.Body.String())
		}
	})

	t.Run("revoked API Key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer "+revokedRawKey)
		rec := httptest.NewRecorder()
		dualHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "revoked") {
			t.Errorf("expected error message to mention revoked, got %s", rec.Body.String())
		}
	})

	t.Run("expired API Key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer "+expiredRawKey)
		rec := httptest.NewRecorder()
		dualHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "expired") {
			t.Errorf("expected error message to mention expired, got %s", rec.Body.String())
		}
	})

	t.Run("unrecognized credentials", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer lensio_live_unrecognized")
		rec := httptest.NewRecorder()
		dualHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})
}

func TestRequireScope_WithOIDCUser(t *testing.T) {
	handler := middleware.RequireScope("ocr:write")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("oidc user with admin role passes any scope", func(t *testing.T) {
		user := &middleware.OIDCUser{Subject: "admin-1", Roles: []string{"admin"}}
		req := httptest.NewRequest(http.MethodPost, "/ocr", nil)
		req = req.WithContext(middleware.WithOIDCUser(req.Context(), user))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("oidc user with exact matching scope passes", func(t *testing.T) {
		user := &middleware.OIDCUser{Subject: "dev-1", Roles: []string{"ocr:write"}}
		req := httptest.NewRequest(http.MethodPost, "/ocr", nil)
		req = req.WithContext(middleware.WithOIDCUser(req.Context(), user))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("oidc user with missing scope fails with 403", func(t *testing.T) {
		user := &middleware.OIDCUser{Subject: "viewer-1", Roles: []string{"ocr:read"}}
		req := httptest.NewRequest(http.MethodPost, "/ocr", nil)
		req = req.WithContext(middleware.WithOIDCUser(req.Context(), user))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden, got %d", rec.Code)
		}
	})

	t.Run("neither key nor oidc user in context returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/ocr", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", rec.Code)
		}
	})
}

func TestOIDCValidator_EdgeCases(t *testing.T) {
	// Test NewOIDCValidator with nil client
	v := middleware.NewOIDCValidator("http://localhost:8080/jwks", nil)
	if v == nil {
		t.Fatalf("expected non-nil validator")
	}

	priv, _ := generateRSAKey(t)
	claims := map[string]any{"sub": "123", "exp": time.Now().Add(1 * time.Hour).Unix()}
	token := signJWT(t, priv, "key-fail", "RS256", claims)

	// Test JWKS with malformed exponent
	malformedEServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"keys":[{"kty":"RSA","kid":"key-fail","n":"AQAB","e":"@@@"}]}`))
	}))
	defer malformedEServer.Close()

	valE := middleware.NewOIDCValidator(malformedEServer.URL, malformedEServer.Client())
	_, err := valE.ValidateToken(context.Background(), token)
	if err == nil {
		t.Errorf("expected error for malformed exponent")
	}

	// Test bad URL creating request error
	badURLVal := middleware.NewOIDCValidator("::://invalid-url", http.DefaultClient)
	_, err = badURLVal.ValidateToken(context.Background(), token)
	if err == nil {
		t.Errorf("expected error for bad URL")
	}
}
