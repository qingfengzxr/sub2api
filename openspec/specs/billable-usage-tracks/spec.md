# billable-usage-tracks Specification

## Purpose
定义真实 usage 与计费 usage（billable usage）的双轨机制，确保上游真实用量不被平台计费策略污染，同时让账单金额可以用官方/渠道单价和持久化 billable tokens 进行校验。

## Requirements
### Requirement: 真实 Usage 保持上游原始值
系统 MUST 保持现有 usage token 字段表示上游返回的真实用量，且 MUST NOT 为平台计费而放大、覆盖或污染这些字段。

#### Scenario: 保存纯文本真实用量
- **WHEN** 上游响应返回 input、output、cache creation 和 cache read token 数量
- **THEN** `input_tokens`、`output_tokens`、`cache_creation_tokens` 和 `cache_read_tokens` 必须原样保存这些真实数量
- **THEN** 这些真实字段继续作为对账、限流、TPM、性能分析和供应商审计的数据来源

#### Scenario: 保存图片输出真实用量
- **WHEN** 支持图片的上游路径返回 image output tokens
- **THEN** `image_output_tokens` 必须原样保存上游返回的图片输出 token 数量
- **THEN** 平台计费逻辑不得修改 `image_output_tokens`

### Requirement: Billable Usage 独立持久化
系统 MUST 为所有支持的计费维度持久化独立 billable usage 字段，并 MUST 记录本次账单使用的 token 放大倍率。

#### Scenario: 保存文本 billable usage 字段
- **WHEN** 系统记录一条按 token 计费的文本请求
- **THEN** usage log 必须包含 `billable_input_tokens`、`billable_output_tokens`、`billable_cache_creation_tokens`、`billable_cache_read_tokens`、`billable_image_output_tokens` 和 `billing_token_multiplier`

#### Scenario: 保存 OpenAI 图片兼容 billable usage 字段
- **WHEN** 系统记录一条 OpenAI 图片能力相关请求
- **THEN** usage log 能够保存 `billable_text_input_tokens`、`billable_cached_text_input_tokens`、`billable_image_input_tokens` 和 `billable_cached_image_input_tokens`
- **THEN** 如果某些上游路径暂时没有更细粒度 usage，系统必须在不污染真实 usage、不阻断请求的前提下兼容保存

#### Scenario: 读取历史 usage log
- **WHEN** 历史 usage log 没有持久化 billable usage 字段
- **THEN** 读取方必须将 billable usage 按 raw-compatible 语义处理，并将 `billing_token_multiplier` 视为 `1`

### Requirement: Billable Usage Builder
系统 SHALL 通过单一服务组件构建 billable usage，该组件输入真实 usage、计费策略和倍率，输出用于平台计费的 billable usage。

#### Scenario: 启用模式下构建 billable usage
- **WHEN** `billing.token_multiplier_billing_enabled` 已启用，且平台级 `billing.billing_token_multiplier` 为 `2.5`
- **THEN** billable token 字段必须由 raw token 字段乘以 `2.5` 并按统一确定性规则取整得到
- **THEN** raw token 字段必须保持不变

#### Scenario: 旧模式下构建 billable usage
- **WHEN** `billing.token_multiplier_billing_enabled` 未启用
- **THEN** billable token 字段必须保持 raw-compatible，且 `billing_token_multiplier` 为 `1`
- **THEN** 现有金额侧 rate multiplier 计费逻辑必须继续生效

### Requirement: 平台级 Billable Token 配置
系统 MUST 在系统设置的“功能开关”中提供平台级 billable token 配置，并 MUST 将该配置作为 `billing_token_multiplier` 的唯一来源。

#### Scenario: 功能开关控制计费用量放大
- **WHEN** `billing.token_multiplier_billing_enabled` 为 `true`
- **THEN** 系统必须使用平台级 `billing.billing_token_multiplier` 放大 billable tokens
- **WHEN** `billing.token_multiplier_billing_enabled` 为 `false`
- **THEN** 系统必须将 `billing_token_multiplier` 视为 `1`，并保持现有金额侧 rate multiplier 计费逻辑

#### Scenario: 管理员配置平台级倍率
- **WHEN** 管理员进入系统设置的“功能开关”tab
- **THEN** 页面必须提供 billable token 计费开关和平台级 token 放大倍率输入项
- **THEN** 保存后配置对后续新 usage 生效

#### Scenario: group/user 倍率不参与 billable token 生成
- **WHEN** token multiplier billing 已启用，且用户存在有效 group `rate_multiplier` 或用户专属 group rate override
- **THEN** builder 不得使用这些金额侧倍率生成 billable tokens
- **THEN** 这些金额侧倍率必须继续用于 `actual_cost` 计算

### Requirement: 计费基于 Billable Usage 且允许金额倍率叠加
系统 MUST 在 token 计费模式下基于官方单价和 billable usage 计算 `total_cost`，并 MUST 允许现有金额侧 `rate_multiplier` 在 `total_cost` 基础上继续计算最终 `actual_cost`。

#### Scenario: 用户按 billable tokens 反推 total_cost
- **WHEN** 官方 input 单价为 `0.000001`，官方 output 单价为 `0.000002`，raw usage 为 `100` input tokens 和 `50` output tokens，且平台级 billing token multiplier 为 `2.5`
- **THEN** 持久化 billable usage 必须为 `250` input tokens 和 `125` output tokens
- **THEN** `total_cost` 必须等于 `250 * 0.000001 + 125 * 0.000002`

#### Scenario: 金额侧倍率继续叠加
- **WHEN** token multiplier billing 已启用，平台级 billing token multiplier 为 `2.5`，且有效金额侧 `rate_multiplier` 为 `1.2`
- **THEN** `total_cost` 必须基于 billable tokens 计算
- **THEN** `actual_cost` 必须等于 `total_cost * 1.2`
- **THEN** 订阅用量、余额扣费和 API Key usage-rate 统计必须使用同一个最终 `actual_cost`

#### Scenario: 保持旧 rate multiplier 行为
- **WHEN** token multiplier billing 未启用，且有效 rate multiplier 为 `2.5`
- **THEN** 计费必须使用 raw usage 计算 `total_cost`
- **THEN** `actual_cost` 必须继续沿用现有金额侧倍率逻辑

### Requirement: 现有定价扩展能力保持可用
系统 MUST 在引入 billable usage 后继续支持渠道定价、group pricing、account stats pricing、image pricing 和 billing model source。

#### Scenario: 渠道定价应用到 billable usage
- **WHEN** 某个 group 对选定 billing model 配置了渠道模型定价或区间定价
- **THEN** 在 token multiplier billing 模式下，系统必须使用选中的官方或渠道单价乘以 billable usage
- **THEN** billing model source 必须继续按照现有渠道配置选择 requested、upstream 或 channel-mapped

#### Scenario: cached token 定价应用到 billable cached usage
- **WHEN** 请求包含 cache creation 或 cache read tokens
- **THEN** 在 token multiplier billing 模式下，cache 单价必须分别乘以 `billable_cache_creation_tokens` 和 `billable_cache_read_tokens`
- **THEN** raw cache token 字段继续保存上游真实数量

#### Scenario: 图片输出 token 定价应用到 billable image output usage
- **WHEN** 图片模型按 image output token 单价计费
- **THEN** 在 token multiplier billing 模式下，image output 单价必须乘以 `billable_image_output_tokens`
- **THEN** 非 token 型图片计费或按次计费模式必须保持现有语义，除非明确使用某个 billable token 维度

#### Scenario: account stats 保持清晰语义
- **WHEN** 系统计量 account stats
- **THEN** token totals 必须继续表示 raw usage totals
- **THEN** `user_cost` 必须使用 `actual_cost`
- **THEN** `standard_cost` 必须使用 `total_cost`
- **THEN** 自定义 account stats pricing 必须默认使用 raw usage，除非显式配置为使用 billable usage

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
