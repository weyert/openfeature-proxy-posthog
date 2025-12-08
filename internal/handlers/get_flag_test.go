package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/openfeature/posthog-proxy/internal/config"
	"github.com/openfeature/posthog-proxy/internal/models"
	"github.com/openfeature/posthog-proxy/internal/posthog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGetFlag_Success_BooleanFlag(t *testing.T) {
	// Arrange
	gin.SetMode(gin.TestMode)
	mockClient := new(posthog.MockClient)
	cfg := &config.Config{
		FeatureFlags: config.FeatureFlagsConfig{
			TypeCoercion: config.TypeCoercionConfig{
				CoerceNumericStrings: true,
				CoerceBooleanStrings: true,
			},
		},
	}
	handler := NewHandler(mockClient, cfg, nil)

	posthogFlag := models.PostHogFeatureFlag{
		ID:     12345,
		Key:    "test-boolean-flag",
		Name:   "Test Boolean Flag",
		Active: true,
		Filters: models.PostHogFilters{
			Groups: []models.PostHogFilterGroup{
				{
					Properties:        []models.PostHogProperty{},
					RolloutPercentage: ptrInt(100),
				},
			},
		},
	}

	mockClient.On("GetFeatureFlagByKey", mock.Anything, "test-boolean-flag").Return(&posthogFlag, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "key", Value: "test-boolean-flag"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/openfeature/v0/manifest/flags/test-boolean-flag", nil)

	// Act
	handler.GetFlag(c)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "read,write,delete", w.Header().Get("X-Manifest-Capabilities"))

	var response models.ManifestFlagResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	assert.NoError(t, err)

	assert.Equal(t, "test-boolean-flag", response.Flag.Key)
	assert.Equal(t, models.FlagTypeBoolean, response.Flag.Type)
	assert.Equal(t, true, response.Flag.DefaultValue) // Boolean flag with 100% rollout defaults to true
	mockClient.AssertExpectations(t)
}

func TestGetFlag_Success_StringFlag(t *testing.T) {
	// Arrange
	gin.SetMode(gin.TestMode)
	mockClient := new(posthog.MockClient)
	cfg := &config.Config{
		FeatureFlags: config.FeatureFlagsConfig{
			TypeCoercion: config.TypeCoercionConfig{
				CoerceNumericStrings: true,
				CoerceBooleanStrings: true,
			},
		},
	}
	handler := NewHandler(mockClient, cfg, nil)

	posthogFlag := models.PostHogFeatureFlag{
		ID:     12346,
		Key:    "test-string-flag",
		Name:   "Test String Flag",
		Active: true,
		Filters: models.PostHogFilters{
			Groups: []models.PostHogFilterGroup{
				{
					Properties:        []models.PostHogProperty{},
					RolloutPercentage: ptrInt(100),
					Variant:           ptrString("control"),
				},
			},
			Multivariate: &models.PostHogMultivariate{
				Variants: []models.PostHogVariant{
					{Key: "control", Name: "Control", RolloutFlag: 50},
					{Key: "test", Name: "Test", RolloutFlag: 50},
				},
			},
		},
	}

	mockClient.On("GetFeatureFlagByKey", mock.Anything, "test-string-flag").Return(&posthogFlag, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "key", Value: "test-string-flag"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/openfeature/v0/manifest/flags/test-string-flag", nil)

	// Act
	handler.GetFlag(c)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "read,write,delete", w.Header().Get("X-Manifest-Capabilities"))

	var response models.ManifestFlagResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	assert.NoError(t, err)

	assert.Equal(t, "test-string-flag", response.Flag.Key)
	assert.Equal(t, models.FlagTypeString, response.Flag.Type)
	assert.Equal(t, "control", response.Flag.DefaultValue) // First variant is default
	mockClient.AssertExpectations(t)
}

func TestGetFlag_FlagNotFound(t *testing.T) {
	// Arrange
	gin.SetMode(gin.TestMode)
	mockClient := new(posthog.MockClient)
	cfg := &config.Config{}
	handler := NewHandler(mockClient, cfg, nil)

	mockClient.On("GetFeatureFlagByKey", mock.Anything, "non-existent-flag").
		Return((*models.PostHogFeatureFlag)(nil), errors.New("flag not found"))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "key", Value: "non-existent-flag"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/openfeature/v0/manifest/flags/non-existent-flag", nil)

	// Act
	handler.GetFlag(c)

	// Assert
	assert.Equal(t, http.StatusNotFound, w.Code)

	var response models.ErrorResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, response.Code)
	mockClient.AssertExpectations(t)
}

func TestGetFlag_InactiveFlag(t *testing.T) {
	// Arrange
	gin.SetMode(gin.TestMode)
	mockClient := new(posthog.MockClient)
	cfg := &config.Config{}
	handler := NewHandler(mockClient, cfg, nil)

	posthogFlag := models.PostHogFeatureFlag{
		ID:     12347,
		Key:    "inactive-flag",
		Name:   "Inactive Flag",
		Active: false, // Inactive flag
		Filters: models.PostHogFilters{
			Groups: []models.PostHogFilterGroup{
				{
					RolloutPercentage: ptrInt(100),
				},
			},
		},
	}

	mockClient.On("GetFeatureFlagByKey", mock.Anything, "inactive-flag").Return(&posthogFlag, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "key", Value: "inactive-flag"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/openfeature/v0/manifest/flags/inactive-flag", nil)

	// Act
	handler.GetFlag(c)

	// Assert
	assert.Equal(t, http.StatusNotFound, w.Code)

	var response models.ErrorResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.Contains(t, response.Details, "inactive")
	mockClient.AssertExpectations(t)
}

func TestGetFlag_MissingKey(t *testing.T) {
	// Arrange
	gin.SetMode(gin.TestMode)
	mockClient := new(posthog.MockClient)
	cfg := &config.Config{}
	handler := NewHandler(mockClient, cfg, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "key", Value: ""}}
	c.Request = httptest.NewRequest(http.MethodGet, "/openfeature/v0/manifest/flags/", nil)

	// Act
	handler.GetFlag(c)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func ptrInt(i int) *int {
	return &i
}

func ptrString(s string) *string {
	return &s
}
