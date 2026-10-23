package service

import "github.com/tidwall/gjson"

func extractOfficialOpenAIEmbeddingsUsage(body []byte) (OpenAIUsage, string) {
	value := gjson.GetBytes(body, "usage.prompt_tokens")
	n, ok := embeddingNonNegativeInt(value)
	if !ok {
		return OpenAIUsage{}, "missing_or_invalid_prompt_tokens"
	}
	if n == 0 {
		return OpenAIUsage{}, "zero_prompt_tokens"
	}
	return OpenAIUsage{InputTokens: n}, ""
}

func validOpenAIEmbeddingsResponse(body []byte) bool {
	if !gjson.ValidBytes(body) {
		return false
	}
	model := gjson.GetBytes(body, "model")
	// Array counts are synthesized (Index == 0); an object's literal "#" value has a source index.
	count := gjson.GetBytes(body, "data.#")
	return gjson.GetBytes(body, "object").String() == "list" &&
		count.Type == gjson.Number && count.Index == 0 && model.Type == gjson.String && model.Str != ""
}
