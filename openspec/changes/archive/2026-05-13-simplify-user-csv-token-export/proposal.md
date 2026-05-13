## Why

普通用户侧 Usage CSV 目前同时导出 raw tokens、billable tokens、`Billing Token Multiplier` 和其他 `Billing...` 内部列，会暴露内部计费用量倍率、计费模式和标准/最终双口径字段。用户侧导出应与页面体验一致：只展示可用于理解和复算最终费用的用户口径字段，不暴露 raw/billable/billing/multiplier 等内部机制。

## What Changes

- 调整普通用户 Usage CSV 导出：保留 `Input Tokens`、`Output Tokens`、`Cache Read Tokens`、`Cache Creation Tokens` 等用户可理解列名，但列值改为页面同口径的 billable-first token 值。
- 移除普通用户 CSV 中的 `Billable Input Tokens`、`Billable Output Tokens`、`Billable Cache Read Tokens`、`Billable Cache Creation Tokens`、`Billable Image Output Tokens` 等 `Billable...` 列。
- 移除普通用户 CSV 中的 `Billing Mode`、`Billing Token Multiplier`、`Billing Base Cost` 等 `Billing...` 内部列。
- 保留 `Final Cost` 和必要的请求元数据，确保用户可用导出的 token 口径和费率复算最终消费。
- 管理员 Usage CSV/审计视图不受影响，继续可查看 raw usage、billable usage 和 multiplier。

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `user-facing-rate-visibility`: 普通用户侧 Usage CSV 不再导出 raw/billable 双口径、`Billable...` 列或 `Billing...` 内部列，改为导出与页面 TOKEN 列一致的 billable-first token 口径。

## Impact

- Affected frontend: `frontend/src/views/user/UsageView.vue` CSV 导出 headers 与 row mapping。
- Affected tests: `frontend/src/views/user/__tests__/UsageView.spec.ts` 中 CSV 导出断言。
- No backend API/database changes; existing usage response fields remain available for frontend rendering and admin audit.
- No administrator usage export behavior changes.
