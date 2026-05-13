## Context

当前 GPT-5.4/5.5 的长上下文加价策略在计费代码中内置：当 `input_tokens + cache_read_tokens` 超过模型定价中的 long-context threshold 时，输入和输出价格分别乘以模型定义的 long-context multiplier。该逻辑没有平台开关，管理员无法关闭或调整触发阈值；同时用户 CSV/页面排查账单时，运营侧也缺少可见配置来解释为什么部分请求触发更高费率。

系统设置已经以 key-value 方式承载功能开关，并在管理后台 Settings → Features 中集中呈现，例如 billable token multiplier。长上下文加价也应纳入同类平台级功能配置。

## Goals / Non-Goals

**Goals:**

- 在系统设置中新增平台级长上下文加价开关和上下文 token 阈值。
- 管理后台“功能开关”tab 可查看和修改该开关与阈值。
- 计费时根据平台配置决定是否应用长上下文加价；关闭时不加价，开启时用配置阈值判断触发。
- 保持现有模型/定价中定义的 long-context input/output multiplier 作为倍率来源。
- 保持渠道 interval pricing 优先，避免在区间定价之外重复叠加长上下文加价。

**Non-Goals:**

- 不在本次变更中提供输入/输出长上下文倍率的后台自定义。
- 不改变 raw usage、billable usage、rate multiplier 或用户余额/订阅扣费口径。
- 不为每个模型、渠道或用户组提供独立长上下文阈值。
- 不回填历史 usage log 的费用。

## Decisions

- 新增两个平台设置键：`long_context_pricing_enabled` 和 `long_context_pricing_threshold_tokens`。
  - Rationale: 与现有 feature settings 模式一致，最小化数据库和 API 变更。
  - Alternative considered: 将配置写入静态 config 或模型 pricing JSON。该方案不便于管理员运行时调整。

- 默认行为建议为“关闭”，阈值默认保留现有 `272000` 作为初始建议值。
  - Rationale: 用户明确希望把该规则变成可选开关，默认关闭可避免继续出现未显式启用的隐藏加价。
  - Alternative considered: 默认开启以保持历史行为。该方案兼容性更强，但不符合“可选打开”的产品意图；若上线需要完全兼容，可在部署前通过设置值开启。

- 计费服务通过 settings/service policy 获取当前长上下文策略，并将策略应用于 token 计费路径。
  - Rationale: 将配置读取与计费判断解耦，便于测试 enabled/disabled/threshold 三类场景。
  - Alternative considered: 直接在 `shouldApplySessionLongContextPricing` 内读取 settings。该方案会增加核心计费函数的外部依赖，不利于单元测试。

- 只覆盖 threshold，不覆盖 multiplier。
  - Rationale: 现有定价模型已经承载 per-model long-context multiplier；阈值是运营希望控制的触发条件。
  - Alternative considered: 后台同时配置 input/output multiplier。该方案需要更多 UI、校验和审计语义，超出本次需求。

## Risks / Trade-offs

- [Risk] 默认关闭可能改变 GPT-5.4/5.5 当前费用结果。→ Mitigation: 在 release notes 或部署步骤中提示如需保持旧行为，可启用开关并设置阈值 `272000`。
- [Risk] 设置读取失败时计费行为不明确。→ Mitigation: 后端应使用安全默认值：关闭长上下文加价，避免意外多扣费。
- [Risk] interval pricing 与长上下文策略重复加价。→ Mitigation: 保留现有规则，存在有效 intervals 时不额外应用长上下文加价。
- [Risk] 管理员配置无效阈值导致频繁触发或永不触发。→ Mitigation: 后端保存时校验阈值为正整数并设置合理范围；前端 number input 同步限制。
