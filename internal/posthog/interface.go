package posthog

import (
	"context"
	"github.com/openfeature/posthog-proxy/internal/models"
)

// ClientInterface defines the interface for PostHog client operations
type ClientInterface interface {
	GetFeatureFlags(ctx context.Context) ([]models.PostHogFeatureFlag, error)
	GetFeatureFlag(ctx context.Context, id int) (*models.PostHogFeatureFlag, error)
	GetFeatureFlagByKey(ctx context.Context, key string) (*models.PostHogFeatureFlag, error)
	CreateFeatureFlag(ctx context.Context, req models.PostHogCreateFlagRequest) (*models.PostHogFeatureFlag, error)
	UpdateFeatureFlag(ctx context.Context, id int, req models.PostHogUpdateFlagRequest) (*models.PostHogFeatureFlag, error)
	DeleteFeatureFlag(ctx context.Context, id int) error
}
