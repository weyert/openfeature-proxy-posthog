package handlers

import (
	"fmt"

	"github.com/openfeature/posthog-proxy/internal/models"
)

// ValidateVariantWeights validates that variants are properly configured
func ValidateVariantWeights(variants map[string]models.Variant) error {
	// Empty variants map is invalid
	if len(variants) == 0 {
		return fmt.Errorf("variants cannot be empty - at least one variant is required")
	}

	total := 0
	hasAnyWeight := false

	for _, variant := range variants {
		if variant.Weight != nil {
			total += *variant.Weight
			hasAnyWeight = true
		}
	}

	// If some weights specified, they must sum to 100 or we'll normalize
	// If no weights specified, we'll auto-distribute equally
	if hasAnyWeight && total > 0 && total != 100 {
		// Allow normalization - don't error, just inform via logs
		// The normalization will happen in NormalizeVariantWeights
	}

	return nil
}

// NormalizeVariantWeights ensures variant weights sum to exactly 100
// Handles multiple scenarios:
// 1. All weights specified and sum to 100 -> use as-is
// 2. No weights specified -> distribute equally
// 3. Some weights specified -> distribute remainder equally
// 4. Weights sum != 100 -> normalize proportionally
func NormalizeVariantWeights(variants map[string]models.Variant) map[string]models.Variant {
	if len(variants) == 0 {
		return variants
	}

	// Count unweighted variants and calculate total specified weight
	unweightedCount := 0
	totalSpecified := 0
	
	for _, variant := range variants {
		if variant.Weight == nil {
			unweightedCount++
		} else {
			totalSpecified += *variant.Weight
		}
	}

	// Case 1: All weights provided and sum to 100 - use as-is
	if unweightedCount == 0 && totalSpecified == 100 {
		return variants
	}

	// Case 2: No weights provided - distribute equally
	if unweightedCount == len(variants) {
		return distributeEqually(variants)
	}

	// Case 3: Some weights provided and sum < 100 - distribute remainder
	if unweightedCount > 0 && totalSpecified < 100 {
		return distributeRemainder(variants, totalSpecified, unweightedCount)
	}

	// Case 4: All weights provided but don't sum to 100 - normalize proportionally
	if unweightedCount == 0 && totalSpecified != 100 {
		return normalizeProportionally(variants, totalSpecified)
	}

	// Case 5: Some weights provided but sum >= 100 - normalize all proportionally
	return normalizeProportionally(variants, totalSpecified)
}

// distributeEqually distributes 100% equally across all variants
func distributeEqually(variants map[string]models.Variant) map[string]models.Variant {
	count := len(variants)
	baseWeight := 100 / count
	remainder := 100 % count

	normalized := make(map[string]models.Variant)
	
	// Sort keys for deterministic distribution of remainder
	keys := sortedKeys(variants)

	for i, key := range keys {
		variant := variants[key]
		weight := baseWeight
		if i < remainder {
			weight++ // Give extra 1% to first variants to reach 100
		}

		normalized[key] = models.Variant{
			Value:  variant.Value,
			Weight: &weight,
		}
	}

	return normalized
}

// distributeRemainder distributes remaining percentage equally to unweighted variants
func distributeRemainder(variants map[string]models.Variant, totalSpecified, unweightedCount int) map[string]models.Variant {
	remaining := 100 - totalSpecified
	baseWeight := remaining / unweightedCount
	remainder := remaining % unweightedCount

	normalized := make(map[string]models.Variant)
	
	// Sort keys for deterministic distribution
	keys := sortedKeys(variants)

	unweightedIndex := 0
	for _, key := range keys {
		variant := variants[key]
		
		if variant.Weight != nil {
			// Keep specified weight
			normalized[key] = variant
		} else {
			// Distribute from remaining
			weight := baseWeight
			if unweightedIndex < remainder {
				weight++
			}
			
			normalized[key] = models.Variant{
				Value:  variant.Value,
				Weight: &weight,
			}
			unweightedIndex++
		}
	}

	return normalized
}

// normalizeProportionally normalizes all weights proportionally to sum to 100
func normalizeProportionally(variants map[string]models.Variant, totalWeight int) map[string]models.Variant {
	if totalWeight == 0 {
		return distributeEqually(variants)
	}

	normalized := make(map[string]models.Variant)
	
	// First pass: calculate normalized weights
	calculatedTotal := 0
	for key, variant := range variants {
		weight := 0
		if variant.Weight != nil {
			// Proportional calculation: (weight / total) * 100
			weight = int(float64(*variant.Weight) / float64(totalWeight) * 100)
		}
		
		normalized[key] = models.Variant{
			Value:  variant.Value,
			Weight: &weight,
		}
		calculatedTotal += weight
	}

	// Adjust for rounding errors to ensure sum = 100
	if calculatedTotal != 100 {
		adjustWeightsForRounding(normalized, calculatedTotal)
	}

	return normalized
}

// adjustWeightsForRounding adjusts weights to ensure they sum to exactly 100
func adjustWeightsForRounding(variants map[string]models.Variant, calculatedTotal int) {
	diff := 100 - calculatedTotal
	
	// Sort keys for deterministic adjustment
	keys := make([]string, 0, len(variants))
	for key := range variants {
		keys = append(keys, key)
	}
	sortStrings(keys)

	// Add/subtract difference to first variant
	if len(keys) > 0 {
		firstKey := keys[0]
		variant := variants[firstKey]
		adjustedWeight := *variant.Weight + diff
		variant.Weight = &adjustedWeight
		variants[firstKey] = variant
	}
}

// sortedKeys returns sorted keys from a variant map
func sortedKeys(variants map[string]models.Variant) []string {
	keys := make([]string, 0, len(variants))
	for key := range variants {
		keys = append(keys, key)
	}
	sortStrings(keys)
	return keys
}

// sortStrings is a simple string sort for deterministic ordering
func sortStrings(s []string) {
	for i := 0; i < len(s); i++ {
		for j := i + 1; j < len(s); j++ {
			if s[i] > s[j] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}
