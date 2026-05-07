## MODIFIED Requirements

### Requirement: C 端前端不展示倍率文案
系统 MUST 在 C 端普通用户前端隐藏所有直接倍率展示，包括 `x倍`、`倍率`、`rate multiplier`、标准/实际双口径消费、原始/计费双口径消费、raw/billable 双口径 token tooltip 和等价内部计费系数文案。系统 MAY 在导出中返回可用于账单校验的 raw token、billable token 和 `billing_token_multiplier` 字段。系统 SHALL 在普通用户 Usage 页面 token tooltip 中直接展示最终 billable token 明细，而不是将 raw usage 与 billable usage 渲染成两个并列口径。管理员后台不受该 C 端隐藏规则限制。

#### Scenario: 用户查看 API Key 分组
- **WHEN** 普通用户在 API Key 页面查看或切换分组
- **THEN** 前端展示分组名称、平台和用户可理解的状态，不展示分组倍率或用户专属倍率

#### Scenario: 用户查看可用渠道
- **WHEN** 普通用户查看可用渠道和可绑定分组
- **THEN** 前端展示可用性、平台、分组名称和说明，不展示倍率 badge 或 `x倍` 文案

#### Scenario: 用户查看用量或支付页面
- **WHEN** 普通用户查看用量、支付套餐、订阅计划或当前订阅权益
- **THEN** 前端不展示计费倍率字段或倍率文案

#### Scenario: 用户查看使用记录总消费
- **WHEN** 普通用户查看使用记录页总消费卡片
- **THEN** 前端只展示最终总消费金额，不展示“实际”“标准”、删除线标准金额或等价双口径消费说明

#### Scenario: 用户查看单条 token 明细
- **WHEN** 普通用户在使用记录表格查看单条 token tooltip
- **THEN** 前端必须直接展示 billable token 明细和 billable token 总量
- **THEN** 前端不得同时展示 raw token 明细和独立的 billable token 明细区块
- **THEN** 前端不得展示“账单校验用量”这类让 raw 与 billable 并列理解的分区
- **THEN** 前端不得把 `billing_token_multiplier` 渲染为 group/account 内部倍率 badge、token 放大倍率或内部运营策略说明

#### Scenario: 用户查看单条费用明细
- **WHEN** 普通用户在使用记录表格查看单条费用 tooltip 或成本明细
- **THEN** 前端不展示“原始”“计费”或等价标准/最终双口径费用字段，只展示用户需要理解的最终消费结果和非倍率用量信息

#### Scenario: 用户导出账单校验字段
- **WHEN** 普通用户导出使用记录或通过 API 获取用量明细
- **THEN** 系统可以返回 raw token、billable token、`billing_token_multiplier` 和最终费用字段用于账单校验
- **THEN** 普通用户前端不得把 `billing_token_multiplier` 渲染为 group/account 内部倍率 badge 或内部运营策略说明

#### Scenario: 管理员查看使用记录明细
- **WHEN** 管理员在管理后台 Usage 表格查看 token 或费用 tooltip
- **THEN** 管理后台可以展示 raw usage、billable usage、`billing_token_multiplier`、金额侧倍率和费用明细
- **THEN** 这些管理员审计信息不得被 C 端隐藏规则移除
