-- Add plan_id to organizations referencing plans
ALTER TABLE organizations ADD COLUMN IF NOT EXISTS plan_id UUID REFERENCES plans(id);

-- Backfill default organization with the seeded 'free' plan
UPDATE organizations 
SET plan_id = (SELECT id FROM plans WHERE code = 'free' LIMIT 1)
WHERE plan_id IS NULL;

-- Add request_id to usage_records
ALTER TABLE usage_records ADD COLUMN IF NOT EXISTS request_id VARCHAR(100);

-- Indexes for fast analytics and quota queries
CREATE INDEX IF NOT EXISTS idx_usage_records_request_id ON usage_records (request_id);
CREATE INDEX IF NOT EXISTS idx_usage_records_org_timestamp ON usage_records (org_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_usage_records_org_endpoint ON usage_records (org_id, endpoint);
