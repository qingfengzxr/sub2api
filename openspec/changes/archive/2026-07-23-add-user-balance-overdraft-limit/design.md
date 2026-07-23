## Context

余额模式当前有两次准入检查：API Key 中间件使用认证快照中的余额执行历史 `balance <= 0` 检查，gateway handler 随后通过 `BillingCacheService` 使用 Redis/数据库余额和 `minimum_balance_reserve` 再次检查。实际 usage 结算已经允许当前请求把余额扣成负数，因此本变更主要改变“能否发起下一次请求”，不需要重写通用 usage 扣费事务。

异步批量图片是例外：提交时已经知道预占金额，并通过原子 SQL 将可用余额转为 `frozen_balance`。该路径当前要求余额足以覆盖全部预占金额，需要显式适配用户透支额度。

本仓库需要持续合并上游更新。设计必须把手写改动集中在稳定边界，避免在多个协议 handler、计费记录器或前端页面中复制额度判断。

## Goals / Non-Goals

**Goals:**

- 管理员可以在现有用户编辑入口配置非负的用户级透支额度。
- `0` 完全保持未配置用户的现有准入语义，包括 `minimum_balance_reserve`。
- API Key 鉴权预检和 billing cache 预检使用同一个领域规则。
- 配置调整后通过现有认证缓存失效机制及时生效。
- 已知预占金额的批量图片不得因预占操作越过用户透支线。
- 手写业务逻辑集中在少量文件，不修改每个 gateway handler。

**Non-Goals:**

- 不提供严格覆盖所有流式及并发请求的信用硬上限。
- 不预估普通请求费用，不引入通用余额预冻结或结算退款框架。
- 不新增独立 endpoint、repository 接口、feature flag、批量配置或用户列表列。
- 不改变订阅限额、API Key 独立额度、用户平台额度、计价或最终扣费口径。
- 不新增欠费账单、催收、告警或自动停用流程。

## Decisions

### 1. 使用非空 `overdraft_limit` 字段，`0` 表示关闭

在 `users` 表和 Ent schema 中增加 `decimal(20,8) NOT NULL DEFAULT 0` 字段，并增加 `CHECK (overdraft_limit >= 0)` 数据库约束；服务层使用 `float64` 与现有余额字段保持一致。管理员输入和服务层同样拒绝负值。

Rationale: 单个数值已经能同时表达关闭和额度，不需要 nullable、启用开关或单独配置表。默认值使存量数据和旧行为自然兼容。

Alternative considered: `enabled + limit` 两个字段或 nullable limit。它们增加状态组合、迁移和表单分支，但没有增加当前需求需要的能力。

### 2. 在独立领域文件中收敛余额准入策略

在 service 层新增独立的用户余额准入策略文件，基于 `balance`、规范化后的非负 `overdraft_limit` 和调用方传入的 `minimumReserve` 返回是否可用。API Key 鉴权使用 `minimumReserve = 0`，billing cache 使用现有配置值。

等价边界为：

```text
minimumReserve > 0: balance >= minimumReserve - overdraftLimit
minimumReserve = 0: balance > -overdraftLimit
```

这使 `overdraftLimit = 0` 时分别保持 billing cache 的 reserve 语义和鉴权层的 `balance > 0` 历史语义。达到 `-overdraftLimit` 时不再允许发起新请求。

Rationale: 两个现有检查点确实承担不同层次的保护，但它们不应各自实现额度公式。独立文件降低与上游大型 service 文件冲突的概率，也给边界逻辑留下直接单测入口。

Alternative considered: 只修改 billing cache。API Key 中间件会先拒绝负余额，功能无法生效。另一方案是在所有 handler 中改判断，会产生十多个重复修改点。

### 3. 复用现有 `User` 参数，不扩散 handler 改动

`CheckBillingEligibility` 已经接收完整的 `*User`，内部余额检查改为使用该对象和缓存中的实时余额。所有 gateway handler 的调用签名保持不变。扣费后的余额缓存同步也使用现有 `postUsageBillingParams.User` 计算是否已到达用户透支线。

Rationale: 现有调用链已经携带所需上下文，无需新接口、全局配置查询或 handler fan-out。

### 4. 额度进入 API Key 认证快照并随配置变更失效

认证快照的 user 部分携带 `overdraft_limit`，快照版本递增以拒绝没有该字段语义的旧缓存。管理员更新额度时，将额度变化加入现有 `InvalidateAuthCacheByUserID` 条件。

余额 Redis cache 继续只存余额数值，不把额度复制进去。billing check 从认证快照取得额度，从 balance cache 取得实时余额。

Rationale: 避免设计第二份额度缓存及其一致性协议。认证快照本来就承载用户并发、状态和 RPM 等请求准入配置。

### 5. 批量图片预占执行已知金额的硬检查

批量图片 reserve SQL 在同一条原子更新中要求：

```text
balance - hold_amount >= -overdraft_limit
```

SQL 直接读取同一 `users` 行的额度，不向 hold command 增加可伪造或可能过期的额度快照。capture/release 继续按 frozen balance 结算，不改变。

Rationale: 预占金额已知，因此可以且应当严格避免预占本身越过额度；数据库行内判断也能正确处理并发提交。

### 6. 管理能力只扩展现有编辑链路

管理员用户 DTO 和 update request 增加额度字段，普通用户 DTO 不需要暴露该内部授信配置。前端仅在 `UserEditModal` 增加原生 number input，复用现有 update API、校验展示和 i18n 结构。

Rationale: 满足“用户管理处设置”的最小范围，并减少新页面、组件和 endpoint 带来的上游合并冲突。

## Risks / Trade-offs

- [Risk] 普通请求费用在完成前未知，最后一次请求可以让余额低于软透支线。并发请求会放大越界金额。 -> Mitigation: 产品和字段文案明确这是“停止新请求的软透支线”；本变更不承诺严格信用上限，并增加并发越界测试/说明。
- [Risk] 鉴权快照与实时余额来自不同缓存，规则不一致会造成一层放行、一层拒绝。 -> Mitigation: 两层调用同一个 service 领域策略；额度更新失效认证快照；余额继续由现有 cache 维护。
- [Risk] 扣费后仍按余额 `<= 0` 失效 cache 会使透支用户每次请求都回源数据库。 -> Mitigation: 扣费后 cache 同步也按用户透支线判断，未到线时继续增量更新缓存。
- [Risk] 管理员意外配置过大的企业授信。 -> Mitigation: API 和 UI 只接受非负有限数值；不引入未经确认的全局最大值，运营侧额度治理留给后续明确需求。
- [Risk] Ent 重新生成会产生多份机械文件差异。 -> Mitigation: 这是 schema 字段的不可避免成本；除此之外不改生成器或公共接口，手写策略使用新文件隔离。
- [Risk] 旧实例与新实例滚动运行时，旧实例不了解透支额度。 -> Mitigation: 数据库字段向后兼容，但功能一致性要求完成新版本实例滚动后再依赖额度；快照版本升级会清除新实例读取到的旧格式。

## Migration Plan

1. 通过新的、幂等的 SQL migration 为 `users` 增加默认值为 `0` 的字段和非负 CHECK 约束，不修改历史 migration。
2. 部署包含新 Ent schema、服务字段和认证快照版本的应用；存量用户自动保持原行为。
3. 完成所有实例滚动后，由管理员按需给企业用户配置额度。
4. 应用回滚时旧版本会忽略新增列并继续按余额大于零的旧规则运行；数据库列可保留，避免破坏性回滚。

## Open Questions

None. 本方案按已确认的软透支语义推进；严格信用硬上限属于独立后续变更。
