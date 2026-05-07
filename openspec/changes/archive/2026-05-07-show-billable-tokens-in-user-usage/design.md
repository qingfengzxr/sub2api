## Context

`add-billable-usage-tracks` 已经让 usage API 同时返回 raw usage 和 billable usage。当前 C 端 Usage 页的 token tooltip 先展示 raw token 明细，再单独展示“账单校验用量”区块。这个形式对管理员和审计很清楚，但对普通用户来说会看到两套 token 数，容易误以为系统有两种可选计费口径。

本变更只调整 C 端展示。后端仍保留 raw usage、billable usage 和 `billing_token_multiplier`，CSV/API 仍可用于对账和审计。

## Goals / Non-Goals

**Goals:**
- C 端 Usage 页 token tooltip 的主明细直接展示 billable tokens。
- C 端不再在 tooltip 中并列展示 raw 明细与单独 billable 明细。
- tooltip 总 Token 与明细保持同一计费用量口径。
- 历史日志或关闭 token multiplier billing 时，billable 字段 raw-compatible，因此展示结果自然兼容旧行为。

**Non-Goals:**
- 不改后端计费公式、数据结构、migration 或 API 字段。
- 不删除 raw usage 字段，也不改变 dashboard/ranking/account stats 的 raw token 聚合语义。
- 不改变管理员使用 usage API 查看 raw/billable 双轨数据的能力。
- 不在 C 端暴露内部倍率策略说明。

## Decisions

### Tooltip 使用 billable-first 的展示模型

Usage 页 token tooltip 的每个展示维度都从 `billable_*` 字段取值。若某个 billable 字段缺失、为 `null` 或历史兼容场景下不可用，则回退到对应 raw 字段，保证旧数据和异常响应仍可显示。

替代方案是继续展示 raw 明细并保留“账单校验用量”分区。该方案虽然审计信息完整，但普通用户需要理解两套 token 口径，不符合 C 端账单体验目标。

### 保留导出/API 的双轨信息

CSV 导出和 API 响应继续包含 raw 与 billable 字段。导出是用户主动获取账单校验数据的场景，允许包含更多审计字段；页面 tooltip 是快速阅读场景，应保持单一口径。

替代方案是从导出移除 raw usage。该方案会削弱用户自助核验和客服排查能力，因此不采用。

### 不新增后端计算字段

前端可以基于已有 `billable_*` 字段计算 tooltip 明细和总计，不需要后端新增 `display_*_tokens` 字段。这样改动面小，也不会改变 API 契约。

替代方案是后端返回专门的 C 端展示字段。当前需求只涉及一个前端展示点，新增 API 字段会扩大契约面，暂不需要。

## Risks / Trade-offs

- 用户可能把 tooltip 中的 token 数当作上游真实用量 → tooltip 文案使用“Token 明细”或“计费 Token 明细”时要避免内部倍率解释，同时保持费用核验口径一致。
- 历史数据 billable 字段为 0 → 前端需要使用 billable-first fallback，避免显示空明细。
- 测试 fixture 缺少 billable 字段 → 更新 UsageView 测试数据，确保类型和 tooltip 断言稳定。
