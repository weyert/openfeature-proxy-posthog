package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/openfeature/posthog-proxy/internal/models"
	"github.com/openfeature/posthog-proxy/internal/transformer"
)

// UpdateFlag handles PUT /openfeature/v0/manifest/flags/:key
func (h *Handler) UpdateFlag(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: "Flag key is required",
		})
		return
	}

	var req models.UpdateFlagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body",
			Details: err.Error(),
		})
		return
	}

	// Validate and normalize variant weights if variants are being updated
	if req.Variants != nil {
		if err := ValidateVariantWeights(*req.Variants); err != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Code:    http.StatusBadRequest,
				Message: "Invalid variant configuration",
				Details: err.Error(),
			})
			return
		}
		
		// Normalize weights to sum to 100
		normalized := NormalizeVariantWeights(*req.Variants)
		req.Variants = &normalized
	}

	// Find the flag in PostHog by key
	existingFlag, err := h.posthogClient.GetFeatureFlagByKey(c.Request.Context(), key)
	if err != nil {
		if h.metrics != nil {
			h.metrics.PostHogAPIErrors.Add(c.Request.Context(), 1)
		}
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Code:    http.StatusNotFound,
			Message: "Feature flag not found",
			Details: err.Error(),
		})
		return
	}

	// Transform OpenFeature update request to PostHog format
	// Pass existing flag to preserve groups and other settings
	posthogReq := transformer.OpenFeatureToPostHogUpdate(req, existingFlag)

	// Update flag in PostHog
	updatedFlag, err := h.posthogClient.UpdateFeatureFlag(c.Request.Context(), existingFlag.ID, posthogReq)
	if err != nil {
		if h.metrics != nil {
			h.metrics.PostHogAPIErrors.Add(c.Request.Context(), 1)
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Code:    http.StatusInternalServerError,
			Message: "Failed to update feature flag in PostHog",
			Details: err.Error(),
		})
		return
	}

	if h.metrics != nil {
		h.metrics.FlagsUpdated.Add(c.Request.Context(), 1)
	}

	// Transform back to OpenFeature format
	openFeatureFlag := transformer.PostHogToOpenFeatureFlag(*updatedFlag, h.config.FeatureFlags.TypeCoercion)

	// Return ManifestFlagResponse according to spec
	response := models.ManifestFlagResponse{
		Flag:      openFeatureFlag,
		UpdatedAt: updatedFlag.UpdatedAt,
	}

	// Add X-Manifest-Capabilities header per spec
	c.Header("X-Manifest-Capabilities", "read,write,delete")

	c.JSON(http.StatusOK, response)
}
