package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// OCRRequest represents the non-PII execution metadata record of an OCR operation.
type OCRRequest struct {
	ID         string    `json:"id"`
	OrgID      string    `json:"org_id"`
	APIKeyID   *string   `json:"api_key_id,omitempty"`
	Status     string    `json:"status"`
	Confidence float64   `json:"confidence"`
	LatencyMS  int       `json:"latency_ms"`
	DocType    string    `json:"doc_type"`
	CreatedAt  time.Time `json:"created_at"`
}

// OCRRequestStore defines operations for persisting and retrieving OCR request metadata.
type OCRRequestStore interface {
	CreateOCRRequest(ctx context.Context, req *OCRRequest) error
	GetOCRRequestByID(ctx context.Context, orgID string, id string) (*OCRRequest, error)
}

// CreateOCRRequest inserts an OCR request metadata record.
func (db *DB) CreateOCRRequest(ctx context.Context, req *OCRRequest) error {
	if req == nil {
		return errors.New("ocr request is nil")
	}

	if req.DocType == "" {
		req.DocType = "ktp"
	}

	query := `
		INSERT INTO ocr_requests (id, org_id, api_key_id, status, confidence, latency_ms, doc_type)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at;
	`

	err := db.QueryRowContext(
		ctx,
		query,
		req.ID,
		req.OrgID,
		req.APIKeyID,
		req.Status,
		req.Confidence,
		req.LatencyMS,
		req.DocType,
	).Scan(&req.CreatedAt)
	if err != nil {
		return fmt.Errorf("inserting ocr request: %w", err)
	}

	return nil
}

// GetOCRRequestByID retrieves an OCR request metadata record by ID ensuring tenant isolation (org_id match).
func (db *DB) GetOCRRequestByID(ctx context.Context, orgID string, id string) (*OCRRequest, error) {
	query := `
		SELECT id, org_id, api_key_id, status, confidence, latency_ms, doc_type, created_at
		FROM ocr_requests
		WHERE id = $1 AND org_id = $2;
	`

	var (
		req        OCRRequest
		apiKeyID   sql.NullString
		confidence sql.NullFloat64
	)

	err := db.QueryRowContext(ctx, query, id, orgID).Scan(
		&req.ID,
		&req.OrgID,
		&apiKeyID,
		&req.Status,
		&confidence,
		&req.LatencyMS,
		&req.DocType,
		&req.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("querying ocr request by id: %w", err)
	}

	if apiKeyID.Valid {
		req.APIKeyID = &apiKeyID.String
	}
	if confidence.Valid {
		req.Confidence = confidence.Float64
	}

	return &req, nil
}
