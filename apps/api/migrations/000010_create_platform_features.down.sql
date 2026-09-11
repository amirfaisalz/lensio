DROP INDEX IF EXISTS idx_usage_records_org_endpoint;
DROP INDEX IF EXISTS idx_usage_records_org_timestamp;
DROP INDEX IF EXISTS idx_usage_records_request_id;
ALTER TABLE usage_records DROP COLUMN IF EXISTS request_id;
ALTER TABLE organizations DROP COLUMN IF EXISTS plan_id;
