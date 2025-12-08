package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/openfeature/posthog-proxy/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetManifest_Success_EmptyFlags(t *testing.T) {
	// Create mock PostHog server with no flags
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/projects/123/feature_flags/", r.URL.Path)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		assert.Equal(t, http.MethodGet, r.Method)

		response := models.PostHogFeatureFlagsResponse{
			Results: []models.PostHogFeatureFlag{},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	handler := setupTestHandler(t, server)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/openfeature/v0/manifest", nil)

	// Execute
	handler.GetManifest(c)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "read,write,delete", w.Header().Get("X-Manifest-Capabilities"))

	var response models.Manifest
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.NotNil(t, response.Flags)
	assert.Empty(t, response.Flags)
}

func TestGetManifest_Success_MultipleFlags(t *testing.T) {
	// Create mock PostHog server with multiple flags
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rollout100 := 100
		rollout50 := 50
		response := models.PostHogFeatureFlagsResponse{
			Results: []models.PostHogFeatureFlag{
				{
					ID:     1,
					Key:    "boolean-flag",
					Name:   "Boolean Flag",
					Active: true,
					Filters: models.PostHogFilters{
						Groups: []models.PostHogFilterGroup{
							{RolloutPercentage: &rollout100},
						},
					},
				},
				{
					ID:     2,
					Key:    "string-flag",
					Name:   "String Flag",
					Active: true,
					Filters: models.PostHogFilters{
						Groups: []models.PostHogFilterGroup{
							{RolloutPercentage: &rollout50},
						},
						Multivariate: &models.PostHogMultivariate{
							Variants: []models.PostHogVariant{
								{Key: "control", RolloutFlag: 50},
								{Key: "test", RolloutFlag: 50},
							},
						},
					},
				},
				{
					ID:     3,
					Key:    "disabled-flag",
					Name:   "Disabled Flag",
					Active: false,
					Filters: models.PostHogFilters{
						Groups: []models.PostHogFilterGroup{
							{RolloutPercentage: &rollout100},
						},
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	handler := setupTestHandler(t, server)

	// Setup Gin context
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/openfeature/v0/manifest", nil)

	// Execute
	handler.GetManifest(c)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)

	var response models.Manifest
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.NotNil(t, response.Flags)
	assert.Len(t, response.Flags, 3)

	// Helper to find flag by key
	findFlag := func(key string) *models.ManifestFlag {
		for _, flag := range response.Flags {
			if flag.Key == key {
				return &flag
			}
		}
		return nil
	}

	// Verify boolean flag
	booleanFlag := findFlag("boolean-flag")
	assert.NotNil(t, booleanFlag)
	assert.Equal(t, models.FlagTypeBoolean, booleanFlag.Type)
	assert.Equal(t, models.FlagStateEnabled, booleanFlag.State)
	assert.Nil(t, booleanFlag.Variants)

	// Verify string flag with variants
	stringFlag := findFlag("string-flag")
	assert.NotNil(t, stringFlag)
	assert.Equal(t, models.FlagTypeString, stringFlag.Type)
	assert.Equal(t, models.FlagStateEnabled, stringFlag.State)
	assert.NotNil(t, stringFlag.Variants)
	assert.Len(t, stringFlag.Variants, 2)

	// Verify disabled flag
	disabledFlag := findFlag("disabled-flag")
	assert.NotNil(t, disabledFlag)
	assert.Equal(t, models.FlagStateDisabled, disabledFlag.State)
}

func TestGetManifest_Success_VariantWeights(t *testing.T) {
	// Create mock PostHog server with variant flags
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rollout100 := 100
		response := models.PostHogFeatureFlagsResponse{
			Results: []models.PostHogFeatureFlag{
				{
					ID:     1,
					Key:    "multi-variant-flag",
					Name:   "Multi Variant Flag",
					Active: true,
					Filters: models.PostHogFilters{
						Groups: []models.PostHogFilterGroup{
							{RolloutPercentage: &rollout100},
						},
						Multivariate: &models.PostHogMultivariate{
							Variants: []models.PostHogVariant{
								{Key: "control", RolloutFlag: 25},
								{Key: "variant-a", RolloutFlag: 25},
								{Key: "variant-b", RolloutFlag: 25},
								{Key: "variant-c", RolloutFlag: 25},
							},
						},
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	handler := setupTestHandler(t, server)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/openfeature/v0/manifest", nil)

	handler.GetManifest(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response models.Manifest
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Find the flag by key
	assert.Len(t, response.Flags, 1)
	flag := response.Flags[0]
	assert.Equal(t, "multi-variant-flag", flag.Key)
	assert.Len(t, flag.Variants, 4)

	// Verify each variant has correct weight
	for _, variant := range flag.Variants {
		require.NotNil(t, variant.Weight)
		assert.Equal(t, 25, *variant.Weight)
	}

	// Verify total weight sums to 100
	totalWeight := 0
	for _, variant := range flag.Variants {
		totalWeight += *variant.Weight
	}
	assert.Equal(t, 100, totalWeight)
}

func TestGetManifest_PostHogError(t *testing.T) {
	// Create mock PostHog server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Internal server error",
		})
	}))
	defer server.Close()

	handler := setupTestHandler(t, server)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/openfeature/v0/manifest", nil)

	handler.GetManifest(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response models.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, http.StatusInternalServerError, response.Code)
	assert.Equal(t, "Failed to retrieve feature flags from PostHog", response.Message)
}

func TestGetManifest_TypeCoercion(t *testing.T) {
	tests := []struct {
		name          string
		typeCoercion  bool
		expectedType  string
	}{
		{
			name:         "With type coercion enabled",
			typeCoercion: true,
			expectedType: "boolean",
		},
		{
			name:         "With type coercion disabled",
			typeCoercion: false,
			expectedType: "boolean",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				rollout100 := 100
				response := models.PostHogFeatureFlagsResponse{
					Results: []models.PostHogFeatureFlag{
						{
							ID:     1,
							Key:    "test-flag",
							Name:   "Test Flag",
							Active: true,
							Filters: models.PostHogFilters{
								Groups: []models.PostHogFilterGroup{
									{RolloutPercentage: &rollout100},
								},
							},
						},
					},
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()

			handler := setupTestHandler(t, server)
			handler.config.FeatureFlags.TypeCoercion.CoerceNumericStrings = tt.typeCoercion
			handler.config.FeatureFlags.TypeCoercion.CoerceBooleanStrings = tt.typeCoercion

			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/openfeature/v0/manifest", nil)

			handler.GetManifest(c)

			assert.Equal(t, http.StatusOK, w.Code)

			var response models.Manifest
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Len(t, response.Flags, 1)
			flag := response.Flags[0]
			assert.Equal(t, "test-flag", flag.Key)
			assert.Equal(t, models.FlagType(tt.expectedType), flag.Type)
		})
	}
}

func TestGetManifest_LargeFlagSet(t *testing.T) {
	// Test with large number of flags to ensure performance
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Generate 100 flags
		rollout100 := 100
		flags := make([]models.PostHogFeatureFlag, 100)
		for i := 0; i < 100; i++ {
			flags[i] = models.PostHogFeatureFlag{
				ID:     i + 1,
				Key:    "flag-" + string(rune(i)),
				Name:   "Flag " + string(rune(i)),
				Active: true,
				Filters: models.PostHogFilters{
					Groups: []models.PostHogFilterGroup{
						{RolloutPercentage: &rollout100},
					},
				},
			}
		}

		response := models.PostHogFeatureFlagsResponse{
			Results: flags,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	handler := setupTestHandler(t, server)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/openfeature/v0/manifest", nil)

	handler.GetManifest(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response models.Manifest
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Len(t, response.Flags, 100)
}
