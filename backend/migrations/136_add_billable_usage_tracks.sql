-- Add billable usage fields while preserving raw upstream usage columns.
ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS billable_input_tokens INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS billable_output_tokens INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS billable_cache_creation_tokens INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS billable_cache_read_tokens INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS billable_image_output_tokens INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS billable_text_input_tokens INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS billable_cached_text_input_tokens INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS billable_image_input_tokens INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS billable_cached_image_input_tokens INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS billing_token_multiplier NUMERIC(10,4) NOT NULL DEFAULT 1.0;

COMMENT ON COLUMN usage_logs.billable_input_tokens IS '平台计费用 input tokens；raw input_tokens 保持上游真实值';
COMMENT ON COLUMN usage_logs.billable_output_tokens IS '平台计费用 output tokens；raw output_tokens 保持上游真实值';
COMMENT ON COLUMN usage_logs.billable_cache_creation_tokens IS '平台计费用 cache creation tokens；raw cache_creation_tokens 保持上游真实值';
COMMENT ON COLUMN usage_logs.billable_cache_read_tokens IS '平台计费用 cache read tokens；raw cache_read_tokens 保持上游真实值';
COMMENT ON COLUMN usage_logs.billable_image_output_tokens IS '平台计费用 image output tokens；raw image_output_tokens 保持上游真实值';
COMMENT ON COLUMN usage_logs.billable_text_input_tokens IS 'OpenAI 图片链路文本输入计费用 tokens';
COMMENT ON COLUMN usage_logs.billable_cached_text_input_tokens IS 'OpenAI 图片链路 cached text input 计费用 tokens';
COMMENT ON COLUMN usage_logs.billable_image_input_tokens IS 'OpenAI 图片链路 image input 计费用 tokens';
COMMENT ON COLUMN usage_logs.billable_cached_image_input_tokens IS 'OpenAI 图片链路 cached image input 计费用 tokens';
COMMENT ON COLUMN usage_logs.billing_token_multiplier IS '本次账单使用的平台级 billable token 放大倍率';
