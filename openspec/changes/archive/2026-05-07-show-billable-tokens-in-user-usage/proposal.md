## Why

C 端使用记录 tooltip 当前同时展示真实用量和“账单校验用量”，对普通用户来说会形成两套 token 数的认知负担。既然 C 端账单金额按 billable usage 核验，使用记录中的 token 明细应直接展示计费用量，让用户看到的 token 数与账单金额口径一致。

## What Changes

- 调整 C 端 Usage 页面 token tooltip：主 token 明细直接使用 `billable_*` 计费用量字段。
- 移除 tooltip 中独立的“账单校验用量”分区，避免 raw 与 billable 双口径并列展示。
- tooltip 总 Token 改为计费用量总和；当 billable 字段缺失或为历史兼容值时，继续回退到 raw-compatible 口径。
- 表格中 token 列和页面汇总卡保持现有展示语义，除非实现中发现它们也需要与 tooltip 一起改为 billable 口径。
- CSV/API/管理员侧仍保留 raw 与 billable 两套字段，用于审计、对账和排查。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `billable-usage-tracks`: C 端 Usage 页面 token tooltip 的展示口径从 raw 明细 + 独立 billable 明细，调整为直接展示 billable 明细。
- `user-facing-rate-visibility`: C 端仍不展示内部倍率，也不展示 raw/billable 双口径解释；普通用户看到的 token 明细应是计费用量口径。

## Impact

- 前端：`frontend/src/views/user/UsageView.vue` 的 token tooltip 计算和模板。
- 前端文案：可能清理不再使用的 `billableTokenDetails` 等 tooltip 文案。
- 测试：更新 UsageView 相关单元测试或快照，增加 billable tooltip 展示断言。
- 后端/API：不需要变更；继续返回 raw 与 billable 字段。
