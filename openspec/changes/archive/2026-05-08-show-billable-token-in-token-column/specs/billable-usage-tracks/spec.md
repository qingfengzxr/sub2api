## MODIFIED Requirements

### Requirement: 用户可核验 Billable Usage 展示
系统 MUST 在用户可见的账单、用量和导出场景中提供能够核验最终消费的 billable usage 信息，同时 MUST 避免在 C 端普通用户前端制造 raw 与 billable 双口径混淆。

#### Scenario: C 端 Usage 表格使用 billable token 主显示
- **WHEN** 普通用户在 Usage 表格查看 `TOKEN` 列
- **THEN** `TOKEN` 列必须优先使用 `billable_input_tokens`、`billable_output_tokens`、`billable_cache_creation_tokens`、`billable_cache_read_tokens`、`billable_image_output_tokens` 和等价图片兼容 billable 字段计算主显示值
- **THEN** 对缺失 billable 字段的历史 usage log，系统必须按 raw-compatible 语义回退到对应 raw token 字段
- **THEN** `TOKEN` 列显示的输入、输出、缓存/总量等值必须与本条记录的费用核验口径一致

#### Scenario: 管理员 Usage 表格保留审计明细
- **WHEN** 管理员在管理后台 Usage 表格查看单条记录的 token tooltip
- **THEN** tooltip 必须展示 raw input、output、cache 和 image output token 明细
- **THEN** tooltip 必须展示 billable input、output、cache 和 image output token 明细
- **THEN** tooltip 必须展示 billable token 总量，且该总量必须与费用核验口径一致
- **THEN** tooltip 必须展示本条日志的 `billing_token_multiplier` 或以等价管理员审计标签展示该倍率快照

#### Scenario: C 端页面保持费用口径清晰
- **WHEN** 普通用户查看 Usage、Dashboard、Keys、KeyUsage、Payment、Subscriptions、Orders 或 Available Channels 页面
- **THEN** 页面必须继续展示最终消费或用户账单可核验的 token 用量这类用户可理解信息
- **THEN** Usage 表格 `TOKEN` 列必须直接展示 billable token 口径的用户可核验用量
- **THEN** 页面不得展示内部 group/account 倍率 badge，也不得恢复旧的标准/实际双口径费用解释
