package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/openfeature/posthog-proxy/internal/models"
)

// DeleteFlag handles DELETE /openfeature/v0/manifest/flags/:key
func (h *Handler) DeleteFlag(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: "Flag key is required",
		})
		return
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

	// Check if we should archive or hard delete
	if h.config.FeatureFlags.ArchiveInsteadOfDelete {
		// Archive flag by setting it to inactive
		updateReq := models.PostHogUpdateFlagRequest{
			Active: &[]bool{false}[0],
		}

		updatedFlag, err := h.posthogClient.UpdateFeatureFlag(c.Request.Context(), existingFlag.ID, updateReq)
		if err != nil {
			if h.metrics != nil {
				h.metrics.PostHogAPIErrors.Add(c.Request.Context(), 1)
			}
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Code:    http.StatusInternalServerError,
				Message: "Failed to archive feature flag in PostHog",
				Details: err.Error(),
			})
			return
		}

		if h.metrics != nil {
			h.metrics.FlagsDeleted.Add(c.Request.Context(), 1)
		}

		// Return ArchiveResponse according to spec
		response := models.ArchiveResponse{
			Message:    "Flag \"" + key + "\" archived. Restore it using your management interface if needed.",
			ArchivedAt: &updatedFlag.UpdatedAt,
		}
		
		// Add X-Manifest-Capabilities header per spec
		c.Header("X-Manifest-Capabilities", "read,write,delete")
		
		c.JSON(http.StatusNoContent, response)
	} else {
		// Hard delete the flag
		err = h.posthogClient.DeleteFeatureFlag(c.Request.Context(), existingFlag.ID)
		if err != nil {
			if h.metrics != nil {
				h.metrics.PostHogAPIErrors.Add(c.Request.Context(), 1)
			}
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Code:    http.StatusInternalServerError,
				Message: "Failed to delete feature flag in PostHog",
				Details: err.Error(),
			})
			return
		}

		if h.metrics != nil {
			h.metrics.FlagsDeleted.Add(c.Request.Context(), 1)
		}

		// For hard delete, return ArchiveResponse with null archivedAt
		response := models.ArchiveResponse{
			Message:    "Flag \"" + key + "\" deleted successfully.",
			ArchivedAt: nil,
		}
		
		// Add X-Manifest-Capabilities header per spec
		c.Header("X-Manifest-Capabilities", "read,write,delete")
		
		c.JSON(http.StatusNoContent, response)
	}
}
