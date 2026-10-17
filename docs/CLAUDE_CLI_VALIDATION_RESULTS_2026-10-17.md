# Claude CLI 本机验证记录：2026-10-17

本批已执行真实 CLI 抓取、后缀差分、入口准入、实际 HTTP 出站、TLS ClientHello 对比与 handler 回退测试，修复了三个问题。**仍不能宣称整个网络指纹与 Claude CLI 完全一致，也不能由格式检查证明请求来自官方客户端。**

## 执行范围与证据

| 项目 | 实际环境/结果 |
| --- | --- |
| CLI | 本机原生二进制，`2.1.301 (Claude Code)`，入口 `/opt/homebrew/bin/claude` |
| 二进制 SHA-256 | `1f04c69e1c24e4d0a716a863af0aad8fc64d49a499334c29701f073700276b99` |
| 平台 | Darwin 23.6.0 / arm64；实际 HTTP runtime header 为 `v26.3.0`，SDK 为 `0.128.0` |
| 认证与网络 | 临时 HOME/config/工作目录；虚假 API Key；本地回环模拟服务；未读取现有登录凭据、未调用真实模型服务 |
| 首批扩展采集 | 352 组输入，368 条实际 CLI 请求；351 组产生请求，空输入一组 exit=1 且无请求 |
| 稳定性复采 | 152 组固定场景再独立跑两轮，每轮168条请求；合计三批656组输入、704条请求 |
| 随机探索 | 固定种子 `20261017`，200组 Unicode 输入；期望值取自原始 CLI 请求，未从项目算法生成 |
| 常规回归样本 | 已保存23组/40条原始 CLI 请求到 `backend/internal/service/testdata/claude_cli/`，普通单测无需安装 CLI |
| 完整矩阵状态 | 159组：48 passed、37 captured、61 pending、9 blocked、4 failed；每组范围与缺口见 CSV |

矩阵中的 `passed` 只适用于本批明确记录的版本、平台和认证环境。`captured` 表示已有部分实采或结构测试，但该组要求尚未全部完成。四条 `failed` 是仍存在网络报文差异的矩阵行，不代表四个独立根因。空输入无请求不是伪造一个后缀来算通过；Go 样本测试会明确 skip 这条。

证据目录：[validation/claude-cli-2026-10-17/](validation/claude-cli-2026-10-17/)。其中包含：

- [matrix-results.csv](validation/claude-cli-2026-10-17/matrix-results.csv)：159组逐项状态、证据类型、实际请求数、未覆盖条件。
- [manifest.json](validation/claude-cli-2026-10-17/manifest.json)：352组原始采集索引与二进制/脚本哈希。
- `captures.tar.gz`：三批原始请求、启动参数、合成输入、退出状态和输出；每条原始正文另有 SHA-256。认证/cookie 字段在落盘前移除。
- `capture-script-used.py`：执行352组采集时的脚本原文，哈希与 manifest 匹配；当前仓库脚本随后仅修正了异常退出时的清理逻辑。
- `tls-before.json` / `tls-after.json`、`http-headers.json`：实际网络测量。
- `expanded-after.log`、`repeat1-after.log`、`repeat2-after.log`、`final-unit.log`、`final-handler.log`、`final-race.log`、`tls-final.log`：复测记录。
- `artifact-sha256.json`：归档文件校验值。

## 发现与修复

### 1. 已准入 CLI 的合法 billing 后缀被改错

普通配置下，CLI 在计算后缀之后会向消息插入 `<system-reminder>`。原始输入 `0123456789ABCDEFGHIJKLMNO` 的实际后缀是 `fc8`；项目用最终正文首块 reminder 重新计算后得到 `70e`。因此即使入口通过校验，出站仍会被项目改变。

另一个独立反例来自 stream-json：输入是 `[空 text, 非空 text]` 时，CLI 按空首块计算出 `eca`，发送前删除空块，wire 正文只剩非空 text。若从最终正文重算会得到 `fc8`。这证明仅凭最终请求体通常无法还原 CLI 当时的算法输入。

修复：`buildUpstreamRequest` 和 `buildCountTokensRequest` 对已取得准入标记的请求保留原有 billing。入口仍核对 billing 版本与 User-Agent 的一致性。未准入的内部生成路径保留原有版本同步逻辑。

复测同时检查 identity service 有/无与不同版本缓存；比较完整 billing 文本和实际 HTTP 对端收到的正文 SHA。上述两类反例均永久保存为原始 CLI 样本。样本测试对这两类正规化情形按独立记录的原始输入核对算法，而不是悄悄跳过后缀比较。

### 2. 无效 UTF-8 可通过准入解析

Go 的 `encoding/json` 会替换部分无效 UTF-8。原校验因而可能接受并非合法 UTF-8 的原始正文，后续解析/转发又面对另一种内容解释。

修复：在 JSON 解码前显式检查 `utf8.Valid(body)`。复测涵盖非法单字节、过长编码、UTF-8 编码 surrogate，以及 null、数组、截断、尾随 JSON。另补11个要求头部的缺失/错误/重复，共33个单变量变异用例。

### 3. 内置 TLS profile 与实际 CLI ClientHello 不一致

同一台机器、同一 localhost SNI 上的实际抓取显示：旧内置模板多出 ECH 扩展 `65037`，缺少 hybrid group `4588`，key share 只有 `29`。实际 CLI 不发送该 ECH 扩展，groups 为 `[4588,29,23,24]`，key shares 为 `[4588,29]`。

修复：更新内置扩展顺序、groups 和默认 key shares；显式配置 curves 时，将默认 key shares 限定在已声明 curves 中，避免自定义配置被加入未声明组。已有显式 profile 参数仍按其配置执行。

复测：实际内置 dialer 与已记录 CLI 的以下字段一致：legacy version、cipher 顺序、extension ID 顺序、groups、signature algorithms、ALPN、supported versions、key-share groups、point formats、PSK modes。三次独立测量一致。使用显式信任的本地测试证书完成 uTLS 握手与 HTTP 请求；未关闭证书验证。

本批没有比较所有 extension payload、TLS session/resumption 或完整网络状态机，所以结论限定为上述字段。ClientHello 测量在认证之前主动断开，不属于完整 CLI HTTPS 端到端认证测试。

## 请求与回退验证

真实 CLI 工作流包括：bare/普通配置、Unicode边界、200组随机、多个文本块、空块正规化、图片在文本前/后、工具成功/错误、3轮会话、同进程10轮、独立进程10次、手动 compact、system 替换/追加、JSON/stream-json、effort、两种模型以及429/503后成功。

`TestClaudeCLIRecordedSamples` 对每条原始请求检查准入与后缀，然后用真实构造器生成 OAuth 出站请求，通过 socket 发往本地 HTTP 服务，检查 path/query、白名单头值、CLI beta 未丢失，以及完整 body 字节 SHA。API Key 采集转为 OAuth 出站时，补入 `oauth-2025-04-20` 是明确的认证模式差异，不能将它称为直接 OAuth CLI 对照。

`TestGatewayMessagesAdmissionThroughActualAntigravityFallback` 实际进入 handler：模拟 Antigravity 返回400 “Prompt is too long”，生成 `PromptTooLongError`，切换分组、重新调度，再调用 Anthropic Forward。普通/专用路径分别覆盖实采 CLI、合成短探测、非 CLI、重复 JSON，共8个用例。合法请求到达模拟 Anthropic 上游；负例在 Anthropic 调用前返回403，调用记录仅包含 Antigravity。注册路由的真实中间件链另由 routes 测试覆盖；未把人工 context 单测当成完整 handler 集成。

动态字段审计确认：本批3轮/10轮、重试、compact 中，device/session保持稳定，header session 与 metadata 一致；独立进程10次有10个不同session。本版本429/503重试的 `X-Stainless-Retry-Count` 实测为 `[0,0]`，不是预想的 `[0,1]`。实际转发保留这些值，见 `lifecycle-audit.json`。

## 尚存差异与未执行范围

| 项目 | 已知结果/下一步 |
| --- | --- |
| HTTP 原始头序列 | 原始 TCP 测量确认顺序不同、`Content-Type` 被写为小写、CLI `Connection: keep-alive` 未转发；字段语义通过不等于原始报文一致。需在 transport 序列化层另做设计，改 Header map 无法保证线序 |
| TLS profile 关闭 | Go 默认 ClientHello 与 CLI 不同；默认 Go ALPN 提供 h2/http1.1，实测 CLI 只提供 http1.1。账号未启用 TLS profile 的流量不能套用“内置模板字段已对齐”的结论 |
| HTTPS 代理 | `backend/internal/repository/http_upstream.go` 的 HTTPS proxy 分支回到普通 Go transport；只做了代码核对，尚无该代理路径的成对实采。HTTP CONNECT、SOCKS也待完整成对采集 |
| 自定义 TLS profile | 固定/随机/失效配置完整组合未执行；不应默认视作当前 CLI 基线 |
| 客户端来源 | 测试程序复制完整合法 CLI 报文后可通过准入。这是格式校验的能力边界，三位后缀也不是不可伪造的签名；目前不能保证“所有非 Claude Code 行为一律拒绝” |
| OAuth/真实服务 | 未使用真实 OAuth、SetupToken、专用真实API Key；不推断官方风控或认证成功率 |
| 平台/版本 | 只有本机2.1.301/macOS-arm64；Linux、Windows、WSL、其他版本和发行方式仍缺证据 |
| 辅助调用 | 704条请求未自然产生 count_tokens；其正例目前是人工结构测试。交互终端、resume、自动compact、MCP、子代理等尚未覆盖 |
| 故障与并发 | 尚缺共用项目账户缓存的混合版本10并发、Redis故障、完整取消/断流/重试矩阵；采集脚本多进程并发不能替代该验证 |

## 复现命令

在仓库根目录执行采集；输出目录必须尚不存在：

```bash
python3 backend/scripts/claude_cli_sample_capture.py --out /tmp/claude-cli-new-capture --random-count 200 --workers 4
cd backend
CLAUDE_CLI_CAPTURE_DIR=/tmp/claude-cli-new-capture go test -tags unit ./internal/service -run '^TestClaudeCLIRecordedSamples$' -count=1 -v
go test -tags unit ./internal/service ./internal/server/routes ./internal/handler ./internal/pkg/claude ./internal/pkg/tlsfingerprint -count=1
CLAUDE_CLI_BINARY=/opt/homebrew/bin/claude CLAUDE_CLI_TLS_RESULT=/tmp/cli-tls.json go test ./internal/pkg/tlsfingerprint -run 'Test(CaptureInstalledCLIClientHello|BuiltinProfileObservedCLI301|BuiltinProfileTrustedLocalTLSHandshake|ProfileDefaultKeySharesRespectCustomCurves)$' -count=3 -v
CLAUDE_CLI_HTTP_RESULT=/tmp/cli-http.json go test -tags unit ./internal/service -run '^TestClaudeCLIRecordedRawHTTPHeaders$' -count=1 -v
go test -race -tags unit ./internal/service ./internal/server/routes ./internal/handler ./internal/pkg/tlsfingerprint -run 'Test(ClaudeCLIRecorded|ClaudeCodeAdmission|AdmittedClaude|GatewayMessagesAdmissionThroughActualAntigravityFallback|BuiltinProfile)' -count=1
```

全包 unit 结果：service、routes、handler、claude、tlsfingerprint 五包通过。补入真实 CLI handler 样本之后又完整重跑 handler 包通过。全量样本、两轮固定样本重放和 TLS 三轮复测通过。相关四包的定向 race 检查通过，见 `final-race.log`；它不替代尚未完成的共用账户10并发测试。所有结论以归档日志为准。

本批算法结论来自固定真实二进制的可复核样本，不是官方开源算法证明，也不是所有场景覆盖完毕的承诺。

保存说明：仓库现有 ignore 规则忽略 `docs/`。按用户随后提出的提交要求，本报告、验证矩阵和相关完整证据包显式纳入版本控制，常规回归的23组样本位于 backend testdata 目录；未更改全局 ignore 规则。
