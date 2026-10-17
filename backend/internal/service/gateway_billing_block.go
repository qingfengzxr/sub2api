package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"unicode/utf16"

	"github.com/tidwall/gjson"
)

// fingerprintSalt 是计算 cc_version 后缀指纹的盐值。
//
// 参考第三方 Parrot src/transform/cc_mimicry.py，并由固定 CLI 样本检验。
// 这不是官方发布的规范，也不代表对其他版本或服务端识别规则的保证。
const fingerprintSalt = "59cf53e54c78"

// computeClaudeCodeFingerprint 实现已观测样本中的 cc_version 后缀计算：
//
//  1. 取 messages 中第一条 role=user 的纯文本（首块 text）
//  2. 按 JavaScript UTF-16 字符串下标取第 4、7、20 个码元（不足以 '0' 补齐）
//  3. SHA256(SALT + chars + cc_version) 取 hex 前 3 字符
//
// CLI 可能在计算后插入 reminder 块或删除空 text 块。最终 payload 的首块文本不一定
// 是原始输入，因此不得用此函数重新计算已准入 CLI 请求的现有后缀。
func computeClaudeCodeFingerprint(body []byte, version string) string {
	firstText := extractFirstUserText(body)
	// Only the first 21 code units can affect the fingerprint. Avoid allocating
	// a second full-size copy of a potentially large first user message.
	units := make([]uint16, 0, 22)
	for _, r := range firstText {
		if r <= 0xffff {
			units = append(units, uint16(r))
		} else {
			hi, lo := utf16.EncodeRune(r)
			units = append(units, uint16(hi), uint16(lo))
		}
		if len(units) > 20 {
			break
		}
	}
	indices := []int{4, 7, 20}
	chars := make([]uint16, 0, 3)
	for _, i := range indices {
		if i < len(units) {
			chars = append(chars, units[i])
		} else {
			chars = append(chars, '0')
		}
	}
	// Decode after concatenation: sampled surrogates may form a pair, while
	// lone surrogates become U+FFFD, matching Node's UTF-8 string encoding.
	sum := sha256.Sum256([]byte(fingerprintSalt + string(utf16.Decode(chars)) + version))
	return hex.EncodeToString(sum[:])[:3]
}

// extractFirstUserText 提取 messages 中第一条 user 消息的首段 text 内容。
// 兼容 string 和 []block 两种 content 格式。
func extractFirstUserText(body []byte) string {
	messages := gjson.GetBytes(body, "messages")
	if !messages.IsArray() {
		return ""
	}
	first := ""
	messages.ForEach(func(_, msg gjson.Result) bool {
		if msg.Get("role").String() != "user" {
			return true
		}
		content := msg.Get("content")
		if content.Type == gjson.String {
			first = content.String()
			return false
		}
		if content.IsArray() {
			content.ForEach(func(_, block gjson.Result) bool {
				if block.Get("type").String() == "text" {
					first = block.Get("text").String()
					return false
				}
				return true
			})
			return false
		}
		return false
	})
	return first
}

// buildBillingAttributionText 构造 system 数组的 billing attribution 文本。
//
// 形态对齐真实 Claude Code CLI：
//
//	x-anthropic-billing-header: cc_version=2.1.161.{fp}; cc_entrypoint=cli;
//
// 本机 2.1.301 样本未发送 cch 字段，此处不生成。cc_entrypoint 在已观测
// CLI 模式中包含 cli/sdk-cli；本函数仅供项目内部构造 cli 入口文本，不能
// 用它覆盖已准入客户端的原始文本，也不能据此推断上游客户端识别规则。
//
// 此 block 不带 cache_control（与真实 CLI 一致；cache breakpoint 由后续的
// Claude Code prompt block 承担）。
func buildBillingAttributionText(body []byte, cliVersion string) (string, error) {
	if cliVersion == "" {
		return "", fmt.Errorf("cliVersion required")
	}
	fp := computeClaudeCodeFingerprint(body, cliVersion)
	return fmt.Sprintf(
		"x-anthropic-billing-header: cc_version=%s.%s; cc_entrypoint=cli;",
		cliVersion, fp,
	), nil
}
