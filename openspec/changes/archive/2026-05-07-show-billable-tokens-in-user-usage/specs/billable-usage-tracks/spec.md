## MODIFIED Requirements

### Requirement: 报表和 API 同时暴露两套 Usage
系统 SHALL 在 usage API 和前端数据模型中暴露 raw usage、billable usage、`billing_token_multiplier`、`total_cost` 和 `actual_cost`，同时不得破坏现有 dashboard、ranking 和 export。

#### Scenario: usage log API 返回两套 usage
- **WHEN** 用户或管理员查询 usage logs
- **THEN** 每条日志必须包含现有 raw token 字段、billable token 字段、`billing_token_multiplier`、`total_cost` 和 `actual_cost`

#### Scenario: dashboard token totals 保持 raw
- **WHEN** dashboard、ranking 和 usage summaries 聚合 token 数量
- **THEN** 默认 token 数量指标必须聚合 raw usage 字段
- **THEN** cost 指标必须聚合持久化 cost 字段

#### Scenario: export 包含足够字段用于账单校验
- **WHEN** 用户导出 usage logs
- **THEN** 导出内容必须包含 raw usage 字段、billable usage 字段、`billing_token_multiplier`、选定模型/定价上下文、`total_cost` 和 `actual_cost`

#### Scenario: C 端 Usage tooltip 展示计费用量
- **WHEN** 普通用户在 Usage 页面查看单条记录的 token tooltip
- **THEN** tooltip 的 input、output、cache 和 image output token 明细必须直接展示 billable usage 口径
- **THEN** tooltip 的总 Token 必须使用同一 billable usage 口径计算
- **THEN** tooltip 不得将 raw usage 与 billable usage 作为两个并列分区展示

#### Scenario: C 端页面保持费用口径清晰
- **WHEN** 普通用户查看 Usage、Dashboard、Keys、KeyUsage、Payment、Subscriptions、Orders 或 Available Channels 页面
- **THEN** 页面必须继续展示最终消费或用户账单可核验的 token 用量这类用户可理解信息
- **THEN** 页面不得展示内部 group/account 倍率 badge，也不得恢复旧的标准/实际双口径费用解释
