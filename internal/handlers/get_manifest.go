package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/openfeature/posthog-proxy/internal/models"
	"github.com/openfeature/posthog-proxy/internal/transformer"
)

// GetManifest handles GET /openfeature/v0/manifest
func (h *Handler) GetManifest(c *gin.Context) {
	// Get feature flags from PostHog
	posthogFlags, err := h.posthogClient.GetFeatureFlags(c.Request.Context())
	if err != nil {
		if h.metrics != nil {
			h.metrics.PostHogAPIErrors.Add(c.Request.Context(), 1)
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Code:    http.StatusInternalServerError,
			Message: "Failed to retrieve feature flags from PostHog",
			Details: err.Error(),
		})
		return
	}

	if h.metrics != nil {
		h.metrics.ManifestRequests.Add(c.Request.Context(), 1)
	}

	// Transform PostHog flags to OpenFeature manifest
	manifest := transformer.PostHogToOpenFeatureManifest(posthogFlags, h.config.FeatureFlags.TypeCoercion)

	// Add X-Manifest-Capabilities header per spec
	c.Header("X-Manifest-Capabilities", "read,write,delete")
	
	c.JSON(http.StatusOK, manifest)
}
