## Why

C 端普通用户的使用记录当前不展示请求 IP，用户排查 API Key 调用来源、异常扣费或跨环境请求时只能依赖管理员后台协助。管理员后台已经在使用记录中展示 `ip_address`，普通用户侧也应能查看自己记录对应的请求 IP。

## What Changes

- 在 C 端 Usage 使用记录表格中新增 IP 列，展示每条记录的请求 IP。
- 用户 Usage API 响应需要向记录所属用户返回 `ip_address`，用于前端页面渲染和导出；该字段仍仅限用户自己的使用记录。
- 用户 Usage CSV 导出新增 IP 列，与页面使用记录保持一致。
- 管理员后台现有 IP 展示和导出行为保持不变。

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `user-facing-rate-visibility`: C 端普通用户 Usage 页面和 CSV 导出在展示自己使用记录时需要包含请求 IP，同时继续遵守现有隐藏内部计费倍率和 raw/billable 双口径的规则。

## Impact

- Affected backend/API: `/usage` and related user usage detail/list response serialization if `ip_address` is currently omitted from user-facing responses.
- Affected frontend: `frontend/src/types/index.ts`, `frontend/src/views/user/UsageView.vue`, and user UsageView tests.
- Affected exports: user Usage CSV headers and row mapping add IP.
- No database migration expected if usage logs already persist `ip_address` for admin usage records.
- No change to actual billing, usage aggregation, admin usage table, or admin export semantics.
