## Context

普通用户 Usage 页面已经将表格 `TOKEN` 列和 token tooltip 调整为 billable-first 口径，避免用户看到 raw/billable 双口径。当前 CSV 导出仍同时包含 raw token、`Billable...` token、`Billing Token Multiplier`、`Billing Mode` 和 `Billing Base Cost`，这与页面隐藏内部计费机制的目标不一致，也容易让用户用 raw token 复算时误判费用错误。

## Goals / Non-Goals

**Goals:**

- 普通用户 CSV token 列与页面 `TOKEN` 列保持同口径：优先导出 billable token，缺失时回退 raw token。
- 普通用户 CSV 不导出 `Billable...`、`Billing...`、`billing_token_multiplier` 或其他内部计费策略列。
- 保留 `Final Cost`，使用户看到的 token 口径能够对应最终扣费。
- 保持管理员后台审计能力不变。

**Non-Goals:**

- 不改变后端 usage log 存储、API 响应字段或实际计费逻辑。
- 不改变管理员 Usage CSV/表格中的 raw usage、billable usage 或 multiplier 展示。
- 不调整页面表格和 tooltip 当前 billable-first 展示逻辑。

## Decisions

- 用户 CSV 继续使用现有前端导出流程，但调整 headers 与 row mapping。
  - Rationale: 当前导出已在用户页面拉取完整分页数据，需求仅为输出列和口径调整，无需新增后端导出接口。
  - Alternative considered: 新增后端用户 CSV 导出接口。该方案增加 API 和权限面，当前收益不足。

- `Input Tokens`、`Output Tokens`、`Cache Read Tokens`、`Cache Creation Tokens` 等列名保留用户可理解名称，值改为 `buildBillableFirstTokenBreakdown` 对应值。
  - Rationale: 用户侧不需要理解 billable 术语，但导出值必须与最终费用可核验口径一致。
  - Alternative considered: 保留 `Billable...` 列名。该方案继续暴露内部口径，不符合隐藏规则。

- 删除所有用户 CSV 中以 `Billing` 或 `Billable` 表达内部计费机制的列。
  - Rationale: `Billing Token Multiplier`、`Billing Base Cost`、`Billing Mode` 等字段属于内部审计/运营口径，普通用户侧不应导出。
  - Alternative considered: 仅删除 multiplier。该方案仍通过 `Billable...` 和 `Billing Base Cost` 暴露双口径。

## Risks / Trade-offs

- [Risk] 用户无法在用户侧 CSV 中同时审计 raw 上游 token 和 billable token 差异。→ Mitigation: 管理员后台继续保留审计列；普通用户侧以最终账单可核验为优先。
- [Risk] 依赖旧 CSV 列名的用户脚本会受影响。→ Mitigation: 这是用户侧导出契约收敛，保留核心请求元数据、token 和 final cost；管理员审计导出不变。
- [Risk] 历史记录缺失 billable 字段时导出 token 为空。→ Mitigation: 复用现有 billable-first helper，缺失 billable 时回退 raw token。
