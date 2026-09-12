package store_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/apikey"
	"github.com/amirfaisalz/lensio/apps/api/internal/store"
)

func TestCreateAPIKey_NilKey(t *testing.T) {
	var db store.DB
	err := db.CreateAPIKey(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error when inserting nil key, got nil")
	}
}

func TestAPIKeyStore_LiveDB(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://lensio:lensio_dev_password@localhost:5432/lensio?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := store.New(ctx, dbURL)
	if err != nil {
		t.Skipf("skipping live database tests: %v", err)
	}
	defer db.Close()

	if err := store.RunMigrationsUp(db.DB); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	testOrg := createTestOrg(t, db)
	testOrgID := testOrg.ID

	// 1. Generate & Insert Key
	gen, err := apikey.Generate(apikey.EnvLive)
	if err != nil {
		t.Fatalf("failed generating key: %v", err)
	}

	exp := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)
	k := &store.APIKey{
		OrgID:       testOrgID,
		Name:        "Test Key",
		KeyHash:     gen.KeyHash,
		Prefix:      gen.Prefix,
		Scopes:      []string{"ocr:read", "ocr:write"},
		Environment: gen.Environment,
		ExpiresAt:   &exp,
	}

	if err := db.CreateAPIKey(ctx, k); err != nil {
		t.Fatalf("failed creating api key in db: %v", err)
	}
	if k.ID == "" {
		t.Fatal("expected generated key ID, got empty")
	}

	// 2. Fetch by Hash
	fetched, err := db.GetAPIKeyByHash(ctx, gen.KeyHash)
	if err != nil {
		t.Fatalf("failed fetching key by hash: %v", err)
	}
	if fetched.ID != k.ID {
		t.Errorf("expected ID %q, got %q", k.ID, fetched.ID)
	}
	if fetched.Name != "Test Key" {
		t.Errorf("expected Name 'Test Key', got %q", fetched.Name)
	}
	if len(fetched.Scopes) != 2 || fetched.Scopes[0] != "ocr:read" {
		t.Errorf("unexpected scopes: %+v", fetched.Scopes)
	}

	// 3. Touch Last Used
	touchTime := time.Now().UTC().Truncate(time.Second)
	if err := db.TouchAPIKeyLastUsed(ctx, k.ID, touchTime); err != nil {
		t.Fatalf("failed touching last used: %v", err)
	}
	refetched, _ := db.GetAPIKeyByHash(ctx, gen.KeyHash)
	if refetched.LastUsedAt == nil {
		t.Fatal("expected last_used_at to be set, got nil")
	}

	// 4. List by Org
	list, err := db.ListAPIKeysByOrg(ctx, testOrgID)
	if err != nil {
		t.Fatalf("failed listing keys: %v", err)
	}
	if len(list) == 0 {
		t.Fatal("expected at least 1 key in list")
	}

	// 5. Revoke Key
	if err := db.RevokeAPIKey(ctx, testOrgID, k.ID); err != nil {
		t.Fatalf("failed revoking key: %v", err)
	}

	// 6. Revoke non-existent key returns ErrNotFound
	err = db.RevokeAPIKey(ctx, testOrgID, "00000000-0000-0000-0000-999999999999")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound for non-existent key, got %v", err)
	}

	// 7. Non-existent hash query
	_, err = db.GetAPIKeyByHash(ctx, "non_existent_hash_123456789012345678901234567890123456789012345678")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound for non-existent hash, got %v", err)
	}
}
