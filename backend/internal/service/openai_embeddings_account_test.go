//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestEmbeddingsAccountErrorsKeepPolicyButOmitInput(t *testing.T) {
	for _, status := range []int{401, 403} {
		repo := &rateLimitAccountRepoStub{}
		svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
		ctx := context.WithValue(context.Background(), embeddingsDiagnosticPrivacyKey{}, true)
		require.True(t, svc.HandleUpstreamError(ctx, &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}, status, http.Header{}, []byte(`{"error":{"message":"private input echoed by provider"}}`)))
		require.Equal(t, 1, repo.setErrorCalls)
		require.NotContains(t, repo.lastErrorMsg, "private input")
	}
}

func TestEmbeddingsSchedulersOnlyChooseEligibleAPIKeys(t *testing.T) {
	for _, advanced := range []string{"false", "true"} {
		t.Run(advanced, func(t *testing.T) {
			resetOpenAIAdvancedSchedulerSettingCacheForTest()
			accounts := []Account{
				{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true},
				{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: false},
				{ID: 3, Platform: PlatformKimi, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true},
				{ID: 4, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Credentials: map[string]any{"model_mapping": map[string]any{"other": "other"}}},
				{ID: 5, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true},
			}
			svc := &OpenAIGatewayService{accountRepo: schedulerTestOpenAIAccountRepo{accounts: accounts}, cache: &schedulerTestGatewayCache{}, cfg: &config.Config{}, rateLimitService: newOpenAIAdvancedSchedulerRateLimitService(advanced), concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{})}
			groupID := int64(1)
			selection, _, err := svc.SelectAccountWithSchedulerForCapability(context.Background(), &groupID, "", "", "text-embedding-3-small", nil, OpenAIUpstreamTransportHTTPSSE, OpenAIEndpointCapabilityEmbeddings, false, false, true)
			require.NoError(t, err)
			require.Equal(t, int64(5), selection.Account.ID)
			if selection.ReleaseFunc != nil {
				selection.ReleaseFunc()
			}
			_, _, err = svc.SelectAccountWithSchedulerForCapability(context.Background(), &groupID, "", "", "text-embedding-3-small", map[int64]struct{}{5: {}}, OpenAIUpstreamTransportHTTPSSE, OpenAIEndpointCapabilityEmbeddings, false, false, true)
			require.Error(t, err)
		})
	}
}
