package posthog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/openfeature/posthog-proxy/internal/config"
	"github.com/openfeature/posthog-proxy/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	cfg := config.PostHogConfig{
		APIKey:    "test-key",
		Host:      "https://test.posthog.com",
		ProjectID: "123",
	}

	client := NewClient(cfg, false)

	require.NotNil(t, client)
	assert.Equal(t, cfg.APIKey, client.config.APIKey)
	assert.Equal(t, cfg.Host, client.config.Host)
	assert.NotNil(t, client.httpClient)
	assert.Equal(t, 30*time.Second, client.httpClient.Timeout)
}

func TestNewClient_CustomTimeout(t *testing.T) {
	cfg := config.PostHogConfig{
		APIKey:    "test-key",
		Host:      "https://test.posthog.com",
		ProjectID: "123",
		Timeout:   60,
	}

	client := NewClient(cfg, false)

	require.NotNil(t, client)
	assert.Equal(t, 60*time.Second, client.httpClient.Timeout)
}

func TestGetFeatureFlags_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "/api/projects/123/feature_flags/", r.URL.Path)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		// Send mock response
		response := models.PostHogFeatureFlagsResponse{
			Results: []models.PostHogFeatureFlag{
				{
					ID:     1,
					Key:    "test-flag-1",
					Name:   "Test Flag 1",
					Active: true,
				},
				{
					ID:     2,
					Key:    "test-flag-2",
					Name:   "Test Flag 2",
					Active: false,
				},
			},
			Next: nil,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create client with mock server
	client := NewClient(config.PostHogConfig{
		APIKey:    "test-key",
		Host:      server.URL,
		ProjectID: "123",
	}, false)

	// Test
	flags, err := client.GetFeatureFlags(context.Background())

	require.NoError(t, err)
	require.Len(t, flags, 2)
	assert.Equal(t, "test-flag-1", flags[0].Key)
	assert.Equal(t, "test-flag-2", flags[1].Key)
	assert.True(t, flags[0].Active)
	assert.False(t, flags[1].Active)
}

func TestGetFeatureFlags_Pagination(t *testing.T) {
	callCount := 0
	var serverURL string

	// Create mock server with pagination
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		if callCount == 1 {
			// First page
			response := models.PostHogFeatureFlagsResponse{
				Results: []models.PostHogFeatureFlag{
					{ID: 1, Key: "flag-1", Name: "Flag 1"},
				},
				Next: stringPtr(serverURL + "/api/projects/123/feature_flags/?offset=1"),
			}
			json.NewEncoder(w).Encode(response)
		} else {
			// Second page (last)
			response := models.PostHogFeatureFlagsResponse{
				Results: []models.PostHogFeatureFlag{
					{ID: 2, Key: "flag-2", Name: "Flag 2"},
				},
				Next: nil,
			}
			json.NewEncoder(w).Encode(response)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	client := NewClient(config.PostHogConfig{
		APIKey:    "test-key",
		Host:      server.URL,
		ProjectID: "123",
	}, false)

	// Test
	flags, err := client.GetFeatureFlags(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 2, callCount, "Should have made 2 API calls for pagination")
	require.Len(t, flags, 2, "Should have returned all flags from both pages")
	assert.Equal(t, "flag-1", flags[0].Key)
	assert.Equal(t, "flag-2", flags[1].Key)
}

func TestGetFeatureFlagByKey_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The client uses the key directly in the URL path
		assert.Equal(t, "/api/projects/123/feature_flags/test-flag/", r.URL.Path)
		
		response := models.PostHogFeatureFlag{
			ID:     123,
			Key:    "test-flag",
			Name:   "Test Flag",
			Active: true,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(config.PostHogConfig{
		APIKey:    "test-key",
		Host:      server.URL,
		ProjectID: "123",
	}, false)

	// Test
	flag, err := client.GetFeatureFlagByKey(context.Background(), "test-flag")

	require.NoError(t, err)
	require.NotNil(t, flag)
	assert.Equal(t, "test-flag", flag.Key)
	assert.Equal(t, "Test Flag", flag.Name)
	assert.True(t, flag.Active)
}

func TestGetFeatureFlagByKey_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return 404 for non-existent flag
		w.WriteHeader(http.StatusNotFound)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"detail": "Not found",
		})
	}))
	defer server.Close()

	client := NewClient(config.PostHogConfig{
		APIKey:    "test-key",
		Host:      server.URL,
		ProjectID: "123",
	}, false)

	// Test
	flag, err := client.GetFeatureFlagByKey(context.Background(), "non-existent")

	require.Error(t, err)
	assert.Nil(t, flag)
}

func TestGetFeatureFlagByKey_UsesKeyInURL(t *testing.T) {
	// Test that GetFeatureFlagByKey uses the flag key (not ID) in the URL
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the URL contains the key, not a numeric ID
		assert.Equal(t, "/api/projects/123/feature_flags/my-feature-flag/", r.URL.Path)
		
		response := models.PostHogFeatureFlag{
			ID:     456,
			Key:    "my-feature-flag",
			Name:   "My Feature Flag",
			Active: true,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(config.PostHogConfig{
		APIKey:    "test-key",
		Host:      server.URL,
		ProjectID: "123",
	}, false)

	// Test
	flag, err := client.GetFeatureFlagByKey(context.Background(), "my-feature-flag")

	require.NoError(t, err)
	require.NotNil(t, flag)
	assert.Equal(t, 456, flag.ID)
	assert.Equal(t, "my-feature-flag", flag.Key)
}

func TestGetFeatureFlag_UsesIDInURL(t *testing.T) {
	// Test that GetFeatureFlag uses numeric ID in the URL
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the URL contains the numeric ID
		assert.Equal(t, "/api/projects/123/feature_flags/456/", r.URL.Path)
		
		response := models.PostHogFeatureFlag{
			ID:     456,
			Key:    "my-feature-flag",
			Name:   "My Feature Flag",
			Active: true,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(config.PostHogConfig{
		APIKey:    "test-key",
		Host:      server.URL,
		ProjectID: "123",
	}, false)

	// Test
	flag, err := client.GetFeatureFlag(context.Background(), 456)

	require.NoError(t, err)
	require.NotNil(t, flag)
	assert.Equal(t, 456, flag.ID)
	assert.Equal(t, "my-feature-flag", flag.Key)
	assert.Equal(t, "My Feature Flag", flag.Name)
}

func TestCreateFeatureFlag_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/projects/123/feature_flags/", r.URL.Path)

		// Decode request body
		var req models.PostHogCreateFlagRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		assert.Equal(t, "new-flag", req.Key)
		assert.Equal(t, "New Flag", req.Name)

		// Send response
		response := models.PostHogFeatureFlag{
			ID:     456,
			Key:    req.Key,
			Name:   req.Name,
			Active: req.Active,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(config.PostHogConfig{
		APIKey:    "test-key",
		Host:      server.URL,
		ProjectID: "123",
	}, false)

	// Test
	req := models.PostHogCreateFlagRequest{
		Key:    "new-flag",
		Name:   "New Flag",
		Active: false,
	}

	flag, err := client.CreateFeatureFlag(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, flag)
	assert.Equal(t, "new-flag", flag.Key)
	assert.Equal(t, "New Flag", flag.Name)
}

func TestUpdateFeatureFlag_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method)
		assert.Equal(t, "/api/projects/123/feature_flags/456/", r.URL.Path)

		// Decode request body
		var req models.PostHogUpdateFlagRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		// Send response
		response := models.PostHogFeatureFlag{
			ID:     456,
			Key:    "updated-flag",
			Name:   *req.Name,
			Active: *req.Active,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(config.PostHogConfig{
		APIKey:    "test-key",
		Host:      server.URL,
		ProjectID: "123",
		}, false)

	// Test
	name := "Updated Name"
	active := true
	req := models.PostHogUpdateFlagRequest{
		Name:   &name,
		Active: &active,
	}

	flag, err := client.UpdateFeatureFlag(context.Background(), 456, req)

	require.NoError(t, err)
	require.NotNil(t, flag)
	assert.Equal(t, "Updated Name", flag.Name)
	assert.True(t, flag.Active)
}

func TestDeleteFeatureFlag_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Equal(t, "/api/projects/123/feature_flags/456/", r.URL.Path)

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(config.PostHogConfig{
		APIKey:    "test-key",
		Host:      server.URL,
		ProjectID: "123",
		}, false)

	// Test
	err := client.DeleteFeatureFlag(context.Background(), 456)

	require.NoError(t, err)
}

func TestClient_ErrorHandling(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		responseBody   string
		expectedError  string
	}{
		{
			name:          "400 Bad Request",
			statusCode:    http.StatusBadRequest,
			responseBody:  `{"detail":"Invalid request"}`,
			expectedError: "status 400",
		},
		{
			name:          "401 Unauthorized",
			statusCode:    http.StatusUnauthorized,
			responseBody:  `{"detail":"Invalid API key"}`,
			expectedError: "status 401",
		},
		{
			name:          "500 Internal Server Error",
			statusCode:    http.StatusInternalServerError,
			responseBody:  `{"detail":"Internal error"}`,
			expectedError: "status 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			client := NewClient(config.PostHogConfig{
				APIKey:    "test-key",
				Host:      server.URL,
				ProjectID: "123",
				}, false)

			_, err := client.GetFeatureFlags(context.Background())

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func stringPtr(s string) *string {
	return &s
}
