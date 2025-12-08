package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/openfeature/posthog-proxy/internal/config"
	"github.com/openfeature/posthog-proxy/internal/handlers"
	"github.com/openfeature/posthog-proxy/internal/models"
	"github.com/openfeature/posthog-proxy/internal/posthog"
	"github.com/openfeature/posthog-proxy/internal/telemetry"
)

// MockPostHogServer simulates the PostHog API
type MockPostHogServer struct {
	Server    *httptest.Server
	Flags     map[int]models.PostHogFeatureFlag
	NextID    int
	ProjectID string
	mu        sync.Mutex
}

func NewMockPostHogServer(projectID string) *MockPostHogServer {
	mock := &MockPostHogServer{
		Flags:     make(map[int]models.PostHogFeatureFlag),
		NextID:    1,
		ProjectID: projectID,
	}

	mux := http.NewServeMux()
	
	// Base path for feature flags: /api/projects/:id/feature_flags/
	basePath := fmt.Sprintf("/api/projects/%s/feature_flags/", projectID)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Simple routing logic
		if !strings.HasPrefix(r.URL.Path, basePath) {
			http.NotFound(w, r)
			return
		}

		// Handle /api/projects/:id/feature_flags/ (List and Create)
		if r.URL.Path == basePath {
			switch r.Method {
			case http.MethodGet:
				mock.handleListFlags(w, r)
			case http.MethodPost:
				mock.handleCreateFlag(w, r)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}

		// Handle /api/projects/:id/feature_flags/:id_or_key/
		// Extract ID or Key
		idOrKey := strings.TrimPrefix(r.URL.Path, basePath)
		idOrKey = strings.TrimSuffix(idOrKey, "/")

		switch r.Method {
		case http.MethodGet:
			mock.handleGetFlag(w, r, idOrKey)
		case http.MethodPatch:
			mock.handleUpdateFlag(w, r, idOrKey)
		case http.MethodDelete:
			mock.handleDeleteFlag(w, r, idOrKey)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mock.Server = httptest.NewServer(mux)
	return mock
}

func (m *MockPostHogServer) Close() {
	m.Server.Close()
}

func (m *MockPostHogServer) URL() string {
	return m.Server.URL
}

func (m *MockPostHogServer) handleListFlags(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var results []models.PostHogFeatureFlag
	for _, flag := range m.Flags {
		results = append(results, flag)
	}

	resp := models.PostHogFeatureFlagsResponse{
		Results: results,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (m *MockPostHogServer) handleCreateFlag(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var req models.PostHogCreateFlagRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check for duplicates
	for _, f := range m.Flags {
		if f.Key == req.Key {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"type": "validation_error",
				"code": "unique",
				"detail": fmt.Sprintf("There is already a feature flag with the key '%s'.", req.Key),
			})
			return
		}
	}

	flag := models.PostHogFeatureFlag{
		ID:      m.NextID,
		Key:     req.Key,
		Name:    req.Name,
		Active:  true,
		Filters: req.Filters,
	}
	
	// Handle deleted field if present (default false)
	flag.Deleted = false

	m.Flags[m.NextID] = flag
	m.NextID++

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(flag)
}

func (m *MockPostHogServer) handleGetFlag(w http.ResponseWriter, r *http.Request, idOrKey string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	flag, found := m.findFlag(idOrKey)
	if !found {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(flag)
}

func (m *MockPostHogServer) handleUpdateFlag(w http.ResponseWriter, r *http.Request, idOrKey string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	flag, found := m.findFlag(idOrKey)
	if !found {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	var req models.PostHogUpdateFlagRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Update fields
	if req.Name != nil {
		flag.Name = *req.Name
	}
	if req.Active != nil {
		flag.Active = *req.Active
	}
	if req.Filters != nil {
		flag.Filters = *req.Filters
	}
	// PostHogUpdateFlagRequest doesn't have Deleted field in models.go
	// if req.Deleted != nil {
	// 	flag.Deleted = *req.Deleted
	// }

	m.Flags[flag.ID] = flag

	json.NewEncoder(w).Encode(flag)
}

func (m *MockPostHogServer) handleDeleteFlag(w http.ResponseWriter, r *http.Request, idOrKey string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	flag, found := m.findFlag(idOrKey)
	if !found {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	// Soft delete logic usually, but for API it might return 204 or 200 with deleted=true
	// PostHog API returns 204 on delete usually, or we can simulate soft delete
	// The client expects 204 or 200.
	
	// Actually remove it from our map for simplicity in tests
	delete(m.Flags, flag.ID)

	w.WriteHeader(http.StatusNoContent)
}

func (m *MockPostHogServer) findFlag(idOrKey string) (models.PostHogFeatureFlag, bool) {
	// Try as ID
	if id, err := strconv.Atoi(idOrKey); err == nil {
		if flag, ok := m.Flags[id]; ok {
			return flag, true
		}
	}

	// Try as Key
	for _, flag := range m.Flags {
		if flag.Key == idOrKey {
			return flag, true
		}
	}

	return models.PostHogFeatureFlag{}, false
}

// SetupProxy creates a test instance of the proxy server connected to the mock PostHog server
func SetupProxy(t *testing.T, mockPostHog *MockPostHogServer) *httptest.Server {
	gin.SetMode(gin.TestMode)

	// Config
	cfg := config.Config{
		PostHog: config.PostHogConfig{
			Host:      mockPostHog.URL(),
			ProjectID: mockPostHog.ProjectID,
			APIKey:    "test-key",
		},
		Proxy: config.ProxyConfig{
			InsecureMode: true, // Disable auth for easier testing
		},
	}

	// Dependencies
	phClient := posthog.NewClient(cfg.PostHog, true)
	metrics, _ := telemetry.NewMetrics()
	handler := handlers.NewHandler(phClient, &cfg, metrics)

	// Router
	router := gin.New()
	api := router.Group("/openfeature/v0")
	
	// We skip auth middleware since we set InsecureMode=true, 
	// but the handler.AuthMiddleware() checks that config.
	api.Use(handler.AuthMiddleware())

	api.GET("/manifest", handler.GetManifest)
	api.POST("/manifest/flags", handler.CreateFlag)
	api.GET("/manifest/flags/:key", handler.GetFlag)
	api.PUT("/manifest/flags/:key", handler.UpdateFlag)
	api.DELETE("/manifest/flags/:key", handler.DeleteFlag)

	return httptest.NewServer(router)
}
