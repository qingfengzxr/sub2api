# Claude CLI 2.1.301 HTTP/1.1 头部顺序实现

基于 `adb6d3a8d`。本轮为已准入、最终 User-Agent 版本为 `2.1.301` 的 Anthropic messages/count_tokens 请求选择独立 HTTP/1.1 序列化和连接池。目标是已测样本的头名、大小写、排列顺序一致；认证值、目标 Host、正文长度及账号策略决定的其他头值仍由实际请求生成。

## 来源与实测基线

- 同一实际 CLI 二进制 `2.1.301`，SHA-256：`1f04c69e1c24e4d0a716a863af0aad8fc64d49a499334c29701f073700276b99`。
- 23个已有fixture内共40条请求具有相同的非认证头名序列，继续作为正文与头序列回归基线。
- 新增16个真实CLI场景、24条请求：HTTP/HTTPS × 合成API Key/静态Bearer × 普通请求/429/503/可选头。HTTPS使用明确受信任的本地CA，没有关闭证书校验。
- 新采集仅保存头名，不保存认证值、请求正文、CLI输出或本机认证配置。静态Bearer实验不代表验证了真实OAuth登录、刷新或订阅请求。

[原始头名样本](../backend/internal/pkg/claudehttp/testdata/cli-2.1.301-header-order.json)和[可复现采集脚本](../backend/scripts/claude_cli_header_order_capture.py)。

实测排列规则：应用头按已观察到的大小写进行字符排序，随后为 `Connection`、`Host`、`Accept-Encoding`、`Content-Length`。API Key 的 `x-api-key` 位于 `x-app` 前；Bearer 的 `Authorization` 位于 `Accept` 后。可选 `X-Stainless-Helper-Method` 和 `X-Synthetic-Extra` 插入相应排序位置。429/503重试未改变序列。

## 实现

使用固定依赖 `github.com/bogdanfinn/fhttp v0.6.9` 提供显式头序列和连接池，通过适配层继续使用 `net/http.Client` 的重定向、请求取消及项目响应处理。不在TCP写缓冲上重排字节，也不自行实现HTTP响应解析器。[依赖源码](https://github.com/bogdanfinn/fhttp/tree/v0.6.9)

- 标准请求头映射到已测大小写；应用头显式排序，尾部传输头固定排列。正文保持原字节，Host/Content-Length由真实目的地和正文长度生成，内部排序字段不会发到网络。
- HTTP头序列配置和TLS profile分别控制：未启用TLS profile时仍使用标准Go目标TLS；已启用时继续使用原uTLS拨号器。该有序路径明确使用HTTP/1.1，冲突的h2 ALPN配置报错。
- 直连、HTTP/HTTPS CONNECT、SOCKS5/SOCKS5H共用已有可信代理/目标握手路径；不向目标发送Proxy-Authorization。
- 连接池键包含 `claude-cli-2.1.301-h1`，与普通请求隔离；保留TLS profile内容哈希、并发计数、关闭空闲连接及响应解压行为。
- 在messages和count_tokens的普通/API Key透传构建器最终覆盖头之后选择profile。仅有UA而没有准入上下文不会启用。未知版本继续原有传输策略。
- 拒绝大小写别名重复头、非法字段、未知正文长度、请求chunked/trailer等未支持形态；错误路径关闭请求体，不先发出半个请求。
- 修复count_tokens透传构建器使用标准Header.Get/Set处理小写原始键导致的重复 `Content-Type`/`anthropic-version`，避免新transport拒绝合法请求。

主要代码：[claudehttp](../backend/internal/pkg/claudehttp/transport.go)、[profile选择](../backend/internal/pkg/claudehttp/profile.go)、[repository接入](../backend/internal/repository/claude_http1_transport.go)。依赖新增fhttp及其utls、circl间接依赖；现有 `refraction-networking/utls` 的配置与实现没有替换。

## 回归验证

| 覆盖项 | 断言 |
| --- | --- |
| 历史40条请求 | 原始TCP头名/大小写/顺序逐项相等，正文逐字节相等，40次请求复用同一连接，调用方Header map不被修改 |
| 新采集24条序列 | API Key、静态Bearer、可选头、重试样本序列全部匹配；重放本身使用本地HTTP，真实CLI采集包含HTTP和HTTPS |
| 项目端到端40次请求 | 5条网络路径 × TLS profile开/关 × 两种认证 × 两次复用；在目标TLS解密后读取原始头序列，验证正文、连接复用和请求计数归零 |
| 并发 | 40个并发请求交替API Key/Bearer头集合，最多8条连接，序列无交叉污染；共享Header map不变 |
| 流与取消 | 提前关闭SSE响应后丢弃旧连接；读取期间取消；MaxConnsPerHost=1下排队请求取消及后续请求成功；响应trailer正常读取 |
| 重试与校验 | 429/503状态交还调用方；不会因切换serializer自行重试业务状态；非法/歧义头在拨号前拒绝 |
| 构建器 | 准入/未准入、已知/未知版本、messages/count_tokens、API Key透传；原始HTTP头测试升级为顺序断言 |

完整race回归曾暴露一个旧测试的时序问题：测试代理在接收隧道内HTTP请求前即发送HTTP响应，触发Go空闲连接读取器的“unsolicited response”。已将该fixture改为收到请求后响应；CONNECT缓冲数据保留仍由独立dialer测试覆盖。

验证命令：

```bash
cd backend
go test -race ./internal/pkg/claudehttp ./internal/pkg/tlsfingerprint ./internal/repository
go test ./internal/service ./internal/pkg/proxyutil ./internal/pkg/proxyurl
go test -race ./internal/service -run 'TestAdmittedClaudeHTTP1ProfileSelection|TestClaudeCLIRecordedRawHTTPHeaders|TestAdmittedClaudeProfilePreservedOnWire' -count=1
go test -race ./internal/pkg/claudehttp
```

以上检查全部通过；count_tokens重复头修复后又完成service全量测试，补充并发/排队取消后重新完成序列化包race测试。最终测试结果与哈希归档于 [validation/claude-http1-2026-10-18/](validation/claude-http1-2026-10-18/)。

## 保证范围

- 已测CLI版本、头集合和HTTP/1.1场景的**头名、大小写、顺序一致**。没有把头序列一致升级为全部HTTP字节或全部网络指纹一致。
- `count_tokens`接入同一模板且构建器通过测试；本次真实CLI头名补采来自messages，不声称获得了真实count_tokens全部场景的独立样本。
- 未实测的任意自定义头、显式覆盖传输头、其他版本/实际操作系统、HTTP/2、真实OAuth生命周期，仍不能保证与CLI一致。
- 有序路径的HTTP/HTTPS代理使用CONNECT（包括明文目标），需要代理允许目标端口；被拒绝时返回错误，不直连绕过代理。
- 代理外层TLS、HTTP CONNECT控制请求的头序列，以及会解密并重写请求的中间节点，不属于目标HTTP请求头序列保证。

这更新了[此前报告](CLAUDE_TLS_PROXY_FIX_2026-10-18.md)中“HTTP头顺序未实现”的状态，历史测量仍按原日期保留。
