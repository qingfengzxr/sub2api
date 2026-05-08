## Why

C 端 Usage 表格的 token tooltip 已经按计费 token 展示明细，但同一行的 `TOKEN` 列主显示值仍使用真实 token，导致用户看到的列表数值与账单校验口径不一致。需要让 C 端用户在列表中直接看到与计费、tooltip 和费用核验一致的 billable token 值。

## What Changes

- 调整 C 端 Usage 表格 `TOKEN` 列输入、输出、缓存/总量等可见 token 值，优先使用已持久化的 billable token 字段。
- 保持 token tooltip 继续展示 C 端可理解的 billable token 明细，且列表值与 tooltip 计费口径一致。
- 保持管理员后台审计视图不变，仍可查看 raw usage、billable usage、倍率快照和费用明细。
- 保持后端计费、扣费、导出字段和历史数据兼容逻辑不变。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `user-facing-rate-visibility`: C 端 Usage 表格 `TOKEN` 列的主显示 token 值必须使用最终 billable token 口径，而不是 raw token 口径。
- `billable-usage-tracks`: C 端页面展示的用户可核验 token 用量应覆盖 Usage 表格主列展示，确保列表值与 billable usage 一致。

## Impact

- 影响的 UI：C 端 Usage/使用记录表格的 `TOKEN` 列渲染和相关格式化 helper。
- 影响的数据契约：复用现有 usage log 响应中的 billable token 字段；缺失 billable 字段的历史记录按 raw-compatible 语义回退。
- 不涉及数据库、定价、API、依赖或管理员行为变更。
