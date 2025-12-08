package transformer

import (
	"github.com/openfeature/posthog-proxy/internal/config"
	"github.com/openfeature/posthog-proxy/internal/models"
)

// TypeDetector interface for strategy pattern
type TypeDetector interface {
	Detect(phFlag models.PostHogFeatureFlag) (models.FlagType, interface{}, bool)
}

// PayloadObjectDetector detects object types from payloads
type PayloadObjectDetector struct{}

func (d *PayloadObjectDetector) Detect(phFlag models.PostHogFeatureFlag) (models.FlagType, interface{}, bool) {
	if phFlag.Filters.Payloads == nil {
		return "", nil, false
	}

	for _, payload := range phFlag.Filters.Payloads {
		if isJSONObject(payload) {
			if obj, err := parseJSONObject(payload); err == nil {
				return models.FlagTypeObject, obj, true
			}
		}
	}
	return "", nil, false
}

// PayloadCoercionDetector handles type coercion from payloads
type PayloadCoercionDetector struct {
	Config config.TypeCoercionConfig
}

func (d *PayloadCoercionDetector) Detect(phFlag models.PostHogFeatureFlag) (models.FlagType, interface{}, bool) {
	if phFlag.Filters.Payloads == nil {
		return "", nil, false
	}

	for _, payload := range phFlag.Filters.Payloads {
		// Try boolean coercion first (more specific)
		if d.Config.CoerceBooleanStrings {
			if boolValue, isBool := tryParseBooleanString(payload); isBool {
				return models.FlagTypeBoolean, boolValue, true
			}
		}

		// Try numeric coercion
		if d.Config.CoerceNumericStrings {
			if numValue, isNum := tryParseNumericString(payload); isNum {
				return models.FlagTypeInteger, numValue, true
			}
		}
	}
	return "", nil, false
}

// MultivariateDetector handles multivariate flag type detection
type MultivariateDetector struct{}

func (d *MultivariateDetector) Detect(phFlag models.PostHogFeatureFlag) (models.FlagType, interface{}, bool) {
	if phFlag.Filters.Multivariate == nil || len(phFlag.Filters.Multivariate.Variants) == 0 {
		return "", nil, false
	}

	firstVariant := phFlag.Filters.Multivariate.Variants[0]

	// Check if variants are numeric
	if isNumeric(firstVariant.Key) {
		if numValue, err := parseNumeric(firstVariant.Key); err == nil {
			return models.FlagTypeInteger, numValue, true
		}
	}

	// Default to string variants
	return models.FlagTypeString, firstVariant.Key, true
}

// BooleanDetector handles simple boolean flags
type BooleanDetector struct{}

func (d *BooleanDetector) Detect(phFlag models.PostHogFeatureFlag) (models.FlagType, interface{}, bool) {
	// If flag is inactive, always return false
	if !phFlag.Active {
		return models.FlagTypeBoolean, false, true
	}

	// Check rollout percentage to determine true/false
	// PostHog boolean flags use rollout_percentage to control the default behavior:
	// - rollout 0% = defaultValue: false (no users get true)
	// - rollout > 0% = defaultValue: true (some/all users get true)
	if len(phFlag.Filters.Groups) > 0 && phFlag.Filters.Groups[0].RolloutPercentage != nil {
		rollout := *phFlag.Filters.Groups[0].RolloutPercentage
		return models.FlagTypeBoolean, rollout > 0, true
	}

	// Active flag without rollout percentage defaults to true
	return models.FlagTypeBoolean, true, true
}

// TypeDetectionChain orchestrates detection strategies using Chain of Responsibility pattern
type TypeDetectionChain struct {
	detectors []TypeDetector
}

// NewTypeDetectionChain creates a new detection chain with standard detectors
func NewTypeDetectionChain(cfg config.TypeCoercionConfig) *TypeDetectionChain {
	return &TypeDetectionChain{
		detectors: []TypeDetector{
			&PayloadObjectDetector{},
			&PayloadCoercionDetector{Config: cfg},
			&MultivariateDetector{},
			&BooleanDetector{},
		},
	}
}

// DetectTypeAndValue runs through the detection chain to determine flag type and default value
func (c *TypeDetectionChain) DetectTypeAndValue(phFlag models.PostHogFeatureFlag) (models.FlagType, interface{}) {
	for _, detector := range c.detectors {
		if flagType, value, found := detector.Detect(phFlag); found {
			return flagType, value
		}
	}

	// Absolute fallback (should never reach here due to BooleanDetector always matching)
	return models.FlagTypeBoolean, false
}
