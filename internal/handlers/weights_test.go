package handlers

import (
	"testing"

	"github.com/openfeature/posthog-proxy/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestValidateVariantWeights_EmptyVariants(t *testing.T) {
	err := ValidateVariantWeights(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one variant")

	err = ValidateVariantWeights(map[string]models.Variant{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one variant")
}

func TestValidateVariantWeights_ValidVariants(t *testing.T) {
	weight := 100
	variants := map[string]models.Variant{
		"control": {Weight: &weight},
	}
	
	err := ValidateVariantWeights(variants)
	assert.NoError(t, err)
}

func TestNormalizeVariantWeights_AllWeightsSpecifiedSum100(t *testing.T) {
	// Case 1: All weights provided and sum to 100 - use as-is
	weight25 := 25
	weight50 := 50
	
	variants := map[string]models.Variant{
		"control":   {Weight: &weight25},
		"variant-a": {Weight: &weight50},
		"variant-b": {Weight: &weight25},
	}
	
	normalized := NormalizeVariantWeights(variants)
	
	// Verify all weights preserved
	assert.Equal(t, 25, *normalized["control"].Weight)
	assert.Equal(t, 50, *normalized["variant-a"].Weight)
	assert.Equal(t, 25, *normalized["variant-b"].Weight)
	
	// Verify sum to 100
	total := 0
	for _, v := range normalized {
		total += *v.Weight
	}
	assert.Equal(t, 100, total)
}

func TestNormalizeVariantWeights_NoWeightsSpecified(t *testing.T) {
	// Case 2: No weights provided - distribute equally
	variants := map[string]models.Variant{
		"control":   {},
		"variant-a": {},
		"variant-b": {},
	}
	
	normalized := NormalizeVariantWeights(variants)
	
	// With 3 variants: 100/3 = 33 each, with 1 remainder
	// Should be 34, 33, 33
	total := 0
	weights := []int{}
	for _, v := range normalized {
		assert.NotNil(t, v.Weight)
		weights = append(weights, *v.Weight)
		total += *v.Weight
	}
	
	assert.Equal(t, 100, total)
	assert.Contains(t, weights, 34) // One variant gets the extra 1%
	assert.Contains(t, weights, 33)
}

func TestNormalizeVariantWeights_SomeWeightsSpecified(t *testing.T) {
	// Case 3: Some weights provided - distribute remainder
	weight40 := 40
	weight30 := 30
	
	variants := map[string]models.Variant{
		"control":   {Weight: &weight40},
		"variant-a": {Weight: &weight30},
		"variant-b": {}, // No weight specified
		"variant-c": {}, // No weight specified
	}
	
	normalized := NormalizeVariantWeights(variants)
	
	// control and variant-a should keep their weights
	assert.Equal(t, 40, *normalized["control"].Weight)
	assert.Equal(t, 30, *normalized["variant-a"].Weight)
	
	// Remaining 30% distributed: 15% each
	assert.Equal(t, 15, *normalized["variant-b"].Weight)
	assert.Equal(t, 15, *normalized["variant-c"].Weight)
	
	// Verify sum to 100
	total := 0
	for _, v := range normalized {
		total += *v.Weight
	}
	assert.Equal(t, 100, total)
}

func TestNormalizeVariantWeights_AllWeightsSpecifiedDontSum100(t *testing.T) {
	// Case 4: All weights provided but don't sum to 100 - normalize proportionally
	weight20 := 20
	weight30 := 30
	weight40 := 40
	// Total = 90, need to normalize to 100
	
	variants := map[string]models.Variant{
		"control":   {Weight: &weight20},
		"variant-a": {Weight: &weight30},
		"variant-b": {Weight: &weight40},
	}
	
	normalized := NormalizeVariantWeights(variants)
	
	// Proportional: 20/90*100 = 22.22 -> 22
	//               30/90*100 = 33.33 -> 33
	//               40/90*100 = 44.44 -> 44
	// But we need to adjust for rounding to sum to 100
	assert.NotNil(t, normalized["control"].Weight)
	assert.NotNil(t, normalized["variant-a"].Weight)
	assert.NotNil(t, normalized["variant-b"].Weight)
	
	// Verify sum to 100
	total := 0
	for _, v := range normalized {
		total += *v.Weight
	}
	assert.Equal(t, 100, total)
	
	// Check proportions are maintained approximately
	assert.True(t, *normalized["control"].Weight < *normalized["variant-a"].Weight)
	assert.True(t, *normalized["variant-a"].Weight < *normalized["variant-b"].Weight)
}

func TestNormalizeVariantWeights_SomeWeightsSumOver100(t *testing.T) {
	// Case 5: Some weights sum >= 100 - normalize all proportionally
	weight60 := 60
	weight50 := 50
	// Total specified = 110, more than 100
	
	variants := map[string]models.Variant{
		"control":   {Weight: &weight60},
		"variant-a": {Weight: &weight50},
		"variant-b": {}, // No weight specified
	}
	
	normalized := NormalizeVariantWeights(variants)
	
	// All should be normalized proportionally
	total := 0
	for _, v := range normalized {
		assert.NotNil(t, v.Weight)
		total += *v.Weight
	}
	assert.Equal(t, 100, total)
}

func TestNormalizeVariantWeights_TwoVariantsEqual(t *testing.T) {
	// Common case: A/B test with two equal variants
	variants := map[string]models.Variant{
		"control":   {},
		"variant-a": {},
	}
	
	normalized := NormalizeVariantWeights(variants)
	
	// Should be 50/50
	assert.Equal(t, 50, *normalized["control"].Weight)
	assert.Equal(t, 50, *normalized["variant-a"].Weight)
}

func TestNormalizeVariantWeights_FourVariantsEqual(t *testing.T) {
	// Four variants: 100/4 = 25 each
	variants := map[string]models.Variant{
		"control":   {},
		"variant-a": {},
		"variant-b": {},
		"variant-c": {},
	}
	
	normalized := NormalizeVariantWeights(variants)
	
	// Should all be 25%
	for key, v := range normalized {
		assert.Equal(t, 25, *v.Weight, "Variant %s should have 25%%", key)
	}
	
	// Verify sum
	total := 0
	for _, v := range normalized {
		total += *v.Weight
	}
	assert.Equal(t, 100, total)
}

func TestNormalizeVariantWeights_RemainderDistribution(t *testing.T) {
	// Test with 3 variants where 100/3 = 33 remainder 1
	variants := map[string]models.Variant{
		"a": {},
		"b": {},
		"c": {},
	}
	
	normalized := NormalizeVariantWeights(variants)
	
	// One should be 34, two should be 33
	weights := []int{}
	for _, v := range normalized {
		weights = append(weights, *v.Weight)
	}
	
	assert.Contains(t, weights, 34)
	assert.Contains(t, weights, 33)
	
	total := 0
	for _, v := range normalized {
		total += *v.Weight
	}
	assert.Equal(t, 100, total)
}

func TestNormalizeVariantWeights_SingleVariant(t *testing.T) {
	// Edge case: single variant should get 100%
	variants := map[string]models.Variant{
		"control": {},
	}
	
	normalized := NormalizeVariantWeights(variants)
	
	assert.Equal(t, 100, *normalized["control"].Weight)
}

func TestNormalizeVariantWeights_PreservesNonWeightFields(t *testing.T) {
	// Ensure normalization preserves other variant fields
	weight50 := 50
	variants := map[string]models.Variant{
		"control": {
			Value:  "control-value",
			Weight: &weight50,
		},
		"variant-a": {
			Value:  "variant-a-value",
			Weight: &weight50,
		},
	}
	
	normalized := NormalizeVariantWeights(variants)
	
	// Verify weights
	assert.Equal(t, 50, *normalized["control"].Weight)
	assert.Equal(t, 50, *normalized["variant-a"].Weight)
	
	// Verify other fields preserved
	assert.Equal(t, "control-value", normalized["control"].Value)
	assert.Equal(t, "variant-a-value", normalized["variant-a"].Value)
}
