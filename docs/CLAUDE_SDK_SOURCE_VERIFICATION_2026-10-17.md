# 固定版本 SDK 与 Claude CLI 核对：2026-10-17

原有实现、测试及704条请求验证证据已先提交为 `0ebdb8adb`。本补充只新增验证工具、测量记录和结论，不改变生产请求转发行为。

**结论：公开 SDK 足以解释部分头部生成、覆盖顺序和 SDK 内部重试规则，但不足以推出 Claude CLI 的完整网络指纹。这个边界已通过相同版本标签的实际网络测量确认。**

## 1. 固定来源与版本

| 对象 | 已核对来源 |
| --- | --- |
| CLI | `2.1.301`，与上一批完全相同的 Mach-O arm64 二进制；SHA-256 `1f04c69e1c24e4d0a716a863af0aad8fc64d49a499334c29701f073700276b99` |
| API SDK | 官方 npm `@anthropic-ai/sdk@0.128.0` 与 `0.127.0`，分别核对下载包 SHA-512 与 registry `dist.integrity` |
| SDK 源码 | 0.128.0 发布 provenance 指向 GitHub commit `1926adb4d292090975e6b5d19ebafe2274d2469e`；下载该提交的4个关键源码文件，与 npm 包源码逐字节相同 |
| 运行时一 | 系统 Node 23.11.0 / OpenSSL 3.6.2 / Undici 6.21.2 |
| 运行时二 | 单独下载官方 Node 26.3.0 darwin-arm64，核对官方发布 SHA-256；OpenSSL 3.5.6 / Undici 8.3.0；没有替换系统 Node |

SDK 0.128.0 发布包 SHA-256 为 `53b1dccb6c16a4c7baf33673f96b830378538f0c492a568fdd16d5f2c0d916b8`。Node 26.3.0 tar.xz 发布包 SHA-256 为 `cc1e06417e62351b90c9ed5721c98aeb235cdc0248e292c13975f77fbf61657c`。完整来源、SRI、源码哈希与依赖锁文件见 [source-manifest.json](validation/claude-sdk-2026-10-17/source-manifest.json) 和 [source-comparison.json](validation/claude-sdk-2026-10-17/source-comparison.json)。这里只核对发布包完整性和源码字节一致性，没有把下载 provenance 等同于完成其签名验证。

本机 npm 包的 `bin` 指向原生可执行文件。已安装 `install.cjs` 的说明与实际文件类型一致：安装器复制平台原生二进制，启动后不依赖常驻的系统 Node。因此“npm 安装”或请求头 `X-Stainless-Runtime: node` 均不能证明 CLI 正在使用本机 Node 的默认 HTTP/TLS 实现。

另外，CLI 宣称 SDK 0.128.0 只能作为选择公开 SDK 对照版本的依据；本次没有证明原生 CLI 内嵌实现与公开 SDK 包逐字相同，也没有从请求头断言其真实内部运行时。

## 2. 源码推导与独立实验

| 核对项 | 源码可支持的结论 | 实验及限制 |
| --- | --- | --- |
| 标准 HTTP 头 | SDK 根据自身版本、process 平台信息和请求选项构造 Stainless 头；默认 UA 为 `Anthropic/JS` 系列 | 默认 SDK 请求没有自动变成 CLI 请求。额外注入 CLI 专属头、重放同一正文并声明600秒请求超时后，仍有实际传输差异 |
| 合并优先级 | 默认头→认证头→客户端 defaultHeaders→bodyHeaders→请求 headers；内部 Headers 合并处理覆盖、null删除及 helper 累积 | 覆盖、null删除、helper去重追加均直接调用该发布 SDK 实现验证通过。不能把合并顺序当成网络头线序 |
| 内部重试 | SDK 默认最多重试2次；内部重试计数来自剩余次数，429及服务端错误可触发重试 | 控制为一次SDK内部重试，429/503均实测 `0,1`；关闭SDK重试并由调用方再次发请求，则实测 `0,0` |
| 平台标签 | OS/arch标签来自 process 等属性的正规化，并缓存平台头 | macOS/Linux/Windows × 两种架构共6组模拟值通过。它们是代码分支测试，不是这些物理平台的CLI抓取 |
| 静态 Bearer | 显式 authToken 可产生 Bearer 认证；API Key 与该分支可区分 | 使用假值分别验证只发API Key或只发Bearer；没有完成登录、真实OAuth、刷新或订阅能力验证 |
| TLS/代理 | SDK默认使用运行时 global fetch，也允许调用方替换 fetch 和传入请求配置 | 相同SDK、相同Node版本标签仍不能推出CLI ClientHello；实际CLI代理测量另列于下节 |

以上规则来自固定提交的 [client.ts](https://github.com/anthropics/anthropic-sdk-typescript/blob/1926adb4d292090975e6b5d19ebafe2274d2469e/src/client.ts)、[headers.ts](https://github.com/anthropics/anthropic-sdk-typescript/blob/1926adb4d292090975e6b5d19ebafe2274d2469e/src/internal/headers.ts)、[detect-platform.ts](https://github.com/anthropics/anthropic-sdk-typescript/blob/1926adb4d292090975e6b5d19ebafe2274d2469e/src/internal/detect-platform.ts) 和 [shims.ts](https://github.com/anthropics/anthropic-sdk-typescript/blob/1926adb4d292090975e6b5d19ebafe2274d2469e/src/internal/shims.ts)。

0.127.0 与0.128.0的上述4个文件字节相同，版本常量等其他文件仍可能不同；这不能扩大为“整个SDK相同”，更不能据此推定相邻CLI版本一致。

在0.128.0的 `src/` 中未找到 `x-anthropic-billing-header`、`cc_entrypoint`、本项目后缀盐值或 `claude-cli/` 的字面生成代码；默认 SDK 实采也没有注入 CLI billing block。本次没有从公开SDK中得到该后缀的官方实现证明。

### 实验规模

- SDK 0.128.0 × Node 23.11.0、SDK 0.128.0 × Node 26.3.0、SDK 0.127.0 × Node 26.3.0，共3种组合。
- 每种6个HTTP场景：默认、加入CLI专属头的原正文重放、429内部重试、503内部重试、调用方外层重试、静态Bearer。共18场景、27条HTTP请求，均为本地模拟响应。
- 每种组合执行头部合并断言、6组平台属性模拟和实际ClientHello采集。平台模拟不等于跨平台验证。
- 认证字段写盘前剔除，仅保留“API Key/Bearer是否存在”的布尔值。所有凭据均为合成值；未访问用户现有认证配置。

工具：[claude_sdk_source_verify.mjs](../backend/scripts/claude_sdk_source_verify.mjs)。它通过发布 SDK 的 `post` 方法重放HTTP请求，使用自建JSON响应；这验证 SDK 请求构造与传输，不声称执行了完整CLI或模型SSE工作流。

## 3. 相同版本标签仍不一致的直接反例

以原生CLI 2.1.301与官方SDK 0.128.0 + 官方Node 26.3.0做对照：

| 实测字段 | 原生 CLI | 公开 SDK + Node 26.3.0 |
| --- | --- | --- |
| cipher数目 | 17 | 52 |
| cipher顺序 | 以4865、4866、4867开头 | 以4866、4867、4865开头 |
| extension数目 | 13 | 12；顺序也不同 |
| ALPN | `http/1.1` | `http/1.1,h2` |
| point formats | `[0]` | `[0,1,2]` |
| groups / signature algorithms | CLI实测列表 | 与CLI不同，详见比较JSON |
| 本地HTTP Accept-Encoding | `gzip, deflate, br, zstd` | `gzip, deflate` |
| 本地HTTP额外字段 | 无下列两个字段 | `accept-language: *`、`sec-fetch-mode: cors` |
| HTTP头部线序 | CLI记录顺序 | 不同；不是由SDK源码对象插入顺序直接决定 |

这一HTTP比较保留相同CLI原始body字节，CLI专属头由测试显式传入，Stainless平台/版本头由SDK生成；目的在于定位SDK与运行时的边界，不把这些自定义头声称为SDK默认行为。完整差异见 [comparison.json](validation/claude-sdk-2026-10-17/comparison.json)。

SDK内部重试和CLI样本的计数差异也给出同样结论：CLI 429/503样本为 `0,0`，公开SDK内部重试为 `0,1`。外层重试实验可以解释如何出现 `0,0`，但CLI也可能覆盖头部或使用其他路径，所以它只是候选解释，不是对CLI内部代码的证明。

## 4. 新补充的真实 CLI 代理验证

使用独立临时配置、虚假API Key、无效公共域名作为目标及本地CONNECT代理，实际运行同一CLI二进制：

- HTTP代理：接收到 `CONNECT claude-sdk-audit.invalid:443`，记录隧道内目标ClientHello。
- HTTPS代理：通过 `NODE_EXTRA_CA_CERTS` 显式信任本地测试证书，正常验证代理证书后建立CONNECT；未关闭证书验证。
- 两条代理路径均与同次测试的新鲜CLI直连捕获比较，已测字段完全一致。断开点在目标TLS握手阶段，尚未发送目标认证。
- TLS包开启race检查完整执行3轮，包括两类代理、直连捕获、内置profile对照与可信证书握手；全部通过。

测量范围是隧道内目标ClientHello的10组已解析字段，不包括代理外层ClientHello的完整对齐、代理认证、目标完整TLS握手、会话恢复或真实OAuth。

结果见 [cli-proxies.json](validation/claude-sdk-2026-10-17/cli-proxies.json)；测试见 [cli_proxy_sample_capture_test.go](../backend/internal/pkg/tlsfingerprint/cli_proxy_sample_capture_test.go)。

这关闭了“本机该CLI经本地HTTP/HTTPS CONNECT后是否改变这些字段”的疑问。**它没有修复项目的HTTPS代理路径**：项目 `buildUpstreamTransportWithTLSFingerprint` 仍在 `https` 分支使用普通Go transport；本轮也没有把项目完整代理链实测通过作为结论。

官方当前文档声明支持HTTP/HTTPS代理、不支持SOCKS。项目自己的SOCKS支持可以独立测试，但不能把它当成官方CLI原生支持的已知基线。该文档是当前支持说明，不是固定CLI二进制的实现源码。[官方网络配置文档](https://code.claude.com/docs/en/network-config)

## 5. 对原矩阵的影响

| 原矩阵项 | 本轮增加的证据 | 保留的边界 |
| --- | --- | --- |
| HDR-01/02/04/09 | 固定源码、SDK原始HTTP采集、429/503内外层重试对照 | SDK规则可推导；原始头线序不同仍成立 |
| ENV-02/03、BIL-02 | 相邻API SDK版本的4个源码文件及真实SDK请求比较 | 其他CLI版本未采，不升级为已验证 |
| ENV-04/05/06 | 6种平台属性输入的SDK标签分支测试 | 物理Linux/Windows/WSL网络行为未验证 |
| ENV-10/11/12 | 合成API Key/Bearer请求与认证源码核对 | 真实OAuth、登录、刷新未验证 |
| NET-02/03/05/06 | 本机CLI的HTTP/HTTPS代理目标ClientHello、精确Node版本SDK反例 | 不能将CLI代理结果当成项目代理结果；其余网络行为仍缺 |
| NET-04 | 公开SDK默认TLS也不同于CLI的直接反例 | 更换成同版本SDK本身不等于消除默认Go TLS差异 |
| NEG-12 | SDK可通过自定义头改变声明的客户端信息 | 可复制的格式与标签仍不能证明请求来自官方CLI |

原159组状态是第一轮快照；本补充不把SDK模拟或源码推导改记为“真实CLI全场景通过”。

## 6. 复现与归档

在临时目录安装指定SDK，禁止执行安装脚本，再用所需Node可执行文件运行工具：

```bash
npm install --prefix /tmp/claude-sdk-verify --ignore-scripts --no-audit --no-fund --registry=https://registry.npmjs.org @anthropic-ai/sdk@0.128.0
node backend/scripts/claude_sdk_source_verify.mjs /tmp/claude-sdk-verify/node_modules/@anthropic-ai/sdk backend/internal/service/testdata/claude_cli/ascii/capture.json /tmp/sdk-report.json
cd backend
CLAUDE_CLI_BINARY=/opt/homebrew/bin/claude CLAUDE_CLI_PROXY_RESULT=/tmp/cli-proxies.json CLAUDE_CLI_TLS_RESULT=/tmp/cli-direct.json go test -race ./internal/pkg/tlsfingerprint -count=3 -v
```

要复现Node 26.3.0的结果，必须将命令中的 `node` 替换成经哈希核对的该版本可执行文件。上述安装会生成依赖锁文件；本次所用锁文件及所有测量均归档于 [validation/claude-sdk-2026-10-17/](validation/claude-sdk-2026-10-17/)，文件哈希见 `artifact-sha256.json`。
