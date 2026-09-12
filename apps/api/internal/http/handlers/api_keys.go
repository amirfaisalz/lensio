package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/apikey"
	"github.com/amirfaisalz/lensio/apps/api/internal/authz"
	"github.com/amirfaisalz/lensio/apps/api/internal/http/middleware"
	"github.com/amirfaisalz/lensio/apps/api/internal/http/response"
	"github.com/amirfaisalz/lensio/apps/api/internal/store"
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
// Enforces SpiceDB ReBAC authorization (project->manage_api_keys) when authorizer is configured.
func CreateAPIKeyHandler(keyStore store.APIKeyStore, auditStore store.AuditStore, authorizer authz.Authorizer, defaultOrgID string) http.HandlerFunc {
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

		var accountStore store.AccountStore
		if as, ok := keyStore.(store.AccountStore); ok {
			accountStore = as
		}

		// Resolve org ID
		orgID := resolveOrgIDWithAccount(r, req.OrgID, accountStore, defaultOrgID)
		if orgID == "" {
			response.ErrorWithRequest(w, r, http.StatusBadRequest, response.CodeInvalidRequest, "Organisasi belum dipilih atau tidak valid")
			return
		}

		if accountStore != nil {
			if _, err := accountStore.GetOrganization(r.Context(), orgID); err != nil {
				if errors.Is(err, store.ErrNotFound) {
					response.ErrorWithRequest(w, r, http.StatusBadRequest, response.CodeInvalidRequest, "Organisasi tidak ditemukan")
					return
				}
			}
		}

		// Enforce SpiceDB ReBAC authorization if authorizer is configured
		var actorSubject authz.Subject
		if authorizer != nil {
			var hasActor bool
			actorSubject, hasActor = resolveActorSubject(r)
			if !hasActor {
				response.ErrorWithRequest(w, r, http.StatusUnauthorized, response.CodeInvalidAPIKey, "Authentication required to manage API keys")
				return
			}

			allowed, err := authorizer.CheckPermission(r.Context(), authz.NewResource("project", orgID), "manage_api_keys", actorSubject)
			if err != nil {
				response.ErrorWithRequest(w, r, http.StatusInternalServerError, response.CodeInternalError, "Failed checking authorization")
				return
			}
			if !allowed {
				response.ErrorWithRequest(w, r, http.StatusForbidden, response.CodePermissionDenied, fmt.Sprintf("Actor %q lacks permission 'manage_api_keys' on project %q", actorSubject.ID, orgID))
				return
			}
		}

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

		// Record Zanzibar relationships in SpiceDB
		if authorizer != nil {
			_ = authorizer.WriteRelationship(r.Context(), authz.NewRelationship(
				authz.NewResource("api_key", keyModel.ID),
				"project",
				authz.Subject{Type: "project", ID: authz.SanitizeID(keyModel.OrgID)},
			))
			_ = authorizer.WriteRelationship(r.Context(), authz.NewRelationship(
				authz.NewResource("api_key", keyModel.ID),
				"creator",
				actorSubject,
			))
		}

		if auditStore != nil {
			actorID := "system"
			if actorSubject.ID != "" {
				actorID = actorSubject.String()
			} else if authKey := middleware.GetAPIKey(r.Context()); authKey != nil && authKey.ID != "" {
				actorID = "api_key:" + authKey.ID
			}
			_ = auditStore.RecordAuditLog(r.Context(), &store.AuditLog{
				OrgID:          keyModel.OrgID,
				ActorID:        actorID,
				Action:         "api_key.create",
				TargetResource: "api_key:" + keyModel.ID,
				Metadata: map[string]any{
					"name":        keyModel.Name,
					"prefix":      keyModel.Prefix,
					"scopes":      keyModel.Scopes,
					"environment": keyModel.Environment,
				},
			})
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
		var accountStore store.AccountStore
		if as, ok := keyStore.(store.AccountStore); ok {
			accountStore = as
		}
		orgID := resolveOrgIDWithAccount(r, r.URL.Query().Get("org_id"), accountStore, defaultOrgID)
		if orgID == "" {
			response.JSON(w, http.StatusOK, map[string]any{
				"data": []APIKeyListItem{},
			})
			return
		}

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
// Enforces SpiceDB ReBAC authorization (api_key->revoke or project->manage_api_keys) when authorizer is configured.
func RevokeAPIKeyHandler(keyStore store.APIKeyStore, auditStore store.AuditStore, authorizer authz.Authorizer, defaultOrgID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		keyID := strings.TrimSpace(r.PathValue("id"))
		if keyID == "" {
			response.ErrorWithRequest(w, r, http.StatusBadRequest, response.CodeInvalidRequest, "Missing key ID in path")
			return
		}

		var accountStore store.AccountStore
		if as, ok := keyStore.(store.AccountStore); ok {
			accountStore = as
		}
		orgID := resolveOrgIDWithAccount(r, r.URL.Query().Get("org_id"), accountStore, defaultOrgID)

		var actorSubject authz.Subject
		if authorizer != nil {
			var hasActor bool
			actorSubject, hasActor = resolveActorSubject(r)
			if !hasActor {
				response.ErrorWithRequest(w, r, http.StatusUnauthorized, response.CodeInvalidAPIKey, "Authentication required to manage API keys")
				return
			}

			// Check if actor has 'revoke' permission on the api_key
			allowed, err := authorizer.CheckPermission(r.Context(), authz.NewResource("api_key", keyID), "revoke", actorSubject)
			if err != nil {
				response.ErrorWithRequest(w, r, http.StatusInternalServerError, response.CodeInternalError, "Failed checking authorization")
				return
			}
			if !allowed {
				// Fallback: check if actor has 'manage_api_keys' on the parent project
				allowedOnProject, errProj := authorizer.CheckPermission(r.Context(), authz.NewResource("project", orgID), "manage_api_keys", actorSubject)
				if errProj != nil {
					response.ErrorWithRequest(w, r, http.StatusInternalServerError, response.CodeInternalError, "Failed checking authorization")
					return
				}
				if !allowedOnProject {
					response.ErrorWithRequest(w, r, http.StatusForbidden, response.CodePermissionDenied, fmt.Sprintf("Actor %q lacks permission to revoke api_key %q", actorSubject.ID, keyID))
					return
				}
			}
		}

		if err := keyStore.RevokeAPIKey(r.Context(), orgID, keyID); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				response.ErrorWithRequest(w, r, http.StatusNotFound, response.CodeInvalidRequest, "API key not found or already revoked")
				return
			}
			response.ErrorWithRequest(w, r, http.StatusInternalServerError, response.CodeInternalError, "Failed to revoke API key")
			return
		}

		if auditStore != nil {
			actorID := "system"
			if actorSubject.ID != "" {
				actorID = actorSubject.String()
			} else if authKey := middleware.GetAPIKey(r.Context()); authKey != nil && authKey.ID != "" {
				actorID = "api_key:" + authKey.ID
			}
			_ = auditStore.RecordAuditLog(r.Context(), &store.AuditLog{
				OrgID:          orgID,
				ActorID:        actorID,
				Action:         "api_key.revoke",
				TargetResource: "api_key:" + keyID,
				Metadata:       map[string]any{},
			})
		}

		response.JSON(w, http.StatusOK, map[string]any{
			"message": "API key successfully revoked",
			"id":      keyID,
		})
	}
}

func resolveActorSubject(r *http.Request) (authz.Subject, bool) {
	if user := middleware.GetOIDCUser(r.Context()); user != nil {
		id := strings.TrimSpace(user.Subject)
		if id == "" {
			id = strings.TrimSpace(user.Email)
		}
		if id != "" {
			return authz.NewSubject("user", authz.SanitizeID(id)), true
		}
	}

	if authKey := middleware.GetAPIKey(r.Context()); authKey != nil && authKey.ID != "" {
		return authz.NewSubject("user", "api_key_"+authz.SanitizeID(authKey.ID)), true
	}

	return authz.Subject{}, false
}

func resolveOrgIDWithAccount(r *http.Request, explicit string, accountStore store.AccountStore, fallback string) string {
	if explicit = strings.TrimSpace(explicit); explicit != "" {
		return explicit
	}
	if authKey := middleware.GetAPIKey(r.Context()); authKey != nil && authKey.OrgID != "" {
		return authKey.OrgID
	}
	if oidcUser := middleware.GetOIDCUser(r.Context()); oidcUser != nil && accountStore != nil {
		org, err := accountStore.GetUserOrganization(r.Context(), oidcUser.Subject)
		if err == nil && org != nil && org.ID != "" {
			return org.ID
		}
		if errors.Is(err, store.ErrNotFound) {
			return ""
		}
	}
	if fallback != "" {
		return fallback
	}
	return DefaultOrgID
}

