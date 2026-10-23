package openai

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestDefaultModelsIncludeEmbeddings(t *testing.T) {
	for _, model := range []string{"text-embedding-3-small", "text-embedding-3-large", "text-embedding-ada-002"} {
		require.Contains(t, DefaultModelIDs(), model)
	}
	require.Equal(t, "gpt-5.6-sol", DefaultModels[0].ID)
}
