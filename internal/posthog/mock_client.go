package posthog

import (
	"context"
	
	"github.com/openfeature/posthog-proxy/internal/models"
	"github.com/stretchr/testify/mock"
)

// MockClient is a mock implementation of the PostHog client for testing
type MockClient struct {
	mock.Mock
}

func (m *MockClient) GetFeatureFlags(ctx context.Context) ([]models.PostHogFeatureFlag, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.PostHogFeatureFlag), args.Error(1)
}

func (m *MockClient) GetFeatureFlag(ctx context.Context, id int) (*models.PostHogFeatureFlag, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PostHogFeatureFlag), args.Error(1)
}

func (m *MockClient) GetFeatureFlagByKey(ctx context.Context, key string) (*models.PostHogFeatureFlag, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PostHogFeatureFlag), args.Error(1)
}

func (m *MockClient) CreateFeatureFlag(ctx context.Context, req models.PostHogCreateFlagRequest) (*models.PostHogFeatureFlag, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PostHogFeatureFlag), args.Error(1)
}

func (m *MockClient) UpdateFeatureFlag(ctx context.Context, id int, req models.PostHogUpdateFlagRequest) (*models.PostHogFeatureFlag, error) {
	args := m.Called(ctx, id, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PostHogFeatureFlag), args.Error(1)
}

func (m *MockClient) DeleteFeatureFlag(ctx context.Context, id int) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
