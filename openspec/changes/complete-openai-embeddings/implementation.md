# 实现交付记录

基于 `cooper/dev` / `8db8d322e`，2026-10-12。业务代码与自动化测试已实现，未提交或部署。详细使用方式见 [embedding-api.md](../../../docs/embedding-api.md)。

## 实现与合并接入点

| 模块 | 实际变更 |
| --- | --- |
| `service/openai_embeddings_request.go` | 三模型识别、四类 input 和可选参数验证；非目标模型保留扩展 |
| `service/openai_embeddings_usage.go` | 严格 prompt_tokens 解析和基本成功信封验证 |
| `service/openai_embeddings_billing.go` | 精确 fallback、有效模型/显式别名定价、请求价格快照、关闭长上下文、统一费用计算 |
| `service/openai_embeddings_privacy.go` | endpoint 识别、Ops 正文清理、仅用于诊断脱敏的 context 标志 |
| `service/openai_embeddings.go` | 入口防御、取消传播、网络 Failover、读取错误、缺 Usage 事件、错误响应头保留 |
| `handler/openai_embeddings.go` | 5 分钟预算、单响应/504/取消分类、使用账户终检后的快照、准确模型诊断、必达计费提交与数据捕获 |
| `service/account.go` | 仅收紧 Embeddings capability 的平台条件 |
| `service/billing_service.go` | fallback 初始化与三模型精确查找接入，无通用算法复制 |
| `service/openai_gateway_usage.go` | 一个可选 EmbeddingsBilling 输入及局部算价分支；复用账务写入/幂等，nil 输入保持其他调用者旧行为 |
| `handler/ops_error_logger.go` | 入持久化队列前调用端点限定的脱敏方法 |
| `service/ratelimit_service.go`、`openai_upstream_transport_error.go` | 分类仍使用原始上游错误，日志和账户错误状态写入时对 Embeddings 隐去正文，避免破坏自定义匹配规则 |
| `securityaudit/embeddings_privacy.go` 及既有 Coordinator/PromptService/Guard/Snapshot 的少量调用点 | 观察模式内存检测、不入正文队列；保留检测结果，落库去掉 FullPrompt、预览和 ScannerEvidence |
| `service/content_moderation.go` | 旧观察审核路径同样内存检测，日志去掉正文与错误回显 |
| 后端 `pkg/openai/constants.go`、前端 `useModelWhitelist.ts` | 各追加三项候选，保留原顺序和权限语义 |
| 专用 Embeddings 测试文件 | 协议、双调度模式混合池、HTTP mock、计费、隐私、队列溢出/Context 复用、账务与并发回归 |

公共接入主要是调用、守卫或一个可选输入；没有新增数据库字段、HTTP Client、调度器、Repository 接口或第三方依赖。安全诊断标志只用于移除正文，核心计费决策通过显式结构传递。没有为减少冲突而复制公共账务逻辑。

额外修复了一处基线测试编译错误：`billing_service_test.go` 的旧 `computeTokenBreakdown(..., false)` 改为 `LongContextPricingPolicy{}`，原费用断言保留。`openai_embeddings_test.go` 的既有 Failover 测试改为检查响应头保留，不再要求丢弃响应头。

## 验证结论

- 后端 Embeddings 定向测试与 race 检查通过，包含真实本机 HTTP mock 的两条 512 维向量、429 后切号、OAuth 排除、单次结算、客户端取消、队列 drop/sample 回退、延迟任务及写失败结算。
- 完整 `internal/service` unit 测试通过（全量运行约 206 秒）；完整 Handler 修正后重跑通过（约 42 秒）。安全审计、路由和其他包的结果保留在全量记录中。
- PostgreSQL/Redis 集成测试选择 `UsageBilling|UsageLog|GatewayRouting|APIKey`：189 个通过条目，0 跳过（包括子测试），框架自动清理临时容器。
- 前端 5 个测试文件、163 个测试通过；`pnpm typecheck` 通过。
- 后端构建通过；受影响包 `go vet -tags=unit` 通过。
- 全量 `go test -tags=unit ./...` 未全部通过：管理设置接口两个黄金快照缺少 4 个既有计费配置字段。同一失败已在 `/tmp/sub2api-embeddings-baseline` 的原始 HEAD 副本复现；没有修改这些无关快照。
- `golangci-lint` 未安装，未运行；没有把 go vet 表述为完整 lint 的替代通过。

全量首轮还发现新增 Ops 测试应检查入队后序列化的 `UpstreamErrorsJSON`，该断言已修正，完整 Handler 和定向 race 重跑通过；未删除或弱化对异常类型和正文不落库的检查。

## 限制与未验证

- 未使用真实 OpenAI Key 请求部署环境，未部署或提交代码。真实模型权限、代理配置和运营倍率仍需部署环境验收。
- 没有真实 Usage 时不能精确算 token 费用；目前有可观测异常，无自动补账。
- 必达提交沿用现有进程内回退，进程硬崩溃或数据库长期故障的恢复能力沿用项目现状。
- 安全审计观察模式在此端点改为请求内检测，可能增加延迟；没有新增审计开关或改变其他协议。
- 主干升级仍需人工核对上述少量公共接入点；不能承诺零冲突。

## 2026-10-13：按 Review 修复 1、3、4、5、6

用户指定修复五项，Review 第 2 项保持现状，未改 `billing_model_source=requested` 的模型选择逻辑。

| 项目 | 修复与验证 |
| --- | --- |
| 1 输入价缺失被误认为免费 | 价格目录解析保留 `InputTokenPricingAbsent`；只对三个官方 Embedding 模型忽略缺输入价的目录条目，回退精确内置价。覆盖三个模型的缺字段、null、显式零价和非零价，保留运营者明确配置免费语义 |
| 3 内联观察审核无并发限制 | 新增专用 `openai_embeddings_moderation.go`，按现有 WorkerCount 对本进程的 Embeddings 内联观察审核限流；满载时不排队保留正文，沿用观察模式放行并增加 asyncDropped/安全告警。成功、错误与取消均释放容量；测试 WorkerCount=1 时八请求仅一个进入审核上游 |
| 4 批量文本未进入旧审核 | `content_moderation_input.go` 增加 Embeddings 分支，文本数组逐条进入公共归一化和审核；关键词与远端审核使用相同提取。测试首项命中关键词而末项正常仍被阻断；token 数组不被误转为文本 |
| 5 LastError 提前保存错误回显 | 在 checkSync 传递仅用于诊断的隐私标志，在 callModeration 内记录 Key 健康状态前脱敏；状态码、重试和其他协议诊断行为不变。测试上游回显输入时 LastError 不含原文 |
| 6 响应校验复制整组向量 | 查询生成的数组长度而非提取整个 data，结合来源 Index 排除对象的字面 `#` 键。约 12 MB 响应基准从约 12,001,352 B/op 降到 34 B/op（3 次迭代）；正常数组、对象、转义键、null 与其他类型均有回归断言 |

本轮先把复现测试写入 `openai_embeddings_review_regression_test.go` 并确认失败，再修复。没有新增依赖、数据库变更或审核配置项。并发保护只用于旧审核的 Embeddings 观察模式，其他协议及阻断模式保持原行为。

验证：定向回归和 Service/Handler/SecurityAudit 的定向 race 通过；后端构建、上述包 go vet 和 diff 格式检查通过。相关包完整回归结果见 `verification.md` 的本轮记录。

保留风险：第 2 项尚未修复，`billing_model_source=requested` 且请求模型映射到另一个模型时，可能仍按上游映射模型计费；不能把本轮五项修复当成全部 Review 问题关闭。真实 OpenAI 请求、部署和全量 lint 本轮未执行。
