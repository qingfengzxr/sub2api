## Why

当前分支已具备 OpenAI Embeddings 路由、转发、调度和基础测试，但仍缺少完整的账户防御检查、取消传播、严格用量语义、模型目录与离线定价保障。需要在既有实现上补齐这些边界，使 `text-embedding-3-small`、`text-embedding-3-large`、`text-embedding-ada-002` 能稳定接入现有计费体系。

本变更依据 2026-10-12 的工作树 `8db8d322e`、PR #945 实际 diff 和 OpenAI 官方文档。规划已完成并经用户授权实施；实际变更、验证结果及限制见 [implementation.md](implementation.md)，任务状态见 [tasks.md](tasks.md)。

已按用户“模块化、降低主干合并冲突”的要求复审。新增逻辑集中在 Embeddings 专用文件，公共代码仅保留有测试覆盖的少量接入点；具体问题与修订见 [review.md](review.md)。

## What Changes

- 沿用 `POST /v1/embeddings` 及已有 `/embeddings` 别名，补全标准请求、参数透传和响应完整性测试。
- 调度阶段与转发阶段都要求 `platform=openai`、`type=apikey`、支持 Embeddings 能力并满足模型权限；保留现有可调度状态、并发和模型映射机制。
- 复用共享 HTTPUpstream、URL 构造、代理、响应头过滤和大小限制，修复取消传播及网络错误未接入公共 Failover 处理的问题。
- 三个官方模型严格使用输入 Token 计费，禁用输出、缓存与长上下文附加计费；补齐精确 fallback 价格，缺价不得回退到对话模型。
- 合法成功响应缺失有效 Usage 时返回原结果，记录可检索异常并使用现有告警计数；不估算 Token 扣费、不自动重发。显式按次定价仍沿用现有已知次数收费，不能笼统把缺 Usage 等同于总费用为零。
- 在 Embeddings 入口复用已有必达计费提交方法，提交前准备好结算值；异步任务不访问 Gin Context。使用同一请求内的已解析价格，避免预检和结算分叉。
- 补齐默认模型和前端候选；模型展示不授予账户权限。
- 修正 Embeddings 相关诊断日志，避免记录输入、向量、凭据和上游回显敏感文本。
- **BREAKING（内部边界）**：拒绝直接调用 Embeddings 服务的非 OpenAI 平台账户。正常 HTTP 路由已限制 OpenAI，实施前仍需检查所有内部调用者。

## Capabilities

### New Capabilities

- `openai-embeddings`: 定义标准 Embeddings 协议、账户隔离、转发/重试、计费、模型配置与可观测性的完整验收契约。现有主 specs 中尚无该能力。

### Modified Capabilities

None. 保持现有 `billable-usage-tracks`、余额和用户费率展示契约；只对本端点设置正确的计量输入与定价边界。

## Impact

- 后端：Embeddings Handler/Service、账户能力判断、现有计费入口/价格 fallback、默认模型列表，以及对应测试。
- 前端：模型白名单候选与已有账户配置组件测试，通常不需要新增组件。
- 文档：实现后新增 `docs/embedding-api.md`，解释账户配置、价格、异常 Usage 与 curl 验收。
- 不新增依赖、数据库字段、独立调度系统或 HTTP Client；不移植 PR 中 Gemini 相关变更。
- 不搬迁公共大文件、不新增插件/策略注册框架、不复制账务写入流程；新增协议/计费测试放入独立的 Embeddings 测试文件。
- 自定义 Base URL 仍是受现有 URL 安全策略约束的 OpenAI API Key 账户配置；不强制绑定 `api.openai.com`，也不自动改变上游。
