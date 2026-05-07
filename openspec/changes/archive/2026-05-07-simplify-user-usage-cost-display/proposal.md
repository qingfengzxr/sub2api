## Why

C 端用户使用记录页当前同时展示“实际”和带删除线的“标准”消费，容易让用户联想到倍率、折扣或内部计费系数。用户侧目标是只理解最终扣费结果，不需要看到标准价、实际价等内部口径差异。

## What Changes

- 调整 C 端使用记录页总消费卡片，只展示最终总消费金额。
- 移除普通用户可见的“实际”“标准”、删除线标准金额及等价双口径消费说明。
- 调整 C 端使用记录表格费用 tooltip，移除“原始”“计费”等会暴露标准/最终双口径差异的字段，只保留用户需要理解的最终消费结果和必要的非倍率用量信息。
- 保持费用数据来源为最终扣费口径，不改变后端 API、计费、扣费、导出或管理员侧展示。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `user-facing-rate-visibility`: 强化 C 端使用记录页消费展示要求，普通用户只看到最终消费结果，不暴露实际/标准、原始/计费等双口径及其差异。

## Impact

- Affected code: `frontend/src/views/user/UsageView.vue` 及相关 i18n 文案（如存在不再使用的 `actualCost` / `standardCost` / `original` / `billed` 用户侧文案）。
- APIs: 无后端接口契约变化，继续使用现有统计字段。
- Billing: 无计费、扣费、余额、订阅额度或用量统计逻辑变化。
- Admin: 管理员侧用量分析和倍率相关展示不受影响。
