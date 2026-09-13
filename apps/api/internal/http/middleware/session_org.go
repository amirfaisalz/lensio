package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/amirfaisalz/lensio/apps/api/internal/http/response"
	"github.com/amirfaisalz/lensio/apps/api/internal/store"
)

// SessionOrg lets cookie-authenticated callers (e.g. dashboard Playground,
// OpenAI-style) use organization-attributed endpoints without a
// server-to-server API key. When the request carries no API key but a valid
// OIDC session user, it resolves the user's organization and injects a
// lightweight synthetic identity carrying only OrgID and the user's roles.
// Real API keys are never overridden, and the synthetic identity carries an
// empty ID so usage metering and OCR metadata record a NULL api_key_id.
func SessionOrg(accountStore store.AccountStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if GetAPIKey(r.Context()) != nil {
				next.ServeHTTP(w, r)
				return
			}
			user := GetOIDCUser(r.Context())
			if user == nil || accountStore == nil {
				next.ServeHTTP(w, r)
				return
			}

			orgID := ""
			if org, err := accountStore.GetUserOrganization(r.Context(), user.Subject); err == nil && org != nil {
				orgID = strings.TrimSpace(org.ID)
			} else if email := strings.TrimSpace(user.Email); email != "" {
				if u, err := accountStore.GetUserByEmail(r.Context(), email); err == nil && u != nil {
					orgID = strings.TrimSpace(u.OrgID)
				}
			}
			if orgID == "" {
				response.ErrorWithRequest(
					w,
					r,
					http.StatusUnauthorized,
					response.CodeInvalidAPIKey,
					"Organisasi tidak ditemukan. Buat atau pilih organisasi dulu.",
				)
				return
			}

			ctx := WithAPIKey(r.Context(), &store.APIKey{
				OrgID:       orgID,
				Scopes:      user.Roles,
				Environment: "live",
			})
			if car, ok := w.(interface{ SetRequestContext(context.Context) }); ok {
				car.SetRequestContext(ctx)
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
