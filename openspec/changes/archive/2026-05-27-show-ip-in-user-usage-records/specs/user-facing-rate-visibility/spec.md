## ADDED Requirements

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
