## Context

API 密钥已有 5h、1d、7d 三个固定消费窗口，数据同时存在 PostgreSQL、认证快照和 Redis 限速缓存。30 天限额应作为第四个同构窗口加入，不引入第二套算法。

## Goals / Non-Goals

**Goals:**
- 在编辑密钥时配置固定 30 天 USD 限额。
- 复用现有原子累计、缓存检查、过期和重置链路。
- 保持旧客户端和现有三档窗口行为不变。

**Non-Goals:**
- 不实现自然月或逐日滑动求和。
- 不修改创建密钥界面、密钥列表展示或其他配额系统。

## Decisions

1. 新增 `rate_limit_30d`、`usage_30d`、`window_30d_start` 三列，窗口时长为 `30 * 24h`。
2. 更新 API 仅增加可选 `rate_limit_30d`；省略不修改，`0` 表示无限制。
3. PostgreSQL 原子更新和现有 Redis Lua 脚本增加第四档；不重构为动态窗口框架。
4. 认证快照携带新限额并升级版本，确保编辑后不会读取旧缓存。
5. HTTP 429 沿用现有 `rate_limit_exceeded` 客户端错误码。

## Risks / Trade-offs

- “月度”可能被理解为自然月 -> UI 明确显示“30天限额”。
- 多条存储路径可能漏字段 -> 用数据库、Redis、认证快照和前端提交测试覆盖。
- Ent 生成文件数量较多 -> 只修改 schema，生成文件保持工具输出，不手工抽象。

## Migration Plan

先执行只新增带默认值字段的 migration，再部署应用；回滚时旧版本忽略新增列。
