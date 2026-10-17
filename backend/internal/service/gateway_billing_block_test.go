package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// Fixed suffixes captured from Claude Code 2.1.301 using synthetic prompts
// against a loopback endpoint. Expectations do not use the implementation.
var observedClaudeBillingSamples = []struct{ name, text, suffix string }{
	{"ascii", "Reply OK", "a3d"},
	{"chinese", "请审查这段代码是否存在安全问题并给出改进建议，只回复好。", "6be"},
	{"unpaired_sampled_surrogate", "abcd😀efghijklmnopqrstuvwxyz", "ae9"},
	{"paired_sampled_surrogates", "abcd😀😀abcdefghijklmnopqrstuv", "981"},
}

func TestComputeClaudeCodeFingerprintObservedUnicode(t *testing.T) {
	for _, sample := range observedClaudeBillingSamples {
		t.Run(sample.name, func(t *testing.T) {
			for _, content := range []any{sample.text, []any{map[string]any{"type": "text", "text": sample.text}}} {
				body, err := json.Marshal(map[string]any{"messages": []any{map[string]any{"role": "user", "content": content}}})
				require.NoError(t, err)
				require.Equal(t, sample.suffix, computeClaudeCodeFingerprint(body, "2.1.301"))
			}
		})
	}
}
