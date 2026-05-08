## 1. 定位当前用量渲染

- [x] 1.1 定位渲染 C 端 Usage 表格 `TOKEN` 列的组件。
- [x] 1.2 定位已经使用 billable token 值的现有 token tooltip/helper。
- [x] 1.3 确认管理员 Usage 渲染是否复用同一组件，还是使用独立路径。

## 2. 实现 TOKEN 列计费 token 展示

- [x] 2.1 新增或复用共享 helper，计算 billable 输入、输出、缓存、图片和总 token，并支持 raw-compatible 回退。
- [x] 2.2 更新 C 端 `TOKEN` 列，使输入/输出箭头和缓存/总量行都来自 billable helper。
- [x] 2.3 确保 `TOKEN` 列和 token tooltip 使用相同的 billable token 数据来源与格式化逻辑。
- [x] 2.4 保持管理员 token 展示和 raw/billable 审计 tooltip 行为不变。

## 3. 验证行为

- [x] 3.1 为 billable token helper 添加或更新聚焦测试，覆盖没有 billable 字段的历史记录。
- [x] 3.2 验证 C 端 Usage 表格行展示的 billable token 值与 tooltip 总量和明细一致。
- [x] 3.3 验证管理员 Usage 视图仍暴露 raw usage、billable usage 和 billing multiplier 审计明细。
- [x] 3.4 运行被修改包可用的相关前端测试、类型检查或构建命令。
