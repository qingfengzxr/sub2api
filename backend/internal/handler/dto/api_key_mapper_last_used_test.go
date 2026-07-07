package dto

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyFromService_MapsLastUsedAt(t *testing.T) {
	lastUsed := time.Now().UTC().Truncate(time.Second)
	src := &service.APIKey{
		ID:                 1,
		UserID:             2,
		Key:                "sk-map-last-used",
		Name:               "Mapper",
		Status:             service.StatusActive,
		LastUsedAt:         &lastUsed,
		CurrentConcurrency: 3,
	}

	out := APIKeyFromService(src)
	require.NotNil(t, out)
	require.NotNil(t, out.LastUsedAt)
	require.WithinDuration(t, lastUsed, *out.LastUsedAt, time.Second)
	require.Equal(t, 3, out.CurrentConcurrency)
}

func TestAPIKeyFromService_MapsNilLastUsedAt(t *testing.T) {
	src := &service.APIKey{
		ID:     1,
		UserID: 2,
		Key:    "sk-map-last-used-nil",
		Name:   "MapperNil",
		Status: service.StatusActive,
	}

	out := APIKeyFromService(src)
	require.NotNil(t, out)
	require.Nil(t, out.LastUsedAt)
}

func TestUserAPIKeyFromService_HidesGroupRateMultipliers(t *testing.T) {
	src := &service.APIKey{
		ID:     1,
		UserID: 2,
		Key:    "sk-user-no-rate",
		Name:   "UserNoRate",
		Status: service.StatusActive,
		Group: &service.Group{
			ID:                   10,
			Name:                 "Plus",
			Description:          "User-facing group",
			Platform:             service.PlatformOpenAI,
			RateMultiplier:       3,
			ImageRateMultiplier:  4,
			ImageRateIndependent: true,
			AllowImageGeneration: true,
			Status:               service.StatusActive,
			SubscriptionType:     service.SubscriptionTypeStandard,
		},
	}

	out := UserAPIKeyFromService(src)
	require.NotNil(t, out)
	require.NotNil(t, out.Group)
	require.Equal(t, int64(10), out.Group.ID)
	require.Equal(t, "Plus", out.Group.Name)
	require.True(t, out.Group.AllowImageGeneration)
	require.True(t, out.Group.ImageRateIndependent)

	adminOut := APIKeyFromService(src)
	require.NotNil(t, adminOut.Group)
	require.Equal(t, 3.0, adminOut.Group.RateMultiplier)
	require.Equal(t, 4.0, adminOut.Group.ImageRateMultiplier)
}
