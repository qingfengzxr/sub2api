## 1. C 端 Usage Tooltip 精简

- [x] 1.1 检查 `frontend/src/views/user/UsageView.vue` 当前 token tooltip 是否仍展示 raw 明细、账单校验用量分区或 `billing_token_multiplier`
- [x] 1.2 保留或补齐 billable-first helper，输出 input/output/cache/image output 和 total billable tokens
- [x] 1.3 将 C 端 token tooltip 模板改为只展示最终计费用量明细和总量
- [x] 1.4 移除 C 端 token tooltip 中 raw/billable 并列分区、账单校验用量标题和 token 放大倍率说明

## 2. 管理后台 Usage Tooltip 明细

- [x] 2.1 检查 `frontend/src/components/admin/usage/UsageTable.vue` 当前 token tooltip raw 展示逻辑
- [x] 2.2 新增管理员用 token breakdown helper，分别生成 raw 明细、billable-first 明细、raw total、billable total 和 `billing_token_multiplier`
- [x] 2.3 调整管理员 token tooltip 模板，展示“真实用量”和“计费用量”两个分区
- [x] 2.4 在管理员 token tooltip 中展示 billable token 总量和倍率快照，确保可与费用 tooltip 的 `total_cost`/`actual_cost` 对账
- [x] 2.5 保持管理员费用 tooltip 的用户扣费、账号成本和金额侧倍率明细不被 C 端隐藏规则影响

## 3. 文案和导出

- [x] 3.1 检查 i18n 文案，确保 C 端不出现 raw/billable 双口径解释，管理员端使用“真实用量”“计费用量”“倍率快照”等审计文案
- [x] 3.2 确认现有 CSV/API 导出继续包含 raw usage、billable usage、`billing_token_multiplier` 和费用字段；如 UI 导出缺少管理员审计字段，按本 change 范围补齐

## 4. 测试和验证

- [x] 4.1 更新或新增 C 端 UsageView 测试，断言 token tooltip 只展示 billable 明细和总量
- [x] 4.2 更新或新增管理员 UsageTable 测试，断言 token tooltip 展示 raw/billable 双轨明细和倍率快照
- [x] 4.3 覆盖 billable 字段缺失或为 0 时的 raw-compatible fallback
- [x] 4.4 运行相关前端测试
- [x] 4.5 运行 `openspec validate separate-user-admin-usage-token-tooltips --strict`
