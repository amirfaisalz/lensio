package idempotency

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// PostgresStore implements Store using a PostgreSQL database handle.
type PostgresStore struct {
	db         *sql.DB
	defaultTTL time.Duration
	nowFunc    func() time.Time
}

// NewPostgresStore initializes a PostgreSQL-backed idempotency store.
func NewPostgresStore(db *sql.DB, defaultTTL time.Duration) *PostgresStore {
	if defaultTTL <= 0 {
		defaultTTL = 24 * time.Hour
	}
	return &PostgresStore{
		db:         db,
		defaultTTL: defaultTTL,
		nowFunc:    time.Now,
	}
}

// LockOrGet attempts to atomically acquire or retrieve an idempotency key.
func (p *PostgresStore) LockOrGet(ctx context.Context, orgID, key, requestHash string, ttl time.Duration) (*Record, bool, error) {
	if orgID == "" {
		return nil, false, ErrOrgIDRequired
	}
	if key == "" {
		return nil, false, ErrKeyRequired
	}
	if p.db == nil {
		return nil, false, errors.New("database handle is nil")
	}
	if ttl <= 0 {
		ttl = p.defaultTTL
	}

	now := p.nowFunc().UTC()
	expiresAt := now.Add(ttl)

	upsertQuery := `
		INSERT INTO idempotency_keys (key, org_id, request_hash, status, created_at, expires_at)
		VALUES ($1, $2, $3, 'in_progress', $4, $5)
		ON CONFLICT (org_id, key) DO UPDATE
		SET request_hash = EXCLUDED.request_hash,
		    status = EXCLUDED.status,
		    status_code = NULL,
		    headers = NULL,
		    response_body = NULL,
		    created_at = EXCLUDED.created_at,
		    expires_at = EXCLUDED.expires_at
		WHERE idempotency_keys.expires_at <= $4
		RETURNING key, org_id, request_hash, status, status_code, headers, response_body, created_at, expires_at;
	`

	var (
		rec          Record
		statusCode   sql.NullInt64
		headersRaw   []byte
		responseBody []byte
	)

	err := p.db.QueryRowContext(ctx, upsertQuery, key, orgID, requestHash, now, expiresAt).Scan(
		&rec.Key,
		&rec.OrgID,
		&rec.RequestHash,
		&rec.Status,
		&statusCode,
		&headersRaw,
		&responseBody,
		&rec.CreatedAt,
		&rec.ExpiresAt,
	)

	if err == nil {
		// Insert or expired-overwrite succeeded -> this is a new lock acquisition
		if statusCode.Valid {
			rec.StatusCode = int(statusCode.Int64)
		}
		if len(headersRaw) > 0 {
			_ = json.Unmarshal(headersRaw, &rec.Headers)
		}
		rec.ResponseBody = responseBody
		return &rec, true, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return nil, false, fmt.Errorf("upserting idempotency key: %w", err)
	}

	// Conflict with active unexpired record: fetch current state
	selectQuery := `
		SELECT key, org_id, request_hash, status, status_code, headers, response_body, created_at, expires_at
		FROM idempotency_keys
		WHERE org_id = $1 AND key = $2;
	`

	err = p.db.QueryRowContext(ctx, selectQuery, orgID, key).Scan(
		&rec.Key,
		&rec.OrgID,
		&rec.RequestHash,
		&rec.Status,
		&statusCode,
		&headersRaw,
		&responseBody,
		&rec.CreatedAt,
		&rec.ExpiresAt,
	)
	if err != nil {
		return nil, false, fmt.Errorf("fetching existing idempotency key: %w", err)
	}

	if statusCode.Valid {
		rec.StatusCode = int(statusCode.Int64)
	}
	if len(headersRaw) > 0 {
		_ = json.Unmarshal(headersRaw, &rec.Headers)
	}
	rec.ResponseBody = responseBody

	return &rec, false, nil
}

// Complete saves response status, headers, and body, marking the record completed.
func (p *PostgresStore) Complete(ctx context.Context, orgID, key string, statusCode int, headers map[string]string, body []byte) error {
	if orgID == "" {
		return ErrOrgIDRequired
	}
	if key == "" {
		return ErrKeyRequired
	}
	if p.db == nil {
		return errors.New("database handle is nil")
	}

	var headersJSON []byte
	if len(headers) > 0 {
		var err error
		headersJSON, err = json.Marshal(headers)
		if err != nil {
			return fmt.Errorf("marshaling idempotency headers: %w", err)
		}
	}

	query := `
		UPDATE idempotency_keys
		SET status = 'completed',
		    status_code = $3,
		    headers = $4,
		    response_body = $5
		WHERE org_id = $1 AND key = $2;
	`

	_, err := p.db.ExecContext(ctx, query, orgID, key, statusCode, headersJSON, body)
	if err != nil {
		return fmt.Errorf("completing idempotency record: %w", err)
	}

	return nil
}

// Release removes an in-progress record if the request failed with an unrecoverable 5xx or panic.
func (p *PostgresStore) Release(ctx context.Context, orgID, key string) error {
	if orgID == "" {
		return ErrOrgIDRequired
	}
	if key == "" {
		return ErrKeyRequired
	}
	if p.db == nil {
		return errors.New("database handle is nil")
	}

	query := `
		DELETE FROM idempotency_keys
		WHERE org_id = $1 AND key = $2 AND status = 'in_progress';
	`

	_, err := p.db.ExecContext(ctx, query, orgID, key)
	if err != nil {
		return fmt.Errorf("releasing idempotency key: %w", err)
	}

	return nil
}

// CleanupStale deletes all expired idempotency records from PostgreSQL.
func (p *PostgresStore) CleanupStale(ctx context.Context) (int64, error) {
	if p.db == nil {
		return 0, errors.New("database handle is nil")
	}

	now := p.nowFunc().UTC()
	query := `DELETE FROM idempotency_keys WHERE expires_at <= $1;`

	res, err := p.db.ExecContext(ctx, query, now)
	if err != nil {
		return 0, fmt.Errorf("deleting stale idempotency keys: %w", err)
	}

	return res.RowsAffected()
}
