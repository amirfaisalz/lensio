package idempotency

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNilRecord       = errors.New("idempotency record is nil")
	ErrKeyRequired     = errors.New("idempotency key is required")
	ErrOrgIDRequired   = errors.New("orgID is required")
	ErrPayloadMismatch = errors.New("payload hash does not match previous request")
	ErrRequestInFlight = errors.New("request with this idempotency key is already in progress")
)

// RecordStatus represents the lifecycle state of an idempotent request.
type RecordStatus string

const (
	StatusInProgress RecordStatus = "in_progress"
	StatusCompleted  RecordStatus = "completed"
)

// Record holds the state, metadata, and cached response for an idempotent operation.
type Record struct {
	Key          string            `json:"key"`
	OrgID        string            `json:"org_id"`
	RequestHash  string            `json:"request_hash"`
	Status       RecordStatus      `json:"status"`
	StatusCode   int               `json:"status_code"`
	Headers      map[string]string `json:"headers"`
	ResponseBody []byte            `json:"response_body"`
	CreatedAt    time.Time         `json:"created_at"`
	ExpiresAt    time.Time         `json:"expires_at"`
}

// Store defines the repository interface for idempotency records.
type Store interface {
	// LockOrGet attempts to atomically acquire the idempotency key.
	// If the record does not exist or has expired, a new in-progress record is created and isNew is true.
	// If an unexpired record already exists, it is returned and isNew is false.
	LockOrGet(ctx context.Context, orgID, key, requestHash string, ttl time.Duration) (*Record, bool, error)

	// Complete transitions an in-progress record to completed status with the HTTP status, headers, and body.
	Complete(ctx context.Context, orgID, key string, statusCode int, headers map[string]string, body []byte) error

	// Release removes or resets an in-progress record when an unrecoverable failure or 5xx occurs, allowing safe retries.
	Release(ctx context.Context, orgID, key string) error
}
