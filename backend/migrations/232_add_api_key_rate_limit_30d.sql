ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS rate_limit_30d DECIMAL(20,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS usage_30d DECIMAL(20,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS window_30d_start TIMESTAMPTZ;

COMMENT ON COLUMN api_keys.rate_limit_30d IS 'API key spending limit in USD per fixed 30-day window; 0 means unlimited';
COMMENT ON COLUMN api_keys.usage_30d IS 'Used amount in USD for the current fixed 30-day window';
COMMENT ON COLUMN api_keys.window_30d_start IS 'Start time of the current fixed 30-day rate-limit window';
