## Context

C 端 Usage 页面当前在费用 tooltip 中展示输入/输出单价时，从 usage log 的 `input_cost` / `input_tokens`、`output_cost` / `output_tokens` 反推每 1M token 单价。`input_cost` 和 `output_cost` 是实际成本拆分，可能已经包含 service tier、长上下文加价、billable token 放大后的成本分布、渠道定价或金额侧倍率效果。普通用户看到这些反推单价时，会把内部计费策略误认为模型基础标准价。

现有 `user-facing-rate-visibility` 规范已经要求 C 端隐藏倍率、隐藏 raw/billable 双口径和标准/实际费用双口径。本变更把费用 tooltip 里的 unit price 展示纳入同一条用户可见性规则：最终消费金额仍展示最终消费，但单价只展示基础标准单价。

## Goals / Non-Goals

**Goals:**

- 普通用户 Usage 费用 tooltip 中输入/输出/cache 等单价展示使用基础标准单价。
- 单价展示不得受 `billing_token_multiplier`、group/user 金额倍率、service tier、长上下文 multiplier 或最终成本拆分影响。
- 现有最终费用、token 主展示、CSV 的用户口径和管理员审计信息保持不变。
- 测试覆盖一个会放大反推单价的样例，确保 C 端仍显示标准基础单价。

**Non-Goals:**

- 不改变计费、扣费、订阅用量、account stats 或 usage log 持久化。
- 不移除管理员后台的真实成本、倍率或审计口径展示。
- 不重新定义官方/渠道价格来源，也不在 C 端暴露内部倍率。

## Decisions

1. **用户端单价展示不再从成本字段反推。**

   The user Usage tooltip should not calculate unit price from `input_cost / input_tokens` or equivalent final-cost fields. Those fields are for billing/audit amounts, not user-facing base price display.

   Alternative considered: divide by billable tokens instead of raw tokens. That still leaks service-tier and long-context adjusted prices, and can still show non-standard rates.

2. **优先使用标准单价字段或标准定价解析结果。**

   Implementation should provide or derive a user-safe standard unit price for the billing model: input, output, cache read, cache write, and image unit prices when available. "Standard" means the base standard tier for the selected billing model or channel pricing context before user/account/platform multipliers and before service-tier/long-context adjustment.

   Alternative considered: hard-code OpenAI prices in the frontend. That would drift quickly and would not cover non-OpenAI or channel-specific pricing.

3. **If backend support is needed, expose only safe display fields.**

   If the current user usage DTO lacks enough information to display standard prices correctly, add explicit optional display fields such as `standard_input_price_per_million`, `standard_output_price_per_million`, `standard_cache_read_price_per_million`, `standard_cache_write_price_per_million`, and image/request unit price equivalents. These fields must be standard-price-only and must not expose multipliers.

   Alternative considered: expose `rate_multiplier`, `billing_token_multiplier`, or raw pricing internals to let the frontend reverse the math. That conflicts with the existing C 端 hiding requirements.

4. **Keep admin behavior separate.**

   Admin Usage tooltip can continue showing actual pricing, cost breakdowns, raw/billable usage, and multipliers. This change is scoped to ordinary user Usage displays.

## Risks / Trade-offs

- **Risk: Missing standard price for unknown or custom models** -> Show `-` for unavailable unit prices while still showing final cost; do not fall back to final-cost-derived unit prices.
- **Risk: Channel pricing may be the intended user-facing base price for a group** -> Treat channel model pricing as the base standard price when it is the selected pricing context, but still exclude user/account/platform multipliers and service-tier/long-context adjustments.
- **Risk: Frontend-only derivation may be incomplete** -> Prefer a backend helper/DTO field if existing data cannot distinguish standard prices from adjusted cost components.
- **Risk: Historical records lack newly added display fields** -> Preserve page rendering and display `-` for unit price rather than reconstructing a misleading adjusted price.
