package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPIKeyAuthSnapshotRoundTripsRateLimit30d(t *testing.T) {
	svc := &APIKeyService{}
	snapshot := svc.snapshotFromAPIKey(t.Context(), &APIKey{
		ID: 1, UserID: 2, Status: StatusActive, RateLimit30d: 250,
		User: &User{ID: 2, Status: StatusActive},
	})
	require.Equal(t, 250.0, svc.snapshotToAPIKey("k", snapshot).RateLimit30d)
}
