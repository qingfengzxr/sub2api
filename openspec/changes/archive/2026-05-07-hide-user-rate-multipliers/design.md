## Context

当前普通用户侧可以通过多个路径看到分组倍率：`/api/v1/keys` 返回的 API Key 嵌套 `group.rate_multiplier` 会在 `/keys` 页面展示，`/api/v1/groups/rates` 会返回用户专属分组倍率，部分前端页面和共享组件也会直接显示 `x倍`。倍率本质上是内部计费和运营策略，不适合直接暴露给普通用户，尤其当倍率被调高到 3 倍或更高时，会引发解释成本和用户感知问题。

本变更只做最小改动：隐藏 C 端页面中的倍率展示，收敛 `/api/v1/keys` 和 `/api/v1/groups/rates` 这两个直接导致 `/keys` 页面暴露倍率的接口返回。管理员后台体验、管理员 API、管理员管理逻辑、后端计费逻辑和其他用户侧 API DTO 暂不全面重构，以降低长期独立分支同步上游的冲突成本。

## Goals / Non-Goals

**Goals:**

- C 端用户页面不再直接展示分组倍率、用户专属分组倍率或类似 `x倍` 的内部计费倍率展示。
- `/api/v1/keys` 普通用户响应中不再通过嵌套 `group` 暴露真实倍率字段。
- `/api/v1/groups/rates` 不再向普通用户返回真实用户专属分组倍率。
- 管理员后台体验和管理逻辑完全保持现状，继续可以查看和配置所有倍率。
- 后端计费、用量核算、余额扣费和管理员用量分析继续使用真实倍率。

**Non-Goals:**

- 不全面改造所有用户侧 API DTO。
- 不调整 `/api/v1/groups/available` 的响应字段，除非前端展示需要避免继续使用其中的倍率。
- 不调整倍率计算规则、价格公式或计费基数。
- 不删除数据库中的倍率字段。
- 不改动管理员后台页面交互、管理员 API 契约或管理员管理逻辑。
- 不重构 group、billing、gateway 或 usage 的核心领域模型。

## Decisions

### 前端优先复用现有隐藏能力

`GroupBadge` 已支持 `showRate=false`。C 端页面应优先通过现有组件能力隐藏倍率，移除用户侧 `always-show-rate` 和 `userRateMultiplier` 展示。对不支持隐藏的组件，例如直接展示倍率的 option item，应增加最小 prop 或在用户侧调用时不传倍率。

备选方案：

- 大规模重写用户侧分组组件：可以彻底隔离，但改动大且与上游更容易冲突。
- 只改后端不改前端：如果其他接口仍含倍率或静态数据仍被展示，用户侧仍可能看到倍率。

### `/api/v1/keys` 使用最小用户侧响应收敛

`/api/v1/keys` 是 `/keys` 页面当前已绑定分组 badge 的主要来源。应在该用户侧 handler 或 mapper 中对嵌套 `group` 做最小脱敏，不返回真实 `rate_multiplier`、`image_rate_multiplier` 和等价倍率字段。为减少冲突，优先新增小的用户侧 response mapper 或 wrapper，不修改现有通用/admin DTO 结构。

备选方案：

- 直接修改 `dto.Group` 删除倍率字段：会影响管理员和其他调用方，也更容易与上游冲突。
- 只在前端不读 `row.group.rate_multiplier`：可隐藏页面展示，但 API 仍泄露该值。

### `/api/v1/groups/rates` 对普通用户不返回真实倍率

`/api/v1/groups/rates` 专门返回用户专属分组倍率，应让普通用户不再获得真实倍率。首选返回空对象 `{}` 以兼容旧前端；也可以在后续版本改为 404/410 或仅管理员可访问。本 change 只要求普通用户拿不到真实倍率。

备选方案：

- 删除路由：最干净，但可能导致旧前端或第三方客户端报错。
- 保留真实返回但前端不调用：仍然泄露倍率。

### 不全面收敛其他用户侧 API

为了保持最小改动，本 change 不要求全面改造 usage、subscription、available channels 等所有用户侧 API DTO。前端页面展示层仍必须隐藏倍率；如果后续确认其他普通用户 API 字段也需要隐藏，再拆成单独小变更处理。

备选方案：

- 一次性收敛所有用户侧 API 倍率字段：更彻底，但改动范围更大，不利于后续同步上游。

## Risks / Trade-offs

- 其他用户侧 API 仍可能返回倍率字段 → 本 change 先解决当前明确暴露点；后续发现新暴露点再独立收敛。
- 旧前端依赖 `/groups/rates` → 返回空对象比删除路由更兼容。
- 前端漏掉某些倍率展示入口 → 增加搜索检查，重点覆盖 Keys、Available Channels、Usage、Payment 页面。
- 误影响管理员后台 → 本次不得修改管理员体验和管理逻辑；仅确认管理员侧不受影响。
- 计费逻辑被误改 → 实现时不触碰 gateway/billing 核心计算，只改 C 端展示和两个用户侧接口返回。

## Migration Plan

1. 更新 C 端 `/keys` 页面和相关分组组件调用，隐藏倍率和 `x倍` 文案。
2. 更新其他 C 端页面中已有倍率展示调用，优先使用 `showRate=false` 或不传倍率。
3. 为 `/api/v1/keys` 增加最小用户侧 response mapper 或 wrapper，隐藏嵌套 group 的倍率字段。
4. 调整 `/api/v1/groups/rates` 普通用户响应，不再返回真实倍率，优先返回空对象保持兼容。
5. 增加测试或检查，确认 C 端页面不展示倍率，`/api/v1/keys` 和 `/api/v1/groups/rates` 不返回真实倍率。
6. 回归确认管理员后台和计费逻辑保持不变。

## Open Questions

- `/api/v1/groups/rates` 当前阶段返回空对象是否足够，还是应该直接返回 404/410？
- `/api/v1/groups/available` 仍返回倍率字段时，是否接受“页面不展示但 API 仍可见”的阶段性状态？
