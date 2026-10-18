# TLS 代理与配置一致性修复：2026-10-18

基于 `f667920dd` 的 SDK/CLI 对照结果，本轮修复 HTTPS 目标请求在代理路径上的指纹配置失效、连接池配置复用及建连取消问题，并补充压缩响应兼容性。测试全部使用本地服务、合成凭据和测试证书。

## 实现变化

- **HTTPS 代理**：TCP → 使用系统信任库校验代理 TLS 证书和主机名 → CONNECT（可带 Basic 代理认证）→ 使用配置的 uTLS profile 与目标握手。取消此前整个路径回退普通 Go transport 的行为。代理外层仍使用 Go TLS；目标内层使用 uTLS。
- **配置变更**：连接池复用条件增加完整 profile 内容哈希；创建 transport 时保存独立配置快照。同名 profile 修改 cipher、curve 等字段后也会重建连接，旧连接继续完成已在途请求。
- **建连生命周期**：TCP 建连上限10秒，包含代理协商及目标握手的完整建立过程上限30秒；调用方较短 deadline 优先。HTTP CONNECT 写入/读取支持取消并关闭 socket；SOCKS 使用 ContextDialer。建立后的连接清除临时 deadline，长响应不受30秒建连上限限制。
- **路由与缓冲**：HTTPS 请求重定向到 HTTP 时继续通过代理隧道；保留 CONNECT 响应头之后已读入缓冲的数据；不支持的代理协议直接报错，不静默绕过指纹配置。
- **压缩语义**：支持标准 zlib 包装的 `deflate`，保留裸 DEFLATE 兼容性；无法处理的预设字典及不完整头部保留原始数据。新增完整 CLI `Accept-Encoding: gzip, deflate, br, zstd` 的透传检查，未通过改写声明掩盖传输差异。

## 验证范围

| 测试 | 结果与边界 |
| --- | --- |
| 直连、HTTP、HTTPS、SOCKS5、SOCKS5H | 通过真实本地目标 TLS 验证并读取完整 SSE 格式响应；目标收到内置17个 cipher 及 HTTP/1.1 ALPN。SOCKS测试验证项目支持，不代表CLI原生支持SOCKS |
| HTTP/HTTPS隧道指纹 | 内置及自定义两个profile，各自的10组已解析ClientHello字段与对应直连结果相同；原CLI 2.1.301固定样本比较仍通过 |
| 连接池 | 同profile复用连接；同名profile改变cipher后新建连接，并实测收到新的cipher顺序；account、proxy、account_proxy隔离模式分别检查复用条件 |
| 证书验证 | 不受信任的HTTPS代理证书、代理主机名不匹配、目标主机名不匹配均拒绝；未关闭证书验证 |
| 认证隔离 | CONNECT带合成代理认证；目标只收到合成目标认证，不收到Proxy-Authorization；代理不收到目标Authorization |
| 失败与取消 | CONNECT 407/502、HTTP CONNECT等待、代理TLS等待、SOCKS协商等待、目标TLS等待；覆盖主动取消和deadline，检查socket关闭 |
| 重定向与缓冲 | HTTPS→HTTP重定向的合成目标只能由测试代理响应；CONNECT响应紧随的数据不丢失 |
| 压缩 | gzip、br、zstd、zlib deflate、raw deflate及不支持的deflate输入；HTTP/1和HTTP/2上的CLI编码头透传 |

测试CA仅在独立测试子进程中通过 `x509.SetFallbackRoots` 注入，并启用标准证书校验。生产代码没有测试CA或关闭验证的入口。

相关代码：[dialer.go](../backend/internal/pkg/tlsfingerprint/dialer.go)、[http_upstream.go](../backend/internal/repository/http_upstream.go)。新增测试：[代理dialer测试](../backend/internal/pkg/tlsfingerprint/dialer_proxy_test.go)、[项目端到端代理测试](../backend/internal/repository/http_upstream_fingerprint_proxy_test.go)。

## 执行命令与结果

```bash
cd backend
go test -race ./internal/pkg/tlsfingerprint ./internal/repository
go test ./internal/service ./internal/pkg/proxyutil ./internal/pkg/proxyurl
go test -race ./internal/repository -run 'TestDecompressResponseBody' -count=1
go test -tags=unit ./internal/pkg/tlsfingerprint
```

以上四条命令均通过。第一条对TLS和repository完整包启用race检查；第二条覆盖service、proxyutil、proxyurl完整包；第三条复核随后补充的deflate边界处理，第四条覆盖带unit标签的TLS测试（外部网络探针默认跳过）。日志归档于 [validation/claude-tls-proxy-2026-10-18/](validation/claude-tls-proxy-2026-10-18/)。

## 仍保留的边界

- 本轮确认和修复的是**目标TLS**，不声称HTTPS代理外层Go TLS、HTTP头顺序/大小写、HTTP/2或完整网络行为与CLI一致。
- 未开启profile的普通请求继续使用原有传输策略；初始URL为明文HTTP时，`DoWithTLS`仍走普通HTTP路径，其后续升级HTTPS的重定向不在本轮指纹一致性覆盖范围内。
- HTTPS→HTTP代理隧道要求代理允许对对应端口使用CONNECT；被代理策略拒绝时返回错误，不改走直连。
- 真实OAuth、其他实际平台、其他CLI版本未补采；本次无真实上游认证请求。
- 通过请求格式或可复制的指纹不能证明请求来自官方CLI。

上一轮报告保留为历史快照：[SDK核对](CLAUDE_SDK_SOURCE_VERIFICATION_2026-10-17.md)、[CLI样本验证](CLAUDE_CLI_VALIDATION_RESULTS_2026-10-17.md)。
