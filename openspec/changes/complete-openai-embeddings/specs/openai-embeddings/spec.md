## ADDED Requirements

### Requirement: 标准 Embeddings 请求与响应
系统 SHALL 通过 `POST /v1/embeddings` 和既有 `/embeddings` 别名支持三个官方文本 Embedding 模型，接受单文本、文本数组、整数 token 数组和整数 token 二维数组。系统 MUST 透传可选参数与未修改字段，并保留响应对象、向量、编码、顺序、index 和模型名称。

#### Scenario: 两条输入生成 512 维向量
- **WHEN** 合法客户端请求 `text-embedding-3-small`，input 为两条文本，dimensions 为 512，encoding_format 为 float，上游成功
- **THEN** 网关返回上游原始 JSON，其中包含两个 512 维向量、原始 index、model 和 usage

#### Scenario: 编码及 token 输入透传
- **WHEN** 请求使用 float、base64 或四种合法 input 形式，并携带 user
- **THEN** 上游收到相同参数，网关不转换 token 输入或向量编码

#### Scenario: 模型映射
- **WHEN** 客户端使用已授权别名且渠道或账户映射到官方 Embedding 模型
- **THEN** 网关只修改请求 model，按有效模型执行模型相关验证，响应 model 保持上游值，账单保留请求与上游身份

### Requirement: 参数错误在合适边界被拒绝
系统 MUST 拒绝非法 JSON、非对象请求、缺少/空 model 或 input，以及目标官方模型的非法输入类型、非整数 token、非法编码和 dimensions；官方 token 总数限制由上游准确执行，系统 SHALL NOT 以字节数代替 token 数。

#### Scenario: 本地参数错误
- **WHEN** 请求缺少 model/input，input 为空，或者官方模型的 dimensions 为零/小数/超过默认维度
- **THEN** 返回一次 OpenAI 格式 400，且不调用上游

#### Scenario: ada dimensions
- **WHEN** 有效模型是 text-embedding-ada-002 且携带 dimensions
- **THEN** 返回一次 400，且不调用上游

#### Scenario: 上游参数错误
- **WHEN** 上游返回普通 invalid_request_error 或 token 限制错误
- **THEN** 网关返回对应错误，不进行无意义的账户切换

### Requirement: 账户隔离与模型权限
系统 MUST 在调度和转发两个边界要求 OpenAI 平台的 API Key 账户。账户 SHALL 处于现有机制认可的有效可调度状态、具备 Embeddings 能力并允许请求模型；模型目录可见性 SHALL NOT 授予调用权限。

#### Scenario: 混合账户池
- **WHEN** 分组包含 OAuth、其他平台、已禁用、模型不匹配和合法 OpenAI API Key 账户
- **THEN** 两种调度模式都只选择合法候选，失败排除与 sticky 路径也不能绕过条件

#### Scenario: 没有可用 API Key 账户
- **WHEN** 模型已允许但只有 OAuth 或临时不可调度的账户
- **THEN** 返回明确的 503，不发出上游请求

#### Scenario: 模型无权限
- **WHEN** 请求模型未被分组/账户权限允许
- **THEN** 按现有分组准入或 model_not_found 规则拒绝，不因新增默认模型而绕过权限

#### Scenario: 绕过 Handler 的调用
- **WHEN** 内部调用者向 ForwardEmbeddings 传入 nil、OAuth、其他平台或关闭 Embeddings 能力的账户
- **THEN** 服务在构造上游请求前拒绝，不读取 OAuth token 或发送 HTTP 请求

### Requirement: 转发使用共享传输与资源控制
系统 SHALL 使用现有 HTTPUpstream、代理、安全 URL 校验、并发控制、响应头过滤和响应大小限制，保留请求 cancel/deadline，并以不超过 5 分钟的请求总预算覆盖排队、重试与 body 读取；已有更短 deadline 时 SHALL 优先使用更短预算。系统 MUST 使用配置的上游地址且不得重复拼接版本或端点。

#### Scenario: 自定义 URL
- **WHEN** Base URL 为域名、带版本前缀、完整 embeddings 端点或带末尾斜线
- **THEN** URL 构造结果唯一且正确，不出现 v1/v1 或 embeddings/embeddings

#### Scenario: 无效上游地址
- **WHEN** 管理员配置的 Base URL 不符合现有安全规则
- **THEN** 请求被拒绝，不回退访问其他上游

#### Scenario: 请求取消和超时
- **WHEN** 客户端取消、请求 deadline 到期或共享传输发生超时
- **THEN** 请求及时结束，资源和槽位释放；客户端已结束时不继续切号或惩罚账户

#### Scenario: 网关本地总预算耗尽
- **WHEN** 客户端仍保持连接，但网关 5 分钟局部总预算耗尽
- **THEN** 未提交响应时返回一次 504，不误记为客户端主动取消，不惩罚账户或开始下一次重试

#### Scenario: 响应超限或读取失败
- **WHEN** 上游已返回 2xx 但 body 超限或读取失败
- **THEN** 未提交响应时只返回一次 502，记录安全异常元数据，不自动重发或伪造成功用量

### Requirement: Failover 与最终响应唯一性
系统 SHALL 复用现有上游状态/传输错误分类、账户状态处理和切换上限；可切换错误 MUST 在尚未提交客户端响应时交给 Handler。系统 MUST 对每个请求至多提交一个最终 HTTP 响应和一次成功用量结算。

#### Scenario: 可恢复的上游错误
- **WHEN** 上游返回符合现有策略的 401/403/429/500/502、账户侧 model_not_found 或网络连接错误
- **THEN** 更新相应状态并在上限内切换合法账户；最终仅成功请求计量一次

#### Scenario: 上游错误已写入
- **WHEN** Service 已提交非重试错误，包含空 body 或仅已提交 header 的情况
- **THEN** Handler 不追加第二个 JSON 错误或再次调用上游

#### Scenario: 已知 Usage 后客户端断开
- **WHEN** 网关已完整获取成功响应和有效 Usage，但客户端写入失败
- **THEN** 系统仍使用独立计费任务记录并结算一次真实用量

#### Scenario: 计费队列满或请求对象复用
- **WHEN** Embeddings 计费任务入队遇到满队列/停止，或任务在 Handler 返回后才执行
- **THEN** 系统复用现有必达提交的同步回退，不因 drop/sample 丢弃任务
- **THEN** 任务只使用提交前捕获的账单数据，不读取已回收的 Gin Context；账务沿用现有幂等保障

### Requirement: 官方 Embedding 只按输入 Token 计费
系统 MUST 对三个目标模型以 `usage.prompt_tokens` 作为输入 Token，输出和缓存计量为零，且不启用长上下文倍增。系统 SHALL 保留余额、订阅、API Key 限额、渠道显式定价和用户/分组倍率语义。

上述输入 Token 计价为标准默认行为；系统 SHALL 保留运营者显式配置的按次计价，以已知调用次数收费，不伪造 token 用量。严格价格准备仅适用于三个目标模型及其映射别名，其他供应商既有计费准入行为不变。

#### Scenario: 正常计量
- **WHEN** 上游报告 prompt_tokens=12、total_tokens=12
- **THEN** 记录输入 12、输出 0，并按有效 Embedding 价格与现有倍率结算一次

#### Scenario: 额外输出字段或大批量
- **WHEN** 三个官方模型的响应额外包含 output/cache 字段，或批量合计超过对话模型长上下文阈值
- **THEN** 不计输出/cache 费用且不触发长上下文倍增

#### Scenario: 标准价格与缺价
- **WHEN** 外部价格目录不可用但模型为三个官方模型之一
- **THEN** 可使用该模型精确内置 fallback，标准同步价格不使用异步 Batch 折扣
- **WHEN** 真实模型及显式运营配置都无法提供可用价格
- **THEN** 上游调用前返回 503 model_pricing_unavailable，不回退到 GPT/Claude 价格

#### Scenario: 预检后价格变更
- **WHEN** 官方 Embeddings 请求定价准备成功后，动态价格目录在异步结算前更新
- **THEN** 结算使用该请求已确定的模型及价格输入，不重新通过宽松候选链选取其他模型或新单价

### Requirement: 缺失用量不可静默处理
系统 MUST 区分合法 prompt_tokens 与缺失、null、负数、小数、错误类型或不可表示的用量。合法成功信封缺少有效计量时，系统 SHALL 保留原始响应并留下零可确认 token 的记录与可检索异常，记录累计/采样告警；系统 SHALL NOT 估算 token 扣费或自动重发。显式按次计价仍按已知调用次数收费。

#### Scenario: Usage 缺失或非法
- **WHEN** 上游返回成功向量但无合法 prompt_tokens
- **THEN** 返回原始成功结果，保留一次零可确认用量记录，记录 embedding_usage_unavailable 的安全 Ops 元数据和告警
- **THEN** 不以 total_tokens 或文本长度伪造 prompt_tokens，也不将零费用记为准确结算的证明

#### Scenario: 成功状态但不是 Embeddings 响应
- **WHEN** 官方模型上游返回 HTML、空 body 或缺失基本 Embeddings 信封
- **THEN** 返回一次 502 上游协议错误，不套用保留成功向量的缺 Usage 策略，不自动重发

#### Scenario: 成功响应的异常可观测性
- **WHEN** 返回 HTTP 200 的结果缺少有效 Usage
- **THEN** 监控启用且持久化成功时可按请求标识检索安全的异常记录，不仅在 Gin Context 设置标志
- **THEN** 监控关闭或记录失败时仍输出不含正文的结构化异常告警

### Requirement: 模型配置与隐私保持一致
系统 SHALL 在后端默认目录和前端候选中提供三个模型，保留现有模型权限和其他供应商扩展；系统 MUST 防止 Embeddings 输入、向量与凭据进入日志或诊断持久化数据。

#### Scenario: 配置模型
- **WHEN** 管理员创建/编辑 OpenAI API Key 账户或配置模型白名单
- **THEN** 可选择三个官方模型，原有账户权限不被自动放开，OAuth 可服务模型列表不被扩展

#### Scenario: 失败日志与错误回显
- **WHEN** 输入是无效 JSON 或上游错误消息回显了输入文本/敏感信息
- **THEN** 日志与 Ops 只保存安全的状态、错误类别和关联标识，不保存 body 片段、向量或凭据

#### Scenario: 安全审计内容保留
- **WHEN** Embeddings 请求经过已启用的内容审核或提示词审计
- **THEN** 保持安全检测及其失败策略，但不把输入正文写入完整提示、预览、持久化记录或异步审计载荷

#### Scenario: 既有 API 回归
- **WHEN** 调用 Responses、Chat Completions、Claude、Gemini 或已支持的第三方 Embeddings 扩展
- **THEN** 本次目标模型校验与计量变更不改变这些已有协议和费用语义

### Requirement: 实现边界便于主干合并
系统实现 MUST 复用现有调度、传输、费用计算和账务写入；Embeddings 新增协议、用量和计费准备逻辑 SHALL 集中在专用文件，公共接入使用明确可选输入并保持其他调用者缺省行为。

#### Scenario: 未启用 Embeddings 专用计费输入
- **WHEN** Responses、Chat Completions 或其他既有调用者按原参数记录用量
- **THEN** 继续执行原计费路径，无需批量修改这些调用者或启用新的全局策略

#### Scenario: 合并后验证公共接入
- **WHEN** 实现接受审查或随后更新主干
- **THEN** 公共接入点有独立正向与回归契约测试，且不存在复制扣费流程、隐式 context 计费策略或修改共享缓存对象的实现
