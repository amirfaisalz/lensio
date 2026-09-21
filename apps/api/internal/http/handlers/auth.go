package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/auth"
	mailer "github.com/amirfaisalz/lensio/apps/api/internal/email"
	"github.com/amirfaisalz/lensio/apps/api/internal/http/middleware"
	"github.com/amirfaisalz/lensio/apps/api/internal/http/response"
	"github.com/amirfaisalz/lensio/apps/api/internal/store"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// RegisterRequest represents the registration payload.
type RegisterRequest struct {
	FullName string `json:"full_name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// VerifyEmailRequest represents the verification payload.
type VerifyEmailRequest struct {
	Email string `json:"email"`
	Token string `json:"token"`
}

// LoginRequest represents the user login payload.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// CreateOrgRequest represents the organization creation payload.
type CreateOrgRequest struct {
	Name     string `json:"name"`
	PlanCode string `json:"plan_code"`
}

// RegisterHandler handles POST /api/v1/auth/register.
//
// The verification token is delivered by email. It used to be generated, stored
// and then dropped: outside development nobody could ever receive it, so no
// self-service account could ever be verified, and an unverified account cannot
// log in.
func RegisterHandler(accountStore store.AccountStore, sender mailer.Sender, baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if accountStore == nil {
			response.ErrorWithRequest(w, r, http.StatusInternalServerError, response.CodeInternalError, "Account store unavailable")
			return
		}

		var req RegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.ErrorWithRequest(w, r, http.StatusBadRequest, response.CodeInvalidRequest, "Invalid JSON payload")
			return
		}

		fullName := strings.TrimSpace(req.FullName)
		email := strings.ToLower(strings.TrimSpace(req.Email))
		password := strings.TrimSpace(req.Password)

		if fullName == "" {
			response.ErrorWithRequest(w, r, http.StatusBadRequest, response.CodeInvalidRequest, "Nama lengkap tidak boleh kosong")
			return
		}

		if !emailRegex.MatchString(email) {
			response.ErrorWithRequest(w, r, http.StatusBadRequest, response.CodeInvalidRequest, "Alamat email tidak valid")
			return
		}

		if len(password) < 8 {
			response.ErrorWithRequest(w, r, http.StatusBadRequest, response.CodeInvalidRequest, "Kata sandi minimal 8 karakter")
			return
		}

		passwordHash, err := auth.HashPassword(password)
		if err != nil {
			response.ErrorWithRequest(w, r, http.StatusInternalServerError, response.CodeInternalError, "Gagal mengenkripsi kata sandi")
			return
		}

		verificationToken, err := auth.GenerateVerificationToken()
		if err != nil {
			response.ErrorWithRequest(w, r, http.StatusInternalServerError, response.CodeInternalError, "Gagal membuat token verifikasi")
			return
		}

		createdUser, err := accountStore.CreateUser(r.Context(), fullName, email, passwordHash, verificationToken)
		if err != nil {
			if errors.Is(err, store.ErrDuplicateEmail) {
				response.ErrorWithRequest(w, r, http.StatusConflict, response.CodeInvalidRequest, "Email sudah terdaftar. Silakan gunakan email lain atau masuk.")
				return
			}
			slog.ErrorContext(r.Context(), "user registration failed", slog.String("error", err.Error()))
			response.ErrorWithRequest(w, r, http.StatusInternalServerError, response.CodeInternalError, "Gagal mendaftarkan user")
			return
		}

		if sender != nil {
			msg := mailer.VerificationMessage(baseURL, createdUser.Email, verificationToken)
			if err := sender.Send(r.Context(), msg); err != nil {
				// The account exists either way; surface the failure in logs and
				// let the user request a resend rather than failing signup.
				slog.ErrorContext(r.Context(), "failed sending verification email",
					slog.String("error", err.Error()))
			}
		}

		respData := map[string]any{
			"status":  "pending_verification",
			"email":   createdUser.Email,
			"message": "Registrasi berhasil. Silakan periksa email Anda dan lakukan verifikasi sebelum masuk.",
		}
		// Security: never leak verification_token unless explicitly in local development/test.
		// Default deny: empty ENV must NOT expose the token (fail closed for misconfigured prod).
		env := os.Getenv("ENV")
		if env == "development" || env == "test" {
			respData["verification_token"] = verificationToken
		}

		response.JSON(w, http.StatusCreated, respData)
	}
}

// VerifyEmailHandler handles POST /api/v1/auth/verify-email.
func VerifyEmailHandler(accountStore store.AccountStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if accountStore == nil {
			response.ErrorWithRequest(w, r, http.StatusInternalServerError, response.CodeInternalError, "Account store unavailable")
			return
		}

		var req VerifyEmailRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.ErrorWithRequest(w, r, http.StatusBadRequest, response.CodeInvalidRequest, "Invalid JSON payload")
			return
		}

		email := strings.ToLower(strings.TrimSpace(req.Email))
		token := strings.TrimSpace(req.Token)

		if email == "" {
			response.ErrorWithRequest(w, r, http.StatusBadRequest, response.CodeInvalidRequest, "Email tidak boleh kosong")
			return
		}

		if token == "" {
			response.ErrorWithRequest(w, r, http.StatusBadRequest, response.CodeInvalidRequest, "Token verifikasi tidak boleh kosong")
			return
		}

		if err := accountStore.VerifyUserEmail(r.Context(), email, token); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				response.ErrorWithRequest(w, r, http.StatusBadRequest, response.CodeInvalidRequest, "Token verifikasi tidak valid atau email tidak ditemukan")
				return
			}
			response.ErrorWithRequest(w, r, http.StatusInternalServerError, response.CodeInternalError, "Gagal memverifikasi email")
			return
		}

		response.JSON(w, http.StatusOK, map[string]any{
			"status":  "verified",
			"message": "Email berhasil diverifikasi! Silakan masuk dengan akun Anda.",
		})
	}
}

// LoginHandler handles POST /api/v1/auth/login.
func LoginHandler(accountStore store.AccountStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if accountStore == nil {
			response.ErrorWithRequest(w, r, http.StatusInternalServerError, response.CodeInternalError, "Account store unavailable")
			return
		}

		var req LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.ErrorWithRequest(w, r, http.StatusBadRequest, response.CodeInvalidRequest, "Invalid JSON payload")
			return
		}

		email := strings.ToLower(strings.TrimSpace(req.Email))
		password := req.Password

		if email == "" || password == "" {
			response.ErrorWithRequest(w, r, http.StatusBadRequest, response.CodeInvalidRequest, "Email dan kata sandi harus diisi")
			return
		}

		userWithAuth, err := accountStore.GetUserByEmail(r.Context(), email)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				// Spend the same bcrypt time as a real account so the response
				// latency does not reveal whether this email is registered.
				auth.EqualizeLoginTiming(password)
				response.ErrorWithRequest(w, r, http.StatusUnauthorized, response.CodeInvalidAPIKey, "Email atau kata sandi tidak valid.")
				return
			}
			response.ErrorWithRequest(w, r, http.StatusInternalServerError, response.CodeInternalError, "Gagal memproses autentikasi")
			return
		}

		// Verify password hash
		if !auth.VerifyPassword(password, userWithAuth.PasswordHash) {
			response.ErrorWithRequest(w, r, http.StatusUnauthorized, response.CodeInvalidAPIKey, "Email atau kata sandi tidak valid.")
			return
		}

		// Check email verification status
		if !userWithAuth.EmailVerified {
			response.ErrorWithRequest(w, r, http.StatusForbidden, response.CodeEmailNotVerified, "Email belum diverifikasi. Silakan periksa kotak masuk email Anda dan lakukan verifikasi sebelum masuk.")
			return
		}

		// Retrieve associated organization if any
		var orgContext map[string]any
		org, err := accountStore.GetUserOrganization(r.Context(), userWithAuth.ID)
		if err == nil && org != nil {
			orgContext = map[string]any{
				"id":        org.ID,
				"name":      org.Name,
				"slug":      org.Slug,
				"plan_code": org.PlanCode,
			}
		}

		roles := []string{"developer", "ocr:write", "ocr:read", "usage:read"}
		if userWithAuth.Role == "owner" || userWithAuth.Role == "admin" {
			roles = append(roles, "admin")
		}

		username := strings.Split(userWithAuth.Email, "@")[0]
		token, err := auth.CreateSessionToken(userWithAuth.ID, userWithAuth.Email, username, roles, 7*24*time.Hour)
		if err != nil {
			response.ErrorWithRequest(w, r, http.StatusInternalServerError, response.CodeInternalError, "Gagal membuat sesi login")
			return
		}

		isSecure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" || os.Getenv("ENV") == "production"
		http.SetCookie(w, &http.Cookie{
			Name:     "lensio_session",
			Value:    token,
			Path:     "/",
			Expires:  time.Now().Add(7 * 24 * time.Hour),
			MaxAge:   int((7 * 24 * time.Hour).Seconds()),
			HttpOnly: true,
			Secure:   isSecure,
			SameSite: http.SameSiteLaxMode,
		})

		response.JSON(w, http.StatusOK, map[string]any{
			"status": "authenticated",
			"user": map[string]any{
				"id":        userWithAuth.ID,
				"email":     userWithAuth.Email,
				"full_name": userWithAuth.FullName,
				"role":      userWithAuth.Role,
			},
			"organization": orgContext,
		})
	}
}

// SessionRevoker invalidates every session token previously issued to a user.
type SessionRevoker interface {
	RevokeUserSessions(ctx context.Context, userID string) error
}

// SessionCacheInvalidator lets the handler drop the local revocation cache so a
// logout takes effect immediately on the replica that served it.
type SessionCacheInvalidator interface {
	InvalidateRevocationCache(userID string)
}

// LogoutHandler handles POST /api/v1/auth/logout. It clears the session cookie and,
// when a revoker is configured, invalidates the token server-side so a copy taken
// out of the browser stops working instead of living until its 7-day expiry.
func LogoutHandler(revoker SessionRevoker, cache SessionCacheInvalidator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if revoker != nil {
			if userID := sessionUserID(r); userID != "" {
				if err := revoker.RevokeUserSessions(r.Context(), userID); err != nil {
					slog.ErrorContext(r.Context(), "failed revoking user sessions on logout",
						slog.String("error", err.Error()))
				} else if cache != nil {
					cache.InvalidateRevocationCache(userID)
				}
			}
		}

		isSecure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" || os.Getenv("ENV") == "production"
		http.SetCookie(w, &http.Cookie{
			Name:     "lensio_session",
			Value:    "",
			Path:     "/",
			Expires:  time.Unix(0, 0),
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   isSecure,
			SameSite: http.SameSiteLaxMode,
		})

		response.JSON(w, http.StatusOK, map[string]any{
			"status":  "ok",
			"message": "Berhasil keluar dari sesi.",
		})
	}
}

// MeHandler handles GET /api/v1/auth/me to return current authenticated user profile and organization.
func MeHandler(accountStore store.AccountStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if oidcUser := middleware.GetOIDCUser(r.Context()); oidcUser != nil {
			var orgContext map[string]any
			if accountStore != nil {
				org, err := accountStore.GetUserOrganization(r.Context(), oidcUser.Subject)
				if err == nil && org != nil {
					orgContext = map[string]any{
						"id":        org.ID,
						"name":      org.Name,
						"slug":      org.Slug,
						"plan_code": org.PlanCode,
					}
				}
			}

			response.JSON(w, http.StatusOK, map[string]any{
				"user": map[string]any{
					"id":        oidcUser.Subject,
					"email":     oidcUser.Email,
					"full_name": oidcUser.Name,
					"roles":     oidcUser.Roles,
				},
				"organization": orgContext,
			})
			return
		}

		if key := middleware.GetAPIKey(r.Context()); key != nil {
			var orgContext map[string]any
			if accountStore != nil {
				org, err := accountStore.GetOrganization(r.Context(), key.OrgID)
				if err == nil && org != nil {
					orgContext = map[string]any{
						"id":        org.ID,
						"name":      org.Name,
						"slug":      org.Slug,
						"plan_code": org.PlanCode,
					}
				}
			}

			response.JSON(w, http.StatusOK, map[string]any{
				"api_key": map[string]any{
					"id":          key.ID,
					"name":        key.Name,
					"prefix":      key.Prefix,
					"environment": key.Environment,
					"scopes":      key.Scopes,
				},
				"organization": orgContext,
			})
			return
		}

		response.ErrorWithRequest(w, r, http.StatusUnauthorized, response.CodeInvalidAPIKey, "Sesi tidak ditemukan")
	}
}

// CreateOrganizationHandler handles POST /api/v1/account/organizations.
func CreateOrganizationHandler(accountStore store.AccountStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if accountStore == nil {
			response.ErrorWithRequest(w, r, http.StatusInternalServerError, response.CodeInternalError, "Account store unavailable")
			return
		}

		var req CreateOrgRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.ErrorWithRequest(w, r, http.StatusBadRequest, response.CodeInvalidRequest, "Invalid JSON payload")
			return
		}

		name := strings.TrimSpace(req.Name)
		if name == "" {
			response.ErrorWithRequest(w, r, http.StatusBadRequest, response.CodeInvalidRequest, "Nama organisasi tidak boleh kosong")
			return
		}

		planCode := strings.ToLower(strings.TrimSpace(req.PlanCode))
		if planCode == "" {
			planCode = "free"
		}

		// Generate clean slug from name
		slug := strings.ToLower(name)
		slug = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(slug, "-")
		slug = strings.Trim(slug, "-")
		if slug == "" {
			slug = fmt.Sprintf("org-%d", time.Now().Unix())
		} else {
			slug = fmt.Sprintf("%s-%d", slug, time.Now().Unix()%10000)
		}

		org, err := accountStore.CreateOrganization(r.Context(), name, slug, planCode)
		if err != nil {
			slog.ErrorContext(r.Context(), "organization creation failed", slog.String("error", err.Error()))
			response.ErrorWithRequest(w, r, http.StatusInternalServerError, response.CodeInternalError, "Gagal membuat organisasi")
			return
		}

		// Associate current authenticated user with organization
		if oidcUser := middleware.GetOIDCUser(r.Context()); oidcUser != nil {
			// Find user ID by email or subject
			if user, err := accountStore.GetUserByEmail(r.Context(), oidcUser.Email); err == nil && user != nil {
				_ = accountStore.AssignUserToOrg(r.Context(), user.ID, org.ID, "owner")
			}
		}

		response.JSON(w, http.StatusCreated, map[string]any{
			"organization": map[string]any{
				"id":                    org.ID,
				"name":                  org.Name,
				"slug":                  org.Slug,
				"plan_code":             org.PlanCode,
				"plan_name":             org.PlanName,
				"monthly_quota":         org.MonthlyQuota,
				"rate_limit_per_minute": org.RateLimitPerMinute,
			},
		})
	}
}

// sessionUserID extracts the subject from the caller's session cookie or bearer
// token. Logout is an unauthenticated route, so the token is parsed here rather
// than read from a context the auth middleware never populated.
func sessionUserID(r *http.Request) string {
	if user := middleware.GetOIDCUser(r.Context()); user != nil && user.Subject != "" {
		return user.Subject
	}

	token := ""
	if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
		token = strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	} else if cookie, err := r.Cookie("lensio_session"); err == nil {
		token = strings.TrimSpace(cookie.Value)
	}
	if token == "" {
		return ""
	}

	claims, err := auth.ValidateSessionToken(token, auth.GetTokenSecret())
	if err != nil {
		return ""
	}
	return claims.Sub
}

// OrgMembershipLister returns every organization a user belongs to.
type OrgMembershipLister interface {
	ListUserOrganizations(ctx context.Context, userID string) ([]store.OrganizationMembership, error)
}

// ListOrganizationsHandler handles GET /api/v1/account/organizations.
//
// The dashboard used to keep its organization list in localStorage, which the
// user controls and which survived a logout into the next account's session.
// This makes the server the source of truth.
func ListOrganizationsHandler(lister OrgMembershipLister) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		empty := func() {
			response.JSON(w, http.StatusOK, map[string]any{"data": []store.OrganizationMembership{}})
		}

		if lister == nil {
			empty()
			return
		}

		user := middleware.GetOIDCUser(r.Context())
		if user == nil || strings.TrimSpace(user.Subject) == "" {
			// API keys are bound to a single organization and have no membership set.
			empty()
			return
		}

		memberships, err := lister.ListUserOrganizations(r.Context(), user.Subject)
		if err != nil {
			slog.ErrorContext(r.Context(), "failed listing user organizations", slog.String("error", err.Error()))
			response.ErrorWithRequest(w, r, http.StatusInternalServerError, response.CodeInternalError, "Gagal memuat daftar organisasi")
			return
		}

		response.JSON(w, http.StatusOK, map[string]any{"data": memberships})
	}
}
