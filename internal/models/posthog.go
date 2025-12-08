package models

import "time"

// PostHog API models based on the PostHog Feature Flags API

// PostHogFeatureFlag represents a PostHog feature flag
type PostHogFeatureFlag struct {
	ID                         int                    `json:"id"`
	Name                       string                 `json:"name"`
	Key                        string                 `json:"key"`
	Filters                    PostHogFilters         `json:"filters"`
	Deleted                    bool                   `json:"deleted"`
	Active                     bool                   `json:"active"`
	CreatedAt                  time.Time              `json:"created_at"`
	UpdatedAt                  time.Time              `json:"updated_at"`
	Version                    int                    `json:"version"`
	IsSimpleFlag               bool                   `json:"is_simple_flag"`
	RolloutPercentage          *int                   `json:"rollout_percentage"`
	EnsureExperienceContinuity bool                   `json:"ensure_experience_continuity"`
	Tags                       []string               `json:"tags"`
	EvaluationTags            []string               `json:"evaluation_tags"`
	UsageDashboard            *int                   `json:"usage_dashboard"`
	AnalyticsDashboards       []int                  `json:"analytics_dashboards"`
	HasEnrichedAnalytics      bool                   `json:"has_enriched_analytics"`
	UserAccessLevel           string                 `json:"user_access_level"`
	CreationContext           string                 `json:"creation_context"`
	IsRemoteConfiguration     bool                   `json:"is_remote_configuration"`
	HasEncryptedPayloads      bool                   `json:"has_encrypted_payloads"`
	Status                    string                 `json:"status"`
	EvaluationRuntime         string                 `json:"evaluation_runtime"`
	BucketingIdentifier       string                 `json:"bucketing_identifier,omitempty"`
	LastCalledAt              *time.Time             `json:"last_called_at"`
	CreatedBy                 *PostHogUser           `json:"created_by"`
	LastModifiedBy            *PostHogUser           `json:"last_modified_by"`
	ExperimentSet             []interface{}          `json:"experiment_set"`
	Surveys                   []interface{}          `json:"surveys"`
	Features                  []interface{}          `json:"features"`
	RollbackConditions        []interface{}          `json:"rollback_conditions"`
	PerformedRollback         bool                   `json:"performed_rollback"`
	CanEdit                   bool                   `json:"can_edit"`
	CreateInFolder            string                 `json:"_create_in_folder,omitempty"`
	ShouldCreateUsageDashboard bool                  `json:"_should_create_usage_dashboard,omitempty"`
}

// PostHogFilters represents PostHog feature flag filters
type PostHogFilters struct {
	Groups            []PostHogFilterGroup `json:"groups,omitempty"`
	Multivariate      *PostHogMultivariate `json:"multivariate,omitempty"`
	Payloads          map[string]string    `json:"payloads,omitempty"`
	RolloutPercentage *int                 `json:"rollout_percentage,omitempty"`
}

// PostHogFilterGroup represents a group in PostHog filters
type PostHogFilterGroup struct {
	Properties         []PostHogProperty `json:"properties,omitempty"`
	RolloutPercentage  *int              `json:"rollout_percentage,omitempty"`
	Variant            *string           `json:"variant,omitempty"`
}

// PostHogProperty represents a property filter in PostHog
type PostHogProperty struct {
	Key      string      `json:"key"`
	Type     string      `json:"type"`
	Value    interface{} `json:"value"`
	Operator string      `json:"operator"`
}

// PostHogMultivariate represents multivariate configuration
type PostHogMultivariate struct {
	Variants []PostHogVariant `json:"variants"`
}

// PostHogVariant represents a variant in PostHog
type PostHogVariant struct {
	Key           string `json:"key"`
	Name          string `json:"name,omitempty"`
	RolloutFlag   int    `json:"rollout_flag"`
}

// PostHogUser represents a PostHog user
type PostHogUser struct {
	ID                   int                    `json:"id"`
	UUID                 string                 `json:"uuid"`
	DistinctID          string                 `json:"distinct_id"`
	FirstName           string                 `json:"first_name"`
	LastName            string                 `json:"last_name"`
	Email               string                 `json:"email"`
	IsEmailVerified     bool                   `json:"is_email_verified"`
	HedgehogConfig      map[string]interface{} `json:"hedgehog_config"`
	RoleAtOrganization  string                 `json:"role_at_organization"`
}

// PostHogFeatureFlagsResponse represents the response for listing feature flags
type PostHogFeatureFlagsResponse struct {
	Count    int                   `json:"count"`
	Next     *string               `json:"next"`
	Previous *string               `json:"previous"`
	Results  []PostHogFeatureFlag  `json:"results"`
}

// PostHogCreateFlagRequest represents a request to create a PostHog feature flag
type PostHogCreateFlagRequest struct {
	Name                       string         `json:"name"`
	Key                        string         `json:"key"`
	Filters                    PostHogFilters `json:"filters,omitempty"`
	Active                     bool           `json:"active"`
	RolloutPercentage          *int           `json:"rollout_percentage,omitempty"`
	EnsureExperienceContinuity bool           `json:"ensure_experience_continuity"`
	CreationContext            string         `json:"creation_context,omitempty"`
	EvaluationRuntime          string         `json:"evaluation_runtime,omitempty"`
}

// PostHogUpdateFlagRequest represents a request to update a PostHog feature flag
type PostHogUpdateFlagRequest struct {
	Name                       *string         `json:"name,omitempty"`
	Filters                    *PostHogFilters `json:"filters,omitempty"`
	Active                     *bool           `json:"active,omitempty"`
	RolloutPercentage          *int            `json:"rollout_percentage,omitempty"`
	EnsureExperienceContinuity *bool           `json:"ensure_experience_continuity,omitempty"`
}