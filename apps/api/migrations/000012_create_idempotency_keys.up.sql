CREATE TABLE IF NOT EXISTS idempotency_keys (
    key VARCHAR(255) NOT NULL,
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    request_hash VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL, -- 'in_progress', 'completed'
    status_code INT,
    headers JSONB,
    response_body BYTEA,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (org_id, key)
);

CREATE INDEX IF NOT EXISTS idx_idempotency_keys_expires_at ON idempotency_keys (expires_at);
