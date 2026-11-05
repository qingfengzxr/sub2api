# Anthropic 普通客户端兼容恢复

## 状态与范围

- 开发日期：2026-11-04；提交与发布授权日期：2026-11-05。
- 基线：`d72e3bbcabb9a6c17629d96fd17a3e3a8dc94218`。
- 状态：修复随本源码提交交付，开发阶段已完成严格 RED → GREEN 和隔离回归。实际生产版本以 deploy_manager 的 VERSION、发布记录及运行二进制读回为准，不以本文件推断已上线。
- 2026-11-05 用户授权提交、推送并通过既有固定入口发布；不修改生产数据库、Secret、Compose、账户配置或分组开关，不调用真实模型服务。

## 行为与单一配置权威

原来的公开入口把 Anthropic 转发强制绑定到 CLI profile 验证，导致现有普通客户端适配逻辑不可达。本次移除路由与转发层的无条件 CLI-only 门禁，不新增全局配置；继续以现有 `Group.ClaudeCodeOnly` / `FallbackGroupID` 为客户端限制的权威。

| 场景 | 行为 |
| --- | --- |
| 普通客户端，分组未启用 CLI-only | 正常鉴权、限流、计费、选号，再走现有协议转换和服务端适配 |
| 完整识别的 CLI profile | 保留现有 admission/profile 标记，继续保留原始 billing 与 headers 的既有语义 |
| 仅声明 CLI UA，包括 >=2.1.304 | 不再被强制标记为已验证；合法请求作为普通客户端适配 |
| 分组启用 CLI-only，普通客户端且无回退 | 返回 403；Messages / count_tokens 不再误报 503 |
| 分组启用 CLI-only，配置普通回退 | 沿用原有回退分组解析，普通客户端不选择受限分组账号 |
| Antigravity 转入普通 Anthropic 回退分组 | 普通客户端可以正常继续转发 |
| Antigravity 转入 CLI-only Anthropic 回退分组 | 未识别 CLI 的请求仍被拒绝，强制平台入口也不能绕过 |
| 畸形 JSON、非对象正文、无效 UTF-8、重复字段 | 与客户端身份独立，相关 JSON 入口返回 400 |
| 正文超过已有上限 | 继续返回 413 |
| 其他平台非 Messages multipart/binary 入口 | 不套用 Anthropic JSON profile 解析 |

公开入口的正/负识别结果是后续处理的权威，不会被旧的宽松 UA / metadata / billing 检测再次升级为 CLI。内部未经过公开入口的调用保留原有兼容识别。`WithClaudeCodeAdmissionPolicy` 这一既有符号现在表示“已完成原始入口分类”，不再代表全局强制 CLI-only。

没有修改 API Key 鉴权、模型白名单、计费、余额、配额、限流或其他现有安全策略。现有分组若已经启用 CLI-only，本次不会自动关闭它。协议本身是否支持某个端点仍由现有 handler 决定；解除指纹门禁不是新增 System One 或 Anthropic WebSocket 能力。

## TDD 证据

在任何生产代码调整之前，新增测试真实失败：

1. 普通客户端在 Anthropic、组合路由、未分组及注册别名入口收到旧 403。
2. Messages、count_tokens、Chat Completions、Responses 的真实转发函数在接触本地 HTTP 测试端点之前被旧 guard 拒绝。
3. >=2.1.304 的 UA-only 请求被旧兼容分支错误标记为已验证。
4. 后续 handler 的宽松 max_tokens=1 检测覆盖负分类。

实现后扩展分组策略回归，发现 Messages / count_tokens 对既有 `ErrClaudeCodeOnly` 误返回 503；先记录这两个 RED，再补最小错误映射为 403。没有改变调度/回退策略本身。

验证日志保存在本次任务的本地验证产物中；下列数字按实际 Go JSON events 程序化统计，不把无测试文件的包计作测试通过：

- 完整 backend `go test -tags=unit ./... -count=1` 最终通过：59 个有测试包；11,851 个顶层测试通过；19 个测试按其既有条件跳过；0 失败。含子测试的通过 event 为 23,205。
- 定向 race：service / routes / handler 三包通过；22 个顶层测试（含子测试 202 个通过 event）；0 失败。
- `go vet -tags=unit ./...` 通过。
- `go build ./cmd/server` 通过。
- `git diff --check` 通过；go.mod / go.sum 未变。

19 个跳过项包括已有 sentinel、外部 Redis/PostgreSQL、真实 CLI 二进制采集、外网 TLS/JA3、真实 Provider 调用、本地插件和历史 PostgreSQL 特有行为；本次不将它们宣称为已验证。

第一轮完整执行受到测试环境约束影响：Ent schema 测试需要可写 `.entc`，TLS 测试需要 localhost hosts 项，两个现有参数校验用例需要公共域名 DNS 解析。调整验证器为每次独立的公共源码快照，补入 localhost 及当次实际 DNS 解析快照后运行，仍不开放测试网络出口。另一次完整回归中，未改动的全局分配量阈值测试出现一次超阈值；单独连续五次和最终完整回归均通过，没有修改或放宽阈值。

2026-11-05 发布前复验：完整 backend 最终仍为 59 个有测试包通过、11,851 个顶层测试通过、19 个条件跳过、0 失败；扩大定向 race 覆盖后为 52 个顶层测试通过，vet 和构建通过。此次首轮完整回归的同一未改动内存阈值测试测得 8,808,368 字节，高于 8,388,608 字节阈值；保留该失败日志，单独连续五次及随后完整重跑均通过，未改动测试、实现或阈值。

## 验证层次与边界

- routes：真实注册中间件链验证普通请求不再被 CLI 403 截断；该 fixture 缺失认证用户上下文，因此到达 handler 后按原规则返回错误，不把它算成成功鉴权。
- service：真实转发构造与响应转换，通过实际 loopback HTTP server 接收出站请求；使用合成认证和显式测试响应，验证四类请求、出站 CLI headers、原始用户内容与模拟路径。兼容转换器需要上游 SSE 时，fixture 提供完整终止事件。
- handler：实际执行 PromptTooLongError → 换分组 → 重调度 → Anthropic Forward；仅上游响应与账户/调度存储使用受控替代。
- 真实 CLI 已归档样本继续通过现有 billing / headers / body 保留测试。

这些不等于真实 OAuth 登录、上游 Provider 接受、实际计费、生产数据库集成或完整 TLS 指纹等价。新增 loopback 传输仅改变目的地，不宣称验证了实际 TLS profile。

沿既有固定部署入口发布，以精确源码提交、运行镜像和独立读回记录完成验收；真实客户端和测试账户的模型调用仍需单独授权。不要让客户端假报 UA 版本替代服务端修复。

## 服务变更安全评估

Name: Sub2API / ccapi。
Provider: 本地部署的个人 fork；基线提交如上。
Type: 模型 API 网关。
Trust Tier: 5（个人 fork，限定当前变更审查，不作全仓可信声明）。

Architecture:
- Key management：生产 Secret 与上游账户凭据未读取/未改动；测试仅使用合成值。
- Human-in-the-loop：开发阶段未执行生产副作用；2026-11-05 用户明确授权本修复提交、推送与固定入口发布。
- Data flow：生产既有客户端 → 网关 → 配置上游的路径未改；测试只到隔离回环端点。
- Update mechanism：仍由既有 deploy_manager 发布，未改变版本或内置更新策略。

Permissions required: 公共源码读写、独立缓存/产物写入与隔离测试。未挂载生产路径、宿主凭据或 Docker socket；编译/测试阶段无网络出口。依赖仅按原锁从官方 Go proxy 下载，依赖下载与执行分开。

Red flags / limits:
- CLI profile 仍是可复制的协议特征，不是客户端身份认证。
- 默认开放普通客户端会扩大可用客户端范围，必须继续依赖 Key、模型、配额与限流权限，而非 UA。
- 服务端模拟不能承诺上游接受或账号风险为零。

Risk: MEDIUM（客户端准入语义变更；没有生产副作用）。
Verdict: 可作为已测试开发候选；不将候选测试解释为生产上线或真实 Provider 验收。
