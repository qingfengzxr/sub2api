## Why

GPT-5.4/5.5 当前长上下文加价规则由代码内置阈值和倍率决定，管理员无法按业务策略关闭或调整阈值，导致用户账单排查和运营策略调整都需要改代码。需要将“上下文超过阈值后加价”抽象为系统功能开关配置，让管理员可选择启用并配置触发阈值。

## What Changes

- 在管理后台“功能开关”中新增长上下文加价配置：启用/关闭开关，以及上下文 token 阈值输入。
- 后端系统设置新增对应字段，提供默认值、读取、保存和校验。
- 计费逻辑从系统设置读取长上下文加价策略；关闭时不应用长上下文加价，开启时按配置阈值判断是否触发。
- 保留现有模型侧长上下文输入/输出倍率定义作为加价倍率来源，避免本次变更扩大到倍率自定义。
- 保持渠道区间定价优先级：已有区间定价自带上下文分层时，不额外叠加长上下文加价。

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `billable-usage-tracks`: token 计费必须支持平台级长上下文加价策略配置，并在启用时使用配置阈值决定是否触发长上下文加价。

## Impact

- Backend settings: add long-context pricing enabled/threshold settings and validation.
- Backend billing: update GPT-5.4/5.5 long-context pricing application to consult settings rather than hard-coded always-on behavior.
- Admin frontend: add controls in Settings → Features for long-context pricing switch and threshold.
- Tests: update billing/settings/admin settings tests for enabled, disabled, and threshold behavior.
- No database schema migration expected if settings remain key-value based.
