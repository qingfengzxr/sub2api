## Context

API 密钥页当前通过 `/usage/dashboard/api-keys-usage` 批量读取每个密钥的 `today_actual_cost` 和 `total_actual_cost`，前端固定把第二行标成“近30天”。使用记录页已经有通用 `DateRangePicker`，后端仓储层的 `GetBatchAPIKeyUsageStats` 也已经接收 `startTime` 和 `endTime`，只是用户侧 handler 当前传入空时间并依赖默认近 30 天。

## Goals / Non-Goals

**Goals:**
- 在 API 密钥页复用现有时间范围选择交互，默认近 30 天。
- 让 `total_actual_cost` 按用户选择的时间范围重新统计。
- 保持 `today_actual_cost` 的当天统计逻辑不受所选范围影响。
- 保持现有接口响应结构和 API Key 所有权校验。

**Non-Goals:**
- 不新增计费口径、倍率说明或 raw/billable 双口径展示。
- 不新增数据库字段或迁移。
- 不重写 DateRangePicker，也不改变使用记录页的筛选行为。

## Decisions

1. 复用 `DateRangePicker`，不新增选择器组件。
   - Rationale: 使用记录页已经有用户熟悉的 preset 和自定义日期交互，复用它能保持体验一致并减少维护面。
   - Alternative considered: 在 Keys 页单独做一个简化 picker；这会复制日期边界和 preset 逻辑。

2. 扩展现有批量用量接口，而不是新增 endpoint。
   - Rationale: 当前接口已经是 Keys 页专用的批量用量汇总入口，加入可选 `start_date` / `end_date` 后可以保持响应 shape 不变。
   - Alternative considered: 新增 `/api-key-usage-range` 接口；这会让同一页面维护两套统计入口。

3. 日期按自然日范围处理，结束日期在后端转换为次日零点的排他边界。
   - Rationale: DateRangePicker 输出 `YYYY-MM-DD`，用户理解的是包含开始日和结束日的自然日范围；仓储查询已使用 `[startTime, endTime)`。
   - Alternative considered: 前端传时间戳；这会把时区和日边界逻辑分散到浏览器端。

4. `today_actual_cost` 继续由后端按当天独立聚合。
   - Rationale: 用户明确要求今日用量逻辑保持不变；即使选择的范围不包含今天，今日行也应仍显示今天。
   - Alternative considered: 今日也跟随范围清零或隐藏；这会改变现有页面含义。

## Risks / Trade-offs

- 日期边界和时区不一致可能导致首尾日统计偏差 -> 后端统一解析 `YYYY-MM-DD` 并转成应用时区的自然日边界。
- 用户选择自定义范围后文案可能过长 -> 用 preset 显示短标签，自定义范围显示紧凑日期区间。
- 接口调用失败时用量列可能保留旧范围数据 -> 切换范围重新加载时应清理或覆盖当前 stats，并沿用现有错误处理。
