package store_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/store"
)

func TestSessionRevocation_LiveDB(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://lensio:lensio_dev_password@localhost:5432/lensio?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := store.New(ctx, dbURL)
	if err != nil {
		t.Skipf("skipping live database tests: %v", err)
	}
	defer db.Close()

	if err := store.RunMigrationsUp(db.DB); err != nil {
		t.Fatalf("failed running migrations: %v", err)
	}

	email := fmt.Sprintf("revoke_%d@lensio.dev", time.Now().UnixNano())
	user, err := db.CreateUser(ctx, "Revoke Tester", email, "$2a$10$hash", "tok")
	if err != nil {
		t.Fatalf("creating user: %v", err)
	}
	t.Cleanup(func() {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = db.ExecContext(c, "DELETE FROM users WHERE id = $1", user.ID)
	})

	t.Run("a fresh user accepts sessions from the epoch", func(t *testing.T) {
		validFrom, err := db.SessionsValidFrom(ctx, user.ID)
		if err != nil {
			t.Fatalf("SessionsValidFrom: %v", err)
		}
		if validFrom.After(time.Now().Add(-time.Hour)) {
			t.Fatalf("expected an epoch-like default, got %s", validFrom)
		}
	})

	t.Run("revoking moves the cutoff past tokens issued before it", func(t *testing.T) {
		issuedAt := time.Now()
		time.Sleep(10 * time.Millisecond)

		if err := db.RevokeUserSessions(ctx, user.ID); err != nil {
			t.Fatalf("RevokeUserSessions: %v", err)
		}

		validFrom, err := db.SessionsValidFrom(ctx, user.ID)
		if err != nil {
			t.Fatalf("SessionsValidFrom after revoke: %v", err)
		}
		if !issuedAt.Before(validFrom) {
			t.Fatalf("token issued at %s should predate the cutoff %s", issuedAt, validFrom)
		}
	})

	t.Run("unknown user is reported, not silently accepted", func(t *testing.T) {
		if err := db.RevokeUserSessions(ctx, "00000000-0000-0000-0000-0000000000ff"); !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("expected ErrNotFound revoking an unknown user, got %v", err)
		}
		if _, err := db.SessionsValidFrom(ctx, "00000000-0000-0000-0000-0000000000ff"); !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("expected ErrNotFound reading an unknown user, got %v", err)
		}
	})

	t.Run("empty user id is rejected", func(t *testing.T) {
		if err := db.RevokeUserSessions(ctx, "   "); err == nil {
			t.Fatal("expected an error for a blank user id")
		}
		if _, err := db.SessionsValidFrom(ctx, ""); err == nil {
			t.Fatal("expected an error for a blank user id")
		}
	})
}

// Verifying an email must require the token; an empty token used to verify by
// email alone, which activated any account without mailbox access.
func TestVerifyUserEmail_RequiresToken_LiveDB(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://lensio:lensio_dev_password@localhost:5432/lensio?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := store.New(ctx, dbURL)
	if err != nil {
		t.Skipf("skipping live database tests: %v", err)
	}
	defer db.Close()

	if err := store.RunMigrationsUp(db.DB); err != nil {
		t.Fatalf("failed running migrations: %v", err)
	}

	email := fmt.Sprintf("verify_%d@lensio.dev", time.Now().UnixNano())
	user, err := db.CreateUser(ctx, "Verify Tester", email, "$2a$10$hash", "the-real-token")
	if err != nil {
		t.Fatalf("creating user: %v", err)
	}
	t.Cleanup(func() {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = db.ExecContext(c, "DELETE FROM users WHERE id = $1", user.ID)
	})

	if err := db.VerifyUserEmail(ctx, email, ""); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("empty token must not verify, got %v", err)
	}
	if err := db.VerifyUserEmail(ctx, email, "wrong-token"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("wrong token must not verify, got %v", err)
	}

	after, err := db.GetUserByEmail(ctx, email)
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	if after.EmailVerified {
		t.Fatal("account was verified despite no valid token being supplied")
	}

	if err := db.VerifyUserEmail(ctx, email, "the-real-token"); err != nil {
		t.Fatalf("the correct token must verify, got %v", err)
	}
}

func TestPasswordResetToken_LiveDB(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://lensio:lensio_dev_password@localhost:5432/lensio?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	db, err := store.New(ctx, dbURL)
	if err != nil {
		t.Skipf("skipping live database tests: %v", err)
	}
	defer db.Close()

	if err := store.RunMigrationsUp(db.DB); err != nil {
		t.Fatalf("failed running migrations: %v", err)
	}

	email := fmt.Sprintf("reset_%d@lensio.dev", time.Now().UnixNano())
	user, err := db.CreateUser(ctx, "Reset Tester", email, "$2a$10$oldhash", "tok")
	if err != nil {
		t.Fatalf("creating user: %v", err)
	}
	t.Cleanup(func() {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = db.ExecContext(c, "DELETE FROM users WHERE id = $1", user.ID)
	})

	const hash = "a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f90"
	now := time.Now()

	t.Run("an unknown address is reported, not silently accepted", func(t *testing.T) {
		err := db.SetPasswordResetToken(ctx, "nobody@lensio.dev", hash, now.Add(time.Hour))
		if !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("a valid token sets the password and is then single use", func(t *testing.T) {
		if err := db.SetPasswordResetToken(ctx, email, hash, now.Add(time.Hour)); err != nil {
			t.Fatalf("SetPasswordResetToken: %v", err)
		}

		if err := db.ConsumePasswordResetToken(ctx, email, hash, "$2a$10$newhash", now); err != nil {
			t.Fatalf("first consume must succeed: %v", err)
		}

		after, err := db.GetUserByEmail(ctx, email)
		if err != nil {
			t.Fatalf("GetUserByEmail: %v", err)
		}
		if after.PasswordHash != "$2a$10$newhash" {
			t.Errorf("password hash was not updated, got %q", after.PasswordHash)
		}

		// Replaying the same token must fail: the update that matched it cleared it.
		if err := db.ConsumePasswordResetToken(ctx, email, hash, "$2a$10$thirdhash", now); !errors.Is(err, store.ErrNotFound) {
			t.Errorf("a reset token must be single use, second attempt returned %v", err)
		}
	})

	t.Run("a successful reset invalidates existing sessions", func(t *testing.T) {
		validFrom, err := db.SessionsValidFrom(ctx, user.ID)
		if err != nil {
			t.Fatalf("SessionsValidFrom: %v", err)
		}
		if !validFrom.After(now.Add(-time.Minute)) {
			t.Errorf("sessions_valid_from = %s, expected it to advance to the reset time", validFrom)
		}
	})

	t.Run("an expired token is refused", func(t *testing.T) {
		if err := db.SetPasswordResetToken(ctx, email, hash, now.Add(-time.Minute)); err != nil {
			t.Fatalf("SetPasswordResetToken: %v", err)
		}
		if err := db.ConsumePasswordResetToken(ctx, email, hash, "$2a$10$expiredhash", now); !errors.Is(err, store.ErrNotFound) {
			t.Errorf("an expired token must be refused, got %v", err)
		}
	})

	t.Run("a wrong token hash is refused", func(t *testing.T) {
		if err := db.SetPasswordResetToken(ctx, email, hash, now.Add(time.Hour)); err != nil {
			t.Fatalf("SetPasswordResetToken: %v", err)
		}
		wrong := strings.Repeat("f", 64)
		if err := db.ConsumePasswordResetToken(ctx, email, wrong, "$2a$10$wronghash", now); !errors.Is(err, store.ErrNotFound) {
			t.Errorf("a wrong token must be refused, got %v", err)
		}
	})

	t.Run("blank arguments are rejected", func(t *testing.T) {
		if err := db.SetPasswordResetToken(ctx, "", hash, now); err == nil {
			t.Error("expected an error for a blank email")
		}
		if err := db.SetPasswordResetToken(ctx, email, "", now); err == nil {
			t.Error("expected an error for a blank token hash")
		}
		if err := db.ConsumePasswordResetToken(ctx, email, hash, "", now); err == nil {
			t.Error("expected an error for a blank password hash")
		}
	})
}
