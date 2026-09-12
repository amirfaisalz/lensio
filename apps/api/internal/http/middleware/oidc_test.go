package middleware_test

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/apikey"
	"github.com/amirfaisalz/lensio/apps/api/internal/auth"
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

	t.Run("mismatched issuer", func(t *testing.T) {
		val := middleware.NewStaticOIDCValidator(map[string]*rsa.PublicKey{"key-1": pub})
		val.SetExpectedIssuer("https://auth.lensio.id/realms/lensio")

		claims := map[string]any{
			"sub": "user-uuid-1",
			"iss": "https://malicious-issuer.com/realms/fake",
			"exp": time.Now().Add(10 * time.Minute).Unix(),
		}
		token := signJWT(t, priv, "key-1", "RS256", claims)
		_, err := val.ValidateToken(context.Background(), token)
		if err == nil || !strings.Contains(err.Error(), "realm") {
			t.Fatalf("expected ErrInvalidIssuer, got %v", err)
		}
	})

	t.Run("mismatched audience string", func(t *testing.T) {
		val := middleware.NewStaticOIDCValidator(map[string]*rsa.PublicKey{"key-1": pub})
		val.SetExpectedAudience("lensio-api")

		claims := map[string]any{
			"sub": "user-uuid-1",
			"aud": "wrong-client",
			"exp": time.Now().Add(10 * time.Minute).Unix(),
		}
		token := signJWT(t, priv, "key-1", "RS256", claims)
		_, err := val.ValidateToken(context.Background(), token)
		if err == nil || !strings.Contains(err.Error(), "client") {
			t.Fatalf("expected ErrInvalidAudience, got %v", err)
		}
	})

	t.Run("mismatched audience array", func(t *testing.T) {
		val := middleware.NewStaticOIDCValidator(map[string]*rsa.PublicKey{"key-1": pub})
		val.SetExpectedAudience("lensio-api")

		claims := map[string]any{
			"sub": "user-uuid-1",
			"aud": []string{"wrong-client-1", "wrong-client-2"},
			"exp": time.Now().Add(10 * time.Minute).Unix(),
		}
		token := signJWT(t, priv, "key-1", "RS256", claims)
		_, err := val.ValidateToken(context.Background(), token)
		if err == nil || !strings.Contains(err.Error(), "client") {
			t.Fatalf("expected ErrInvalidAudience, got %v", err)
		}
	})

	t.Run("valid issuer and audience matching", func(t *testing.T) {
		val := middleware.NewStaticOIDCValidator(map[string]*rsa.PublicKey{"key-1": pub})
		val.SetExpectedIssuer("https://auth.lensio.id/realms/lensio")
		val.SetExpectedAudience("lensio-api")

		claims := map[string]any{
			"sub": "user-uuid-1",
			"iss": "https://auth.lensio.id/realms/lensio",
			"aud": []any{"account", "lensio-api"},
			"exp": time.Now().Add(10 * time.Minute).Unix(),
		}
		token := signJWT(t, priv, "key-1", "RS256", claims)
		u, err := val.ValidateToken(context.Background(), token)
		if err != nil {
			t.Fatalf("expected valid token, got error: %v", err)
		}
		if u.Subject != "user-uuid-1" {
			t.Fatalf("expected sub 'user-uuid-1', got '%s'", u.Subject)
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

	t.Run("mismatched issuer rejected with 401", func(t *testing.T) {
		v := middleware.NewStaticOIDCValidator(map[string]*rsa.PublicKey{"key-1": pub})
		v.SetExpectedIssuer("https://auth.lensio.id/realms/lensio")
		h := middleware.RequireOIDC(v)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		token := signJWT(t, priv, "key-1", "RS256", map[string]any{
			"sub": "test-sub",
			"iss": "https://attacker-idp.com",
			"exp": time.Now().Add(1 * time.Hour).Unix(),
		})
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("mismatched audience rejected with 401", func(t *testing.T) {
		v := middleware.NewStaticOIDCValidator(map[string]*rsa.PublicKey{"key-1": pub})
		v.SetExpectedAudience("lensio-api")
		h := middleware.RequireOIDC(v)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		token := signJWT(t, priv, "key-1", "RS256", map[string]any{
			"sub": "test-sub",
			"aud": "wrong-client",
			"exp": time.Now().Add(1 * time.Hour).Unix(),
		})
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
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
		user := middleware.GetOIDCUser(r.Context())
		key := middleware.GetAPIKey(r.Context())
		if user != nil && key != nil {
			response.JSON(w, http.StatusOK, map[string]any{"type": "dual", "sub": user.Subject, "id": key.ID})
			return
		}
		if user != nil {
			response.JSON(w, http.StatusOK, map[string]any{"type": "oidc", "sub": user.Subject})
			return
		}
		if key != nil {
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

	t.Run("valid session cookie without Authorization header", func(t *testing.T) {
		token := signJWT(t, priv, "key-1", "RS256", map[string]any{
			"sub": "cookie-operator-456",
			"exp": time.Now().Add(1 * time.Hour).Unix(),
		})
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.AddCookie(&http.Cookie{
			Name:  "lensio_session",
			Value: token,
		})
		rec := httptest.NewRecorder()
		dualHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 via session cookie, got %d: %s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "cookie-operator-456") {
			t.Errorf("expected response to contain cookie-operator-456, got %s", rec.Body.String())
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

	t.Run("API Key with companion valid session cookie attaches both", func(t *testing.T) {
		token := signJWT(t, priv, "key-1", "RS256", map[string]any{
			"sub": "cookie-operator-789",
			"exp": time.Now().Add(1 * time.Hour).Unix(),
		})
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer "+validRawKey)
		req.AddCookie(&http.Cookie{
			Name:  "lensio_session",
			Value: token,
		})
		rec := httptest.NewRecorder()
		dualHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "cookie-operator-789") || !strings.Contains(rec.Body.String(), "key-valid") {
			t.Errorf("expected response to contain both cookie-operator-789 and key-valid, got %s", rec.Body.String())
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

func BenchmarkValidateToken(b *testing.B) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		b.Fatalf("failed generating rsa key: %v", err)
	}
	pub := &priv.PublicKey
	validator := middleware.NewStaticOIDCValidator(map[string]*rsa.PublicKey{
		"bench-key": pub,
	})

	claims := map[string]any{
		"sub":                "user-uuid-bench",
		"email":              "bench@lensio.dev",
		"email_verified":     true,
		"preferred_username": "benchuser",
		"name":               "Bench User",
		"iss":                "https://auth.lensio.dev/realms/lensio",
		"aud":                "lensio-api",
		"exp":                time.Now().Add(1 * time.Hour).Unix(),
	}

	header := map[string]any{"alg": "RS256", "kid": "bench-key", "typ": "JWT"}
	hJSON, _ := json.Marshal(header)
	cJSON, _ := json.Marshal(claims)
	signingInput := b64url(hJSON) + "." + b64url(cJSON)
	hashed := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, hashed[:])
	if err != nil {
		b.Fatalf("failed signing jwt: %v", err)
	}
	token := signingInput + "." + b64url(sig)

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := validator.ValidateToken(ctx, token)
		if err != nil {
			b.Fatalf("validation failed: %v", err)
		}
	}
}

func TestDevTokenValidator(t *testing.T) {
	validator := middleware.NewDevTokenValidator()
	ctx := context.Background()

	t.Run("empty token returns ErrMalformedToken", func(t *testing.T) {
		_, err := validator.ValidateToken(ctx, "")
		if !errors.Is(err, middleware.ErrMalformedToken) {
			t.Fatalf("expected ErrMalformedToken, got %v", err)
		}
	})

	t.Run("mock jwt admin token succeeds", func(t *testing.T) {
		user, err := validator.ValidateToken(ctx, "mock_jwt_admin")
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if user.PreferredUsername != "admin" {
			t.Errorf("expected preferred username 'admin', got %s", user.PreferredUsername)
		}
		hasAdmin := false
		for _, r := range user.Roles {
			if r == "admin" {
				hasAdmin = true
			}
		}
		if !hasAdmin {
			t.Errorf("expected admin role in roles %v", user.Roles)
		}
	})

	t.Run("mock jwt developer token succeeds", func(t *testing.T) {
		user, err := validator.ValidateToken(ctx, "mock_jwt_developer@veriform.com")
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if user.Email != "developer@veriform.com" {
			t.Errorf("expected email 'developer@veriform.com', got %s", user.Email)
		}
	})

	t.Run("valid 3-part base64 jwt succeeds", func(t *testing.T) {
		headerJSON := `{"alg":"none","typ":"JWT"}`
		payloadJSON := `{"sub":"user-123","email":"dev@lensio.dev","preferred_username":"dev","roles":["developer"],"exp":` +
			fmt.Sprintf("%d", time.Now().Add(1*time.Hour).Unix()) + `}`

		token := b64url([]byte(headerJSON)) + "." + b64url([]byte(payloadJSON)) + ".mock_sig"
		user, err := validator.ValidateToken(ctx, token)
		if err != nil {
			t.Fatalf("expected valid token parsing, got %v", err)
		}
		if user.Email != "dev@lensio.dev" {
			t.Errorf("expected email 'dev@lensio.dev', got %s", user.Email)
		}
		if user.Subject != "user-123" {
			t.Errorf("expected subject 'user-123', got %s", user.Subject)
		}
	})

	t.Run("expired 3-part jwt returns ErrTokenExpired", func(t *testing.T) {
		headerJSON := `{"alg":"none","typ":"JWT"}`
		payloadJSON := `{"sub":"user-123","email":"dev@lensio.dev","exp":` +
			fmt.Sprintf("%d", time.Now().Add(-1*time.Hour).Unix()) + `}`

		token := b64url([]byte(headerJSON)) + "." + b64url([]byte(payloadJSON)) + ".mock_sig"
		_, err := validator.ValidateToken(ctx, token)
		if !errors.Is(err, middleware.ErrTokenExpired) {
			t.Fatalf("expected ErrTokenExpired, got %v", err)
		}
	})

	t.Run("invalid base64 payload returns error", func(t *testing.T) {
		token := "header.!!!invalid_base64!!!.sig"
		_, err := validator.ValidateToken(ctx, token)
		if err == nil {
			t.Fatal("expected error on invalid base64, got nil")
		}
	})
}

func TestSessionTokenValidator(t *testing.T) {
	ctx := context.Background()
	secret := []byte("secret-key-for-session-token-32b")
	validator := middleware.NewSessionTokenValidator(secret)

	// Valid session token
	token, err := auth.CreateSessionTokenWithSecret("user-1", "test@lensio.dev", "testuser", []string{"developer", "ocr:write"}, 1*time.Hour, secret)
	if err != nil {
		t.Fatalf("failed creating token: %v", err)
	}

	user, err := validator.ValidateToken(ctx, token)
	if err != nil {
		t.Fatalf("expected valid token, got: %v", err)
	}
	if user.Subject != "user-1" || user.Email != "test@lensio.dev" {
		t.Errorf("unexpected user: %+v", user)
	}

	// Invalid token
	_, err = validator.ValidateToken(ctx, "invalid.token.signature")
	if err == nil {
		t.Fatal("expected error for invalid token signature")
	}
}

func TestCompositeTokenValidator(t *testing.T) {
	ctx := context.Background()
	secret := []byte("secret-key-for-session-token-32b")
	sessionVal := middleware.NewSessionTokenValidator(secret)
	devVal := middleware.NewDevTokenValidator()

	composite := middleware.NewCompositeTokenValidator(sessionVal, devVal)

	// 1. Session token handled by sessionVal
	sessionToken, err := auth.CreateSessionTokenWithSecret("user-s", "session@lensio.dev", "sess", []string{"developer"}, 1*time.Hour, secret)
	if err != nil {
		t.Fatalf("failed creating token: %v", err)
	}
	u1, err := composite.ValidateToken(ctx, sessionToken)
	if err != nil || u1.Email != "session@lensio.dev" {
		t.Fatalf("expected valid session token, got user: %+v, err: %v", u1, err)
	}

	// 2. Mock token handled by devVal
	u2, err := composite.ValidateToken(ctx, "mock_jwt_developer")
	if err != nil || u2.PreferredUsername != "developer" {
		t.Fatalf("expected valid dev token, got user: %+v, err: %v", u2, err)
	}

	// 3. Completely invalid token rejected by both
	_, err = composite.ValidateToken(ctx, "completely_invalid_garbage")
	if err == nil {
		t.Fatal("expected error for garbage token")
	}
}


