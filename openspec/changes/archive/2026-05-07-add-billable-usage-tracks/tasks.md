## 1. 数据模型和迁移

- [x] 1.1 为 `usage_logs` 新增 billable token 字段和 `billing_token_multiplier` 的幂等 migration
- [x] 1.2 更新 `service.UsageLog`、token 结构、usage log DTO 和前端 TypeScript 类型，加入 billable usage 字段
- [x] 1.3 更新 usage log repository 的 select、insert、scan、batch insert 路径，并补充 repository 测试
- [x] 1.4 确保历史行读取时按 raw-compatible billable usage 和 multiplier `1` 处理
- [x] 1.5 更新系统设置 DTO/存储结构，新增 `billing.token_multiplier_billing_enabled` 和 `billing.billing_token_multiplier`

## 2. 计费 Usage Builder

- [x] 2.1 新增 service 层 billable usage builder，输入 raw usage、计费策略和倍率，输出 billable usage
- [x] 2.2 为 builder 覆盖启用模式、关闭模式、零 usage、cached tokens、image output tokens 和 OpenAI 图片细分维度
- [x] 2.3 在 builder 中接入平台级 `billing.token_multiplier_billing_enabled` 和 `billing.billing_token_multiplier`，默认关闭且倍率为 `1`
- [x] 2.4 确保 builder 不使用 group `rate_multiplier`、用户专属 group rate override、图片 `image_rate_multiplier` 或 `account_rate_multiplier` 生成 billable tokens

## 3. 计费逻辑

- [x] 3.1 更新 token 模式费用计算，使 token multiplier billing 启用时使用 billable usage
- [x] 3.2 确保启用模式下 `total_cost = sum(price_i * billable_tokens_i)`，`actual_cost` 继续按现有金额侧 `rate_multiplier` 逻辑在 `total_cost` 基础上计算
- [x] 3.3 保持关闭模式行为：raw usage 计算 `total_cost`，现有 rate multiplier 逻辑计算 `actual_cost`
- [x] 3.4 更新 cached token pricing、long-context/service-tier pricing、渠道区间定价和 image output token pricing，使其使用当前选定的 usage 轨道
- [x] 3.5 保持非 token 型图片计费和按次计费语义不变，除非明确使用 token 计价维度

## 4. Record Usage 路径

- [x] 4.1 更新 Anthropic/Gemini gateway record usage 路径，构建并保存 billable usage，同时不修改 raw usage
- [x] 4.2 更新 OpenAI chat、responses、WS record usage 路径，构建并保存 billable usage，同时不修改 raw usage
- [x] 4.3 更新 OpenAI image 和 image-output-token 路径，保存可用的 billable image 维度
- [x] 4.4 确保余额扣费、订阅用量、API Key usage-rate 统计和重复请求处理都使用最终 `actual_cost`
- [x] 4.5 确保 billing model source 继续兼容 requested、upstream 和 channel-mapped

## 5. 报表、API 和前端

- [x] 5.1 更新用户/管理员 usage API 响应和 mapper，返回 raw usage、billable usage、`billing_token_multiplier`、`total_cost` 和 `actual_cost`
- [x] 5.2 更新 usage export，包含 billable usage 字段和倍率快照
- [x] 5.3 更新 dashboard、ranking、trend、group/model/endpoint 和 user breakdown 查询，确保 token counts 保持 raw，costs 使用持久化 cost 字段
- [x] 5.4 更新 account stats 代码、注释和测试，保持 raw token totals、`standard_cost`、`user_cost` 和 account cost 语义清晰
- [x] 5.5 更新 `frontend/src/types/index.ts` 和 `frontend/src/api/usage.ts`，加入 billable usage 字段和 `billing_token_multiplier`
- [x] 5.6 更新 `frontend/src/views/user/UsageView.vue` 的 token 列、token tooltip、费用 tooltip 和 CSV export：raw token 默认不变，导出增加 billable 字段，不展示内部倍率
- [x] 5.7 更新用户 dashboard 相关文件：`frontend/src/views/user/DashboardView.vue`、`frontend/src/components/user/dashboard/UserDashboardStats.vue`、`UserDashboardCharts.vue`、`UserDashboardRecentUsage.vue`，保持 token totals 为 raw、费用为最终消费
- [x] 5.8 检查并更新 `frontend/src/views/user/KeysView.vue`、`frontend/src/views/KeyUsageView.vue`、`PaymentView.vue`、`SubscriptionsView.vue`、`UserOrdersView.vue`、`AvailableChannelsView.vue`、`ChannelStatusView.vue`，确保不出现倍率展示或标准/实际双口径回归
- [x] 5.9 更新前端 i18n 文案，使用“真实用量”“计费用量”“账单校验用量”等标签，避免 C 端出现“倍率 x 倍”文案
- [x] 5.10 更新 `frontend/src/views/admin/SettingsView.vue` 的“功能开关”tab，新增 billable token 计费开关和平台级 token 放大倍率输入项

## 6. 验证

- [x] 6.1 增加纯文本模型在 token multiplier billing 启用和关闭两种模式下的后端测试
- [x] 6.2 增加 cached token pricing 在启用和关闭两种模式下的后端测试
- [x] 6.3 增加 OpenAI 路径、图片模型和 image output token 路径的后端测试
- [x] 6.4 增加测试证明 billable token multiplier 与金额侧 `rate_multiplier` 可以按设计叠加
- [x] 6.5 增加测试证明官方单价乘持久化 billable tokens 可以反推出 `total_cost`
- [x] 6.6 运行 billing、gateway record usage、repository usage logs、account stats 和 dashboard queries 的目标 Go 测试
- [x] 6.7 运行 usage views 和 admin/user usage components 的目标前端类型/单元测试
