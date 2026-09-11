CREATE TABLE IF NOT EXISTS ocr_requests (
    id VARCHAR(64) PRIMARY KEY,
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    api_key_id UUID REFERENCES api_keys(id) ON DELETE SET NULL,
    status VARCHAR(50) NOT NULL,
    confidence NUMERIC(5, 4),
    latency_ms INTEGER NOT NULL DEFAULT 0,
    doc_type VARCHAR(50) NOT NULL DEFAULT 'ktp',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ocr_requests_org_id ON ocr_requests (org_id);
CREATE INDEX IF NOT EXISTS idx_ocr_requests_api_key_id ON ocr_requests (api_key_id);
CREATE INDEX IF NOT EXISTS idx_ocr_requests_created_at ON ocr_requests (created_at);
