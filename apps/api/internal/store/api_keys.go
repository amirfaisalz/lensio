package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var (
	// ErrNotFound indicates a queried entity was not found in the store.
	ErrNotFound = errors.New("entity not found")
)

// APIKey represents an authenticated API key entity in the database.
type APIKey struct {
	ID          string     `json:"id"`
	OrgID       string     `json:"org_id"`
	Name        string     `json:"name"`
	KeyHash     string     `json:"-"`
	Prefix      string     `json:"prefix"`
	Scopes      []string   `json:"scopes"`
	Environment string     `json:"environment"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// APIKeyStore specifies operations for managing API keys.
type APIKeyStore interface {
	CreateAPIKey(ctx context.Context, key *APIKey) error
	GetAPIKeyByHash(ctx context.Context, keyHash string) (*APIKey, error)
	ListAPIKeysByOrg(ctx context.Context, orgID string) ([]*APIKey, error)
	RevokeAPIKey(ctx context.Context, orgID string, keyID string) error
	TouchAPIKeyLastUsed(ctx context.Context, keyID string, lastUsed time.Time) error
}

// CreateAPIKey inserts a new API key record.
func (db *DB) CreateAPIKey(ctx context.Context, key *APIKey) error {
	if key == nil {
		return errors.New("key is nil")
	}

	scopesJSON, err := json.Marshal(key.Scopes)
	if err != nil {
		return fmt.Errorf("marshaling scopes: %w", err)
	}

	query := `
		INSERT INTO api_keys (org_id, name, key_hash, prefix, scopes, environment, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at;
	`

	err = db.QueryRowContext(
		ctx,
		query,
		key.OrgID,
		key.Name,
		key.KeyHash,
		key.Prefix,
		scopesJSON,
		key.Environment,
		key.ExpiresAt,
	).Scan(&key.ID, &key.CreatedAt)
	if err != nil {
		return fmt.Errorf("inserting api key: %w", err)
	}

	return nil
}

// GetAPIKeyByHash retrieves an API key by its SHA-256 hash.
func (db *DB) GetAPIKeyByHash(ctx context.Context, keyHash string) (*APIKey, error) {
	query := `
		SELECT id, org_id, name, key_hash, prefix, scopes, environment, last_used_at, expires_at, revoked_at, created_at
		FROM api_keys
		WHERE key_hash = $1;
	`

	var (
		k          APIKey
		scopesRaw  []byte
		lastUsedAt sql.NullTime
		expiresAt  sql.NullTime
		revokedAt  sql.NullTime
	)

	err := db.QueryRowContext(ctx, query, keyHash).Scan(
		&k.ID,
		&k.OrgID,
		&k.Name,
		&k.KeyHash,
		&k.Prefix,
		&scopesRaw,
		&k.Environment,
		&lastUsedAt,
		&expiresAt,
		&revokedAt,
		&k.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("querying api key by hash: %w", err)
	}

	if err := json.Unmarshal(scopesRaw, &k.Scopes); err != nil {
		k.Scopes = []string{}
	}

	if lastUsedAt.Valid {
		k.LastUsedAt = &lastUsedAt.Time
	}
	if expiresAt.Valid {
		k.ExpiresAt = &expiresAt.Time
	}
	if revokedAt.Valid {
		k.RevokedAt = &revokedAt.Time
	}

	return &k, nil
}

// ListAPIKeysByOrg lists all API keys belonging to an organization.
func (db *DB) ListAPIKeysByOrg(ctx context.Context, orgID string) ([]*APIKey, error) {
	query := `
		SELECT id, org_id, name, key_hash, prefix, scopes, environment, last_used_at, expires_at, revoked_at, created_at
		FROM api_keys
		WHERE org_id = $1
		ORDER BY created_at DESC;
	`

	rows, err := db.QueryContext(ctx, query, orgID)
	if err != nil {
		return nil, fmt.Errorf("listing api keys: %w", err)
	}
	defer rows.Close()

	var keys []*APIKey
	for rows.Next() {
		var (
			k          APIKey
			scopesRaw  []byte
			lastUsedAt sql.NullTime
			expiresAt  sql.NullTime
			revokedAt  sql.NullTime
		)

		if err := rows.Scan(
			&k.ID,
			&k.OrgID,
			&k.Name,
			&k.KeyHash,
			&k.Prefix,
			&scopesRaw,
			&k.Environment,
			&lastUsedAt,
			&expiresAt,
			&revokedAt,
			&k.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning api key row: %w", err)
		}

		if err := json.Unmarshal(scopesRaw, &k.Scopes); err != nil {
			k.Scopes = []string{}
		}
		if lastUsedAt.Valid {
			k.LastUsedAt = &lastUsedAt.Time
		}
		if expiresAt.Valid {
			k.ExpiresAt = &expiresAt.Time
		}
		if revokedAt.Valid {
			k.RevokedAt = &revokedAt.Time
		}

		keys = append(keys, &k)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating api key rows: %w", err)
	}

	if keys == nil {
		keys = []*APIKey{}
	}

	return keys, nil
}

// RevokeAPIKey marks an API key as revoked for a specific organization.
func (db *DB) RevokeAPIKey(ctx context.Context, orgID string, keyID string) error {
	query := `
		UPDATE api_keys
		SET revoked_at = NOW()
		WHERE id = $1 AND org_id = $2 AND revoked_at IS NULL;
	`

	res, err := db.ExecContext(ctx, query, keyID, orgID)
	if err != nil {
		return fmt.Errorf("revoking api key: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected on revoke: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

// TouchAPIKeyLastUsed asynchronously or synchronously updates last_used_at.
func (db *DB) TouchAPIKeyLastUsed(ctx context.Context, keyID string, lastUsed time.Time) error {
	query := `
		UPDATE api_keys
		SET last_used_at = $2
		WHERE id = $1;
	`

	_, err := db.ExecContext(ctx, query, keyID, lastUsed)
	if err != nil {
		return fmt.Errorf("updating api key last used: %w", err)
	}

	return nil
}
