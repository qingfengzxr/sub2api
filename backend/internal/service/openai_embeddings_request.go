package service

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
)

func openAIEmbeddingDimensions(model string) int {
	switch model {
	case "text-embedding-3-small", "text-embedding-ada-002":
		return 1536
	case "text-embedding-3-large":
		return 3072
	default:
		return 0
	}
}

// ValidateOpenAIEmbeddingsRequest preserves provider extensions unless the effective model is official.
func ValidateOpenAIEmbeddingsRequest(body []byte, effectiveModel string) error {
	if !gjson.ValidBytes(body) || !gjson.ParseBytes(body).IsObject() {
		return fmt.Errorf("request body must be a JSON object")
	}
	model := gjson.GetBytes(body, "model")
	if model.Type != gjson.String || strings.TrimSpace(model.Str) == "" {
		return fmt.Errorf("model is required")
	}
	input := gjson.GetBytes(body, "input")
	if !input.Exists() || input.Type == gjson.Null {
		return fmt.Errorf("input is required")
	}
	maxDimensions := openAIEmbeddingDimensions(effectiveModel)
	if maxDimensions == 0 {
		return nil
	}
	if !validOpenAIEmbeddingInput(input) {
		return fmt.Errorf("input must contain non-empty text or arrays of non-negative integer tokens")
	}
	if dimensions := gjson.GetBytes(body, "dimensions"); dimensions.Exists() {
		n, ok := embeddingNonNegativeInt(dimensions)
		if !ok || n == 0 || n > maxDimensions || effectiveModel == "text-embedding-ada-002" {
			return fmt.Errorf("dimensions is not supported for this model or is out of range")
		}
	}
	if encoding := gjson.GetBytes(body, "encoding_format"); encoding.Exists() &&
		(encoding.Type != gjson.String || (encoding.Str != "float" && encoding.Str != "base64")) {
		return fmt.Errorf("encoding_format must be float or base64")
	}
	if user := gjson.GetBytes(body, "user"); user.Exists() && user.Type != gjson.String {
		return fmt.Errorf("user must be a string")
	}
	return nil
}

func validOpenAIEmbeddingInput(input gjson.Result) bool {
	if input.Type == gjson.String {
		return input.Str != ""
	}
	if !input.IsArray() || len(input.Array()) == 0 {
		return false
	}
	values := input.Array()
	first := values[0]
	for _, value := range values {
		switch {
		case first.Type == gjson.String:
			if value.Type != gjson.String || value.Str == "" {
				return false
			}
		case first.Type == gjson.Number:
			if _, ok := embeddingNonNegativeInt(value); !ok {
				return false
			}
		case first.IsArray():
			if !value.IsArray() || len(value.Array()) == 0 {
				return false
			}
			for _, token := range value.Array() {
				if _, ok := embeddingNonNegativeInt(token); !ok {
					return false
				}
			}
		default:
			return false
		}
	}
	return true
}

func embeddingNonNegativeInt(value gjson.Result) (int, bool) {
	if value.Type != gjson.Number {
		return 0, false
	}
	n, err := strconv.Atoi(value.Raw)
	return n, err == nil && n >= 0
}
