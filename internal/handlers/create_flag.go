package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/openfeature/posthog-proxy/internal/models"
	"github.com/openfeature/posthog-proxy/internal/transformer"
)

// CreateFlag handles POST /openfeature/v0/manifest/flags
func (h *Handler) CreateFlag(c *gin.Context) {
	var req models.CreateFlagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body",
			Details: err.Error(),
		})
		return
	}

	// Normalize variant weights to ensure they sum to 100
	if req.Variants != nil && len(req.Variants) > 0 {
		req.Variants = NormalizeVariantWeights(req.Variants)
	}

	// Transform OpenFeature request to PostHog format
	posthogReq := transformer.OpenFeatureToPostHogCreate(req, h.config.FeatureFlags.DefaultRolloutPercentage)

	// Create flag in PostHog
	posthogFlag, err := h.posthogClient.CreateFeatureFlag(c.Request.Context(), posthogReq)
	if err != nil {
		if h.metrics != nil {
			h.metrics.PostHogAPIErrors.Add(c.Request.Context(), 1)
		}
		// Check if it's a duplicate key error (PostHog returns 400 with "unique" code)
		if isPostHogDuplicateError(err) {
			c.JSON(http.StatusConflict, models.ErrorResponse{
				Code:    http.StatusConflict,
				Message: "Flag with key \"" + req.Key + "\" already exists",
				Details: err.Error(),
			})
			return
		}
		
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Code:    http.StatusInternalServerError,
			Message: "Failed to create feature flag in PostHog",
			Details: err.Error(),
		})
		return
	}

	if h.metrics != nil {
		h.metrics.FlagsCreated.Add(c.Request.Context(), 1)
	}

	// Transform back to OpenFeature format
	openFeatureFlag := transformer.PostHogToOpenFeatureFlag(*posthogFlag, h.config.FeatureFlags.TypeCoercion)

	// Return ManifestFlagResponse according to spec
	response := models.ManifestFlagResponse{
		Flag:      openFeatureFlag,
		UpdatedAt: posthogFlag.UpdatedAt,
	}

	// Add X-Manifest-Capabilities header per spec
	c.Header("X-Manifest-Capabilities", "read,write,delete")

	c.JSON(http.StatusCreated, response)
}

// isPostHogDuplicateError checks if the error is a duplicate key error from PostHog
func isPostHogDuplicateError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// PostHog returns validation_error/unique for duplicate keys
	return strings.Contains(errStr, "validation_error/unique") || 
	       strings.Contains(errStr, "already a feature flag with this key")
}
