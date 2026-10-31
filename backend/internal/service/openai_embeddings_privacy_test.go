package service

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestEmbeddingsOpsEntryDoesNotPersistEchoedInput(t *testing.T) {
	for _, path := range []string{"/v1/embeddings", "/embeddings"} {
		secret := "private-input"
		entry := &OpsInsertErrorLogInput{RequestPath: path, ErrorMessage: secret, ErrorBody: secret, UpstreamErrorMessage: &secret, UpstreamErrorDetail: &secret, UpstreamErrorsJSON: &secret, UpstreamErrors: []*OpsUpstreamErrorEvent{{Kind: "embedding_usage_unavailable", Message: secret, Detail: secret, UpstreamResponseBody: secret}}}
		SanitizeEmbeddingsOpsEntry(entry)
		require.NotContains(t, entry.ErrorMessage, secret)
		require.Empty(t, entry.ErrorBody)
		require.Nil(t, entry.UpstreamErrorsJSON)
		require.Nil(t, entry.UpstreamErrorMessage)
		require.Equal(t, "embedding_usage_unavailable", entry.UpstreamErrors[0].Kind)
		require.Empty(t, entry.UpstreamErrors[0].Detail)
		require.NotContains(t, entry.UpstreamErrors[0].Message, secret)
	}
}

func TestEmbeddingsModerationLogOmitsInput(t *testing.T) {
	svc := &ContentModerationService{}
	log := svc.buildLog(ContentModerationCheckInput{Protocol: "openai_embeddings"}, &ContentModerationConfig{}, "allow", false, "", 0, nil, "private-input", nil, nil, "echo private-input")
	require.Empty(t, log.InputExcerpt)
	require.NotContains(t, log.Error, "private-input")
}
