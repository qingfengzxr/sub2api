## Why

普通用户在 C 端 Usage 使用记录查看费用 tooltip 时，当前输入/输出单价会把平台 token 放大、group/user 金额倍率或其他内部计费倍率折算进去，导致用户看到的是放大后的单价而不是模型基础标准单价。这里容易让用户误解官方/标准模型价格，也暴露了内部计费策略的效果。

## What Changes

- C 端普通用户 Usage 使用记录的费用 tooltip 中，输入单价、输出单价、缓存单价等 unit price 展示改为基础标准单价。
- 基础标准单价不得把 `billing_token_multiplier`、group/user `rate_multiplier`、图片倍率、service tier、长上下文加价或其他内部放大倍率折算进展示值。
- C 端最终费用、余额扣费、订阅用量、billable token 主展示和 CSV 现有口径保持不变。
- 管理员后台 Usage 明细继续允许展示真实成本、最终费用、内部倍率和审计口径，不受本变更限制。

## Capabilities

### New Capabilities

<!-- None. This change tightens an existing user-facing visibility requirement. -->

### Modified Capabilities

- `user-facing-rate-visibility`: 普通用户 Usage 费用明细中的输入/输出等单价展示必须使用基础标准单价，不得展示倍率放大后的派生单价。

## Impact

- Affected frontend: `frontend/src/views/user/UsageView.vue` and related usage tooltip helpers/tests.
- Potential affected API/data mapping: only if the current frontend cannot derive standard unit prices from returned cost/tokens without leaking multipliers; any added field must be user-safe and standard-price-only.
- No billing behavior changes: backend cost calculation, balance deduction, subscription quota, account stats, administrator usage audit, and raw/billable token persistence remain unchanged.
