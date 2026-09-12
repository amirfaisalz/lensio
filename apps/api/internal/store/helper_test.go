package store_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/store"
)

func createTestOrg(t *testing.T, db *store.DB) *store.Organization {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	slug := fmt.Sprintf("test-org-%d", time.Now().UnixNano())
	org, err := db.CreateOrganization(ctx, "Test Org "+slug, slug, "free")
	if err != nil {
		t.Fatalf("failed creating test organization: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM organizations WHERE id = $1", org.ID)
	})
	return org
}
