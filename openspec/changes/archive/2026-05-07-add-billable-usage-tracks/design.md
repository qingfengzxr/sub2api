## 背景

当前 usage log 模型保存 `input_tokens`、`output_tokens`、cache tokens、`image_output_tokens` 等真实 token 数，同时保存 `total_cost`、`actual_cost`、`rate_multiplier`、可选 `account_rate_multiplier` 和可选 `account_stats_cost`。这些 token 字段目前既用于供应商对账和审计，也用于性能分析、限流、dashboard、ranking、导出和 account stats。

本变更新增一条明确的计费 usage 轨道。raw usage 继续表示供应商真实用量；billable usage 表示平台收费时使用的 token 数量，用户可以用官方单价乘以 billable usage 反推出最终平台金额。实现时必须保持渠道定价、group/user pricing、account stats pricing、image pricing、billing model source、dashboard/ranking/export 和用户侧隐藏内部倍率体验不回归。

## 目标 / 非目标

**目标：**
- 保留上游 raw usage，不覆盖、不放大、不污染现有 usage token 字段。
- 在每条 usage log 中持久化 billable token 字段和 `billing_token_multiplier`。
- 新增类似 `BillableUsageBuilder` 的组件，将 raw usage、计费策略和倍率转换为 billable usage。
- 支持 token multiplier billing 模式：`billable = raw * billing_token_multiplier`，`total_cost = official price * billable`，`actual_cost = total_cost * effective_rate_multiplier`。
- 当功能开关关闭时，保持旧的金额侧倍率计费行为。
- API 和前端 usage 数据同时暴露 raw usage、billable usage、费用和倍率快照，语义清晰。
- 明确 account stats 的 raw token、standard cost、user cost 和 account cost 边界，避免限流/性能统计误用 billable tokens。

**非目标：**
- 不重新计算或改写历史 usage 的价格。
- 不移除 `rate_multiplier`、`account_rate_multiplier` 或现有 group/user pricing 配置。
- 不替换渠道定价、区间定价、图片/按次计费模式或 billing model source 解析机制。
- 不改变上游请求或供应商 usage 解析行为。

## 技术决策

### 将 billable usage 与 raw usage 并列存储

在 `usage_logs` 中新增以下字段：
- `billable_input_tokens`
- `billable_output_tokens`
- `billable_cache_creation_tokens`
- `billable_cache_read_tokens`
- `billable_image_output_tokens`
- `billable_text_input_tokens`
- `billable_cached_text_input_tokens`
- `billable_image_input_tokens`
- `billable_cached_image_input_tokens`
- `billing_token_multiplier`

前五个字段对应当前已经广泛支持的 usage 维度。OpenAI 图片相关字段提前加入，避免后续在获得更细粒度 image input usage 时再次做 schema 调整。现有 raw 字段继续作为对账、限流、TPM、供应商审计和模型性能分析的数据源。

考虑过的替代方案：直接把 raw token 字段写成放大后的值。该方案会污染审计和限流语义，也会让供应商对账不可靠，因此放弃。

### 引入独立 billable usage 结构和 builder

新增 service 层结构，例如：
- `RawUsage`，或复用现有 `UsageTokens` 表示供应商真实用量。
- `BillableUsage`，包含所有 billable 维度。
- `BillingTokenPolicy`，包含 `Enabled`、`Multiplier`、`BillingMode`、模型/路径 hint 和未来维度策略字段。

当 token multiplier billing 关闭，或某条路径不参与 token multiplier billing 时，builder 默认输出 raw-compatible billable usage，并将倍率记为 `1`。当启用时，builder 对支持的 billable 维度乘以倍率并使用统一取整规则。取整逻辑必须集中在一个 helper 中，保证测试稳定；建议对非零小数向上取整，避免小请求被静默少计费。

考虑过的替代方案：在每条 record usage 路径中分散写 `raw * multiplier`。该方案容易导致取整不一致、重复加价和未来图片维度遗漏，因此放弃。

### 费用计算使用 billable usage 作为金额基数

token 模式费用计算必须接收“当前用于计算 `total_cost` 的 usage 对象”。旧模式下传入 raw usage，再按现有逻辑应用 `rate_multiplier` 得到 `actual_cost`。token multiplier billing 模式下传入 billable usage，先记录基于 billable usage 的基础费用：

`total_cost = sum(official_price_i * billable_tokens_i)`

然后继续沿用现有金额侧倍率逻辑：

`actual_cost = total_cost * effective_rate_multiplier`

其中 `effective_rate_multiplier` 仍来自现有 group/user pricing 解析结果；图片路径如现有逻辑使用图片独立金额倍率，则继续沿用原图片金额倍率规则。这样 billable token multiplier 和 group/user rate multiplier 是两个可叠加的计费维度：前者改变可按官方单价核验的计费 token 数，后者改变最终扣费金额。

不依赖 token 数量的 per-request 和 image billing mode 保持现有行为；如果图片路径存在 token 计价维度，例如 `image_output_tokens`，则 image output token 价格在 token multiplier billing 模式下使用 `billable_image_output_tokens`。

考虑过的替代方案：启用 billable token 后强制跳过金额侧 `rate_multiplier`。该方案会让 billable token multiplier 与现有 group/user pricing 无法同时表达两层业务策略，因此放弃。

### 平台级 billable token 倍率独立配置

token multiplier billing 的启停和倍率值使用平台级系统设置，不复用 group/user 倍率。建议配置位置为：
- `billing.token_multiplier_billing_enabled`：是否启用 billable token 放大，默认 `false`。
- `billing.billing_token_multiplier`：平台级 billable token 放大倍率，默认 `1.0`，仅在启用开关后参与 builder。

该配置应在管理端系统设置的“功能开关”tab 中提供 UI，按平台全局生效，不按用户、分组、渠道或模型分别配置。配置保存后对后续新 usage 生效；历史 usage log 依赖持久化的 `billing_token_multiplier` 快照，不回填。

builder 只使用平台级 `billing.billing_token_multiplier` 生成 billable tokens。现有 group `rate_multiplier`、`user_group_rate_multipliers.rate_multiplier`、图片金额倍率和 `account_rate_multiplier` 不参与 billable token 生成，继续在各自原有金额或 account stats 口径中生效。

持久化的 `billing_token_multiplier` 记录 builder 实际使用的平台级 token 倍率；关闭或不适用时为 `1`。持久化的 `rate_multiplier` 继续记录现有金额侧有效倍率，允许与 billable token multiplier 叠加。

考虑过的替代方案：复用现有 group/user `rate_multiplier` 作为 billable token multiplier。该方案会把“计费 token 放大”和“金额侧 group/user 定价”两个业务语义绑死，也不利于在系统设置中统一控制平台级 token 放大，因此放弃。

### 管理端配置入口

新增的 billable token 配置放到系统设置“功能开关”中，建议位置与现有开关卡片保持一致：
- `frontend/src/views/admin/SettingsView.vue`：在“功能开关”tab 新增“Billable Token 计费”或“计费用量放大”设置卡片。
- 后端 admin settings API/DTO：新增读取和保存 `billing.token_multiplier_billing_enabled`、`billing.billing_token_multiplier`。
- i18n 文案：说明该倍率会放大 billable tokens，`total_cost = 官方单价 × billable tokens`，且最终 `actual_cost` 仍可能继续叠加 group/user 金额倍率。

现有管理端倍率入口保持原语义：
- `frontend/src/views/admin/GroupsView.vue` 和相关 group 表单继续维护 group `rate_multiplier`、`image_rate_multiplier`。
- `frontend/src/components/admin/group/GroupRateMultipliersModal.vue` 继续维护用户专属 group `rate_multiplier`。
- `frontend/src/views/admin/UsersView.vue`、`frontend/src/components/admin/user/UserEditModal.vue` 或现有用户分组倍率入口继续维护单用户覆盖关系。
- `frontend/src/views/admin/ChannelsView.vue` 和 `frontend/src/components/admin/channel/*` 继续维护渠道定价和 billing model source，不承载倍率开关。

实现时需要在管理端文案或帮助提示中说明：billable token multiplier 是平台级用量放大，group/user `rate_multiplier` 是金额侧倍率，两者可以叠加。普通用户侧仍不展示内部倍率。

### API 语义只做加法

usage log DTO 和前端类型新增 billable 字段，保留现有 raw 字段名不变。API 响应应将 `input_tokens`、`output_tokens`、cache 字段和现有 image 字段定义为 raw usage；billable 字段使用显式命名。导出文件同时包含两条轨道。dashboard/ranking 默认 token totals 继续基于 raw token，cost metrics 使用持久化费用字段。

现有 `user-facing-rate-visibility` 行为仍然保留：普通用户 UI 不展示内部 group/account rate multiplier，也不展示旧的标准/实际双口径费用解释。billable usage 字段可以作为账单校验数据返回和展示，但不应渲染成内部倍率 badge。

### C 端页面改动范围

普通用户侧需要明确修改以下页面和组件：
- `frontend/src/types/index.ts`：为 `UsageLog`、`UsageStatsResponse` 相关类型增加 billable token 字段和 `billing_token_multiplier`。
- `frontend/src/api/usage.ts`：同步用户 dashboard/usage API 类型，保持 token totals 为 raw usage，费用为最终 cost。
- `frontend/src/views/user/UsageView.vue`：usage 表格 token 列默认继续展示 raw usage；token tooltip 增加 billable usage 明细或账单校验区域；费用 tooltip 继续只展示最终费用，不恢复标准/实际双口径；CSV export 增加 raw/billable 字段和倍率快照。
- `frontend/src/views/user/DashboardView.vue`、`frontend/src/components/user/dashboard/UserDashboardStats.vue`、`UserDashboardCharts.vue`、`UserDashboardRecentUsage.vue`：token 统计继续显示 raw totals，费用继续显示 `actual_cost`/最终消费；最近用量如展示 token 明细，可补充 billable tokens，但不得展示内部倍率 badge。
- `frontend/src/views/user/KeysView.vue`：API Key 今日/累计消费继续显示最终费用；如果使用 key usage stats，不改变 token totals 语义。
- `frontend/src/views/KeyUsageView.vue`：公开 key usage 查询中的 token stats 继续显示 raw tokens；费用列使用最终费用；如导出/详情需要账单校验，则增加 billable 字段但不显示内部倍率解释。
- `frontend/src/views/user/PaymentView.vue`、`SubscriptionsView.vue`、`UserOrdersView.vue`：不新增倍率展示；如页面展示套餐额度或消费进度，继续使用最终费用口径，避免出现“倍率”“标准/实际”双口径文案。
- `frontend/src/views/user/AvailableChannelsView.vue`、`ChannelStatusView.vue`：继续只展示渠道可用性、平台和状态，不展示倍率或 billable multiplier。

相关 i18n 文案也要同步新增“真实用量”“计费用量”“账单校验用量”等用户可理解标签，但避免使用“倍率 x 倍”作为 C 端可见文案。

### account stats 语义保持显式

account stats 当前有三类费用语义：账号成本、标准费用、用户费用。本变更保持标签，但明确 base：
- raw token totals 继续表示真实用量。
- `user_cost` 使用 `usage_logs.actual_cost`，表示用户实际扣费，可能同时包含 billable token multiplier 和 group/user rate multiplier。
- `standard_cost` 使用 `usage_logs.total_cost`；在 token multiplier billing 模式下它是官方单价乘 billable tokens 后、尚未叠加 group/user rate multiplier 的费用基数。
- `cost` 优先使用 `account_stats_cost`，否则沿用现有 account multiplier 公式。

如果 account stats custom pricing 从 tokens 重新计算成本，默认使用 raw usage，因为 account stats 主要是上游账号成本视角；除非未来显式配置为使用 billable usage。

## 风险 / 权衡

- 历史行没有 billable 字段 → 读取时将缺失 billable usage 视为 raw-compatible，`billing_token_multiplier` 视为 `1`。
- dashboard token totals 可能被误改为 billable totals → 默认 token-count 指标继续使用 raw 字段，只在明确需要时新增 billable totals。
- 用户侧已有隐藏倍率要求 → 可以返回 billable 字段用于审计，但普通用户界面避免展示内部倍率 badge 或旧的标准/实际费用解释。
- 取整可能造成极小金额差异 → 集中取整规则，并明确持久化 billable tokens 就是费用计算使用的数量。
- 部分图片路径 usage 维度暂时不完整 → 先持久化可用维度；缺失的细粒度 image input 字段按零值或 raw-compatible 语义兼容，等待后续上游解析增强。

## 迁移计划

1. 新增幂等 migration，为 `usage_logs` 增加 billable token 字段和 `billing_token_multiplier NUMERIC(10,4) NOT NULL DEFAULT 1.0`。
2. 更新 repository select/insert/scan 和 service/domain DTO，确保新行能写入 billable usage，历史行能安全读取。
3. 新增 builder 和费用计算接线，受系统设置中的 `billing.token_multiplier_billing_enabled` 和 `billing.billing_token_multiplier` 控制，默认关闭且倍率为 `1`。
4. 更新 record usage 路径，在费用计算前构建 billable usage，并同时持久化 raw 与 billable 两条轨道。
5. 更新 API mapper、前端类型、usage 表格/tooltip/export/dashboard stats 等展示和导出路径。
6. 增加 builder、billing service、record usage、repository mapping、API DTO 和前端展示/export 的聚焦测试。

回滚优先通过关闭 `billing.token_multiplier_billing_enabled` 完成；数据库层面的新增字段为向后兼容字段，可以保留。
