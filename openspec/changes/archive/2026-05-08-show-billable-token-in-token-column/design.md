## Context

C 端 Usage 表格当前已经在 token tooltip 中展示 billable token 明细，但 `TOKEN` 列主区域仍显示 raw input/output token 和 raw-compatible total。用户会同时看到两套不同 token 数字，且列表主列与费用列、tooltip 和账单核验口径不一致。

现有后端已经持久化 billable usage 字段，并为缺失 billable 字段的历史 usage log 定义 raw-compatible 回退语义。本变更应优先在 C 端前端展示层复用这些字段，不改变计费计算、扣费或管理员审计能力。

## Goals / Non-Goals

**目标：**

- C 端 Usage 表格 `TOKEN` 列输入、输出、缓存/总量等主显示值使用 billable token 口径。
- `TOKEN` 列主显示值与 token tooltip、费用核验和历史 billable fallback 语义保持一致。
- 对缺失 billable 字段的历史记录安全回退到 raw token，避免空值或错误展示。
- 保持管理员后台 raw/billable 审计展示不受影响。

**非目标：**

- 不修改计费、扣费、定价、余额或订阅统计逻辑。
- 不新增数据库字段或迁移。
- 不在 C 端展示 raw 与 billable 双口径对比、倍率 badge 或内部运营策略说明。
- 不改变导出/API 中用于账单校验的字段契约。

## Decisions

- 在 Usage 表格展示层新增或复用 billable token 汇总 helper。该 helper 优先读取 `billable_input_tokens`、`billable_output_tokens`、`billable_cache_creation_tokens`、`billable_cache_read_tokens`、`billable_image_output_tokens` 及图片兼容 billable 字段，字段缺失时按现有 raw 字段回退。
  - 理由：展示层调整最小，不污染 raw usage，也避免改变后端计费路径。
  - 备选方案：后端覆盖现有 raw token 响应字段。该方案会破坏 raw usage 语义和管理员/审计依赖，因此不采用。

- 保持 C 端 token tooltip 和 `TOKEN` 列共用同一 billable 计算/格式化来源。
  - 理由：避免 tooltip 已正确但列表主列仍偏离，降低后续维护时再次分叉的风险。
  - 备选方案：只在表格列局部替换字段。该方案容易遗漏缓存、图片或历史回退逻辑，因此不采用。

- 通过现有用户/admin 路由或角色判断限制本次 UI 改动范围。
  - 理由：管理员仍需要 raw usage、billable usage 和倍率快照做审计，本变更只修复 C 端用户可见口径。
  - 备选方案：全局替换 token 展示。该方案会削弱管理员分析能力，因此不采用。

## Risks / Trade-offs

- [风险] 不同 usage 端点字段命名或图片 token 维度不完全一致 → 缓解：helper 集中处理字段别名、null/undefined 和 raw-compatible fallback。
- [风险] 列表主列与 tooltip 使用不同格式化路径再次产生差异 → 缓解：将计算逻辑收敛到共享函数，测试覆盖典型 billable 与历史 raw fallback 数据。
- [风险] 管理员页面误用 C 端 helper 导致 raw 审计信息消失 → 缓解：仅在 C 端 Usage 表格接入，保留管理员后台现有渲染路径。
