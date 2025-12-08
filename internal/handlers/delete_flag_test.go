package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/openfeature/posthog-proxy/internal/config"
"github.com/openfeature/posthog-proxy/internal/posthog"
	"github.com/openfeature/posthog-proxy/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteFlag_Success_HardDelete(t *testing.T) {
	// Create mock PostHog server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			// GET request to find flag by key - return single flag object
			assert.Equal(t, "/api/projects/123/feature_flags/test-flag/", r.URL.Path)
			
			response := models.PostHogFeatureFlag{
				ID:     1,
				Key:    "test-flag",
				Name:   "Test Flag",
				Active: true,
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		} else if r.Method == http.MethodDelete {
			// DELETE request
			assert.Contains(t, r.URL.Path, "/feature_flags/1")
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()

	handler := setupTestHandler(t, server)
	// Ensure hard delete is enabled
	handler.config.FeatureFlags.ArchiveInsteadOfDelete = false

	// Setup Gin context
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{gin.Param{Key: "key", Value: "test-flag"}}
	c.Request = httptest.NewRequest(http.MethodDelete, "/openfeature/v0/manifest/flags/test-flag", nil)

	// Execute
	handler.DeleteFlag(c)

	// Assert
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestDeleteFlag_Success_Archive(t *testing.T) {
	// Create mock PostHog server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			// GET request to find flag by key - return single flag object
			assert.Equal(t, "/api/projects/123/feature_flags/archive-flag/", r.URL.Path)
			
			response := models.PostHogFeatureFlag{
				ID:     2,
				Key:    "archive-flag",
				Name:   "Archive Flag",
				Active: true,
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		} else if r.Method == http.MethodPatch {
			// PATCH request to archive (set active=false)
			var reqBody models.PostHogUpdateFlagRequest
			err := json.NewDecoder(r.Body).Decode(&reqBody)
			require.NoError(t, err)

			// Verify it's being set to inactive
			assert.NotNil(t, reqBody.Active)
			assert.False(t, *reqBody.Active)

			response := models.PostHogFeatureFlag{
				ID:     2,
				Key:    "archive-flag",
				Name:   "Archive Flag",
				Active: false,
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		}
	}))
	defer server.Close()

	handler := setupTestHandler(t, server)
	// Enable archive mode
	handler.config.FeatureFlags.ArchiveInsteadOfDelete = true

	// Setup Gin context
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{gin.Param{Key: "key", Value: "archive-flag"}}
	c.Request = httptest.NewRequest(http.MethodDelete, "/openfeature/v0/manifest/flags/archive-flag", nil)

	// Execute
	handler.DeleteFlag(c)

	// Assert
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestDeleteFlag_MissingKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Should not reach PostHog API")
	}))
	defer server.Close()

	handler := setupTestHandler(t, server)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{gin.Param{Key: "key", Value: ""}}
	c.Request = httptest.NewRequest(http.MethodDelete, "/openfeature/v0/manifest/flags/", nil)

	handler.DeleteFlag(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response models.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Flag key is required", response.Message)
}

func TestDeleteFlag_FlagNotFound(t *testing.T) {
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

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{gin.Param{Key: "key", Value: "non-existent-flag"}}
	c.Request = httptest.NewRequest(http.MethodDelete, "/openfeature/v0/manifest/flags/non-existent-flag", nil)

	handler.DeleteFlag(c)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response models.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Feature flag not found", response.Message)
}

func TestDeleteFlag_ArchiveError(t *testing.T) {
	// Create mock PostHog server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			assert.Equal(t, "/api/projects/123/feature_flags/error-flag/", r.URL.Path)
			
			response := models.PostHogFeatureFlag{
				ID:     3,
				Key:    "error-flag",
				Name:   "Error Flag",
				Active: true,
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		} else if r.Method == http.MethodPatch {
			// Return error on archive
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Internal server error",
			})
		}
	}))
	defer server.Close()

	handler := setupTestHandler(t, server)
	handler.config.FeatureFlags.ArchiveInsteadOfDelete = true

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{gin.Param{Key: "key", Value: "error-flag"}}
	c.Request = httptest.NewRequest(http.MethodDelete, "/openfeature/v0/manifest/flags/error-flag", nil)

	handler.DeleteFlag(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response models.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Failed to archive feature flag in PostHog", response.Message)
}

func TestDeleteFlag_HardDeleteError(t *testing.T) {
	// Create mock PostHog server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			assert.Equal(t, "/api/projects/123/feature_flags/delete-error-flag/", r.URL.Path)
			
			response := models.PostHogFeatureFlag{
				ID:     4,
				Key:    "delete-error-flag",
				Name:   "Delete Error Flag",
				Active: true,
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		} else if r.Method == http.MethodDelete {
			// Return error on delete
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Internal server error",
			})
		}
	}))
	defer server.Close()

	handler := setupTestHandler(t, server)
	handler.config.FeatureFlags.ArchiveInsteadOfDelete = false

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{gin.Param{Key: "key", Value: "delete-error-flag"}}
	c.Request = httptest.NewRequest(http.MethodDelete, "/openfeature/v0/manifest/flags/delete-error-flag", nil)

	handler.DeleteFlag(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response models.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Failed to delete feature flag in PostHog", response.Message)
}

func TestDeleteFlag_ConfigurationToggle(t *testing.T) {
	tests := []struct {
		name                    string
		archiveInsteadOfDelete  bool
		expectedMethod          string
	}{
		{
			name:                   "Hard delete when archive disabled",
			archiveInsteadOfDelete: false,
			expectedMethod:         http.MethodDelete,
		},
		{
			name:                   "Archive when archive enabled",
			archiveInsteadOfDelete: true,
			expectedMethod:         http.MethodPatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualMethod := ""
			
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					assert.Equal(t, "/api/projects/123/feature_flags/test-flag/", r.URL.Path)
					
					response := models.PostHogFeatureFlag{
						ID:     5,
						Key:    "test-flag",
						Name:   "Test",
						Active: true,
					}
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(response)
				} else {
					actualMethod = r.Method
					
					if r.Method == http.MethodPatch {
						response := models.PostHogFeatureFlag{ID: 5, Key: "test-flag", Active: false}
						w.Header().Set("Content-Type", "application/json")
						json.NewEncoder(w).Encode(response)
					} else {
						w.WriteHeader(http.StatusNoContent)
					}
				}
			}))
			defer server.Close()

			cfg := &config.Config{
				PostHog: config.PostHogConfig{
					APIKey:    "test-key",
					Host:      server.URL,
					ProjectID: "123",
				},
				FeatureFlags: config.FeatureFlagsConfig{
					ArchiveInsteadOfDelete: tt.archiveInsteadOfDelete,
				},
			}

			handler := NewHandler(posthog.NewClient(cfg.PostHog, false), cfg, nil)

			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = gin.Params{gin.Param{Key: "key", Value: "test-flag"}}
			c.Request = httptest.NewRequest(http.MethodDelete, "/openfeature/v0/manifest/flags/test-flag", nil)

			handler.DeleteFlag(c)

			assert.Equal(t, http.StatusNoContent, w.Code)
			assert.Equal(t, tt.expectedMethod, actualMethod)
		})
	}
}
