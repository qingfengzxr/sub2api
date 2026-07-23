## Why

企业用户有时无法在余额耗尽前及时充值，当前余额准入会阻止其继续发起按余额计费的请求。系统需要允许管理员为个别用户配置一条软透支线，同时让未配置用户继续保持现有行为。

## What Changes

- 在用户数据中增加非负的 `overdraft_limit`，默认值为 `0`，表示不额外允许透支。
- 在管理员用户编辑界面提供“允许超支额度”输入项，并通过现有用户查询和更新接口读写。
- 余额计费请求使用用户级软透支线判断是否允许发起；请求开始时尚未到达透支线即可继续，已经达到透支线则按余额不足拒绝。
- 保留现有“按实际费用结算”的行为：在途请求和并发请求可能让最终余额越过软透支线。
- 让已知预占金额的异步批量图片余额冻结遵守同一用户额度，且不得在预占时越过透支线。
- 配置变化后失效该用户的 API Key 认证快照，使新额度及时生效。
- 将余额准入规则收敛为独立的小型领域策略，避免在各网关 handler 中复制判断。

## Capabilities

### New Capabilities

- `user-balance-overdraft`: 管理员配置用户级余额软透支额度，以及余额计费、缓存和已知金额预占对该额度的一致执行。

### Modified Capabilities

None.

## Impact

- Database and ORM: `users` 新增默认值为 `0` 的 decimal 字段，并重新生成 Ent 代码。
- Backend domain and repositories: 用户模型/映射透传字段；共享余额准入策略；批量图片预占条件使用用户额度。
- Authentication and billing cache: API Key 认证快照携带额度并升级快照版本；鉴权预检和 billing cache 预检复用同一策略。
- Admin API and frontend: 现有管理员用户 DTO、更新请求和用户编辑弹窗增加一个字段，不新增 endpoint 或组件。
- Tests: 覆盖默认兼容、软透支边界、配置缓存失效、批量图片预占和管理端保存。
- Dependencies: 不新增第三方依赖。
