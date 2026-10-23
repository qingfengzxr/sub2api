# Embeddings 验证计划

业务实现与验证已完成；实际结果见 [implementation.md](implementation.md)。下面保留测试矩阵与复跑命令。OpenSpec 校验只证明产物结构合法，不能代替功能测试。

## 测试矩阵

复用 testify、现有 HTTPUpstream recorder、`httptest.Server`、现有调度器/账单 stubs。服务单元测试证明透传；至少一组 Handler 到真实 mock HTTP 的测试证明最终响应与结算，不能只 stub `ForwardEmbeddings`。

| 场景 | 层级 | 核心断言 |
| --- | --- | --- |
| 单条/批量文本，三个目标模型 | Service/Handler | object/data/model/usage 完整，响应与 mock 字节一致 |
| dimensions=512，两条 input | HTTP mock | 每条恰好 512 维，index 与顺序不变 |
| float/base64/user/扩展字段 | Service | 原样透传、base64 字符串不重编码 |
| integer[] / integer[][] | Service | token 序列与类型保持不变 |
| 缺 model、空 input、非对象、错误类型 | Handler | 400、零上游请求、无输入日志 |
| dimensions/encoding_format/ada 限制 | Handler/Service | 本地可判错的请求只返回一次 400，映射别名使用真实模型校验 |
| 上游 400 token 超限 | Handler + mock | 不切号，客户端收到对应错误 |
| API Key / OAuth-only / 混合池 | 调度器 + Handler | 两种调度模式只选 OpenAI API Key，OAuth-only 零上游请求 |
| 其他平台、停用、限流、能力关闭、无模型权限 | 调度器/Service | 调度排除、Service 防御；sticky 与排除集不可绕过 |
| 分组白名单与复合分组 | 路由 | 原有权限优先，复合目标只能 OpenAI |
| URL 各种后缀、安全校验 | URL/Service | 无重复路径、无意外 fallback、未扩大 URL 访问权限 |
| Proxy/headers/API Key | HTTP mock | 调用参数与过滤规则正确，不传客户端网关密钥当上游密钥 |
| 429、401/403、500/502、model_not_found | Handler + mock | 按现有策略切号、错误状态处理、上限停止、失败不计成功账 |
| DNS/TLS/连接/代理错误 | Service/Handler | 公共分类器与账户隔离生效，最终只写一次响应 |
| cancel/deadline，header/body 卡住 | HTTP mock + race | 有界退出，零额外重试，槽位/预留归还 |
| 2xx body 读失败、响应超限 | Service/Handler | 一次 502、不重发、不记录不完整正常账单 |
| Service 已写错误或只提交 header | Handler | 不追加 JSON、不切号 |
| 有效 prompt_tokens，带伪 output/cache 字段 | Usage | 官方模型 input 精确，output/cache/image 为零 |
| 缺 Usage、缺 prompt_tokens、null/负数/小数/字符串 | Usage/Ops | 有可检索异常、告警累计，零可确认用量仅记录一次 |
| 已知 Usage，客户端响应写失败 | Handler/计费任务 | 仍结算一次，失败日志不含向量 |
| 三模型标准价/动态覆盖/缺价 | Billing | 单 token 精确价格、无 GPT fallback、无价提前拒绝 |
| 别名 + 渠道映射 + 账户映射 | Billing | 原请求/最终上游身份可追踪，预检与最终计费解析一致 |
| 总输入超过通用长上下文阈值 | Billing | 无长上下文附加倍增，仍遵守上游请求 token 限制 |
| 余额、订阅、倍率、限额 | 账单/集成 | 成功一次扣费、Usage 与账务一致、无输出费用 |
| 默认模型/前端候选/OAuth 排除 | 后端/Vitest | 三项可配置但不扩大权限，默认测试模型不变 |
| Jina/Doubao 扩展及其他 API | 回归 | 原有扩展、Responses/Chat/Claude/Gemini 行为保持 |
| 队列满/停止/drop/sample，延迟执行任务 | Handler + worker | 复用必达回退，不丢 Embeddings 结算任务，不读已复用 Gin Context |
| 网关本地总预算与真实客户端取消 | Handler/Service | 本地 504 与客户端 499 观测区分，无错误的账户处罚/重试 |
| 价格预检后热更新 | Billing | 已确定请求的单价与模型不漂移，后续请求使用新价 |
| 显式按次价格且 Usage 缺失 | Billing/Ops | 已知一次费用沿用，token 为零可确认值并留异常；不把按次收费误删 |
| 官方上游 2xx + HTML/空 body/错误信封 | Service/Handler | 一次 502，不当作成功但缺 Usage，不自动重发 |
| HTTP 200 + 缺 Usage，监控开启/关闭 | Ops + repository stub | 开启时验证实际入队/持久化内容，关闭或失败仍有安全结构化告警 |
| 审计开启，异步/同步模式 | securityaudit/旧审核 | 保留检测及失败策略，输入不进入预览、完整提示或异步载荷 |
| 公共计费输入缺省、共享对象不变 | Billing + race | nil 专用输入沿用旧路径，不修改共享 Account/Group/价格数据 |
| 映射别名选号失败诊断 | Handler | 诊断使用 forwardModel，客户端错误仍显示原模型 |

## 模块边界验证

- 新增协议/Usage 测试使用纯函数输入输出，不为了测试实例化数据库或 Gin。
- 公共接入的测试放在 Embeddings 专用测试文件；复用现有 stubs，不搬动大型通用测试文件。
- 审查公共 diff：禁止无关格式化/搬迁、修改共享默认行为、复制费用算法和账务写入。必要审计/Ops 接入应写明原因。
- race 测试增加 Handler 返回后的队列延迟执行、共享价格快照与并发请求；不能仅证明无数据竞争而不验证账单字段正确。
- 必达计费测试断言的是进程内不因队列策略丢弃任务；不以此声称进程崩溃时也能持久化重放。

## 执行顺序与命令

后端定向基线与实现验证，在 `backend/` 执行：

```bash
go test -tags=unit ./internal/service ./internal/handler ./internal/server/routes ./internal/pkg/openai -run 'Embeddings|Embedding|OpenAIEndpoint|OpenAIAccountScheduler|SelectAccountWithScheduler|ModelAllowlist|ModelPricing|RecordUsage' -count=1
go test -race -tags=unit ./internal/service ./internal/handler -run 'Embeddings|Embedding|RecordUsage|SelectAccountWithScheduler' -count=1
go test -tags=unit ./internal/securityaudit ./internal/handler -run 'Embeddings|Embedding|SecurityAudit|Ops' -count=1
go test -tags=unit ./...
go build ./cmd/server
golangci-lint run ./...
```

测试命名应确保新增用例被上面匹配；全量 unit 是补充保障。仓库 `go.mod` 当前要求 Go 1.27.2；依赖或工具不可用时如实记录，不将环境失败当测试通过。

需要 PostgreSQL/Redis 或现有集成依赖时，使用项目已有测试环境，在 `backend/` 执行 `go test -tags=integration ./internal/repository/...`，以及实现中新增加的 Handler 集成测试。只在测试数据库执行，不自动连接生产服务。

前端在 `frontend/` 执行：

```bash
pnpm test:run src/composables/__tests__/useModelWhitelist.spec.ts src/components/account/__tests__/ModelWhitelistSelector.spec.ts src/components/account/__tests__/EditAccountModal.spec.ts src/components/account/__tests__/BulkEditAccountModal.spec.ts
pnpm typecheck
```

OpenSpec 在仓库根目录执行：

```bash
openspec validate complete-openai-embeddings --strict --no-interactive
git diff --check
```

## curl 验收

前提：本地网关运行、`SUB2API_API_KEY` 已设为该网关密钥、分组内有支持 Embeddings 的 OpenAI API Key 账户且价格/余额或订阅可用。下列命令在实现后的验证环境执行，会调用配置的上游。

```bash
curl --fail-with-body --silent --show-error \
  'http://localhost:8080/v1/embeddings' \
  -H "Authorization: Bearer ${SUB2API_API_KEY}" \
  -H 'Content-Type: application/json' \
  -d '{"model":"text-embedding-3-small","input":["Hello world","你好世界"],"dimensions":512,"encoding_format":"float"}'
```

验收除响应外，检查后台只产生一次 Usage：所选账户为 OpenAI/API Key，输入等于 prompt_tokens、输出为 0，费用采用 small 的价格及现有倍率，余额/订阅累计一致。没有实际上游配置时，只能报告 mock 验收通过，不能报告真实 OpenAI 验收通过。

## 实施阶段实际验证

2026-10-13 指定 Review 修复的追加验证：

- 先运行 `go test -tags=unit ./internal/service -run '^TestEmbeddingsReviewRegression' -count=1`，确认价格缺失、批量审核、并发限额与 LastError 回显的失败，再实现修复。
- `go test -tags=unit ./internal/service -run 'EmbeddingsReviewRegression|EmbeddingsResponseEnvelopeTypes|EmbeddingsBatchKeywordBlocking|EmbeddingsObservationReleases|OtherProtocolsRetainModeration' -bench '^BenchmarkEmbeddingsResponseValidation$' -benchtime=3x -count=1`：通过；约 12 MB 响应的额外分配为 34 B/op，修复前约 12 MB/op。
- `go test -race -tags=unit ./internal/service ./internal/handler ./internal/securityaudit -run 'Embeddings|Embedding|OtherProtocolsRetainModeration|CoordinatorModes' -count=1 -timeout=3m`：三个包均通过。
- `go build -o /tmp/sub2api-embeddings-fixed-server ./cmd/server` 与 `go vet -tags=unit ./internal/service ./internal/handler ./internal/securityaudit`：通过。
- `go test -tags=unit ./internal/service ./internal/handler ./internal/securityaudit ./internal/server/routes -count=1 -timeout=5m`：四个包完整回归均通过（Service 约 209 秒，Handler 约 41 秒）。
- 本轮未改前端或数据库，未重复前端/数据库集成与真实上游验收；Review 第 2 项仍未修复。

Go 检查使用 `GOCACHE=/tmp/sub2api-embeddings-go-build`，避免默认缓存目录只读限制；本机 httptest 和 Docker 检查在允许监听端口的环境执行。

| 检查 | 实际结果 |
| --- | --- |
| `go test -race -tags=unit ./internal/service ./internal/handler ./internal/securityaudit ./internal/pkg/openai -run 'Embeddings\|Embedding\|SelectAccountWithScheduler\|SubmitMandatoryUsage\|CoordinatorModes' -count=1 -timeout=3m` | 定向 race 通过 |
| `go test -tags=unit ./... -timeout=5m` | Service 等通过；首轮 Ops 新测试断言已修正并重跑；管理设置 2 个快照失败已在原 HEAD 复现 |
| `go test -tags=unit ./internal/handler -count=1 -timeout=3m` | 完整 Handler 重跑通过 |
| `go test -json -tags=integration ./internal/repository -run 'UsageBilling\|UsageLog\|GatewayRouting\|APIKey' -count=1 -timeout=5m` | 189 通过条目（含子测试），0 跳过 |
| `go build -o /tmp/sub2api-embeddings-server ./cmd/server` | 通过 |
| `go vet -tags=unit ./internal/service ./internal/handler ./internal/securityaudit ./internal/pkg/openai` | 通过 |
| 前端原 4 文件加 `useModelWhitelist.embeddings.spec.ts` | 5 文件、163 测试通过 |
| `pnpm typecheck` | 通过 |
| `golangci-lint run ./...` | 工具未安装，未运行 |

原 HEAD 的接口快照复现命令：在临时只读来源副本的 `backend` 内执行 `go test -tags=unit ./internal/server -run '^TestAPIContracts$' -count=1`。失败均为 `/api/v1/admin/settings` 两个场景，缺少 `billing_token_multiplier`、`token_multiplier_billing_enabled`、`long_context_pricing_enabled`、`long_context_pricing_threshold_tokens` 四个原已返回的字段。

## 规划阶段核查记录

- 已读取用户附件、当前路由/Handler/Service/调度/计费/模型配置与相关测试。
- 已下载并阅读 PR #945 实际 diff，未导入其代码。
- 已查询并打开官方 Embeddings API、指南与三个模型页面。
- 已完成第二轮方案复审，核对工作池、异步请求字段、价格解析、超时分类、成功响应 Ops 与安全审计；修订结论见 [review.md](review.md)。
- `openspec validate complete-openai-embeddings --strict --no-interactive`：通过。
- `openspec status --change complete-openai-embeddings --json`：proposal/design/specs/tasks 四项规划产物均完成；不代表其中实现任务已完成。
- 规划阶段仅静态核查；随后已执行以上开发验证。真实 OpenAI 请求和部署仍未执行。
