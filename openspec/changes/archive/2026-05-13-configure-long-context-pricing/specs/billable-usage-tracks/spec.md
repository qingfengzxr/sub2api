## ADDED Requirements

### Requirement: 平台级长上下文加价配置
系统 MUST 在系统设置的“功能开关”中提供平台级长上下文加价配置，并 MUST 使用该配置决定 token 计费是否应用长上下文加价。该配置 MUST 至少包含启用开关和上下文 token 阈值；上下文 token 数 MUST 按计费使用的 input tokens 与 cache read tokens 之和判断。

#### Scenario: 管理员配置长上下文加价
- **WHEN** 管理员进入系统设置的“功能开关”tab
- **THEN** 页面必须提供长上下文加价开关
- **THEN** 页面必须提供长上下文 token 阈值输入项
- **THEN** 保存后配置必须对后续新 usage 的计费生效

#### Scenario: 长上下文加价关闭
- **WHEN** 平台级长上下文加价开关为 `false`
- **THEN** token 计费不得因为上下文 token 数超过模型默认阈值而提高 input 或 output 单价
- **THEN** 其他计费逻辑，包括 billable usage、service tier、rate multiplier、渠道定价和最终扣费必须继续按现有规则生效

#### Scenario: 长上下文加价开启且超过阈值
- **WHEN** 平台级长上下文加价开关为 `true`
- **AND** 请求的计费 input tokens 与计费 cache read tokens 之和大于配置的长上下文阈值
- **AND** 该模型定价包含有效 long-context input/output multiplier
- **THEN** token 计费必须应用模型定价中的 long-context input/output multiplier
- **THEN** cache read token 单价不得因该长上下文加价规则被额外放大

#### Scenario: 长上下文加价开启但未超过阈值
- **WHEN** 平台级长上下文加价开关为 `true`
- **AND** 请求的计费 input tokens 与计费 cache read tokens 之和小于或等于配置的长上下文阈值
- **THEN** token 计费不得应用 long-context input/output multiplier

#### Scenario: 渠道区间定价优先
- **WHEN** 请求命中渠道 token interval pricing
- **THEN** 系统必须使用命中的区间价格
- **THEN** 系统不得在区间价格之外再次叠加平台级长上下文加价

#### Scenario: 配置缺失或无效
- **WHEN** 平台级长上下文加价配置缺失、读取失败或阈值无效
- **THEN** 系统必须按长上下文加价关闭处理，避免意外多扣费
