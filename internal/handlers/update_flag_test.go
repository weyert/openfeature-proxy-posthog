package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/openfeature/posthog-proxy/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateFlag_Success_BasicUpdate(t *testing.T) {
	// Create mock PostHog server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rollout := 100
		if r.Method == http.MethodGet {
			// GET request to find flag by key - return single flag object
			assert.Equal(t, "/api/projects/123/feature_flags/test-flag/", r.URL.Path)
			
			response := models.PostHogFeatureFlag{
				ID:     1,
				Key:    "test-flag",
				Name:   "Old Name",
				Active: true,
				Filters: models.PostHogFilters{
					Groups: []models.PostHogFilterGroup{
						{RolloutPercentage: &rollout},
					},
				},
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		} else if r.Method == http.MethodPatch {
			// PATCH request to update flag
			var reqBody models.PostHogUpdateFlagRequest
			err := json.NewDecoder(r.Body).Decode(&reqBody)
			require.NoError(t, err)

			// Verify update request
			assert.NotNil(t, reqBody.Name)
			assert.Equal(t, "Updated Name", *reqBody.Name)

			// Send mock response
			response := models.PostHogFeatureFlag{
				ID:     1,
				Key:    "test-flag",
				Name:   *reqBody.Name,
				Active: true,
				Filters: models.PostHogFilters{
					Groups: []models.PostHogFilterGroup{
						{RolloutPercentage: &rollout},
					},
				},
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		}
	}))
	defer server.Close()

	handler := setupTestHandler(t, server)

	// Create update request
	name := "Updated Name"
	requestBody := models.UpdateFlagRequest{
		Description: &name,
	}

	body, err := json.Marshal(requestBody)
	require.NoError(t, err)

	// Setup Gin context
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{gin.Param{Key: "key", Value: "test-flag"}}
	c.Request = httptest.NewRequest(http.MethodPut, "/openfeature/v0/manifest/flags/test-flag", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	// Execute
	handler.UpdateFlag(c)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)

	var response models.ManifestFlagResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "test-flag", response.Flag.Key)
	assert.Equal(t, "Updated Name", response.Flag.Description)
}

func TestUpdateFlag_Success_UpdateVariants(t *testing.T) {
	rollout := 100
	// Create mock PostHog server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			// Return existing flag with variants
			response := models.PostHogFeatureFlagsResponse{
				Results: []models.PostHogFeatureFlag{
					{
						ID:     2,
						Key:    "variant-flag",
						Name:   "Variant Flag",
						Active: true,
						Filters: models.PostHogFilters{
							Groups: []models.PostHogFilterGroup{
								{RolloutPercentage: &rollout},
							},
							Multivariate: &models.PostHogMultivariate{
								Variants: []models.PostHogVariant{
									{Key: "control", RolloutFlag: 50},
									{Key: "test", RolloutFlag: 50},
								},
							},
						},
					},
				},
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		} else if r.Method == http.MethodPatch {
			// Verify updated variants
			var reqBody models.PostHogUpdateFlagRequest
			err := json.NewDecoder(r.Body).Decode(&reqBody)
			require.NoError(t, err)

			assert.NotNil(t, reqBody.Filters)
			assert.NotNil(t, reqBody.Filters.Multivariate)
			
			// Verify weights sum to 100
			totalWeight := 0
			for _, variant := range reqBody.Filters.Multivariate.Variants {
				totalWeight += variant.RolloutFlag
			}
			assert.Equal(t, 100, totalWeight)

			response := models.PostHogFeatureFlag{
				ID:     2,
				Key:    "variant-flag",
				Name:   "Variant Flag",
				Active: true,
				Filters: *reqBody.Filters,
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		}
	}))
	defer server.Close()

	handler := setupTestHandler(t, server)

	// Create update request with new variants
	weight25 := 25
	variants := map[string]models.Variant{
		"control":   {Weight: &weight25},
		"variant-a": {Weight: &weight25},
		"variant-b": {Weight: &weight25},
		"variant-c": {Weight: &weight25},
	}
	requestBody := models.UpdateFlagRequest{
		Variants: &variants,
	}

	body, err := json.Marshal(requestBody)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{gin.Param{Key: "key", Value: "variant-flag"}}
	c.Request = httptest.NewRequest(http.MethodPut, "/openfeature/v0/manifest/flags/variant-flag", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateFlag(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response models.ManifestFlagResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "variant-flag", response.Flag.Key)
	assert.NotNil(t, response.Flag.Variants)
	assert.Len(t, response.Flag.Variants, 4)
}

func TestUpdateFlag_MissingKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Should not reach PostHog API")
	}))
	defer server.Close()

	handler := setupTestHandler(t, server)

	requestBody := models.UpdateFlagRequest{}
	body, _ := json.Marshal(requestBody)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{gin.Param{Key: "key", Value: ""}}
	c.Request = httptest.NewRequest(http.MethodPut, "/openfeature/v0/manifest/flags/", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateFlag(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response models.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Flag key is required", response.Message)
}

func TestUpdateFlag_InvalidRequestBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Should not reach PostHog API")
	}))
	defer server.Close()

	handler := setupTestHandler(t, server)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{gin.Param{Key: "key", Value: "test-flag"}}
	c.Request = httptest.NewRequest(http.MethodPut, "/openfeature/v0/manifest/flags/test-flag", bytes.NewBufferString("invalid json"))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateFlag(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response models.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Invalid request body", response.Message)
}

func TestUpdateFlag_FlagNotFound(t *testing.T) {
	// Create mock PostHog server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			assert.Equal(t, "/api/projects/123/feature_flags/non-existent-flag/", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"detail": "Not found",
			})
		}
	}))
	defer server.Close()

	handler := setupTestHandler(t, server)

	requestBody := models.UpdateFlagRequest{}
	body, _ := json.Marshal(requestBody)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{gin.Param{Key: "key", Value: "non-existent-flag"}}
	c.Request = httptest.NewRequest(http.MethodPut, "/openfeature/v0/manifest/flags/non-existent-flag", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateFlag(c)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response models.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Feature flag not found", response.Message)
}

func TestUpdateFlag_InvalidVariantWeights(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Should not reach PostHog API with invalid weights")
	}))
	defer server.Close()

	handler := setupTestHandler(t, server)

	// Create request with invalid variant weights (empty variants)
	variants := map[string]models.Variant{}
	requestBody := models.UpdateFlagRequest{
		Variants: &variants,
	}

	body, _ := json.Marshal(requestBody)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{gin.Param{Key: "key", Value: "test-flag"}}
	c.Request = httptest.NewRequest(http.MethodPut, "/openfeature/v0/manifest/flags/test-flag", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateFlag(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response models.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Invalid variant configuration", response.Message)
}

func TestUpdateFlag_PostHogUpdateError(t *testing.T) {
	rollout := 100
	// Create mock PostHog server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			response := models.PostHogFeatureFlagsResponse{
				Results: []models.PostHogFeatureFlag{
					{
						ID:     1,
						Key:    "test-flag",
						Name:   "Test Flag",
						Active: true,
						Filters: models.PostHogFilters{
							Groups: []models.PostHogFilterGroup{
								{RolloutPercentage: &rollout},
							},
						},
					},
				},
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		} else if r.Method == http.MethodPatch {
			// Return error on update
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Internal server error",
			})
		}
	}))
	defer server.Close()

	handler := setupTestHandler(t, server)

	name := "Updated Name"
	requestBody := models.UpdateFlagRequest{
		Description: &name,
	}

	body, _ := json.Marshal(requestBody)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{gin.Param{Key: "key", Value: "test-flag"}}
	c.Request = httptest.NewRequest(http.MethodPut, "/openfeature/v0/manifest/flags/test-flag", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateFlag(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response models.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Failed to update feature flag in PostHog", response.Message)
}

// TestUpdateFlag_KeyToIDLookup specifically tests that the handler:
// 1. Receives a flag key in the URL path
// 2. Looks up the flag by key using GetFeatureFlagByKey 
// 3. Uses the numeric ID from the response to update the flag
// This is critical because PostHog PATCH endpoints require numeric IDs, not keys
func TestUpdateFlag_KeyToIDLookup(t *testing.T) {
	rollout := 100
	requestCount := 0
	
	// Create mock PostHog server that tracks requests
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		
		if r.Method == http.MethodGet {
			// First request: GET by key to find the flag
			assert.Equal(t, "/api/projects/123/feature_flags/my-test-flag/", r.URL.Path,
				"GET request should use flag key in URL")
			
			response := models.PostHogFeatureFlag{
				ID:     12345, // This is the numeric ID we need
				Key:    "my-test-flag",
				Name:   "My Test Flag",
				Active: true,
				Filters: models.PostHogFilters{
					Groups: []models.PostHogFilterGroup{
						{RolloutPercentage: &rollout},
					},
				},
			}
			
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			
		} else if r.Method == http.MethodPatch {
			// Second request: PATCH using numeric ID
			assert.Equal(t, "/api/projects/123/feature_flags/12345/", r.URL.Path,
				"PATCH request should use numeric ID (12345) in URL, not key")
			
			var reqBody models.PostHogUpdateFlagRequest
			err := json.NewDecoder(r.Body).Decode(&reqBody)
			require.NoError(t, err)
			
			// Verify the update
			assert.NotNil(t, reqBody.Name)
			assert.Equal(t, "Updated Name", *reqBody.Name)
			
			// Send success response
			response := models.PostHogFeatureFlag{
				ID:     12345,
				Key:    "my-test-flag",
				Name:   "Updated Name",
				Active: true,
				Filters: models.PostHogFilters{
					Groups: []models.PostHogFilterGroup{
						{RolloutPercentage: &rollout},
					},
				},
			}
			
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		}
	}))
	defer server.Close()

	handler := setupTestHandler(t, server)

	// Create update request with just a name change
	name := "Updated Name"
	requestBody := models.UpdateFlagRequest{
		Name: &name,
	}

	body, _ := json.Marshal(requestBody)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	
	// Important: The URL path contains the flag KEY, not the ID
	c.Params = gin.Params{gin.Param{Key: "key", Value: "my-test-flag"}}
	c.Request = httptest.NewRequest(http.MethodPut, "/openfeature/v0/manifest/flags/my-test-flag", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateFlag(c)

	// Verify success
	assert.Equal(t, http.StatusOK, w.Code)
	
	// Verify we made both requests (GET by key, then PATCH by ID)
	assert.Equal(t, 2, requestCount, "Should make 2 requests: GET by key, then PATCH by ID")
}
