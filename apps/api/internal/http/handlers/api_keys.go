package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/amirfaisalz/nusaid/apps/api/internal/apikey"
	"github.com/amirfaisalz/nusaid/apps/api/internal/http/middleware"
	"github.com/amirfaisalz/nusaid/apps/api/internal/http/response"
	"github.com/amirfaisalz/nusaid/apps/api/internal/store"
)

// DefaultOrgID is the seeded fallback organization identifier.
const DefaultOrgID = "00000000-0000-0000-0000-000000000001"

// CreateKeyRequest represents the payload for creating a new API key.
type CreateKeyRequest struct {
	Name        string     `json:"name"`
	Environment string     `json:"environment"`
	Scopes      []string   `json:"scopes"`
	ExpiresAt   *time.Time `json:"expires_at"`
	OrgID       string     `json:"org_id"`
}

// CreateKeyResponse is returned once upon API key generation containing the plaintext token.
type CreateKeyResponse struct {
	ID          string     `json:"id"`
	OrgID       string     `json:"org_id"`
	Name        string     `json:"name"`
	Key         string     `json:"key"`
	Prefix      string     `json:"prefix"`
	Scopes      []string   `json:"scopes"`
	Environment string     `json:"environment"`
	ExpiresAt   *time.Time `json:"expires_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// APIKeyListItem represents a masked API key for dashboard/listing display.
type APIKeyListItem struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Prefix      string     `json:"prefix"`
	MaskedKey   string     `json:"masked_key"`
	Scopes      []string   `json:"scopes"`
	Environment string     `json:"environment"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	ExpiresAt   *time.Time `json:"expires_at"`
	RevokedAt   *time.Time `json:"revoked_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// CreateAPIKeyHandler handles POST /api/v1/auth/api-keys.
func CreateAPIKeyHandler(keyStore store.APIKeyStore, defaultOrgID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CreateKeyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.ErrorWithRequest(w, r, http.StatusBadRequest, response.CodeInvalidRequest, "Invalid JSON payload")
			return
		}

		req.Name = strings.TrimSpace(req.Name)
		if req.Name == "" {
			response.ErrorWithRequest(w, r, http.StatusBadRequest, response.CodeInvalidRequest, "Field 'name' is required")
			return
		}

		// Validate or default environment
		req.Environment = strings.ToLower(strings.TrimSpace(req.Environment))
		if req.Environment == "" {
			req.Environment = apikey.EnvLive
		}
		if req.Environment != apikey.EnvLive && req.Environment != apikey.EnvTest {
			response.ErrorWithRequest(w, r, http.StatusBadRequest, response.CodeInvalidRequest, fmt.Sprintf("Invalid environment %q: must be %q or %q", req.Environment, apikey.EnvLive, apikey.EnvTest))
			return
		}

		// Default scopes if not provided
		if len(req.Scopes) == 0 {
			req.Scopes = []string{"ocr:read", "ocr:write", "usage:read"}
		}

		// Resolve org ID
		orgID := resolveOrgID(r, req.OrgID, defaultOrgID)

		// Generate high-entropy API key
		gen, err := apikey.Generate(req.Environment)
		if err != nil {
			response.ErrorWithRequest(w, r, http.StatusInternalServerError, response.CodeInternalError, "Failed to generate API key")
			return
		}

		keyModel := &store.APIKey{
			OrgID:       orgID,
			Name:        req.Name,
			KeyHash:     gen.KeyHash,
			Prefix:      gen.Prefix,
			Scopes:      req.Scopes,
			Environment: gen.Environment,
			ExpiresAt:   req.ExpiresAt,
		}

		if err := keyStore.CreateAPIKey(r.Context(), keyModel); err != nil {
			response.ErrorWithRequest(w, r, http.StatusInternalServerError, response.CodeInternalError, "Failed to persist API key")
			return
		}

		response.JSON(w, http.StatusCreated, CreateKeyResponse{
			ID:          keyModel.ID,
			OrgID:       keyModel.OrgID,
			Name:        keyModel.Name,
			Key:         gen.Plaintext,
			Prefix:      keyModel.Prefix,
			Scopes:      keyModel.Scopes,
			Environment: keyModel.Environment,
			ExpiresAt:   keyModel.ExpiresAt,
			CreatedAt:   keyModel.CreatedAt,
		})
	}
}

// ListAPIKeysHandler handles GET /api/v1/auth/api-keys.
func ListAPIKeysHandler(keyStore store.APIKeyStore, defaultOrgID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID := resolveOrgID(r, r.URL.Query().Get("org_id"), defaultOrgID)

		keys, err := keyStore.ListAPIKeysByOrg(r.Context(), orgID)
		if err != nil {
			response.ErrorWithRequest(w, r, http.StatusInternalServerError, response.CodeInternalError, "Failed to list API keys")
			return
		}

		items := make([]APIKeyListItem, 0, len(keys))
		for _, k := range keys {
			items = append(items, APIKeyListItem{
				ID:          k.ID,
				Name:        k.Name,
				Prefix:      k.Prefix,
				MaskedKey:   apikey.Mask(k.Prefix),
				Scopes:      k.Scopes,
				Environment: k.Environment,
				LastUsedAt:  k.LastUsedAt,
				ExpiresAt:   k.ExpiresAt,
				RevokedAt:   k.RevokedAt,
				CreatedAt:   k.CreatedAt,
			})
		}

		response.JSON(w, http.StatusOK, map[string]any{
			"data": items,
		})
	}
}

// RevokeAPIKeyHandler handles DELETE /api/v1/auth/api-keys/{id}.
func RevokeAPIKeyHandler(keyStore store.APIKeyStore, defaultOrgID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		keyID := strings.TrimSpace(r.PathValue("id"))
		if keyID == "" {
			response.ErrorWithRequest(w, r, http.StatusBadRequest, response.CodeInvalidRequest, "Missing key ID in path")
			return
		}

		orgID := resolveOrgID(r, r.URL.Query().Get("org_id"), defaultOrgID)

		if err := keyStore.RevokeAPIKey(r.Context(), orgID, keyID); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				response.ErrorWithRequest(w, r, http.StatusNotFound, response.CodeInvalidRequest, "API key not found or already revoked")
				return
			}
			response.ErrorWithRequest(w, r, http.StatusInternalServerError, response.CodeInternalError, "Failed to revoke API key")
			return
		}

		response.JSON(w, http.StatusOK, map[string]any{
			"message": "API key successfully revoked",
			"id":      keyID,
		})
	}
}

func resolveOrgID(r *http.Request, explicit string, fallback string) string {
	if authKey := middleware.GetAPIKey(r.Context()); authKey != nil && authKey.OrgID != "" {
		return authKey.OrgID
	}
	if explicit = strings.TrimSpace(explicit); explicit != "" {
		return explicit
	}
	if fallback != "" {
		return fallback
	}
	return DefaultOrgID
}
