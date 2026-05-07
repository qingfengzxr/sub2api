## 1. Usage Tooltip 展示口径

- [x] 1.1 梳理 `frontend/src/views/user/UsageView.vue` 中 token tooltip 当前 raw 与 billable 展示逻辑
- [x] 1.2 新增或调整 billable-first token helper，按 `billable_*` 优先、raw 兼容回退的规则输出 input/output/cache/image output 明细
- [x] 1.3 将 token tooltip 主明细改为直接展示 billable-first 明细
- [x] 1.4 移除 tooltip 中独立的“账单校验用量”分区，避免 C 端看到 raw/billable 双口径并列
- [x] 1.5 将 tooltip 总 Token 改为 billable-first 总和

## 2. 文案和展示边界

- [x] 2.1 检查并调整 Usage tooltip 文案，必要时将标题从“Token 明细”改成用户可理解的计费用量口径文案
- [x] 2.2 保留 CSV export 中 raw 与 billable 字段，确保导出仍可用于账单校验
- [x] 2.3 确保 C 端页面不展示 `billing_token_multiplier` 为倍率 badge 或内部运营策略说明

## 3. 测试和验证

- [x] 3.1 更新或新增 `UsageView` 前端测试，覆盖 tooltip 显示 billable tokens 而不是 raw tokens
- [x] 3.2 覆盖 billable 字段缺失或为历史兼容值时的 raw fallback
- [x] 3.3 运行前端目标测试或 `pnpm run typecheck`
- [x] 3.4 运行 `openspec validate show-billable-tokens-in-user-usage --strict`
