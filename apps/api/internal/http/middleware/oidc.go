package middleware

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/apikey"
	"github.com/amirfaisalz/lensio/apps/api/internal/http/response"
	"github.com/amirfaisalz/lensio/apps/api/internal/store"
)

var (
	ErrMalformedToken       = errors.New("malformed jwt token")
	ErrUnsupportedAlgorithm = errors.New("unsupported jwt algorithm, expected RS256")
	ErrKeyNotFound          = errors.New("public key for token kid not found")
	ErrInvalidSignature     = errors.New("invalid jwt signature")
	ErrTokenExpired         = errors.New("jwt token has expired")
	ErrInvalidIssuer        = errors.New("jwt issuer does not match expected realm")
	ErrInvalidAudience      = errors.New("jwt audience does not match expected client")
)

// OIDCUser represents the authenticated human identity extracted from an OIDC JWT.
type OIDCUser struct {
	Subject           string   `json:"sub"`
	Email             string   `json:"email"`
	EmailVerified     bool     `json:"email_verified"`
	PreferredUsername string   `json:"preferred_username"`
	Name              string   `json:"name"`
	Issuer            string   `json:"iss"`
	Roles             []string `json:"roles"`
	ExpiresAt         int64    `json:"exp"`
	IssuedAt          int64    `json:"iat"`
}

type ctxKeyOIDCUser struct{}

var oidcUserContextKey = ctxKeyOIDCUser{}

// WithOIDCUser injects the authenticated OIDC user claims into the request context.
func WithOIDCUser(ctx context.Context, user *OIDCUser) context.Context {
	return context.WithValue(ctx, oidcUserContextKey, user)
}

// GetOIDCUser retrieves the authenticated OIDC user claims from the request context.
func GetOIDCUser(ctx context.Context) *OIDCUser {
	if ctx == nil {
		return nil
	}
	if v, ok := ctx.Value(oidcUserContextKey).(*OIDCUser); ok {
		return v
	}
	return nil
}

// TokenValidator defines the contract for validating OIDC JWT bearer tokens.
type TokenValidator interface {
	ValidateToken(ctx context.Context, token string) (*OIDCUser, error)
}

type jwksResponse struct {
	Keys []jwkKey `json:"keys"`
}

type jwkKey struct {
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type jwtHeader struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
	Typ string `json:"typ"`
}

type jwtPayload struct {
	Sub               string   `json:"sub"`
	Email             string   `json:"email"`
	EmailVerified     bool     `json:"email_verified"`
	PreferredUsername string   `json:"preferred_username"`
	Name              string   `json:"name"`
	Iss               string   `json:"iss"`
	Aud               any      `json:"aud"`
	Exp               int64    `json:"exp"`
	Iat               int64    `json:"iat"`
	Roles             []string `json:"roles"`
	RealmAccess       struct {
		Roles []string `json:"roles"`
	} `json:"realm_access"`
}

// OIDCValidator validates JWT bearer tokens against Keycloak JWKS endpoint.
type OIDCValidator struct {
	jwksURL          string
	httpClient       *http.Client
	mu               sync.RWMutex
	keys             map[string]*rsa.PublicKey
	lastFetch        time.Time
	minFetchInterval time.Duration
	expectedIssuer   string
	expectedAudience string
}

// NewOIDCValidator creates a validator that dynamically fetches and caches public keys from jwksURL.
func NewOIDCValidator(jwksURL string, client *http.Client) *OIDCValidator {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	return &OIDCValidator{
		jwksURL:          jwksURL,
		httpClient:       client,
		keys:             make(map[string]*rsa.PublicKey),
		minFetchInterval: 3 * time.Second,
	}
}

// NewStaticOIDCValidator creates a validator pre-seeded with static RSA public keys (primarily for unit tests).
func NewStaticOIDCValidator(keys map[string]*rsa.PublicKey) *OIDCValidator {
	copied := make(map[string]*rsa.PublicKey, len(keys))
	for k, v := range keys {
		copied[k] = v
	}
	return &OIDCValidator{
		httpClient:       &http.Client{Timeout: 5 * time.Second},
		keys:             copied,
		minFetchInterval: 3 * time.Second,
	}
}

// SetExpectedIssuer configures the expected 'iss' claim in validated JWT tokens.
func (v *OIDCValidator) SetExpectedIssuer(iss string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.expectedIssuer = strings.TrimSpace(iss)
}

// SetExpectedAudience configures the expected 'aud' claim in validated JWT tokens.
func (v *OIDCValidator) SetExpectedAudience(aud string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.expectedAudience = strings.TrimSpace(aud)
}

// SetKey directly registers or overrides an RSA public key in the cache.
func (v *OIDCValidator) SetKey(kid string, pubKey *rsa.PublicKey) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.keys[kid] = pubKey
}

// ValidateToken validates an RS256 JWT bearer token, verifies its signature against cached or fetched JWKS,
// checks its expiration, and extracts the OIDC identity claims.
func (v *OIDCValidator) ValidateToken(ctx context.Context, token string) (*OIDCUser, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrMalformedToken
	}

	// 1. Decode and inspect header
	headerBytes, err := decodeBase64URL(parts[0])
	if err != nil {
		return nil, fmt.Errorf("%w: invalid header base64", ErrMalformedToken)
	}

	var header jwtHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, fmt.Errorf("%w: invalid header json", ErrMalformedToken)
	}

	if header.Alg != "RS256" {
		return nil, ErrUnsupportedAlgorithm
	}

	// 2. Resolve Public Key by kid
	pubKey, err := v.getKey(ctx, header.Kid)
	if err != nil {
		return nil, err
	}

	// 3. Verify Signature
	signedContent := parts[0] + "." + parts[1]
	hash := sha256.Sum256([]byte(signedContent))

	sigBytes, err := decodeBase64URL(parts[2])
	if err != nil {
		return nil, fmt.Errorf("%w: invalid signature base64", ErrMalformedToken)
	}

	if err := rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, hash[:], sigBytes); err != nil {
		return nil, ErrInvalidSignature
	}

	// 4. Decode and parse Payload
	payloadBytes, err := decodeBase64URL(parts[1])
	if err != nil {
		return nil, fmt.Errorf("%w: invalid payload base64", ErrMalformedToken)
	}

	var payload jwtPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, fmt.Errorf("%w: invalid payload json", ErrMalformedToken)
	}

	// 5. Expiration check
	now := time.Now().Unix()
	if payload.Exp > 0 && now > payload.Exp {
		return nil, ErrTokenExpired
	}

	// 6. Issuer check
	v.mu.RLock()
	expIss := v.expectedIssuer
	expAud := v.expectedAudience
	v.mu.RUnlock()

	if expIss != "" && payload.Iss != expIss {
		return nil, ErrInvalidIssuer
	}

	// 7. Audience check
	if expAud != "" && !matchAudience(payload.Aud, expAud) {
		return nil, ErrInvalidAudience
	}

	// Collect and deduplicate roles
	roleMap := make(map[string]struct{})
	for _, r := range payload.Roles {
		if r != "" {
			roleMap[r] = struct{}{}
		}
	}
	for _, r := range payload.RealmAccess.Roles {
		if r != "" {
			roleMap[r] = struct{}{}
		}
	}

	roles := make([]string, 0, len(roleMap))
	for r := range roleMap {
		roles = append(roles, r)
	}

	user := &OIDCUser{
		Subject:           payload.Sub,
		Email:             payload.Email,
		EmailVerified:     payload.EmailVerified,
		PreferredUsername: payload.PreferredUsername,
		Name:              payload.Name,
		Issuer:            payload.Iss,
		Roles:             roles,
		ExpiresAt:         payload.Exp,
		IssuedAt:          payload.Iat,
	}

	return user, nil
}

func (v *OIDCValidator) getKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.RLock()
	key, exists := v.keys[kid]
	lastFetch := v.lastFetch
	v.mu.RUnlock()

	if exists {
		return key, nil
	}

	// If no JWKS URL configured or kid not found in static keys, return ErrKeyNotFound
	if v.jwksURL == "" {
		return nil, ErrKeyNotFound
	}

	// Rate-limit JWKS fetches to prevent resource exhaustion / DDOS
	if time.Since(lastFetch) < v.minFetchInterval {
		return nil, ErrKeyNotFound
	}

	if err := v.refreshKeys(ctx); err != nil {
		return nil, fmt.Errorf("failed refreshing jwks: %w", err)
	}

	v.mu.RLock()
	key, exists = v.keys[kid]
	v.mu.RUnlock()

	if !exists {
		return nil, ErrKeyNotFound
	}

	return key, nil
}

func (v *OIDCValidator) refreshKeys(ctx context.Context) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Double check interval under write lock
	if time.Since(v.lastFetch) < v.minFetchInterval {
		return nil
	}
	v.lastFetch = time.Now()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return fmt.Errorf("create jwks request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetch jwks: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jwks endpoint returned status %d", resp.StatusCode)
	}

	var jwks jwksResponse
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return fmt.Errorf("decode jwks json: %w", err)
	}

	for _, k := range jwks.Keys {
		if k.Kty != "RSA" || k.Kid == "" {
			continue
		}
		pubKey, err := parseRSAPublicKey(k.N, k.E)
		if err != nil {
			continue
		}
		v.keys[k.Kid] = pubKey
	}

	return nil
}

func parseRSAPublicKey(nStr, eStr string) (*rsa.PublicKey, error) {
	nBytes, err := decodeBase64URL(nStr)
	if err != nil {
		return nil, fmt.Errorf("invalid modulus n: %w", err)
	}

	eBytes, err := decodeBase64URL(eStr)
	if err != nil {
		return nil, fmt.Errorf("invalid exponent e: %w", err)
	}

	var eInt int
	for _, b := range eBytes {
		eInt = (eInt << 8) | int(b)
	}

	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: eInt,
	}, nil
}

func decodeBase64URL(input string) ([]byte, error) {
	clean := strings.TrimRight(input, "=")
	return base64.RawURLEncoding.DecodeString(clean)
}

func matchAudience(audClaim any, expected string) bool {
	if expected == "" {
		return true
	}
	switch v := audClaim.(type) {
	case string:
		return v == expected
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok && s == expected {
				return true
			}
		}
	case []string:
		for _, s := range v {
			if s == expected {
				return true
			}
		}
	}
	return false
}

// RequireOIDC returns a middleware enforcing valid OIDC JWT bearer tokens.
func RequireOIDC(validator TokenValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				response.ErrorWithRequest(
					w,
					r,
					http.StatusUnauthorized,
					response.CodeInvalidAPIKey,
					"Missing or malformed Authorization header. Expected 'Bearer <token>'",
				)
				return
			}

			token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
			if token == "" {
				response.ErrorWithRequest(
					w,
					r,
					http.StatusUnauthorized,
					response.CodeInvalidAPIKey,
					"Bearer token is empty",
				)
				return
			}

			if validator == nil {
				response.ErrorWithRequest(
					w,
					r,
					http.StatusInternalServerError,
					response.CodeInternalError,
					"OIDC validator not configured",
				)
				return
			}

			user, err := validator.ValidateToken(r.Context(), token)
			if err != nil {
				response.ErrorWithRequest(
					w,
					r,
					http.StatusUnauthorized,
					response.CodeInvalidAPIKey,
					fmt.Sprintf("Invalid OIDC token: %s", err.Error()),
				)
				return
			}

			ctx := WithOIDCUser(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// DualAuth returns a middleware supporting dual authentication:
// - Human operators authenticating via Keycloak OIDC JWTs
// - External machine services authenticating via scoped API keys
func DualAuth(keyStore store.APIKeyStore, oidcValidator TokenValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				response.ErrorWithRequest(
					w,
					r,
					http.StatusUnauthorized,
					response.CodeInvalidAPIKey,
					"Missing or malformed Authorization header. Expected 'Bearer <token>'",
				)
				return
			}

			token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
			if token == "" {
				response.ErrorWithRequest(
					w,
					r,
					http.StatusUnauthorized,
					response.CodeInvalidAPIKey,
					"Authentication token is empty",
				)
				return
			}

			// 1. Attempt OIDC JWT authentication if token has JWT structure (header.payload.sig)
			if strings.Count(token, ".") == 2 && oidcValidator != nil {
				user, err := oidcValidator.ValidateToken(r.Context(), token)
				if err == nil && user != nil {
					ctx := WithOIDCUser(r.Context(), user)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			// 2. Attempt API Key lookup
			if keyStore != nil {
				tokenHash := apikey.Hash(token)
				key, err := keyStore.GetAPIKeyByHash(r.Context(), tokenHash)
				if err == nil && key != nil {
					if key.RevokedAt != nil {
						response.ErrorWithRequest(
							w,
							r,
							http.StatusUnauthorized,
							response.CodeInvalidAPIKey,
							"API key has been revoked",
						)
						return
					}

					if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
						response.ErrorWithRequest(
							w,
							r,
							http.StatusUnauthorized,
							response.CodeInvalidAPIKey,
							"API key has expired",
						)
						return
					}

					// Asynchronously touch last_used_at with debounced pooling (Issue #5)
					touchKeyAsync(r.Context(), keyStore, key.ID)

					ctx := WithAPIKey(r.Context(), key)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			// Neither OIDC nor API Key succeeded
			response.ErrorWithRequest(
				w,
				r,
				http.StatusUnauthorized,
				response.CodeInvalidAPIKey,
				"Invalid or unrecognized credentials",
			)
		})
	}
}
