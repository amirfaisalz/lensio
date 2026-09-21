package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/auth"
	"github.com/amirfaisalz/lensio/apps/api/internal/email"
	"github.com/amirfaisalz/lensio/apps/api/internal/http/response"
	"github.com/amirfaisalz/lensio/apps/api/internal/store"
)

// PasswordResetStore is the narrow slice of the account store this flow needs.
type PasswordResetStore interface {
	SetPasswordResetToken(ctx context.Context, email, tokenHash string, expiresAt time.Time) error
	ConsumePasswordResetToken(ctx context.Context, email, tokenHash, newPasswordHash string, now time.Time) error
}

// RequestPasswordResetRequest is the payload for starting a reset.
type RequestPasswordResetRequest struct {
	Email string `json:"email"`
}

// ConfirmPasswordResetRequest is the payload for completing a reset.
type ConfirmPasswordResetRequest struct {
	Email       string `json:"email"`
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

// HashResetToken hashes a reset token for storage. Only the hash is persisted,
// for the same reason API keys are hashed: possession of the database must not
// be enough to take over an account.
func HashResetToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

// RequestPasswordResetHandler handles POST /api/v1/auth/password-reset.
//
// It always answers the same way whether or not the address is registered.
// Anything else would make this endpoint an account enumeration oracle, which
// matters more here than elsewhere because it is unauthenticated.
func RequestPasswordResetHandler(resetStore PasswordResetStore, sender email.Sender, baseURL string) http.HandlerFunc {
	const genericMessage = "Jika email tersebut terdaftar, tautan pengaturan ulang kata sandi telah dikirim."

	return func(w http.ResponseWriter, r *http.Request) {
		var req RequestPasswordResetRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.ErrorWithRequest(w, r, http.StatusBadRequest, response.CodeInvalidRequest, "Invalid JSON payload")
			return
		}

		addr := strings.ToLower(strings.TrimSpace(req.Email))
		if !emailRegex.MatchString(addr) {
			response.ErrorWithRequest(w, r, http.StatusBadRequest, response.CodeInvalidRequest, "Alamat email tidak valid")
			return
		}

		ok := func() {
			response.JSON(w, http.StatusOK, map[string]any{"status": "ok", "message": genericMessage})
		}

		if resetStore == nil {
			ok()
			return
		}

		token, err := auth.GenerateVerificationToken()
		if err != nil {
			slog.ErrorContext(r.Context(), "failed generating password reset token", slog.String("error", err.Error()))
			response.ErrorWithRequest(w, r, http.StatusInternalServerError, response.CodeInternalError, "Gagal memproses permintaan")
			return
		}

		err = resetStore.SetPasswordResetToken(r.Context(), addr, HashResetToken(token), time.Now().Add(store.PasswordResetTTL))
		if err != nil {
			if !errors.Is(err, store.ErrNotFound) {
				slog.ErrorContext(r.Context(), "failed storing password reset token", slog.String("error", err.Error()))
			}
			// Unknown address or a store failure: answer identically either way.
			ok()
			return
		}

		if sender != nil {
			msg := email.PasswordResetMessage(baseURL, addr, token, store.PasswordResetTTL)
			if err := sender.Send(r.Context(), msg); err != nil {
				slog.ErrorContext(r.Context(), "failed sending password reset email", slog.String("error", err.Error()))
			}
		}

		ok()
	}
}

// ConfirmPasswordResetHandler handles POST /api/v1/auth/password-reset/confirm.
// A successful reset also invalidates every existing session for that user.
func ConfirmPasswordResetHandler(resetStore PasswordResetStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if resetStore == nil {
			response.ErrorWithRequest(w, r, http.StatusInternalServerError, response.CodeInternalError, "Account store unavailable")
			return
		}

		var req ConfirmPasswordResetRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.ErrorWithRequest(w, r, http.StatusBadRequest, response.CodeInvalidRequest, "Invalid JSON payload")
			return
		}

		addr := strings.ToLower(strings.TrimSpace(req.Email))
		token := strings.TrimSpace(req.Token)
		if addr == "" || token == "" {
			response.ErrorWithRequest(w, r, http.StatusBadRequest, response.CodeInvalidRequest, "Email dan token wajib diisi")
			return
		}
		if len(req.NewPassword) < 8 {
			response.ErrorWithRequest(w, r, http.StatusBadRequest, response.CodeInvalidRequest, "Kata sandi minimal 8 karakter")
			return
		}

		newHash, err := auth.HashPassword(req.NewPassword)
		if err != nil {
			response.ErrorWithRequest(w, r, http.StatusInternalServerError, response.CodeInternalError, "Gagal mengenkripsi kata sandi")
			return
		}

		err = resetStore.ConsumePasswordResetToken(r.Context(), addr, HashResetToken(token), newHash, time.Now())
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				// Wrong, already used, or expired — one message for all three.
				response.ErrorWithRequest(w, r, http.StatusBadRequest, response.CodeInvalidRequest,
					"Token tidak valid atau sudah kedaluwarsa. Silakan minta tautan baru.")
				return
			}
			slog.ErrorContext(r.Context(), "failed consuming password reset token", slog.String("error", err.Error()))
			response.ErrorWithRequest(w, r, http.StatusInternalServerError, response.CodeInternalError, "Gagal mengatur ulang kata sandi")
			return
		}

		response.JSON(w, http.StatusOK, map[string]any{
			"status":  "ok",
			"message": "Kata sandi berhasil diatur ulang. Semua sesi lama telah dikeluarkan. Silakan masuk kembali.",
		})
	}
}
