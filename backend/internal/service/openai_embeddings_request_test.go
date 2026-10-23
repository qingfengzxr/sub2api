package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateOpenAIEmbeddingsRequest(t *testing.T) {
	for _, input := range []string{`"hello"`, `["hello","你好"]`, `[1,2,3]`, `[[1,2],[3]]`} {
		require.NoError(t, ValidateOpenAIEmbeddingsRequest([]byte(`{"model":"alias","input":`+input+`,"dimensions":512,"encoding_format":"base64","user":"user-1"}`), "text-embedding-3-small"))
	}
	for _, body := range []string{
		`[]`, `{}`, `{"model":1,"input":"hi"}`, `{"model":"m"}`, `{"model":"m","input":null}`,
		`{"model":"m","input":""}`, `{"model":"m","input":[]}`, `{"model":"m","input":[1,"hi"]}`,
		`{"model":"m","input":[[1],[]]}`, `{"model":"m","input":[1.2]}`, `{"model":"m","input":[-1]}`,
		`{"model":"m","input":"hi","dimensions":0}`, `{"model":"m","input":"hi","dimensions":1537}`,
		`{"model":"m","input":"hi","dimensions":1.5}`, `{"model":"m","input":"hi","encoding_format":"other"}`,
		`{"model":"m","input":"hi","user":12}`,
	} {
		t.Run(body, func(t *testing.T) {
			require.Error(t, ValidateOpenAIEmbeddingsRequest([]byte(body), "text-embedding-3-small"))
		})
	}
	require.Error(t, ValidateOpenAIEmbeddingsRequest([]byte(`{"model":"ada","input":"hi","dimensions":512}`), "text-embedding-ada-002"))
	require.NoError(t, ValidateOpenAIEmbeddingsRequest([]byte(`{"model":"vision","input":{"image":"url"}}`), "doubao-embedding-vision"))
}

func TestOpenAIEmbeddingsStrictUsage(t *testing.T) {
	u, reason := extractOfficialOpenAIEmbeddingsUsage([]byte(`{"usage":{"prompt_tokens":12,"total_tokens":100,"output_tokens":88,"prompt_tokens_details":{"cached_tokens":10}}}`))
	require.Empty(t, reason)
	require.Equal(t, OpenAIUsage{InputTokens: 12}, u)
	for _, body := range []string{`{}`, `{"usage":{}}`, `{"usage":{"total_tokens":12}}`, `{"usage":{"prompt_tokens":null}}`, `{"usage":{"prompt_tokens":-1}}`, `{"usage":{"prompt_tokens":1.2}}`, `{"usage":{"prompt_tokens":"12"}}`, `{"usage":{"prompt_tokens":999999999999999999999999999}}`, `{"usage":{"prompt_tokens":0}}`} {
		u, reason := extractOfficialOpenAIEmbeddingsUsage([]byte(body))
		require.NotEmpty(t, reason, body)
		require.Zero(t, u.InputTokens)
	}
}
