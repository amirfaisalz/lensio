-- Cluster-wide rate limiting.
--
-- Token buckets lived in process memory, so with N replicas a tenant could burst
-- N times its plan limit. RATE_LIMIT_REPLICAS divided the limit as a stopgap, but
-- that mis-counts the moment autoscaling moves. A shared counter is exact.
--
-- A fixed window (one row per key per minute) is used rather than a distributed
-- token bucket because it is a single atomic statement: INSERT .. ON CONFLICT DO
-- UPDATE .. RETURNING both increments and reads the count with no transaction and
-- no SELECT FOR UPDATE on the request hot path.
CREATE TABLE IF NOT EXISTS rate_limit_counters (
    bucket_key   TEXT        NOT NULL,
    window_start TIMESTAMPTZ NOT NULL,
    hits         INTEGER     NOT NULL DEFAULT 0,
    PRIMARY KEY (bucket_key, window_start)
);

-- Supports the cleaner's sweep of elapsed windows.
CREATE INDEX IF NOT EXISTS idx_rate_limit_counters_window_start
    ON rate_limit_counters (window_start);
