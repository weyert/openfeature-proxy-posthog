package transformer

import (
	"github.com/openfeature/posthog-proxy/internal/config"
	"github.com/openfeature/posthog-proxy/internal/models"
)

// PostHogToOpenFeatureManifest transforms PostHog feature flags to OpenFeature manifest format
func PostHogToOpenFeatureManifest(posthogFlags []models.PostHogFeatureFlag, cfg config.TypeCoercionConfig) models.Manifest {
	flags := make([]models.ManifestFlag, 0, len(posthogFlags))

	for _, phFlag := range posthogFlags {
		flags = append(flags, PostHogToOpenFeatureFlag(phFlag, cfg))
	}

	return models.Manifest{
		Flags: flags,
	}
}

// PostHogToOpenFeatureFlag transforms a single PostHog feature flag to OpenFeature format
func PostHogToOpenFeatureFlag(phFlag models.PostHogFeatureFlag, cfg config.TypeCoercionConfig) models.ManifestFlag {
	// Determine flag type and default value
	flagType, defaultValue := determineFlagTypeAndValue(phFlag, cfg)

	// Determine flag state
	state := models.FlagStateDisabled
	if phFlag.Active {
		state = models.FlagStateEnabled
	}

	// Convert variants if any
	variants := convertPostHogVariants(phFlag, cfg)

	// Map PostHog fields to OpenFeature manifest:
	// - PostHog Key -> OpenFeature Key (machine-readable identifier)
	// - PostHog Key -> OpenFeature Name (for consistency, same as key)
	// - PostHog Name -> OpenFeature Description (human-readable description)
	return models.ManifestFlag{
		Key:          phFlag.Key,
		Name:         phFlag.Key,
		Description:  phFlag.Name,
		Type:         flagType,
		DefaultValue: defaultValue,
		Variants:     variants,
		State:        state,
	}
}

// OpenFeatureToPostHogCreate transforms OpenFeature create request to PostHog format
func OpenFeatureToPostHogCreate(req models.CreateFlagRequest, defaultRollout int) models.PostHogCreateFlagRequest {
	// Use Description as PostHog's Name field, fallback to Name if Description is empty
	name := req.Description
	if name == "" {
		name = req.Name
	}
	if name == "" {
		name = req.Key // Fallback to key if both are empty
	}

	filters := createPostHogFilters(req)
	
	// Note: We do NOT store defaultValue in payloads for boolean flags
	// because PostHog only accepts "true" as a payload key for boolean flags.
	// Instead, we use rollout_percentage (set in createPostHogFilters):
	// - defaultValue: true -> rollout_percentage: 100
	// - defaultValue: false -> rollout_percentage: 0

	return models.PostHogCreateFlagRequest{
		Name:                       name,
		Key:                        req.Key,
		Active:                     true, // Start active by default
		RolloutPercentage:          &defaultRollout,
		EnsureExperienceContinuity: true,
		CreationContext:            "feature_flags",
		EvaluationRuntime:          "server",
		Filters:                    filters,
	}
}

// OpenFeatureToPostHogUpdate transforms OpenFeature update request to PostHog format
// It preserves existing PostHog settings (like groups) that aren't part of the OpenFeature update
func OpenFeatureToPostHogUpdate(req models.UpdateFlagRequest, existingFlag *models.PostHogFeatureFlag) models.PostHogUpdateFlagRequest {
	update := mapBasicUpdateFields(req)

	// Handle filters update if variants changed
	if req.Variants != nil {
		filters := reconcileFilters(req, existingFlag)
		update.Filters = filters
	}

	return update
}

// mapBasicUpdateFields maps basic fields (Name, Active) from OpenFeature request to PostHog update
func mapBasicUpdateFields(req models.UpdateFlagRequest) models.PostHogUpdateFlagRequest {
	update := models.PostHogUpdateFlagRequest{}

	// Map Description to Name (OpenFeature uses description, PostHog uses name)
	if req.Description != nil {
		update.Name = req.Description
	} else if req.Name != nil {
		update.Name = req.Name
	}

	if req.State != nil {
		active := *req.State == models.FlagStateEnabled
		update.Active = &active
	}

	return update
}

// reconcileFilters updates filters while preserving existing PostHog configurations
func reconcileFilters(req models.UpdateFlagRequest, existingFlag *models.PostHogFeatureFlag) *models.PostHogFilters {
	filters := models.PostHogFilters{}

	// Preserve existing groups if they exist, otherwise create default
	// This ensures we don't lose targeting rules that may have been configured in PostHog UI
	if len(existingFlag.Filters.Groups) > 0 {
		filters.Groups = existingFlag.Filters.Groups
	} else {
		// Create default group with 100% rollout if none exists
		defaultRolloutPercentage := 100
		filters.Groups = []models.PostHogFilterGroup{
			{
				Properties:        []models.PostHogProperty{},
				RolloutPercentage: &defaultRolloutPercentage,
				Variant:           nil,
			},
		}
	}

	// Preserve other filter properties that may exist
	if existingFlag.Filters.RolloutPercentage != nil {
		filters.RolloutPercentage = existingFlag.Filters.RolloutPercentage
	}
	if len(existingFlag.Filters.Payloads) > 0 {
		filters.Payloads = existingFlag.Filters.Payloads
	}

	// Update multivariate configuration with new variants
	if len(*req.Variants) > 0 {
		filters.Multivariate = convertVariantsToMultivariate(*req.Variants)

		// For multivariate flags, ensure groups don't have specific variant assignments
		// The multivariate configuration handles the distribution
		for i := range filters.Groups {
			filters.Groups[i].Variant = nil
		}
	} else {
		// Clear multivariate if no variants provided
		filters.Multivariate = nil
	}

	return &filters
}

// convertVariantsToMultivariate converts OpenFeature variants to PostHog multivariate configuration
func convertVariantsToMultivariate(variants map[string]models.Variant) *models.PostHogMultivariate {
	phVariants := make([]models.PostHogVariant, 0, len(variants))

	for key, variant := range variants {
		weight := 0
		if variant.Weight != nil {
			weight = *variant.Weight
		}

		phVariants = append(phVariants, models.PostHogVariant{
			Key:         key,
			Name:        key,
			RolloutFlag: weight,
		})
	}

	return &models.PostHogMultivariate{
		Variants: phVariants,
	}
}

// determineFlagTypeAndValue determines the OpenFeature flag type and default value from PostHog flag
// Uses Chain of Responsibility pattern via TypeDetectionChain
func determineFlagTypeAndValue(phFlag models.PostHogFeatureFlag, cfg config.TypeCoercionConfig) (models.FlagType, interface{}) {
	chain := NewTypeDetectionChain(cfg)
	return chain.DetectTypeAndValue(phFlag)
}

// convertPostHogVariants converts PostHog variants to OpenFeature format
func convertPostHogVariants(phFlag models.PostHogFeatureFlag, cfg config.TypeCoercionConfig) map[string]models.Variant {
	variants := make(map[string]models.Variant)

	// Only include actual PostHog multivariate variants
	if phFlag.Filters.Multivariate != nil && len(phFlag.Filters.Multivariate.Variants) > 0 {
		for _, variant := range phFlag.Filters.Multivariate.Variants {
			weight := variant.RolloutFlag
			var variantValue interface{} = variant.Key
			
			// Check if there's a payload for this variant that's a JSON object
			if phFlag.Filters.Payloads != nil {
				if payload, exists := phFlag.Filters.Payloads[variant.Key]; exists {
					if isJSONObject(payload) {
						if obj, err := parseJSONObject(payload); err == nil {
							variantValue = obj
						}
					} else {
						// Apply type coercion if enabled
						coerced := false
						if cfg.CoerceBooleanStrings {
							if boolValue, isBool := tryParseBooleanString(payload); isBool {
								variantValue = boolValue
								coerced = true
							}
						}
						if !coerced && cfg.CoerceNumericStrings {
							if numValue, isNum := tryParseNumericString(payload); isNum {
								variantValue = numValue
								coerced = true
							}
						}
						if !coerced {
							// Use the payload string as-is if no coercion applied
							variantValue = payload
						}
					}
				}
			} else {
				// Try to parse variant key as numeric if it looks like a number
				if numericValue, err := parseNumeric(variant.Key); err == nil {
					variantValue = numericValue
				}
			}
			
			variants[variant.Key] = models.Variant{
				Value:  variantValue,
				Weight: &weight,
			}
		}
		return variants
	}

	// For flags with payloads but no multivariate (like simple flags with object payloads)
	if phFlag.Filters.Payloads != nil {
		for key, payload := range phFlag.Filters.Payloads {
			var variantValue interface{} = payload
			
			// Try to parse as JSON object first
			if isJSONObject(payload) {
				if obj, err := parseJSONObject(payload); err == nil {
					variantValue = obj
				}
			} else {
				// Apply type coercion if enabled
				coerced := false
				if cfg.CoerceBooleanStrings {
					if boolValue, isBool := tryParseBooleanString(payload); isBool {
						variantValue = boolValue
						coerced = true
					}
				}
				if !coerced && cfg.CoerceNumericStrings {
					if numValue, isNum := tryParseNumericString(payload); isNum {
						variantValue = numValue
						coerced = true
					}
				}
				// If no coercion applied, variantValue remains as the original payload string
			}
			
			variants[key] = models.Variant{
				Value: variantValue,
			}
		}
		return variants
	}

	// For simple boolean flags, only add variants if explicitly needed
	// Most boolean flags don't need explicit variants in OpenFeature
	return variants
}

// createPostHogFilters creates PostHog filters from OpenFeature flag request
func createPostHogFilters(req models.CreateFlagRequest) models.PostHogFilters {
	// PostHog requires at least one filter group with rollout_percentage
	// For boolean flags, map defaultValue to rollout_percentage:
	// - defaultValue: true -> rollout_percentage: 100 (enabled for all users)
	// - defaultValue: false -> rollout_percentage: 0 (disabled for all users)
	defaultRolloutPercentage := 100
	
	// Check if this is a boolean flag and adjust rollout based on defaultValue
	if req.Type == models.FlagTypeBoolean {
		if boolVal, ok := req.DefaultValue.(bool); ok && !boolVal {
			defaultRolloutPercentage = 0
		}
	}
	
	filters := models.PostHogFilters{
		Groups: []models.PostHogFilterGroup{
			{
				Properties:        []models.PostHogProperty{},
				RolloutPercentage: &defaultRolloutPercentage,
				Variant:           nil,
			},
		},
	}

	// If there are variants, create multivariate configuration
	if req.Variants != nil && len(req.Variants) > 0 {
		variants := make([]models.PostHogVariant, 0, len(req.Variants))
		
		for key, variant := range req.Variants {
			weight := 0
			if variant.Weight != nil {
				weight = *variant.Weight
			}
			
			variants = append(variants, models.PostHogVariant{
				Key:         key,
				Name:        key,
				RolloutFlag: weight,
			})
		}

		filters.Multivariate = &models.PostHogMultivariate{
			Variants: variants,
		}
	}

	return filters
}

// createPostHogFiltersFromVariants creates PostHog filters from OpenFeature variants
func createPostHogFiltersFromVariants(variants map[string]models.Variant) models.PostHogFilters {
	filters := models.PostHogFilters{}

	if len(variants) > 0 {
		phVariants := make([]models.PostHogVariant, 0, len(variants))
		
		for key, variant := range variants {
			weight := 0
			if variant.Weight != nil {
				weight = *variant.Weight
			}
			
			phVariants = append(phVariants, models.PostHogVariant{
				Key:         key,
				Name:        key,
				RolloutFlag: weight,
			})
		}

		filters.Multivariate = &models.PostHogMultivariate{
			Variants: phVariants,
		}
	}

	return filters
}