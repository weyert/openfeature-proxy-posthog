package transformer

import (
	"testing"
	"time"

	"github.com/openfeature/posthog-proxy/internal/config"
	"github.com/openfeature/posthog-proxy/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostHogToOpenFeatureFlag(t *testing.T) {
	cfg := config.TypeCoercionConfig{
		CoerceNumericStrings: true,
		CoerceBooleanStrings: true,
	}

	tests := []struct {
		name     string
		input    models.PostHogFeatureFlag
		expected models.ManifestFlag
	}{
		{
			name: "Simple boolean flag - active with rollout",
			input: models.PostHogFeatureFlag{
				Key:    "test-flag",
				Name:   "Test Flag",
				Active: true,
				Filters: models.PostHogFilters{
					Groups: []models.PostHogFilterGroup{
						{
							Properties:        []models.PostHogProperty{},
							RolloutPercentage: intPtr(100),
						},
					},
				},
			},
			expected: models.ManifestFlag{
				Key:          "test-flag",
				Name:         "test-flag",
				Description:  "Test Flag",
				Type:         models.FlagTypeBoolean,
				DefaultValue: true,
				State:        models.FlagStateEnabled,
			},
		},
		{
			name: "Simple boolean flag - inactive",
			input: models.PostHogFeatureFlag{
				Key:    "inactive-flag",
				Name:   "Inactive Flag",
				Active: false,
				Filters: models.PostHogFilters{
					Groups: []models.PostHogFilterGroup{
						{Properties: []models.PostHogProperty{}},
					},
				},
			},
			expected: models.ManifestFlag{
				Key:          "inactive-flag",
				Name:         "inactive-flag",
				Description:  "Inactive Flag",
				Type:         models.FlagTypeBoolean,
				DefaultValue: false,
				State:        models.FlagStateDisabled,
			},
		},
		{
			name: "String flag with multivariate",
			input: models.PostHogFeatureFlag{
				Key:    "variant-flag",
				Name:   "Variant Flag",
				Active: true,
				Filters: models.PostHogFilters{
					Groups: []models.PostHogFilterGroup{
						{Properties: []models.PostHogProperty{}},
					},
					Multivariate: &models.PostHogMultivariate{
						Variants: []models.PostHogVariant{
							{Key: "control", Name: "control", RolloutFlag: 50},
							{Key: "test", Name: "test", RolloutFlag: 50},
						},
					},
				},
			},
			expected: models.ManifestFlag{
				Key:          "variant-flag",
				Name:         "variant-flag",
				Description:  "Variant Flag",
				Type:         models.FlagTypeString,
				DefaultValue: "control",
				State:        models.FlagStateEnabled,
				Variants: map[string]models.Variant{
					"control": {Value: "control", Weight: intPtr(50)},
					"test":    {Value: "test", Weight: intPtr(50)},
				},
			},
		},
		{
			name: "Flag with expiry tag",
			input: models.PostHogFeatureFlag{
				Key:    "expiry-flag",
				Name:   "Expiry Flag",
				Active: true,
				Tags:   []string{"team:core", "expiry:2025-12-31T00:00:00Z"},
				Filters: models.PostHogFilters{
					Groups: []models.PostHogFilterGroup{
						{Properties: []models.PostHogProperty{}, RolloutPercentage: intPtr(100)},
					},
				},
			},
			expected: models.ManifestFlag{
				Key:          "expiry-flag",
				Name:         "expiry-flag",
				Description:  "Expiry Flag",
				Type:         models.FlagTypeBoolean,
				DefaultValue: true,
				State:        models.FlagStateEnabled,
				Expiry:       isoTimePtr("2025-12-31T00:00:00Z"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PostHogToOpenFeatureFlag(tt.input, cfg)

			assert.Equal(t, tt.expected.Key, result.Key)
			assert.Equal(t, tt.expected.Name, result.Name)
			assert.Equal(t, tt.expected.Description, result.Description)
			assert.Equal(t, tt.expected.Type, result.Type)
			assert.Equal(t, tt.expected.State, result.State)
			assert.Equal(t, tt.expected.DefaultValue, result.DefaultValue)

			if tt.expected.Variants != nil {
				require.NotNil(t, result.Variants)
				assert.Equal(t, len(tt.expected.Variants), len(result.Variants))
			}

			if tt.expected.Expiry != nil {
				require.NotNil(t, result.Expiry)
				assert.True(t, tt.expected.Expiry.Equal(*result.Expiry))
			} else {
				assert.Nil(t, result.Expiry)
			}
		})
	}
}

func TestOpenFeatureToPostHogCreate(t *testing.T) {
	tests := []struct {
		name                string
		input               models.CreateFlagRequest
		defaultRollout      int
		expectedKey         string
		expectedActive      bool
		expectedGroupsCount int
		expectedHasMultivar bool
		expectedTags        []string
	}{
		{
			name: "Simple boolean flag",
			input: models.CreateFlagRequest{
				Key:          "test-bool",
				Name:         "Test Boolean",
				Type:         models.FlagTypeBoolean,
				DefaultValue: false,
			},
			defaultRollout:      100,
			expectedKey:         "test-bool",
			expectedActive:      true,
			expectedGroupsCount: 1,
			expectedHasMultivar: false,
		},
		{
			name: "String flag with variants",
			input: models.CreateFlagRequest{
				Key:          "test-variants",
				Name:         "Test Variants",
				Type:         models.FlagTypeString,
				DefaultValue: "control",
				Variants: map[string]models.Variant{
					"control": {Value: "control", Weight: intPtr(50)},
					"test":    {Value: "test", Weight: intPtr(50)},
				},
			},
			defaultRollout:      100,
			expectedKey:         "test-variants",
			expectedActive:      true,
			expectedGroupsCount: 1,
			expectedHasMultivar: true,
		},
		{
			name: "Flag with expiry",
			input: models.CreateFlagRequest{
				Key:          "expiry-flag",
				Name:         "Expiry Flag",
				Type:         models.FlagTypeBoolean,
				DefaultValue: true,
				Expiry:       isoTimePtr("2025-12-31T00:00:00Z"),
			},
			defaultRollout:      100,
			expectedKey:         "expiry-flag",
			expectedActive:      true,
			expectedGroupsCount: 1,
			expectedHasMultivar: false,
			expectedTags:        []string{"expiry:2025-12-31T00:00:00Z"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := OpenFeatureToPostHogCreate(tt.input, tt.defaultRollout)

			assert.Equal(t, tt.expectedKey, result.Key)
			assert.Equal(t, tt.expectedActive, result.Active)
			assert.Equal(t, tt.expectedGroupsCount, len(result.Filters.Groups))

			if tt.expectedHasMultivar {
				require.NotNil(t, result.Filters.Multivariate)
				assert.NotEmpty(t, result.Filters.Multivariate.Variants)
			} else {
				assert.Nil(t, result.Filters.Multivariate)
			}

			if len(tt.expectedTags) > 0 {
				assert.Equal(t, tt.expectedTags, result.Tags)
			} else {
				assert.Nil(t, result.Tags)
			}
		})
	}
}

func TestOpenFeatureToPostHogUpdate(t *testing.T) {
	tests := []struct {
		name           string
		input          models.UpdateFlagRequest
		existingFlag   models.PostHogFeatureFlag
		expectedName   string
		expectedActive *bool
		expectedGroups int
		expectedTags   []string
	}{
		{
			name: "Update name only",
			input: models.UpdateFlagRequest{
				Name: stringPtr("New Name"),
			},
			existingFlag: models.PostHogFeatureFlag{
				Name:   "Old Name",
				Active: true,
				Filters: models.PostHogFilters{
					Groups: []models.PostHogFilterGroup{{}},
				},
			},
			expectedName:   "New Name",
			expectedActive: nil,
			expectedGroups: 0, // Filters not updated
		},
		{
			name: "Update state",
			input: models.UpdateFlagRequest{
				State: flagStatePtr(models.FlagStateDisabled),
			},
			existingFlag: models.PostHogFeatureFlag{
				Name:   "Flag",
				Active: true,
			},
			expectedName:   "",
			expectedActive: boolPtr(false),
			expectedGroups: 0,
		},
		{
			name: "Update variants (preserves existing groups)",
			input: models.UpdateFlagRequest{
				Variants: &map[string]models.Variant{
					"A": {Value: "A", Weight: intPtr(50)},
					"B": {Value: "B", Weight: intPtr(50)},
				},
			},
			existingFlag: models.PostHogFeatureFlag{
				Name:   "Flag",
				Active: true,
				Filters: models.PostHogFilters{
					Groups: []models.PostHogFilterGroup{
						{Properties: []models.PostHogProperty{{Key: "prop", Value: "val"}}},
					},
				},
			},
			expectedName:   "",
			expectedActive: nil,
			expectedGroups: 1,
		},
		{
			name: "Update expiry",
			input: models.UpdateFlagRequest{
				Expiry: nullableTimePtr("2025-12-31T00:00:00Z"),
			},
			existingFlag: models.PostHogFeatureFlag{
				Tags: []string{"team:core"},
			},
			expectedTags: []string{"team:core", "expiry:2025-12-31T00:00:00Z"},
		},
		{
			name: "Clear expiry",
			input: models.UpdateFlagRequest{
				Expiry: nullableTimeNil(),
			},
			existingFlag: models.PostHogFeatureFlag{
				Tags: []string{"team:core", "expiry:2025-12-31T00:00:00Z"},
			},
			expectedTags: []string{"team:core"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := OpenFeatureToPostHogUpdate(tt.input, &tt.existingFlag)

			if tt.expectedName != "" {
				assert.NotNil(t, result.Name)
				assert.Equal(t, tt.expectedName, *result.Name)
			}
			if tt.expectedActive != nil {
				assert.Equal(t, *tt.expectedActive, *result.Active)
			}
			if tt.expectedGroups > 0 {
				require.NotNil(t, result.Filters)
				assert.Equal(t, tt.expectedGroups, len(result.Filters.Groups))
			}

			if tt.expectedTags != nil {
				require.NotNil(t, result.Tags)
				assert.Equal(t, tt.expectedTags, *result.Tags)
			} else {
				assert.Nil(t, result.Tags)
			}
		})
	}
}

func TestDetermineFlagTypeAndValue(t *testing.T) {
	cfg := config.TypeCoercionConfig{
		CoerceNumericStrings: true,
		CoerceBooleanStrings: true,
	}

	tests := []struct {
		name          string
		input         models.PostHogFeatureFlag
		expectedType  models.FlagType
		expectedValue interface{}
	}{
		{
			name: "Boolean true with rollout",
			input: models.PostHogFeatureFlag{
				Active: true,
				Filters: models.PostHogFilters{
					Groups: []models.PostHogFilterGroup{{
						Properties:        []models.PostHogProperty{},
						RolloutPercentage: intPtr(100),
					}},
				},
			},
			expectedType:  models.FlagTypeBoolean,
			expectedValue: true,
		},
		{
			name: "Boolean false",
			input: models.PostHogFeatureFlag{
				Active: false,
				Filters: models.PostHogFilters{
					Groups: []models.PostHogFilterGroup{{Properties: []models.PostHogProperty{}}},
				},
			},
			expectedType:  models.FlagTypeBoolean,
			expectedValue: false,
		},
		{
			name: "String with multivariate",
			input: models.PostHogFeatureFlag{
				Active: true,
				Filters: models.PostHogFilters{
					Groups: []models.PostHogFilterGroup{{Properties: []models.PostHogProperty{}}},
					Multivariate: &models.PostHogMultivariate{
						Variants: []models.PostHogVariant{
							{Key: "variant-a", Name: "Variant A"},
						},
					},
				},
			},
			expectedType:  models.FlagTypeString,
			expectedValue: "variant-a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flagType, value := determineFlagTypeAndValue(tt.input, cfg)

			assert.Equal(t, tt.expectedType, flagType)
			assert.Equal(t, tt.expectedValue, value)
		})
	}
}

func TestConvertPostHogVariants(t *testing.T) {
	cfg := config.TypeCoercionConfig{
		CoerceNumericStrings: true,
		CoerceBooleanStrings: true,
	}

	tests := []struct {
		name          string
		input         models.PostHogFeatureFlag
		expectedCount int
		expectedKeys  []string
	}{
		{
			name: "No multivariate",
			input: models.PostHogFeatureFlag{
				Filters: models.PostHogFilters{
					Multivariate: nil,
				},
			},
			expectedCount: 0,
			expectedKeys:  []string{},
		},
		{
			name: "Two variants",
			input: models.PostHogFeatureFlag{
				Filters: models.PostHogFilters{
					Multivariate: &models.PostHogMultivariate{
						Variants: []models.PostHogVariant{
							{Key: "control", Name: "Control", RolloutFlag: 50},
							{Key: "test", Name: "Test", RolloutFlag: 50},
						},
					},
				},
			},
			expectedCount: 2,
			expectedKeys:  []string{"control", "test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertPostHogVariants(tt.input, cfg)

			if tt.expectedCount == 0 {
				// Function returns empty map, not nil
				assert.NotNil(t, result)
				assert.Equal(t, 0, len(result))
			} else {
				require.NotNil(t, result)
				assert.Equal(t, tt.expectedCount, len(result))

				for _, key := range tt.expectedKeys {
					_, exists := result[key]
					assert.True(t, exists, "Expected key %s to exist", key)
				}
			}
		})
	}
}

func isoTimePtr(value string) *time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic(err)
	}
	return &parsed
}

func nullableTimePtr(value string) *models.NullableTime {
	parsed := isoTimePtr(value)
	return &models.NullableTime{Value: parsed}
}

func nullableTimeNil() *models.NullableTime {
	return &models.NullableTime{Value: nil}
}
