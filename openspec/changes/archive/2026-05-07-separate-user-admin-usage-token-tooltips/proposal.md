## Why

C 端 Usage 表格的 token tooltip 已经收敛到最终 billable 口径，需要用本变更防止回退到 raw/billable 双口径；管理员后台 Usage 表格仍缺少 billable token 明细，导致管理员无法一眼核对 raw → billable → cost。

## What Changes

- 保持并测试 C 端用户 Usage 页面 token 感叹号 tooltip：只展示最终计费 token 明细和总量，口径包含 token 放大后的 billable usage。
- 防止 C 端 tooltip 回退到 raw usage 与 billable usage 并列展示、`billing_token_multiplier`、内部倍率解释和“账单校验用量”分区。
- 调整管理后台 Usage 表格 token 感叹号 tooltip：展示 raw usage、billable usage、`billing_token_multiplier` 和可核验的 token 总量明细。
- 保持管理员费用 tooltip 中用户扣费、账号成本、倍率等审计信息可见；必要时让 token tooltip 和费用 tooltip 的口径互相对得上。
- API 不需要新增字段；继续使用现有 raw usage、billable usage、`billing_token_multiplier`、`total_cost` 和 `actual_cost`。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `billable-usage-tracks`: 明确 C 端 usage token tooltip 只展示 billable-first 计费用量，管理员 usage token tooltip 必须展示 raw/billable 双轨明细用于审计。
- `user-facing-rate-visibility`: 强化普通用户前端不得通过 tooltip 暴露 raw/billable 双口径、账单校验分区或 token 放大倍率说明，同时保留管理员可见性。

## Impact

- 前端：`frontend/src/views/user/UsageView.vue` 的 token tooltip 模板与 helper。
- 前端：`frontend/src/components/admin/usage/UsageTable.vue` 的 token tooltip 模板、helper 和可能的 i18n 文案。
- 测试：用户 Usage tooltip 断言最终 billable 口径；管理员 UsageTable tooltip 断言 raw/billable 双轨明细。
- 后端/API：不需要变更。
