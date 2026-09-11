package store_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/store"
)

func TestNew_EmptyURL(t *testing.T) {
	ctx := context.Background()
	db, err := store.New(ctx, "")
	if err == nil {
		if db != nil {
			_ = db.Close()
		}
		t.Fatal("expected error for empty database URL, got nil")
	}
}

func TestNew_UnreachableHost(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Use an unreachable port/host that will immediately fail ping
	db, err := store.New(ctx, "postgres://invalid:user@127.0.0.1:54399/lensio?sslmode=disable&connect_timeout=1")
	if err == nil {
		if db != nil {
			_ = db.Close()
		}
		t.Fatal("expected error for unreachable database, got nil")
	}
}

func TestNew_ValidConnection(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://lensio:lensio_dev_password@localhost:5432/lensio?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	db, err := store.New(ctx, dbURL)
	if err != nil {
		t.Skipf("skipping live database test (db unavailable): %v", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("failed to ping live db: %v", err)
	}
}
