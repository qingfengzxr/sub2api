package service

import (
	"context"
	"strings"
)

type embeddingsDiagnosticPrivacyKey struct{}

// Only diagnostics are redacted; classification still sees the original error in memory.
func embeddingSafeDiagnostic(ctx context.Context, message string) string {
	if ctx != nil && ctx.Value(embeddingsDiagnosticPrivacyKey{}) == true {
		return "Embedding upstream error (body omitted)"
	}
	return message
}

// IsOpenAIEmbeddingsEndpoint also recognizes the existing unversioned alias.
func IsOpenAIEmbeddingsEndpoint(path string) bool {
	path = strings.TrimRight(strings.SplitN(path, "?", 2)[0], "/")
	return path == "/v1/embeddings" || path == "/embeddings"
}

// SanitizeEmbeddingsOpsEntry removes upstream echoes before the shared async queue owns them.
func SanitizeEmbeddingsOpsEntry(entry *OpsInsertErrorLogInput) {
	if entry == nil || (!IsOpenAIEmbeddingsEndpoint(entry.RequestPath) && !IsOpenAIEmbeddingsEndpoint(entry.InboundEndpoint)) {
		return
	}
	entry.ErrorBody = ""
	entry.ErrorMessage = "Embedding request diagnostic"
	entry.UpstreamErrorMessage, entry.UpstreamErrorDetail = nil, nil
	entry.UpstreamErrorsJSON = nil
	for _, event := range entry.UpstreamErrors {
		if event == nil {
			continue
		}
		event.Message = "Embedding upstream diagnostic"
		event.Detail, event.UpstreamResponseBody, event.UpstreamURL = "", "", ""
	}
}
