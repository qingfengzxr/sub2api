# user-facing-rate-visibility Specification

## Purpose
TBD - created by archiving change hide-user-rate-multipliers. Update Purpose after archive.
## Requirements
### Requirement: C 端前端不展示倍率文案
系统 MUST 在 C 端普通用户前端隐藏所有直接倍率展示，包括 `x倍`、`倍率`、`rate multiplier` 和等价内部计费系数文案。

#### Scenario: 用户查看 API Key 分组
- **WHEN** 普通用户在 API Key 页面查看或切换分组
- **THEN** 前端展示分组名称、平台和用户可理解的状态，不展示分组倍率或用户专属倍率

#### Scenario: 用户查看可用渠道
- **WHEN** 普通用户查看可用渠道和可绑定分组
- **THEN** 前端展示可用性、平台、分组名称和说明，不展示倍率 badge 或 `x倍` 文案

#### Scenario: 用户查看用量或支付页面
- **WHEN** 普通用户查看用量、支付套餐、订阅计划或当前订阅权益
- **THEN** 前端不展示计费倍率字段或倍率文案

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

