package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// AuditLog represents an immutable record of an administrative or security event.
type AuditLog struct {
	ID             string         `json:"id"`
	OrgID          string         `json:"org_id"`
	ActorID        string         `json:"actor_id,omitempty"`
	Action         string         `json:"action"`
	TargetResource string         `json:"target_resource"`
	Metadata       map[string]any `json:"metadata"`
	CreatedAt      time.Time      `json:"created_at"`
}

// AuditStore specifies repository operations for persisting and listing security audit trails.
type AuditStore interface {
	RecordAuditLog(ctx context.Context, log *AuditLog) error
	ListAuditLogsByOrg(ctx context.Context, orgID string) ([]*AuditLog, error)
}

// RecordAuditLog inserts a new audit log entry into PostgreSQL.
func (db *DB) RecordAuditLog(ctx context.Context, log *AuditLog) error {
	if log == nil {
		return errors.New("audit log is nil")
	}
	if log.OrgID == "" {
		return errors.New("orgID is required")
	}
	if log.Action == "" {
		return errors.New("action is required")
	}

	metadataJSON, err := json.Marshal(log.Metadata)
	if err != nil {
		return fmt.Errorf("marshaling audit log metadata: %w", err)
	}

	query := `
		INSERT INTO audit_logs (org_id, actor_id, action, target_resource, metadata)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at;
	`

	err = db.QueryRowContext(
		ctx,
		query,
		log.OrgID,
		log.ActorID,
		log.Action,
		log.TargetResource,
		metadataJSON,
	).Scan(&log.ID, &log.CreatedAt)
	if err != nil {
		return fmt.Errorf("inserting audit log: %w", err)
	}

	return nil
}

// ListAuditLogsByOrg retrieves the recent audit logs for an organization.
func (db *DB) ListAuditLogsByOrg(ctx context.Context, orgID string) ([]*AuditLog, error) {
	if orgID == "" {
		return nil, errors.New("orgID is required")
	}

	query := `
		SELECT id, org_id, COALESCE(actor_id, ''), action, target_resource, metadata, created_at
		FROM audit_logs
		WHERE org_id = $1
		ORDER BY created_at DESC;
	`

	rows, err := db.QueryContext(ctx, query, orgID)
	if err != nil {
		return nil, fmt.Errorf("querying audit logs: %w", err)
	}
	defer rows.Close()

	var logs []*AuditLog
	for rows.Next() {
		var (
			al          AuditLog
			metadataRaw []byte
		)

		if err := rows.Scan(
			&al.ID,
			&al.OrgID,
			&al.ActorID,
			&al.Action,
			&al.TargetResource,
			&metadataRaw,
			&al.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning audit log row: %w", err)
		}

		if err := json.Unmarshal(metadataRaw, &al.Metadata); err != nil {
			al.Metadata = make(map[string]any)
		}

		logs = append(logs, &al)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating audit log rows: %w", err)
	}

	if logs == nil {
		logs = []*AuditLog{}
	}

	return logs, nil
}
