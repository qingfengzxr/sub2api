## 背景

当前计费系统主要在金额侧应用平台加价倍率，而持久化的 token 字段又同时承担对账、限流、性能分析、审计、dashboard、排行和导出的数据来源。这样会导致用户很难用“官方单价 × token 数”反推出平台收费金额；如果直接污染原始 token，又会破坏上游对账和统计语义。

本变更引入“真实 usage”和“计费 usage（billable usage）”双轨机制：真实 usage 保持上游原始值，billable usage 专门用于平台计费和账单校验。

## 变更内容

- 新增双轨 usage 模型：现有 usage token 字段继续表示上游真实用量，新增 `billable_*` token 字段表示平台计费用量。
- 在 usage log 中新增 `billing_token_multiplier` 快照，记录本次账单使用的平台级 token 放大倍率。
- 在系统设置的“功能开关”中新增平台级 billable token 配置，至少包含启用开关和倍率值，例如 `billing.token_multiplier_billing_enabled` 与 `billing.billing_token_multiplier`。
- 调整 token 模式计费：`total_cost` 使用官方单价乘以 billable usage；`actual_cost` 仍可继续应用现有 group/user `rate_multiplier`，即允许 billable token 放大后再叠加金额侧倍率。
- 当 token multiplier billing 关闭时，保持现有金额侧 rate multiplier 计费行为。
- API 和前端 usage 响应同时返回 raw usage、billable usage、`billing_token_multiplier` 和最终费用，同时保持 usage log、dashboard、ranking、export、渠道定价、group pricing、account stats pricing、image pricing 和 billing model source 的现有扩展能力。
- 增加测试覆盖纯文本、cached tokens、OpenAI 路径、图片路径、平台级倍率配置、金额侧倍率叠加，以及用户按官方单价乘 billable tokens 反推 `total_cost`。

## 能力

### 新增能力
- `billable-usage-tracks`: 定义真实 usage 保留、billable usage 字段、token multiplier billing 语义、报表/API 暴露和兼容性要求。

### 修改能力
- `user-facing-rate-visibility`: 明确用户侧 API 可以返回用于账单校验的 billable token 审计字段，但普通用户界面仍不得展示内部 group/account 倍率或旧的标准/实际双口径费用解释。

## 影响范围

- 后端数据模型和迁移：`usage_logs`、`service.UsageLog`、usage log repository insert/select/scan 路径、DTO mapper、迁移和 schema 测试。
- 计费逻辑：usage token 结构、费用计算 helper、Anthropic/Gemini/OpenAI/OpenAI image 相关 record usage 路径、平台级 billable token 倍率配置、group/user 专属金额倍率解析、订阅和余额扣费命令。
- 定价集成：model pricing resolver、渠道定价、group pricing、account stats pricing、image billing、billing model source 选择和区间定价必须在不改变 raw usage 语义的前提下使用 billable usage。
- 报表和 API：用户/管理员 usage logs、usage stats、dashboard、export、ranking、account stats 和前端 TypeScript 类型/组件需要清晰区分 raw 与 billable 语义。
- 配置：在系统设置“功能开关”中新增向后兼容的平台级 billable token 配置，默认关闭且倍率默认为 `1`。
