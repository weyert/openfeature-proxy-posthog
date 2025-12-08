package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/openfeature/posthog-proxy/internal/models"
	"github.com/openfeature/posthog-proxy/internal/transformer"
)

// GetFlag handles GET /openfeature/v0/manifest/flags/:key
func (h *Handler) GetFlag(c *gin.Context) {
	flagKey := c.Param("key")

	if flagKey == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: "flag key is required",
		})
		return
	}

	// Get the flag from PostHog by key
	posthogFlag, err := h.posthogClient.GetFeatureFlagByKey(c.Request.Context(), flagKey)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Code:    http.StatusNotFound,
			Message: "flag not found",
			Details: err.Error(),
		})
		return
	}

	// Check if flag is active
	if !posthogFlag.Active {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Code:    http.StatusNotFound,
			Message: "flag not found",
			Details: "flag is inactive",
		})
		return
	}

	// Convert PostHog flag to OpenFeature format
	openFeatureFlag := transformer.PostHogToOpenFeatureFlag(*posthogFlag, h.config.FeatureFlags.TypeCoercion)

	// Add X-Manifest-Capabilities header per spec
	c.Header("X-Manifest-Capabilities", "read,write,delete")
	
	// Wrap in ManifestFlagResponse
	response := models.ManifestFlagResponse{
		Flag:      openFeatureFlag,
		UpdatedAt: posthogFlag.UpdatedAt,
	}
	
	c.JSON(http.StatusOK, response)
}
