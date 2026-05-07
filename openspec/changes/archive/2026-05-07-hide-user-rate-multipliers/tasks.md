## 1. C 端前端展示隐藏

- [x] 1.1 更新用户 API Key 页面，隐藏当前分组和分组选择器中的倍率 badge、用户专属倍率和 `x倍` 文案
- [x] 1.2 更新用户可用渠道页面，移除 `always-show-rate` 或等价调用，隐藏公开/专属分组 badge 中的倍率显示
- [x] 1.3 检查用户用量、支付和订阅页面，隐藏已有倍率字段、倍率 tooltip 或等价内部计费系数文案
- [x] 1.4 对 `GroupBadge`、`GroupOptionItem` 等共享组件做最小兼容调整，确保用户侧可隐藏倍率且管理员侧现有行为不变
- [x] 1.5 全局搜索 C 端用户页面中的 `rate_multiplier`、`image_rate_multiplier`、`userGroupRates`、`倍率`、`x` 倍率文案并清理展示入口

## 2. `/api/v1/keys` 最小响应收敛

- [x] 2.1 为普通用户 API Key 响应新增最小 mapper 或 wrapper，隐藏嵌套 `group` 中的 `rate_multiplier`、`image_rate_multiplier` 和等价倍率字段
- [x] 2.2 更新用户侧 API Key 列表接口 `/api/v1/keys` 使用隐藏倍率后的响应结构
- [x] 2.3 更新用户侧单个 API Key 详情接口使用隐藏倍率后的响应结构
- [x] 2.4 不修改现有通用/admin DTO 结构，避免影响管理员接口和后续上游同步
- [x] 2.5 添加或更新后端测试，确认 `/api/v1/keys` 不返回真实倍率，同时管理员相关接口不受影响

## 3. `/api/v1/groups/rates` 返回策略

- [x] 3.1 调整普通用户 `/api/v1/groups/rates`，返回空对象、废弃响应或拒绝访问，确保不返回真实用户专属倍率
- [x] 3.2 移除或兼容 C 端前端对 `userGroupsAPI.getUserGroupRates` 的调用，避免页面依赖专属倍率
- [x] 3.3 添加或更新测试，确认普通用户无法通过 `/api/v1/groups/rates` 获取真实倍率

## 4. 回归和文档

- [x] 4.1 回归确认管理员分组管理、用户专属倍率管理和管理员用量分析现有体验不变
- [x] 4.2 回归确认 gateway、billing、usage 计算逻辑未改变，用户请求仍按真实倍率扣费
- [x] 4.3 更新开发文档或变更说明，记录本次只做 C 端展示和两个用户接口的最小收敛
- [x] 4.4 运行相关后端测试和前端测试/build，确认 C 端不展示倍率且管理员侧正常
