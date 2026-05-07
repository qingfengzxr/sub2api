## Context

Usage API 已经返回两套 usage：raw token 字段表示上游真实用量，`billable_*` 字段表示平台计费用量，`billing_token_multiplier` 表示本条日志使用的 token 放大倍率。当前展示层的职责分配需要更清晰：

- 普通用户只需要看到能解释最终扣费的 token 结果，避免 raw/billable 两套口径并列。
- 管理员需要看到完整审计链路，能核对 raw token、billable token、倍率和费用之间的关系。

## Goals / Non-Goals

Goals:

- C 端 token tooltip 精简为单一最终计费用量口径。
- 管理后台 token tooltip 展示 raw 与 billable 明细，并包含 token 放大倍率快照。
- 保持现有 usage API、CSV/API 导出和扣费逻辑不变。

Non-Goals:

- 不改变实际扣费、`total_cost` 或 `actual_cost` 计算。
- 不新增后端字段。
- 不要求 C 端展示内部倍率、标准/实际费用双口径或 raw usage 明细。

## Decisions

### C 端 tooltip 使用 billable-only 展示模型

C 端 Usage 页面 token 感叹号 tooltip 的输入、输出、cache、image output 和总量都从 `billable_*` 字段取值。为了兼容历史日志或异常响应，前端 helper 仍采用 billable-first fallback：当对应 billable 字段缺失、非正或不可用时回退到 raw 字段。

模板不再渲染 raw 明细区、独立“账单校验用量”分区、`billing_token_multiplier` 或等价倍率说明。标题可以保留“Token 明细”，但内容必须是最终计费用量口径。

### 管理后台 tooltip 使用审计展示模型

管理后台 Usage 表格 token tooltip 显示两组信息：

- 真实用量：raw input/output/cache/image output tokens。
- 计费用量：billable input/output/cache/image output tokens 和计费 token 总量。

当 `billing_token_multiplier` 大于 0 且不等于 1 时，管理员 tooltip 显示倍率快照；当倍率为 1 时可显示 `1x` 或省略，但不得影响审计字段显示。历史数据缺少 billable 字段时使用 raw-compatible 回退，避免 tooltip 空白。

### 费用 tooltip 保持管理员审计信息

管理员费用 tooltip 继续展示 `total_cost`、`actual_cost`、用户侧 `rate_multiplier`、账号侧 `account_rate_multiplier` 和账号计费结果。token tooltip 负责解释 raw/billable token，费用 tooltip 负责解释金额侧倍率和费用归属。

## Risks

- 管理员 tooltip 信息过多：用分区和短标签区分“真实用量”和“计费用量”，避免挤在同一个列表。
- 历史日志 billable 字段为 0：统一 helper 使用 billable-first fallback。
- C 端测试可能仍期待“账单校验用量”：更新断言，确保用户侧不出现 raw/billable 双口径分区。
