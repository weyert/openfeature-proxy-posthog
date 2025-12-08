package transformer

import (
	"testing"

	"github.com/openfeature/posthog-proxy/internal/config"
	"github.com/openfeature/posthog-proxy/internal/models"
	"github.com/stretchr/testify/assert"
)

// Helper function for tests (shared across test files)
func intPtr(i int) *int {
	return &i
}

func stringPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}

func flagStatePtr(s models.FlagState) *models.FlagState {
	return &s
}

func TestPayloadObjectDetector(t *testing.T) {
	detector := &PayloadObjectDetector{}

	tests := []struct {
		name          string
		phFlag        models.PostHogFeatureFlag
		expectFound   bool
		expectType    models.FlagType
		expectValue   map[string]interface{}
	}{
		{
			name: "Valid JSON object payload",
			phFlag: models.PostHogFeatureFlag{
				Filters: models.PostHogFilters{
					Payloads: map[string]string{
						"variant-a": `{"foo": "bar", "count": 42}`,
					},
				},
			},
			expectFound: true,
			expectType:  models.FlagTypeObject,
			expectValue: map[string]interface{}{
				"foo":   "bar",
				"count": float64(42),
			},
		},
		{
			name: "No payloads",
			phFlag: models.PostHogFeatureFlag{
				Filters: models.PostHogFilters{},
			},
			expectFound: false,
		},
		{
			name: "String payload (not object)",
			phFlag: models.PostHogFeatureFlag{
				Filters: models.PostHogFilters{
					Payloads: map[string]string{
						"variant-a": "simple-string",
					},
				},
			},
			expectFound: false,
		},
		{
			name: "Invalid JSON object",
			phFlag: models.PostHogFeatureFlag{
				Filters: models.PostHogFilters{
					Payloads: map[string]string{
						"variant-a": `{invalid json}`,
					},
				},
			},
			expectFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flagType, value, found := detector.Detect(tt.phFlag)

			assert.Equal(t, tt.expectFound, found)
			if found {
				assert.Equal(t, tt.expectType, flagType)
				assert.Equal(t, tt.expectValue, value)
			}
		})
	}
}

func TestPayloadCoercionDetector(t *testing.T) {
	tests := []struct {
		name          string
		config        config.TypeCoercionConfig
		phFlag        models.PostHogFeatureFlag
		expectFound   bool
		expectType    models.FlagType
		expectValue   interface{}
	}{
		{
			name: "Coerce boolean string - enabled",
			config: config.TypeCoercionConfig{
				CoerceBooleanStrings: true,
			},
			phFlag: models.PostHogFeatureFlag{
				Filters: models.PostHogFilters{
					Payloads: map[string]string{
						"variant-a": "true",
					},
				},
			},
			expectFound: true,
			expectType:  models.FlagTypeBoolean,
			expectValue: true,
		},
		{
			name: "Coerce boolean string - disabled",
			config: config.TypeCoercionConfig{
				CoerceBooleanStrings: false,
			},
			phFlag: models.PostHogFeatureFlag{
				Filters: models.PostHogFilters{
					Payloads: map[string]string{
						"variant-a": "true",
					},
				},
			},
			expectFound: false,
		},
		{
			name: "Coerce numeric string - enabled",
			config: config.TypeCoercionConfig{
				CoerceNumericStrings: true,
			},
			phFlag: models.PostHogFeatureFlag{
				Filters: models.PostHogFilters{
					Payloads: map[string]string{
						"variant-a": "123",
					},
				},
			},
			expectFound: true,
			expectType:  models.FlagTypeInteger,
			expectValue: 123,
		},
		{
			name: "Coerce float string",
			config: config.TypeCoercionConfig{
				CoerceNumericStrings: true,
			},
			phFlag: models.PostHogFeatureFlag{
				Filters: models.PostHogFilters{
					Payloads: map[string]string{
						"variant-a": "123.45",
					},
				},
			},
			expectFound: true,
			expectType:  models.FlagTypeInteger,
			expectValue: 123.45,
		},
		{
			name: "No coercion possible",
			config: config.TypeCoercionConfig{
				CoerceBooleanStrings: true,
				CoerceNumericStrings: true,
			},
			phFlag: models.PostHogFeatureFlag{
				Filters: models.PostHogFilters{
					Payloads: map[string]string{
						"variant-a": "not-coercible",
					},
				},
			},
			expectFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detector := &PayloadCoercionDetector{Config: tt.config}
			flagType, value, found := detector.Detect(tt.phFlag)

			assert.Equal(t, tt.expectFound, found)
			if found {
				assert.Equal(t, tt.expectType, flagType)
				assert.Equal(t, tt.expectValue, value)
			}
		})
	}
}

func TestMultivariateDetector(t *testing.T) {
	detector := &MultivariateDetector{}

	tests := []struct {
		name        string
		phFlag      models.PostHogFeatureFlag
		expectFound bool
		expectType  models.FlagType
		expectValue interface{}
	}{
		{
			name: "String variants",
			phFlag: models.PostHogFeatureFlag{
				Filters: models.PostHogFilters{
					Multivariate: &models.PostHogMultivariate{
						Variants: []models.PostHogVariant{
							{Key: "control", Name: "Control"},
							{Key: "test", Name: "Test"},
						},
					},
				},
			},
			expectFound: true,
			expectType:  models.FlagTypeString,
			expectValue: "control",
		},
		{
			name: "Numeric variants",
			phFlag: models.PostHogFeatureFlag{
				Filters: models.PostHogFilters{
					Multivariate: &models.PostHogMultivariate{
						Variants: []models.PostHogVariant{
							{Key: "100", Name: "Hundred"},
							{Key: "200", Name: "Two Hundred"},
						},
					},
				},
			},
			expectFound: true,
			expectType:  models.FlagTypeInteger,
			expectValue: 100,
		},
		{
			name: "No multivariate",
			phFlag: models.PostHogFeatureFlag{
				Filters: models.PostHogFilters{},
			},
			expectFound: false,
		},
		{
			name: "Empty variants",
			phFlag: models.PostHogFeatureFlag{
				Filters: models.PostHogFilters{
					Multivariate: &models.PostHogMultivariate{
						Variants: []models.PostHogVariant{},
					},
				},
			},
			expectFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flagType, value, found := detector.Detect(tt.phFlag)

			assert.Equal(t, tt.expectFound, found)
			if found {
				assert.Equal(t, tt.expectType, flagType)
				assert.Equal(t, tt.expectValue, value)
			}
		})
	}
}

func TestBooleanDetector(t *testing.T) {
	detector := &BooleanDetector{}

	tests := []struct {
		name        string
		phFlag      models.PostHogFeatureFlag
		expectFound bool
		expectValue bool
	}{
		{
			name: "Active flag with 100% rollout",
			phFlag: models.PostHogFeatureFlag{
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
			expectFound: true,
			expectValue: true,
		},
		{
			name: "Active flag with 0% rollout",
			phFlag: models.PostHogFeatureFlag{
				Active: true,
				Filters: models.PostHogFilters{
					Groups: []models.PostHogFilterGroup{
						{
							Properties:        []models.PostHogProperty{},
							RolloutPercentage: intPtr(0),
						},
					},
				},
			},
			expectFound: true,
			expectValue: false,
		},
		{
			name: "Inactive flag",
			phFlag: models.PostHogFeatureFlag{
				Active: false,
				Filters: models.PostHogFilters{
					Groups: []models.PostHogFilterGroup{
						{
							Properties:        []models.PostHogProperty{},
							RolloutPercentage: intPtr(100),
						},
					},
				},
			},
			expectFound: true,
			expectValue: false,
		},
		{
			name: "Active flag without rollout percentage",
			phFlag: models.PostHogFeatureFlag{
				Active: true,
				Filters: models.PostHogFilters{
					Groups: []models.PostHogFilterGroup{
						{Properties: []models.PostHogProperty{}},
					},
				},
			},
			expectFound: true,
			expectValue: true,
		},
		{
			name: "Active flag with no groups",
			phFlag: models.PostHogFeatureFlag{
				Active:  true,
				Filters: models.PostHogFilters{},
			},
			expectFound: true,
			expectValue: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flagType, value, found := detector.Detect(tt.phFlag)

			assert.Equal(t, tt.expectFound, found)
			assert.Equal(t, models.FlagTypeBoolean, flagType)
			assert.Equal(t, tt.expectValue, value)
		})
	}
}

func TestTypeDetectionChain(t *testing.T) {
	cfg := config.TypeCoercionConfig{
		CoerceBooleanStrings: true,
		CoerceNumericStrings: true,
	}

	tests := []struct {
		name        string
		phFlag      models.PostHogFeatureFlag
		expectType  models.FlagType
		expectValue interface{}
	}{
		{
			name: "Object payload takes precedence",
			phFlag: models.PostHogFeatureFlag{
				Active: true,
				Filters: models.PostHogFilters{
					Payloads: map[string]string{
						"variant": `{"key": "value"}`,
					},
					Multivariate: &models.PostHogMultivariate{
						Variants: []models.PostHogVariant{
							{Key: "control"},
						},
					},
				},
			},
			expectType: models.FlagTypeObject,
			expectValue: map[string]interface{}{
				"key": "value",
			},
		},
		{
			name: "Coercion takes precedence over multivariate",
			phFlag: models.PostHogFeatureFlag{
				Active: true,
				Filters: models.PostHogFilters{
					Payloads: map[string]string{
						"variant": "true",
					},
					Multivariate: &models.PostHogMultivariate{
						Variants: []models.PostHogVariant{
							{Key: "control"},
						},
					},
				},
			},
			expectType:  models.FlagTypeBoolean,
			expectValue: true,
		},
		{
			name: "Multivariate takes precedence over simple boolean",
			phFlag: models.PostHogFeatureFlag{
				Active: true,
				Filters: models.PostHogFilters{
					Multivariate: &models.PostHogMultivariate{
						Variants: []models.PostHogVariant{
							{Key: "variant-a"},
							{Key: "variant-b"},
						},
					},
				},
			},
			expectType:  models.FlagTypeString,
			expectValue: "variant-a",
		},
		{
			name: "Fallback to boolean",
			phFlag: models.PostHogFeatureFlag{
				Active: true,
				Filters: models.PostHogFilters{
					Groups: []models.PostHogFilterGroup{
						{RolloutPercentage: intPtr(100)},
					},
				},
			},
			expectType:  models.FlagTypeBoolean,
			expectValue: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain := NewTypeDetectionChain(cfg)
			flagType, value := chain.DetectTypeAndValue(tt.phFlag)

			assert.Equal(t, tt.expectType, flagType)
			assert.Equal(t, tt.expectValue, value)
		})
	}
}
