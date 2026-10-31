## Context

本文记录先规划、复审后经用户授权实施的设计。实际交付与验证见 [implementation.md](implementation.md)。基线为 `8db8d322e`，调研日期为 2026-10-12；调研开始时工作树无改动。

本方案已根据用户的模块化与低合并成本要求复审；[review.md](review.md) 记录发现的问题。以下为修订后的设计，以这些明确的接入边界指导实现。

### 代码现状与证据

下列路径均相对于仓库根目录；“已存在”表示已静态核对，不等于本次已运行测试。

| 分类 | 结论 | 代码证据与处理 |
| --- | --- | --- |
| 已存在 | 两个入口都有平台门控与请求体限制 | `backend/internal/server/routes/gateway.go`：`/v1/embeddings`、`/embeddings`；复合分组先解析目标平台 |
| 已存在 | 选号使用现有能力调度，OAuth 已被排除 | `handler/openai_embeddings.go` 调用 `SelectAccountWithSchedulerForCapability`；`service/account.go: SupportsOpenAIEndpointCapability` 要求 API Key；不要误报“当前 HTTP 请求必然选中 OAuth” |
| 已存在 | 模型权限、可调度状态、并发与切号 | `service/openai_gateway_scheduling.go`、`openai_account_scheduler.go` 与 Handler 的用户槽/账户槽、失败排除集 |
| 已存在 | 自定义 URL 防重复拼接 | `service/openai_endpoint_url.go`；`openai_embeddings_test.go` 已覆盖域名、`/v1`、完整端点和第三方版本路径 |
| 已存在 | 代理、共享 HTTP、响应头过滤和读限额 | `service/openai_embeddings.go`、`openai_plugin_transport.go`、`upstream_response_limit.go`；当前默认读限额为 128 MiB，沿用配置 |
| 已存在 | 服务已写响应后有 Handler 保护 | Handler 比较 Writer Size，成功/上游错误写入函数检查 Written；仍需空 body、仅写 header、读取失败的端到端证明 |
| 已确认缺口 | 能力检查可接受部分其他兼容平台，Service 无显式平台/能力防御 | `GetOpenAIProtocolAPIKey` 覆盖多协议供应商；外层路由当前阻止普通非 OpenAI 流量，问题在绕过路由的调用边界 |
| 已确认缺陷 | 非流式请求取消和 deadline 被剥离 | `ForwardEmbeddings` 调用 `detachUpstreamContext`，后者使用 `context.WithoutCancel` |
| 已确认缺口 | 网络失败直接写 502，未使用现有公共传输错误切号策略 | `ForwardEmbeddings` 的 `doOpenAIUpstream` 错误分支；已有 `handleOpenAIUpstreamTransportError` 可以复用 |
| 已确认缺陷 | Usage 可以接受输出 Token、缓存 Token，并将缺失 Usage 当成全零成功 | `extractOpenAIEmbeddingsUsage`；缺少 Embeddings 专用完整性信号 |
| 已确认缺口 | 三个官方模型不在当前默认目录、前端默认候选或内置价格 fallback | `backend/internal/pkg/openai/constants.go`、`frontend/src/composables/useModelWhitelist.ts`、`service/billing_service.go`；运行时外部价格文件可能有模型，不应声称线上一定缺价 |
| 已确认缺陷 | 无效 JSON 日志可能包含输入内容 | Handler 调用 `logRequestBodyParseFailure`，该函数记录 body 头尾片段，仅转义、不脱敏 |
| 待验证风险 | 上游错误回显、Ops 和安全审计链可能保存输入 | `Gateway.LogUpstreamErrorBody`、`appendOpsUpstreamError`、`checkSecurityAudit`；需审查实际持久化字段与日志开关 |
| 待验证风险 | 映射计价、无价候选回退、长上下文和预留金额 | `openai_gateway_usage.go`、`billing_inflight_reservation.go`；通用 RecordUsage 会遍历模型候选，最终缺价为零费用加告警，不能把这当作严格预检 |

已有 `openai_embeddings_test.go` 主要覆盖批量 float 透传/映射、URL、403 access-state 和 429 sentinel；调度器测试覆盖两个模式和 sticky 能力过滤。未发现专用 Embeddings Handler 测试，附件要求的完整矩阵仍需补齐。

### PR #945 对照

已读取 [PR #945](https://github.com/Wei-Shaw/sub2api/pull/945) 和其 633 行 [diff](https://github.com/Wei-Shaw/sub2api/pull/945.diff)。当前本地历史可见 `ccace69d4 Add OpenAI embeddings gateway`；没有据此断言 PR #945 原提交已合并。

| PR 内容 | 当前情况 | 决策 |
| --- | --- | --- |
| 新增路由、Handler 与原生透传 Service | 已有独立实现，且增加了能力调度、并发、利润检查、渠道映射等 | 保留当前链路，不覆盖大文件、不整包 cherry-pick |
| 通用 `GetAccessToken` 取凭据 | 当前使用协议 API Key getter，可避免直接把 OAuth token 当 API Key | 增加显式 OpenAI/API Key/能力门控，不恢复旧取 token 方法 |
| 字符串拼接 `base + /v1/embeddings` | 当前共享 URL helper 已解决重复版本/端点 | 保留并补测试 |
| 只取 prompt_tokens、output=0 | 当前实现扩展到其他兼容 Embedding 模型，包括图片输入 | 三个官方模型恢复严格计量，保留现有其他模型的明确扩展，避免破坏 Doubao/Jina 兼容测试 |
| 增加默认模型和前端候选 | 当前候选缺失这三项 | 选择性补回，并验证 OAuth 模型清单不被错误扩展 |
| Gemini 原生 Embedding、Gemini 模型列表与计费 | 超出本次范围 | 不移植 |
| Handler 计费错误空处理、无专用自动化测试 | 当前已记录计费错误，且有部分测试 | 使用当前工作池和日志，补验收矩阵 |

### 官方协议基线

模型名为 `text-embedding-3-small`，不是 `test-embedding-3-small`。

| 模型 | 默认向量维度 | dimensions | 标准同步输入价（USD / 1M tokens） |
| --- | --- | --- | --- |
| text-embedding-3-small | 1536 | 支持缩短，包括 512 | 0.02 |
| text-embedding-3-large | 3072 | 支持缩短，包括 512 | 0.13 |
| text-embedding-ada-002 | 1536 | 不支持 | 0.10 |

依据：[Embeddings guide](https://developers.openai.com/api/docs/guides/embeddings)、[small](https://developers.openai.com/api/docs/models/text-embedding-3-small)、[large](https://developers.openai.com/api/docs/models/text-embedding-3-large)、[ada](https://developers.openai.com/api/docs/models/text-embedding-ada-002)。价格为调研时官方标准价，仍允许现有运营定价覆盖；`input` 数组批量与异步 Batch API 不同，不能套 Batch 折扣。

[Create embeddings](https://developers.openai.com/api/reference/resources/embeddings/methods/create) 定义 `string`、`string[]`、`integer[]`、`integer[][]` 四种输入、`float`/`base64` 编码，Usage 为 prompt_tokens/total_tokens。官方列出单输入 8192 tokens、请求合计 300000 tokens；对数组限制使用了“2048 dimensions”的措辞，应区分批次数量、token 序列长度和输出向量维度，不能统一限制为 2048。网关不新增 tokenizer，不用字节长度模拟官方 token 校验，精确 token 限制由上游执行。

## Goals / Non-Goals

**Goals:** 完成附件验收：两个文本输入得到两个 512 维向量，标准响应原样返回，合法 OpenAI API Key 账户处理，按输入用量结算；正常与错误路径都有自动化测试。

**Non-Goals:** OAuth/订阅凭据调用 Embeddings、Gemini 转换、向量库/RAG、异步 Batch API、新计费系统、新依赖、全局网关重构。网关订阅计费模式仍须支持；“不支持 ChatGPT 订阅账户”不等于“不支持 sub2api 的订阅结算”。

## Decisions

### 0. 模块边界与主干合并约束

采用现有 Go package 内的专用文件组织，复用现有 Service 和依赖注入；不新增一套 EmbeddingsService、Repository、HTTP Client 或接口框架。仅仅拆文件不会减少语义耦合，因此还必须约束公共接入点和共享对象写入。

| 归属 | 职责 | 依赖边界 |
| --- | --- | --- |
| `handler/openai_embeddings.go` | HTTP 协调、一次总 deadline、并发/切号、捕获账单数据与必达提交 | 只调用现有调度、Service 和工作池；不自行解析价格或写数据库 |
| `service/openai_embeddings.go` | 鉴权防御、URL/HTTP 转发、响应分类和透传 | 保留已有转发签名，使用现有 transport/error helpers |
| 新增 `service/openai_embeddings_request.go` | 标准模型参数校验、输入形态检查 | 同包纯函数，不依赖 Gin、仓储、Redis；复用现有模型映射函数，不复制映射规则 |
| 新增 `service/openai_embeddings_usage.go` | 官方用量有效性、异常原因、严格计量 | 同包纯解析逻辑；模型标识由转发层传入，不读取请求全局状态 |
| 新增 `service/openai_embeddings_billing.go` | 三模型精确 fallback、请求级定价准备、专用计费输入约束 | 复用 `ResolvedPricing`/`CostInput` 和现有计费算式；不复制余额/订阅扣减或 Usage 落库 |
| 独立 `*_embeddings*_test.go` | 协议、Handler、计费和公共接入点契约 | 复用 stubs；避免把大量案例塞入主干持续增长的通用测试文件 |

文件名称为实施落点，不要求把几行简单逻辑再拆成更多文件。已有方法留在原处，不为满足“模块化”制造移动式大 diff。

预期公共接入点：`account.go` 的 Embeddings 分支；`billing_service.go` 的 fallback 初始化/精确查找调用；`openai_gateway_usage.go` 的可选请求级 Embeddings 计费输入和一次准备调用；前后端各一个默认模型候选位置。专用计费输入默认 nil，其他调用者保持原路径，不要求批量修改所有 RecordUsage 调用点。新增三项常量无需建立全局模型注册中心或依赖 `init()` 隐式注册。

如安全审计或 Ops 必须新增接入，使用端点/协议限定的局部 helper 和最小调用点，列入实现 diff 清单，不能以“少冲突”为由省略隐私保护。新函数不得修改共享 Account/Group/定价缓存来关闭计费选项；对本请求的解析结果按需复制。禁止用隐式 context value 携带核心计费决策，避免工作池包装 context 后意外丢失策略。

合并维护要求：实现按“协议和转发”“计费接入”“目录与文档”划分可独立审查的改动，公共接入点都有独立契约测试。无关格式化、公共大文件搬迁、修改共享方法默认行为均不在范围内；以后升级主干优先复跑这些契约测试。无法保证完全无冲突，但应把需人工核对的变化集中到可枚举的少数接入点。

### 1. 保持单一现有处理链，双层账户限制

`鉴权/分组准入 → 参数检查 → 用户并发/余额检查 → 现有 capability 调度 → 账户并发/最终检查 → 转发 → 原始响应 → 现有异步结算`。

在 `Account.SupportsOpenAIEndpointCapability(Embeddings)` 仅对这一能力要求 `PlatformOpenAI + AccountTypeAPIKey`，继续尊重显式能力集。两种调度模式、排除集、模型权限和 sticky 都沿用现有路径；禁止先选 OAuth 再通过失败消耗重试次数。

Service 最前面检查 nil、平台、类型、Embeddings 能力和请求模型权限，之后才读取凭据/构造 URL。模型权限检查使用与调用方相同的“渠道映射后、账户映射前”口径，避免用上游模型名反查别名白名单。账号动态状态由现有调度与账户槽终检负责，避免另建缓存。无合适账户：模型不被任何账户允许时沿用 404 model_not_found；模型存在但仅有 OAuth/容量不足时 503，消息明确要求可用的 OpenAI API Key Embeddings 账户。

诊断传入选号使用的 `forwardModel` 和对外展示的原请求模型，不能继续把二者都传为 reqModel。503 不应无依据断言“仅有 OAuth”，只提示所需的可用账户条件；测试区分无模型支持、无能力和无容量。

API Key 账户的自定义 Base URL 仍支持兼容供应商，这属于管理员显式配置，不以域名或 Key 字符串格式猜测供应商真实性。禁止无效 URL 静默回退默认官方地址。普通跨平台分组继续按已有路由拒绝，复合分组只允许解析到 OpenAI 的目标。

### 2. 原始 body 透传，只做必要校验

JSON 必须是对象，model 必须为非空字符串，input 必须存在。三个目标模型及映射到它们的别名验证四种输入类型、非空值、整数 token 元素；dimensions 为合法正整数且不超已知模型默认维度，ada 不接受该参数；encoding_format 仅允许 float/base64，user 为字符串。按映射后有效模型判断模型相关限制。

未知扩展参数保留；现有其他供应商 Embedding 扩展格式不因本次三模型验证被误伤。失败返回 OpenAI 风格 400，不进行无意义切号；账户导致的 model_not_found 则继续走现有可重试分类。

没有模型映射时请求字节保持不变；有映射时只替换 model。响应不反序列化后重建向量，不排序、不改 index、不做 base64 编解码或截断向量，不重写响应 model。Usage 用小字段读取，避免额外复制整组向量。

校验时序固定为：Handler 校验通用信封；选中账户后复用映射函数确定有效模型，执行该模型的参数检查和定价准备，再发送 HTTP。不能在选号前按别名猜 dimensions 上限；也不能在不同文件各复制一份映射决策。官方模型成功响应需要合法 JSON 和基本 Embeddings 信封（object=list、data 数组、非空 model）；HTML、空 204 或非成功状态不能被归类为“成功但缺 Usage”。信封失败按上游协议错误返回一次 502，不做已成功请求的自动重发。

### 3. HTTP 生命周期与最终响应约定

保留 `validateUpstreamBaseURL`、`GetOpenAIFormatBaseURL`、`buildOpenAIEmbeddingsURL`、`doOpenAIUpstream` 和公共响应头过滤。URL 测试覆盖域名、`/v1`、完整端点、末尾 `/`、版本前缀和已有 query；由当前安全策略拒绝非法地址，不放宽 SSRF 规则。

Embeddings 是非流式：使用请求原 context 或 `detachStreamUpstreamContext(ctx, false)`，完整保留 cancel/deadline，不改动其他端点依赖的 detach helper。复用现有建连/TLS/header timeout；这些不是整个 body 读取的总时限，且 OpenAI profile 可以关闭 header timeout。计划在该 Handler 的资源获取前设置局部 5 分钟总预算，覆盖排队、全部重试和响应读取，继承更短的已有 deadline；实现若发现已有等价且更严格的入口预算则复用。该常量采用保守上限，先不增加配置项或给每次重试重置预算。计费任务继续使用现有独立 context，避免请求超时造成已知用量丢账。

| 情况 | 行为 |
| --- | --- |
| 客户端参数错误或普通上游 400 | 一次 400，不切号；保留安全的上游错误内容 |
| 401/402/403/429、500/502 等 | 复用 `shouldFailoverOpenAIUpstreamResponse` 和账户错误状态处理，不自建状态码表；权限/账户失效的 typed failover 保持语义 |
| 明确账户侧 model_not_found | 保留当前网关的模型可用性切号例外，不把所有 400 都判为参数错误 |
| DNS/代理/连接失败 | 调用 `handleOpenAIUpstreamTransportError`；handler 切换账户，持久故障沿用临时隔离 |
| 客户端主动取消 | 停止处理和切号，不惩罚账户；沿用日志中的 499 约定，释放槽位和预留 |
| 网关总预算耗尽且客户端仍连接 | 不继续切号、不惩罚账户；未提交响应时返回一次 504，不能借公共 client-gone helper 误标为 499 |
| 2xx 后读取失败或超限 | 未写响应时只返回一次 502，记录安全元数据，不再次调用上游；已写响应则停止追加 |
| 已完整获得成功响应但客户端写失败 | 仍提交一次已有真实 Usage 的结算，不能因客户端断开丢账 |

Service 返回 failover sentinel 时必须未提交响应；普通错误由 Service 已写或 Handler 兜底，但最多一个最终响应。优先使用现有 Written/Size 约定和回归测试修正局部边界，不新建全网关返回协议。继续限次切号，失败尝试不提交成功用量；不能承诺网络歧义下上游绝不重复执行。

公共传输错误 helper 只特殊处理 `context.Canceled`；调用它之前，专用路径必须先区分本地总预算耗尽与独立的上游连接超时。Handler 保留原客户端 context，局部预算用 child context，错误检查顺序不能让 child deadline 被 `failoverClientGone` 当作客户端断开。

### 4. 用量、定价与缺失数据策略

针对有效上游为三个官方模型的请求：`InputTokens = usage.prompt_tokens`，`OutputTokens = 0`，缓存/图片拆分为零。prompt_tokens 必须存在、为非负整数且可表示；与缺字段、null、字符串、负数、小数区分。不得用 total_tokens 或 output_tokens 替代。非空成功输入的零用量需要异常观测。

保留非目标模型现有多模态/兼容 Usage 行为，不全局删除图片计量；用显式目标模型判断或等价最小局部方式隔离。响应 model 原样返回；账单保留 requested、mapped、upstream 等现有身份字段。

默认计价使用实际映射后的 Embedding 模型。现有运营者显式渠道价格、billing_model_source、用户/分组倍率、订阅额度、API Key 限额与 pricingAt 继续生效，但不能因 Embedding 未定价而悄悄使用 GPT/Claude 候选。别名无显式价格时必须解析到真实 Embedding 价格。复用现有价格解析器，在上游调用前核实可用价格；缺价返回明确的 503 `model_pricing_unavailable` 并记录配置异常，零上游请求。不同账户映射到不同模型时，在每次实际发送前按选中账户重新解析。

此严格策略仅针对三个有效官方模型及映射到它们的别名；现有其他供应商 Embeddings 不因本次引入新缺价准入而改变行为。精确价格准入复用 `HasIdentifiedTokenPricing`/`GetIdentifiedModelPricing` 的语义，不能只验证通用 `GetModelPricing` 是否返回非 nil。alias 的子串即使命中 GPT/Claude 系列也不算有效定价来源；运营者显式配置的 alias 价格仍有效。

定价准备在 `openai_embeddings_billing.go` 完成，将所选模型、已解析价格的请求私有快照与计费选项交给现有 RecordUsage。优先复用 `ResolvedPricing` 和 `CostInput.Resolved`；只给 `OpenAIRecordUsageInput` 增加一个可选的专用输入，默认 nil 保持全部现有路径。公共 RecordUsage 仅接收该输入并沿用统一费用计算/扣费/落库，不再对本请求重新跑宽松候选链。`pricingAt` 只固定时刻相关因子，不能固定动态单价，不能把它当作价格快照。显式 response_model 计价也必须经专用价格准入，禁止重新回到对话模型兜底。

三项内置 fallback 单 token 输入价分别为 `0.00000002`、`0.00000013`、`0.00000010` USD，输出价零，不配置长上下文档位。目录/运营显式覆盖优先于 fallback。通过本端点计费上下文显式关闭长上下文倍增，防止批量合计超过通用阈值触发误计；不修改对话端点语义。余额预留是估算，最终必须使用实际 prompt_tokens，不能将 Handler 的 `MaxTokens: 1` 当成输出用量。

成功但 Usage 缺失/非法的默认方案：

1. 保留成功的向量响应，不因为上游已成功而自动重发，也不根据输入字节数估算真实费用。
2. 复用 `logOpenAISuccessMissingUsage` 的采样/累计机制，并通过现有 Ops 记录安全的 `embedding_usage_unavailable` 异常元数据，包含账户、模型、上游 request ID 和字段异常原因；不保存 input/vector/body。须实际验证 200 响应的异常经过 `logOpsRecoveredUpstream` 落库，不能只断言 Gin Context 设置成功。Ops 关闭或队列丢弃时仍有独立结构化告警；不得改变用户 HTTP 状态来强行触发错误日志。
3. 走现有一次 Usage 记录流程，以零可确认 Token 留痕；token 模式费用为零，不把这一行解释为“已准确结算”。若运营者显式配置 per_request，则保留已知的成功调用一次费用，这不是估算 Token。Ops 异常用于识别和人工核查。
4. 若上游 request ID 可用于运营核查，可据此对账；不在本任务新增自动补账队列或数据库状态。

这是沿用项目现有 OpenAI 成功响应的可用性优先策略，满足“不静默”的要求，**不保证缺 Usage 时仍能准确扣费**。严格拒绝缺 Usage 的产品策略需要另行调整契约；直接返回 502 同样不能消除已经发生的上游费用。

该端点在 Handler 内直接复用 `submitMandatoryUsageRecordTask`，无需修改公共工作池或 `submitOpenAIUsageRecordTask` 的全局分类。后者的普通文本分支在 drop/sample 配置下可能丢任务，不满足本需求。异步闭包提交前计算 `clientRequestedUsageFields` 并组装完整结算输入，不捕获 Gin Context；仅传稳定值/请求私有对象，计费任务持有预留引用直到处理完成。必达方法提供进程内不因队列溢出丢弃的保障，不等同于跨进程崩溃的持久化消息或端到端 exactly-once；数据库结算仍沿用已有幂等标识。

### 5. 模型目录和隐私

在 `backend/internal/pkg/openai/constants.go` 与 `frontend/src/composables/useModelWhitelist.ts` 追加三项候选，保留首选账号测试模型和顺序约定。这里只有三项静态数据，直接小 diff 比新建目录注册框架更易合并；不要移动原列表或拆分全部平台。检查 `gateway_handler.go` 模型展示、`account_test_service.go`、前端创建/编辑/批量编辑、白名单与固定账户模型聚合的调用者。候选显示不自动修改已有 model_mapping/allowlist，不把 Embeddings 加入 OAuth 可服务模型列表。

Embeddings JSON 解析失败只记长度/结构错误位置，不调用会记录头尾文本的诊断路径。上游错误在内存中用于分类和客户端响应，持久化诊断只记录状态、错误 code、请求 ID 等；禁用该端点原始错误 body 落日志，防止上游把 input 回显进去。检查安全审计/运维中间件，不通过移除安全检查来规避日志问题；需要内容检测时可内存处理，但不得持久化输入正文。

复审确认 `securityaudit/prompt_snapshot.go` 对未单独列出的协议会尝试提取 input，快照含 FullPrompt/RedactedPreview/ScanText，异步审计会复制请求入队。不能假设仅不写转发日志就满足隐私契约。实现须为 `openai_embeddings` 增加协议限定的内容保留策略，阻断输入正文进入异步存储/审计记录；如需保留检测，使用现有同步检测路径及仅元数据记录，不将空 body 当作“检测通过”。这是必需安全接入，不扩展成全平台审计改造；覆盖旧内容审核与新审计两条路径。

### 改动范围

| 文件/模块 | 计划改动 |
| --- | --- |
| `backend/internal/handler/openai_embeddings.go` | 输入协调、价格检查调用、取消/单响应约定、安全诊断与一次结算 |
| `backend/internal/service/openai_embeddings.go` | 转发防御、context、公共网络错误处理、目标模型 Usage 与缺失告警 |
| `backend/internal/service/account.go` | 仅收紧 Embeddings capability 的平台判断 |
| 新增 `backend/internal/service/openai_embeddings_{request,usage,billing}.go` | 协议/用量纯逻辑、定价准备与策略的主要落点；保持同包调用 |
| `backend/internal/service/openai_gateway_usage.go`、`billing_service.go` | 仅增加可选计费输入接入和 fallback 注册/查找调用，详细逻辑放专用文件；不移动公共代码 |
| `backend/internal/securityaudit/`、`handler/ops_error_logger.go` | 按已确认内容保留风险加入端点限定策略接入；独立 helper 与测试，不改其他协议行为 |
| `backend/internal/pkg/openai/constants.go`、`frontend/src/composables/useModelWhitelist.ts` | 三项模型候选 |
| 各模块现有 `*_test.go`、新增 Handler Embeddings 测试 | 覆盖隔离、协议、错误与结算行为 |
| `backend/internal/server/routes/gateway.go`、`openai_endpoint_url.go`、`repository/http_upstream.go` | 以回归验证为主，只有证实缺陷才修改 |
| `docs/embedding-api.md` | 实现完成后写配置、异常策略和验收说明 |

## Risks / Trade-offs

- 能力过滤比现有通用兼容范围更窄 → 只改 Embeddings 能力，检查内部调用者；正常路由已限制平台，OpenAI 平台的自定义 Base URL 保留。
- 目标模型专用校验误伤已有第三方多模态 Embeddings → 以有效模型识别严格分支，并保留现有 Jina/Doubao 测试。
- 动态价格文件未提供 → 内置三项精确 fallback；缺价检查不使用通用“任意可计价候选”兜底。
- 价格预检与异步结算之间价格更新 → 对三个官方模型传递请求私有的已解析价格，不把 pricingAt 当成冻结单价；使用已有费用计算与账务幂等，不另建快照数据库。
- 公共接入点仍可能与主干发生冲突 → 保持少量显式调用、独立契约测试及可审查的提交边界；不承诺零冲突，也不为减少 diff 复制公共算法。
- 大批量浮点向量占用内存 → 保留现有 body 上限和并发限制，不做向量反序列化、追加内存队列或自动拆批。
- 缺 Usage 或 2xx 读失败已经可能产生供应商费用 → 留安全元数据并告警，不自动重发；无法凭本地数据恢复真实 Token 的情况明确记为对账限制。

## Migration Plan

无数据库迁移。按任务顺序实现并通过本地 mock 验收后部署；先验证一个带余额/订阅配置的 OpenAI API Key 分组和三个价格，再观察缺 Usage、切号、输入费用与余额变化。回滚为撤回本次代码提交；不回写或删除历史账单。

## Open Questions

没有阻塞规划的用户选择；默认采用上述缺 Usage 可用性策略与 5 分钟请求总预算。进入实现时必须验证：审计仅元数据策略覆盖全部持久化路径、200 的 Ops 异常确实可检索、请求价格快照与 response_model 显式策略正确衔接、必达提交和结算幂等。未验证项不能在交付中声称已保障。
