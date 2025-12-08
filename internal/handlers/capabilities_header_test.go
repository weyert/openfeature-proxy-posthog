package handlers

import (
	"bytes"
	"encoding/json"
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

// TestCapabilitiesHeader_GetManifest verifies that GET /manifest returns X-Manifest-Capabilities header
func TestCapabilitiesHeader_GetManifest(t *testing.T) {
	// Create mock PostHog server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	// Act
	handler.GetManifest(c)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "read,write,delete", w.Header().Get("X-Manifest-Capabilities"),
		"GET /manifest must include X-Manifest-Capabilities header")
}

// TestCapabilitiesHeader_GetFlag verifies that GET /manifest/flags/:key returns X-Manifest-Capabilities header
func TestCapabilitiesHeader_GetFlag(t *testing.T) {
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

	rollout := 100
	posthogFlag := models.PostHogFeatureFlag{
		ID:     12345,
		Key:    "test-flag",
		Name:   "Test Flag",
		Active: true,
		Filters: models.PostHogFilters{
			Groups: []models.PostHogFilterGroup{
				{RolloutPercentage: &rollout},
			},
		},
	}

	mockClient.On("GetFeatureFlagByKey", mock.Anything, "test-flag").Return(&posthogFlag, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "key", Value: "test-flag"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/openfeature/v0/manifest/flags/test-flag", nil)

	// Act
	handler.GetFlag(c)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "read,write,delete", w.Header().Get("X-Manifest-Capabilities"),
		"GET /manifest/flags/:key must include X-Manifest-Capabilities header")
	mockClient.AssertExpectations(t)
}

// TestCapabilitiesHeader_CreateFlag verifies that POST /manifest/flags returns X-Manifest-Capabilities header
func TestCapabilitiesHeader_CreateFlag(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockClient := new(posthog.MockClient)
	cfg := &config.Config{}
	handler := NewHandler(mockClient, cfg, nil)

	requestBody := models.CreateFlagRequest{
		Key:          "new-flag",
		Name:         "New Flag",
		Type:         models.FlagTypeBoolean,
		Description:  "Test flag",
		DefaultValue: true,
	}

	createdFlag := models.PostHogFeatureFlag{
		ID:     99999,
		Key:    "new-flag",
		Name:   "New Flag",
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

	mockClient.On("CreateFeatureFlag", mock.Anything, mock.AnythingOfType("models.PostHogCreateFlagRequest")).
		Return(&createdFlag, nil)

	body, _ := json.Marshal(requestBody)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/openfeature/v0/manifest/flags", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	// Act
	handler.CreateFlag(c)

	// Assert
	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "read,write,delete", w.Header().Get("X-Manifest-Capabilities"),
		"POST /manifest/flags must include X-Manifest-Capabilities header")
	mockClient.AssertExpectations(t)
}

// TestCapabilitiesHeader_UpdateFlag verifies that PATCH /manifest/flags/:key returns X-Manifest-Capabilities header
func TestCapabilitiesHeader_UpdateFlag(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockClient := new(posthog.MockClient)
	cfg := &config.Config{}
	handler := NewHandler(mockClient, cfg, nil)

	existingFlag := models.PostHogFeatureFlag{
		ID:     12345,
		Key:    "existing-flag",
		Name:   "Existing Flag",
		Active: true,
		Filters: models.PostHogFilters{
			Groups: []models.PostHogFilterGroup{
				{RolloutPercentage: ptrInt(100)},
			},
		},
	}

	updatedFlag := existingFlag
	updatedFlag.Name = "Updated Flag"

	mockClient.On("GetFeatureFlagByKey", mock.Anything, "existing-flag").Return(&existingFlag, nil)
	mockClient.On("UpdateFeatureFlag", mock.Anything, 12345, mock.AnythingOfType("models.PostHogUpdateFlagRequest")).
		Return(&updatedFlag, nil)

	updateRequest := models.UpdateFlagRequest{
		Name: ptrStr("Updated Flag"),
	}

	body, _ := json.Marshal(updateRequest)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "key", Value: "existing-flag"}}
	c.Request = httptest.NewRequest(http.MethodPatch, "/openfeature/v0/manifest/flags/existing-flag", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	// Act
	handler.UpdateFlag(c)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "read,write,delete", w.Header().Get("X-Manifest-Capabilities"),
		"PATCH /manifest/flags/:key must include X-Manifest-Capabilities header")
	mockClient.AssertExpectations(t)
}

// TestCapabilitiesHeader_DeleteFlag verifies that DELETE /manifest/flags/:key returns X-Manifest-Capabilities header
func TestCapabilitiesHeader_DeleteFlag(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockClient := new(posthog.MockClient)
	cfg := &config.Config{}
	handler := NewHandler(mockClient, cfg, nil)

	existingFlag := models.PostHogFeatureFlag{
		ID:     12345,
		Key:    "flag-to-delete",
		Name:   "Flag to Delete",
		Active: true,
	}

	mockClient.On("GetFeatureFlagByKey", mock.Anything, "flag-to-delete").Return(&existingFlag, nil)
	mockClient.On("DeleteFeatureFlag", mock.Anything, 12345).Return(nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "key", Value: "flag-to-delete"}}
	c.Request = httptest.NewRequest(http.MethodDelete, "/openfeature/v0/manifest/flags/flag-to-delete", nil)

	// Act
	handler.DeleteFlag(c)

	// Assert
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "read,write,delete", w.Header().Get("X-Manifest-Capabilities"),
		"DELETE /manifest/flags/:key must include X-Manifest-Capabilities header")
	mockClient.AssertExpectations(t)
}

// TestCapabilitiesHeader_AllEndpoints is a comprehensive test that verifies all endpoints return the header
func TestCapabilitiesHeader_AllEndpoints(t *testing.T) {
	tests := []struct {
		name           string
		setupHandler   func() (*Handler, *httptest.Server)
		setupRequest   func() (*gin.Context, *httptest.ResponseRecorder)
		handlerFunc    func(*Handler, *gin.Context)
		expectedStatus int
	}{
		{
			name: "GET /manifest",
			setupHandler: func() (*Handler, *httptest.Server) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					response := models.PostHogFeatureFlagsResponse{Results: []models.PostHogFeatureFlag{}}
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(response)
				}))
				handler := setupTestHandler(t, server)
				return handler, server
			},
			setupRequest: func() (*gin.Context, *httptest.ResponseRecorder) {
				gin.SetMode(gin.TestMode)
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)
				c.Request = httptest.NewRequest(http.MethodGet, "/openfeature/v0/manifest", nil)
				return c, w
			},
			handlerFunc: func(h *Handler, c *gin.Context) {
				h.GetManifest(c)
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, server := tt.setupHandler()
			if server != nil {
				defer server.Close()
			}

			c, w := tt.setupRequest()
			tt.handlerFunc(handler, c)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, "read,write,delete", w.Header().Get("X-Manifest-Capabilities"),
				"Endpoint %s must include X-Manifest-Capabilities header", tt.name)
		})
	}
}

func ptrStr(s string) *string {
	return &s
}
