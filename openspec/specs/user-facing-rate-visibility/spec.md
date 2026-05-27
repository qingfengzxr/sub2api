# user-facing-rate-visibility Specification

## Purpose
TBD - created by archiving change hide-user-rate-multipliers. Update Purpose after archive.
## Requirements
### Requirement: C 端前端不展示倍率文案
系统 MUST 在 C 端普通用户前端隐藏所有直接倍率展示，包括 `x倍`、`倍率`、`rate multiplier`、标准/实际双口径消费、原始/计费双口径消费、raw/billable 双口径 token tooltip 和等价内部计费系数文案。系统 SHALL 在普通用户 Usage 页面 token tooltip 中直接展示最终 billable token 明细，而不是将 raw usage 与 billable usage 渲染成两个并列口径。系统 SHALL 在普通用户 Usage 页面 `TOKEN` 表格列中直接展示最终 billable token 数值，并确保该列与 token tooltip 的计费口径一致。系统 SHALL 在普通用户 Usage CSV 导出中使用与页面一致的 billable-first token 口径，并且不得导出 `Billable...` 列、`Billing...` 列、`billing_token_multiplier` 或等价内部计费策略字段。管理员后台不受该 C 端隐藏规则限制。

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

#### Scenario: 用户查看使用记录 TOKEN 列
- **WHEN** 普通用户在使用记录表格查看 `TOKEN` 列
- **THEN** `TOKEN` 列的输入、输出、缓存/总量等主显示 token 值必须使用 billable token 口径
- **THEN** `TOKEN` 列显示值必须与同一记录 token tooltip 的 billable token 明细和总量一致
- **THEN** `TOKEN` 列不得使用 raw token 口径展示会与计费 token 不一致的主显示值

#### Scenario: 用户查看单条费用明细
- **WHEN** 普通用户在使用记录表格查看单条费用 tooltip 或成本明细
- **THEN** 前端不展示“原始”“计费”或等价标准/最终双口径费用字段，只展示用户需要理解的最终消费结果和非倍率用量信息

#### Scenario: 用户导出使用记录 CSV
- **WHEN** 普通用户从 Usage 页面导出使用记录 CSV
- **THEN** CSV 中的 token 列必须使用与 Usage 页面 `TOKEN` 列一致的 billable-first token 口径
- **THEN** CSV 不得包含 `Billable Input Tokens`、`Billable Output Tokens`、`Billable Cache Read Tokens`、`Billable Cache Creation Tokens`、`Billable Image Output Tokens` 或任何其他 `Billable...` 列
- **THEN** CSV 不得包含 `Billing Mode`、`Billing Token Multiplier`、`Billing Base Cost` 或任何其他 `Billing...` 内部列
- **THEN** CSV 可以包含 `Final Cost` 作为用户最终消费金额

#### Scenario: 普通用户通过 API 获取用量明细
- **WHEN** 普通用户通过 API 获取用量明细
- **THEN** 系统可以返回 raw token、billable token、`billing_token_multiplier` 和最终费用字段用于页面渲染和兼容
- **THEN** 普通用户前端不得把 `billing_token_multiplier` 渲染为 group/account 内部倍率 badge 或内部运营策略说明

#### Scenario: 管理员查看使用记录明细
- **WHEN** 管理员在管理后台 Usage 表格查看 token 或费用 tooltip
- **THEN** 管理后台可以展示 raw usage、billable usage、`billing_token_multiplier`、金额侧倍率和费用明细
- **THEN** 这些管理员审计信息不得被 C 端隐藏规则移除

### Requirement: `/api/v1/keys` 不暴露嵌套分组倍率
系统 MUST 在普通用户 `/api/v1/keys` 响应中隐藏 API Key 嵌套分组对象里的真实倍率字段。

#### Scenario: 用户获取 API Key 列表
- **WHEN** 普通用户请求 `/api/v1/keys`
- **THEN** 响应中的 `items[].group` 不包含真实 `rate_multiplier`、`image_rate_multiplier`、用户专属倍率或其他等价倍率字段

#### Scenario: 用户获取单个 API Key
- **WHEN** 普通用户请求单个 API Key 详情
- **THEN** 响应中的 `group` 不包含真实 `rate_multiplier`、`image_rate_multiplier`、用户专属倍率或其他等价倍率字段

### Requirement: `/api/v1/groups/rates` 不返回真实倍率
系统 MUST 防止普通用户通过 `/api/v1/groups/rates` 获得真实用户专属分组倍率。

#### Scenario: 用户请求专属分组倍率
- **WHEN** 普通用户请求 `/api/v1/groups/rates`
- **THEN** 系统返回空对象、废弃响应或拒绝访问，并且不返回任何真实倍率值

### Requirement: 管理员和计费行为保持不变
系统 SHALL 保留现有管理员倍率管理体验和内部倍率计算能力，隐藏 C 端展示不得改变实际计费、扣费、用量核算和管理员分析结果。

#### Scenario: 管理员体验保持不变
- **WHEN** 管理员通过现有管理员接口或后台页面请求分组、图片倍率或用户专属倍率配置
- **THEN** 系统继续按现有行为返回和展示真实倍率字段，本变更不得改变管理员体验、管理员 API 契约或管理逻辑

#### Scenario: 用户发起计费请求
- **WHEN** 普通用户使用绑定分组的 API Key 发起请求
- **THEN** 系统继续使用真实分组倍率或用户专属倍率计算扣费，但不会在 C 端页面或指定用户接口中暴露倍率

### Requirement: C 端使用记录展示请求 IP
系统 SHALL 在普通用户查看自己使用记录时展示每条记录的请求 IP，并 SHALL 在用户导出的 Usage CSV 中包含同一请求 IP 字段。系统 MUST 仅向记录所属用户返回该记录的 `ip_address`，不得因此暴露其他用户的 IP 或管理员专属字段。缺失请求 IP 的历史记录 SHALL 显示为空态占位，不得阻断使用记录列表、详情或导出。

#### Scenario: 用户查看使用记录 IP 列
- **WHEN** 普通用户打开 C 端 Usage 使用记录列表
- **THEN** 表格包含 IP 列
- **THEN** 每条有 `ip_address` 的记录在该列展示请求 IP
- **THEN** 每条缺失 `ip_address` 的记录在该列展示空态占位

#### Scenario: 用户导出使用记录 CSV 包含 IP
- **WHEN** 普通用户从 Usage 页面导出使用记录 CSV
- **THEN** CSV 包含 `IP` 列
- **THEN** 每行 `IP` 值与页面中同一 usage log 的请求 IP 一致
- **THEN** CSV 继续不得包含管理员专属的 account 审计字段、内部倍率字段或 raw/billable 双口径内部列

#### Scenario: 用户接口只返回本人记录 IP
- **WHEN** 普通用户请求 `/usage` 或 `/usage/:id`
- **THEN** 响应中的 usage log 可以包含该记录的 `ip_address`
- **THEN** 系统必须继续按当前用户过滤列表记录并校验详情记录所有权
- **THEN** 用户不得通过该变更读取其他用户 usage log 的 `ip_address`

#### Scenario: 管理员使用记录保持不变
- **WHEN** 管理员查看或导出管理后台 Usage 使用记录
- **THEN** 管理员后台继续展示和导出请求 IP
- **THEN** 本变更不得移除或弱化管理员已有 usage 审计字段

