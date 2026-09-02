## ADDED Requirements

### Requirement: 用户可编辑 API 密钥的 30 天限额
系统 SHALL 在用户编辑 API 密钥的速率限制区域提供固定 30 天 USD 限额。

#### Scenario: 保存月度限额
- **WHEN** 用户提交大于 `0` 的有限 `rate_limit_30d`
- **THEN** 系统保存该限额，并在再次编辑时返回相同值

#### Scenario: 禁用月度限额
- **WHEN** 用户提交 `rate_limit_30d: 0` 或关闭速率限制总开关
- **THEN** 系统不得因 30 天用量拒绝该密钥

#### Scenario: 省略或提交非法值
- **WHEN** 更新请求省略 `rate_limit_30d`
- **THEN** 系统保持原值不变
- **WHEN** 更新请求提交负数、NaN 或无限值
- **THEN** 系统拒绝请求且不修改密钥

### Requirement: 系统执行固定 30 天消费窗口
系统 SHALL 按现有 5 小时、1 天和 7 天限额的相同模型，维护 `usage_30d` 和 `window_30d_start`。

#### Scenario: 累计和拒绝
- **WHEN** 密钥完成一次计费
- **THEN** 系统在数据库与 Redis 缓存中原子累计 `usage_30d`
- **WHEN** 有效 `usage_30d` 大于或等于正数 `rate_limit_30d`
- **THEN** 系统在访问上游前返回 HTTP 429

#### Scenario: 窗口到期
- **WHEN** 当前时间达到 `window_30d_start` 后 30×24 小时
- **THEN** 限速检查将旧窗口用量视为 `0`
- **THEN** 后续计费按现有窗口重置逻辑开始累计新窗口

### Requirement: 更新和重置保持缓存与用量一致
系统 MUST 保证密钥编辑不会覆盖并发计费产生的月度用量。

#### Scenario: 编辑与重置
- **WHEN** 用户只修改限额或其他密钥字段
- **THEN** 系统不得写回 `usage_30d` 或 `window_30d_start` 的旧快照
- **WHEN** 用户触发现有限速用量重置
- **THEN** 系统清零四档用量与窗口并使相关缓存失效
