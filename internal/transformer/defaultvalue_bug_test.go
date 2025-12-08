package transformer

import (
	"testing"

	"github.com/openfeature/posthog-proxy/internal/config"
	"github.com/openfeature/posthog-proxy/internal/models"
	"github.com/stretchr/testify/assert"
)

// TestBooleanFlagDefaultValueFalse tests that a boolean flag with defaultValue: false
// is correctly transformed from PostHog back to OpenFeature format
func TestBooleanFlagDefaultValueFalse(t *testing.T) {
	rollout := 0 // 0% rollout means defaultValue: false
	
	// Simulate a PostHog flag that was created with defaultValue: false
	phFlag := models.PostHogFeatureFlag{
		ID:     1,
		Key:    "test-bool-false",
		Name:   "Test Boolean False",
		Active: true, // Flag is active but with 0% rollout
		Filters: models.PostHogFilters{
			Groups: []models.PostHogFilterGroup{
				{
					Properties:        []models.PostHogProperty{},
					RolloutPercentage: &rollout,
					Variant:           nil,
				},
			},
		},
	}

	cfg := config.TypeCoercionConfig{
		CoerceBooleanStrings: true,
		CoerceNumericStrings: true,
	}

	result := PostHogToOpenFeatureFlag(phFlag, cfg)

	// Now it should correctly return defaultValue: false
	assert.Equal(t, models.FlagTypeBoolean, result.Type)
	assert.Equal(t, false, result.DefaultValue, "defaultValue should be false when rollout is 0%")
}

// TestBooleanFlagDefaultValueTrue tests that a boolean flag with defaultValue: true
// is correctly transformed
func TestBooleanFlagDefaultValueTrue(t *testing.T) {
	rollout := 100
	
	phFlag := models.PostHogFeatureFlag{
		ID:     2,
		Key:    "test-bool-true",
		Name:   "Test Boolean True",
		Active: true,
		Filters: models.PostHogFilters{
			Groups: []models.PostHogFilterGroup{
				{
					Properties:        []models.PostHogProperty{},
					RolloutPercentage: &rollout,
					Variant:           nil,
				},
			},
		},
	}

	cfg := config.TypeCoercionConfig{
		CoerceBooleanStrings: true,
		CoerceNumericStrings: true,
	}

	result := PostHogToOpenFeatureFlag(phFlag, cfg)

	assert.Equal(t, models.FlagTypeBoolean, result.Type)
	assert.Equal(t, true, result.DefaultValue)
}

// TestBooleanFlagInactive tests that inactive flags return false
func TestBooleanFlagInactive(t *testing.T) {
	rollout := 100
	
	phFlag := models.PostHogFeatureFlag{
		ID:     3,
		Key:    "test-bool-inactive",
		Name:   "Test Boolean Inactive",
		Active: false, // Inactive flag
		Filters: models.PostHogFilters{
			Groups: []models.PostHogFilterGroup{
				{
					Properties:        []models.PostHogProperty{},
					RolloutPercentage: &rollout,
				},
			},
		},
	}

	cfg := config.TypeCoercionConfig{
		CoerceBooleanStrings: true,
		CoerceNumericStrings: true,
	}

	result := PostHogToOpenFeatureFlag(phFlag, cfg)

	assert.Equal(t, models.FlagTypeBoolean, result.Type)
	assert.Equal(t, false, result.DefaultValue)
	assert.Equal(t, models.FlagStateDisabled, result.State)
}

// TestCreateBooleanFlagWithFalseDefaultValue tests that creating a boolean flag
// with defaultValue: false sets rollout_percentage to 0
func TestCreateBooleanFlagWithFalseDefaultValue(t *testing.T) {
	req := models.CreateFlagRequest{
		Key:          "test-bool-false",
		Name:         "Test Boolean False",
		Description:  "A boolean flag with false default",
		Type:         models.FlagTypeBoolean,
		DefaultValue: false,
	}

	result := OpenFeatureToPostHogCreate(req, 100)

	assert.Equal(t, "test-bool-false", result.Key)
	assert.True(t, result.Active)
	
	// Verify rollout is set to 0 for defaultValue: false
	assert.NotNil(t, result.Filters.Groups)
	assert.Len(t, result.Filters.Groups, 1)
	assert.NotNil(t, result.Filters.Groups[0].RolloutPercentage)
	assert.Equal(t, 0, *result.Filters.Groups[0].RolloutPercentage, "rollout should be 0 for defaultValue: false")
	
	// We should NOT have payloads for boolean flags (PostHog only accepts "true")
	if result.Filters.Payloads != nil {
		assert.NotContains(t, result.Filters.Payloads, "false", "Should not store 'false' in payloads")
	}
}

// TestCreateBooleanFlagWithTrueDefaultValue tests that creating a boolean flag
// with defaultValue: true sets rollout_percentage to 100
func TestCreateBooleanFlagWithTrueDefaultValue(t *testing.T) {
	req := models.CreateFlagRequest{
		Key:          "test-bool-true",
		Name:         "Test Boolean True",
		Description:  "A boolean flag with true default",
		Type:         models.FlagTypeBoolean,
		DefaultValue: true,
	}

	result := OpenFeatureToPostHogCreate(req, 100)

	assert.Equal(t, "test-bool-true", result.Key)
	assert.True(t, result.Active)
	
	// Verify rollout is set to 100 for defaultValue: true
	assert.NotNil(t, result.Filters.Groups)
	assert.Len(t, result.Filters.Groups, 1)
	assert.NotNil(t, result.Filters.Groups[0].RolloutPercentage)
	assert.Equal(t, 100, *result.Filters.Groups[0].RolloutPercentage, "rollout should be 100 for defaultValue: true")
}

// TestRoundTripBooleanFalse tests the full round trip: create -> store -> read back
func TestRoundTripBooleanFalse(t *testing.T) {
	// Step 1: Create request with defaultValue: false
	createReq := models.CreateFlagRequest{
		Key:          "test-roundtrip-false",
		Name:         "Test Round Trip False",
		Description:  "Testing round trip with false",
		Type:         models.FlagTypeBoolean,
		DefaultValue: false,
	}

	// Step 2: Transform to PostHog format
	phCreate := OpenFeatureToPostHogCreate(createReq, 100)
	
	// Step 3: Simulate what PostHog would return
	// The rollout should be 0 because defaultValue is false
	phFlag := models.PostHogFeatureFlag{
		ID:     123,
		Key:    phCreate.Key,
		Name:   phCreate.Name,
		Active: phCreate.Active,
		Filters: phCreate.Filters,
	}

	// Step 4: Transform back to OpenFeature format
	cfg := config.TypeCoercionConfig{
		CoerceBooleanStrings: true,
		CoerceNumericStrings: true,
	}
	result := PostHogToOpenFeatureFlag(phFlag, cfg)

	// Step 5: Verify defaultValue is preserved
	assert.Equal(t, models.FlagTypeBoolean, result.Type)
	assert.Equal(t, false, result.DefaultValue, "Round trip should preserve defaultValue: false")
	assert.Equal(t, "test-roundtrip-false", result.Key)
}

// TestRoundTripBooleanTrue tests the full round trip with defaultValue: true
func TestRoundTripBooleanTrue(t *testing.T) {
	createReq := models.CreateFlagRequest{
		Key:          "test-roundtrip-true",
		Name:         "Test Round Trip True",
		Description:  "Testing round trip with true",
		Type:         models.FlagTypeBoolean,
		DefaultValue: true,
	}

	phCreate := OpenFeatureToPostHogCreate(createReq, 100)
	
	rollout := 100
	phFlag := models.PostHogFeatureFlag{
		ID:     124,
		Key:    phCreate.Key,
		Name:   phCreate.Name,
		Active: phCreate.Active,
		Filters: phCreate.Filters,
	}
	if len(phFlag.Filters.Groups) > 0 {
		phFlag.Filters.Groups[0].RolloutPercentage = &rollout
	}

	cfg := config.TypeCoercionConfig{
		CoerceBooleanStrings: true,
		CoerceNumericStrings: true,
	}
	result := PostHogToOpenFeatureFlag(phFlag, cfg)

	assert.Equal(t, models.FlagTypeBoolean, result.Type)
	assert.Equal(t, true, result.DefaultValue, "Round trip should preserve defaultValue: true")
	assert.Equal(t, "test-roundtrip-true", result.Key)
}

// TestBooleanFlagRolloutPercentageForFalse tests that defaultValue: false sets rollout to 0
func TestBooleanFlagRolloutPercentageForFalse(t *testing.T) {
	req := models.CreateFlagRequest{
		Key:          "test-rollout-false",
		Name:         "Test Rollout False",
		Description:  "Should set rollout to 0",
		Type:         models.FlagTypeBoolean,
		DefaultValue: false,
	}

	result := OpenFeatureToPostHogCreate(req, 100)

	assert.NotNil(t, result.Filters.Groups)
	assert.Len(t, result.Filters.Groups, 1)
	assert.NotNil(t, result.Filters.Groups[0].RolloutPercentage)
	assert.Equal(t, 0, *result.Filters.Groups[0].RolloutPercentage, "defaultValue: false should set rollout to 0")
}

// TestBooleanFlagRolloutPercentageForTrue tests that defaultValue: true sets rollout to 100
func TestBooleanFlagRolloutPercentageForTrue(t *testing.T) {
	req := models.CreateFlagRequest{
		Key:          "test-rollout-true",
		Name:         "Test Rollout True",
		Description:  "Should set rollout to 100",
		Type:         models.FlagTypeBoolean,
		DefaultValue: true,
	}

	result := OpenFeatureToPostHogCreate(req, 100)

	assert.NotNil(t, result.Filters.Groups)
	assert.Len(t, result.Filters.Groups, 1)
	assert.NotNil(t, result.Filters.Groups[0].RolloutPercentage)
	assert.Equal(t, 100, *result.Filters.Groups[0].RolloutPercentage, "defaultValue: true should set rollout to 100")
}

// TestNonBooleanFlagRolloutNotAffected tests that non-boolean flags always get rollout 100
func TestNonBooleanFlagRolloutNotAffected(t *testing.T) {
	req := models.CreateFlagRequest{
		Key:          "test-string-flag",
		Name:         "Test String Flag",
		Description:  "String flags should always have rollout 100",
		Type:         models.FlagTypeString,
		DefaultValue: "test-value",
	}

	result := OpenFeatureToPostHogCreate(req, 100)

	assert.NotNil(t, result.Filters.Groups)
	assert.Len(t, result.Filters.Groups, 1)
	assert.NotNil(t, result.Filters.Groups[0].RolloutPercentage)
	assert.Equal(t, 100, *result.Filters.Groups[0].RolloutPercentage, "Non-boolean flags should always have rollout 100")
}
