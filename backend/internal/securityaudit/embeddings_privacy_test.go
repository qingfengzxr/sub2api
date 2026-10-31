package securityaudit

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEmbeddingsAuditScansWithoutEnqueue(t *testing.T) {
	for _, mode := range []Mode{ModeAsync, ModeBlocking} {
		prompt := &fakePromptEngine{mode: mode, decision: &PromptDecision{Kind: DecisionBlock}}
		legacy := &fakeLegacyEngine{}
		decision := NewCoordinator(legacy, prompt).Check(context.Background(), Request{Protocol: "openai_embeddings", Body: []byte(`{"input":"private text"}`)})
		require.Equal(t, int64(1), prompt.evaluates.Load())
		require.Zero(t, prompt.enqueues.Load())
		require.Equal(t, int64(1), legacy.calls.Load())
		require.Equal(t, mode == ModeAsync, decision.AllowNextStage)
	}
}

func TestEmbeddingsAuditSnapshotKeepsDetectionTextOnlyInMemory(t *testing.T) {
	snapshot, err := ExtractPromptSnapshot(Request{Protocol: "openai_embeddings", Body: []byte(`{"input":"private text"}`)})
	require.NoError(t, err)
	require.Equal(t, "private text", snapshot.ScanText)
	redacted := snapshot.Redacted()
	require.Empty(t, redacted.ScanText)
	require.Empty(t, redacted.FullPrompt)
	require.Empty(t, redacted.RedactedPreview)
	require.NotEmpty(t, redacted.PromptHash)
	snapshot.Protocol = "openai_responses"
	require.NotEmpty(t, snapshot.Redacted().FullPrompt, "other protocols retain their existing storage policy")
}

func TestEmbeddingsGuardPersistsDecisionWithoutEvidenceText(t *testing.T) {
	repo := &fakeJobRepository{}
	evaluator := newGuardEvaluator(PromptScannerFunc(func(_ context.Context, _ ActiveEndpoint, text string, _ []string) (*NormalizedResult, error) {
		require.Equal(t, "private text", text)
		return &NormalizedResult{Decision: EventCritical, RiskLevel: RiskCritical, Action: ActionBlock, Safety: "Unsafe", ScannerEvidence: map[string]string{"pii": "private text"}}, nil
	}), repo, nil, 2, 2)
	result, err := evaluator.Evaluate(context.Background(), guardConfig(ActiveEndpoint{ID: "one", Enabled: true, TimeoutMS: 1000, InputLimit: 100}), PromptSnapshot{Protocol: "openai_embeddings", ScanText: "private text", FullPrompt: "private text", RedactedPreview: "private text", PromptLength: 12})
	require.NoError(t, err)
	require.Equal(t, DecisionBlock, result.Kind)
	require.Equal(t, 1, repo.recordBlockingCalls)
	require.Empty(t, repo.recordBlockingSnapshot.ScanText)
	require.Empty(t, repo.recordBlockingSnapshot.FullPrompt)
	require.Empty(t, repo.recordBlockingSnapshot.RedactedPreview)
	require.Empty(t, repo.recordBlockingResult.ScannerEvidence)
}
