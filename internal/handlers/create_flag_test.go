package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/openfeature/posthog-proxy/internal/config"
	"github.com/openfeature/posthog-proxy/internal/models"
	"github.com/openfeature/posthog-proxy/internal/posthog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestHandler(t *testing.T, mockServer *httptest.Server) *Handler {
	cfg := &config.Config{
		PostHog: config.PostHogConfig{
			APIKey:    "test-key",
			Host:      mockServer.URL,
			ProjectID: "123",
		},
		FeatureFlags: config.FeatureFlagsConfig{
			TypeCoercion: config.TypeCoercionConfig{
				CoerceNumericStrings: true,
				CoerceBooleanStrings: true,
			},
			DefaultRolloutPercentage: 100,
			ArchiveInsteadOfDelete:   false,
		},
	}

	posthogClient := posthog.NewClient(cfg.PostHog, false)
	return NewHandler(posthogClient, cfg, nil)
}

func TestCreateFlag_Success_BooleanFlag(t *testing.T) {
	// Create mock PostHog server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/projects/123/feature_flags/", r.URL.Path)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		assert.Equal(t, http.MethodPost, r.Method)

		// Parse request body
		var reqBody models.PostHogCreateFlagRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)

		// Verify request
		assert.Equal(t, "test-boolean-flag", reqBody.Key)
		assert.Equal(t, "Test Boolean Flag", reqBody.Name)
		assert.True(t, reqBody.Active)
		assert.NotNil(t, reqBody.Filters)

		// Send mock response
		rollout := 100
		response := models.PostHogFeatureFlag{
			ID:     1,
			Key:    reqBody.Key,
			Name:   reqBody.Name,
			Active: reqBody.Active,
			Filters: models.PostHogFilters{
				Groups: []models.PostHogFilterGroup{
					{
						RolloutPercentage: &rollout,
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	handler := setupTestHandler(t, server)

	// Create request
	requestBody := models.CreateFlagRequest{
		Key:          "test-boolean-flag",
		Type:         models.FlagTypeBoolean,
		Description:  "Test Boolean Flag",
		DefaultValue: false,
	}

	body, err := json.Marshal(requestBody)
	require.NoError(t, err)

	// Setup Gin context
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/openfeature/v0/manifest/flags", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	// Execute
	handler.CreateFlag(c)

	// Assert
	assert.Equal(t, http.StatusCreated, w.Code)

	var response models.ManifestFlagResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "test-boolean-flag", response.Flag.Key)
	assert.Equal(t, models.FlagTypeBoolean, response.Flag.Type)
	assert.Equal(t, models.FlagStateEnabled, response.Flag.State)
}

func TestCreateFlag_Success_StringFlagWithVariants(t *testing.T) {
	// Create mock PostHog server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse request body
		var reqBody models.PostHogCreateFlagRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)

		// Verify multivariate configuration
		assert.NotNil(t, reqBody.Filters.Multivariate)
		assert.Len(t, reqBody.Filters.Multivariate.Variants, 3)

		// Verify weights sum to 100
		totalWeight := 0
		for _, variant := range reqBody.Filters.Multivariate.Variants {
			totalWeight += variant.RolloutFlag
		}
		assert.Equal(t, 100, totalWeight)

		// Send mock response
		rollout := 100
		response := models.PostHogFeatureFlag{
			ID:     2,
			Key:    reqBody.Key,
			Name:   reqBody.Name,
			Active: reqBody.Active,
			Filters: models.PostHogFilters{
				Groups: []models.PostHogFilterGroup{
					{
						RolloutPercentage: &rollout,
					},
				},
				Multivariate: reqBody.Filters.Multivariate,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	handler := setupTestHandler(t, server)

	// Create request with variants
	requestBody := models.CreateFlagRequest{
		Key:          "test-variant-flag",
		Type:         models.FlagTypeString,
		Description:  "Test Variant Flag",
		DefaultValue: "control",
		Variants: map[string]models.Variant{
			"control": {
				Value:  "control",
				Weight: &[]int{34}[0],
			},
			"variant-a": {
				Value:  "variant-a",
				Weight: &[]int{33}[0],
			},
			"variant-b": {
				Value:  "variant-b",
				Weight: &[]int{33}[0],
			},
		},
	}

	body, err := json.Marshal(requestBody)
	require.NoError(t, err)

	// Setup Gin context
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/openfeature/v0/manifest/flags", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	// Execute
	handler.CreateFlag(c)

	// Assert
	assert.Equal(t, http.StatusCreated, w.Code)

	var response models.ManifestFlagResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "test-variant-flag", response.Flag.Key)
	assert.Equal(t, models.FlagTypeString, response.Flag.Type)
	assert.NotNil(t, response.Flag.Variants)
	assert.Len(t, response.Flag.Variants, 3)
}

func TestCreateFlag_InvalidRequestBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Should not reach PostHog API")
	}))
	defer server.Close()

	handler := setupTestHandler(t, server)

	// Setup Gin context with invalid JSON
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/openfeature/v0/manifest/flags", bytes.NewBufferString("invalid json"))
	c.Request.Header.Set("Content-Type", "application/json")

	// Execute
	handler.CreateFlag(c)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response models.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Equal(t, "Invalid request body", response.Message)
}

func TestCreateFlag_PostHogError(t *testing.T) {
	// Create mock PostHog server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Internal server error",
		})
	}))
	defer server.Close()

	handler := setupTestHandler(t, server)

	// Create valid request
	requestBody := models.CreateFlagRequest{
		Key:          "test-flag",
		Type:         models.FlagTypeBoolean,
		Description:  "Test Flag",
		DefaultValue: false,
	}

	body, err := json.Marshal(requestBody)
	require.NoError(t, err)

	// Setup Gin context
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/openfeature/v0/manifest/flags", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	// Execute
	handler.CreateFlag(c)

	// Assert
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response models.ErrorResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, http.StatusInternalServerError, response.Code)
	assert.Equal(t, "Failed to create feature flag in PostHog", response.Message)
}

func TestCreateFlag_WeightNormalization(t *testing.T) {
	// Create mock PostHog server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody models.PostHogCreateFlagRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)

		// Verify weights were normalized
		totalWeight := 0
		for _, variant := range reqBody.Filters.Multivariate.Variants {
			totalWeight += variant.RolloutFlag
		}
		assert.Equal(t, 100, totalWeight, "Weights should sum to 100 after normalization")

		rollout := 100
		response := models.PostHogFeatureFlag{
			ID:     3,
			Key:    reqBody.Key,
			Name:   reqBody.Name,
			Active: true,
			Filters: models.PostHogFilters{
				Groups: []models.PostHogFilterGroup{
					{RolloutPercentage: &rollout},
				},
				Multivariate: reqBody.Filters.Multivariate,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	handler := setupTestHandler(t, server)

	// Create request with weights that don't sum to 100
	requestBody := models.CreateFlagRequest{
		Key:          "test-normalize-flag",
		Type:         models.FlagTypeString,
		Description:  "Test Normalize Flag",
		DefaultValue: "a",
		Variants: map[string]models.Variant{
			"a": {Value: "a", Weight: &[]int{10}[0]},
			"b": {Value: "b", Weight: &[]int{20}[0]},
			"c": {Value: "c", Weight: &[]int{30}[0]},
		},
	}

	body, err := json.Marshal(requestBody)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/openfeature/v0/manifest/flags", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.CreateFlag(c)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestCreateFlag_WithExpiry(t *testing.T) {
	expiry := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)

		var reqBody models.PostHogCreateFlagRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)

		require.Contains(t, reqBody.Tags, "expiry:2025-12-31T00:00:00Z")

		roll := 100
		response := models.PostHogFeatureFlag{
			ID:     10,
			Key:    reqBody.Key,
			Name:   reqBody.Name,
			Active: true,
			Tags:   reqBody.Tags,
			Filters: models.PostHogFilters{
				Groups: []models.PostHogFilterGroup{
					{RolloutPercentage: &roll},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	handler := setupTestHandler(t, server)

	requestBody := models.CreateFlagRequest{
		Key:          "expiry-flag",
		Name:         "Expiry Flag",
		Type:         models.FlagTypeBoolean,
		DefaultValue: true,
		Expiry:       &expiry,
	}

	body, err := json.Marshal(requestBody)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/openfeature/v0/manifest/flags", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.CreateFlag(c)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response models.ManifestFlagResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	require.NotNil(t, response.Flag.Expiry)
	assert.True(t, expiry.Equal(*response.Flag.Expiry))
}
