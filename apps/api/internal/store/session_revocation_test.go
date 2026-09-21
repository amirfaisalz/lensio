package store_test

import (
	"context"
	"errors"
	"fmt"
	"os"
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
