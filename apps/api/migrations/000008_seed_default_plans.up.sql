INSERT INTO plans (code, name, monthly_quota, rate_limit_per_minute)
VALUES
    ('free', 'Free Tier', 100, 10),
    ('starter', 'Starter Tier', 1000, 30),
    ('pro', 'Pro Tier', 10000, 100)
ON CONFLICT (code) DO UPDATE SET
    name = EXCLUDED.name,
    monthly_quota = EXCLUDED.monthly_quota,
    rate_limit_per_minute = EXCLUDED.rate_limit_per_minute;
