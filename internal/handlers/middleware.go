package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/openfeature/posthog-proxy/internal/models"
)

// AuthMiddleware validates the authorization token (optional in insecure mode)
func (h *Handler) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip authentication in insecure mode
		if h.config.Proxy.InsecureMode {
			// Grant all capabilities in insecure mode
			c.Set("capabilities", []string{"read", "write", "delete"})
			c.Set("insecure_mode", true)
			c.Next()
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, models.ErrorResponse{
				Code:    http.StatusUnauthorized,
				Message: "Authorization header is required",
			})
			c.Abort()
			return
		}

		// Extract token from "Bearer <token>"
		token := extractBearerToken(authHeader)
		if token == "" {
			c.JSON(http.StatusUnauthorized, models.ErrorResponse{
				Code:    http.StatusUnauthorized,
				Message: "Invalid authorization header format",
			})
			c.Abort()
			return
		}

		// Validate token and get capabilities
		capabilities := h.validateToken(token)
		if capabilities == nil {
			c.JSON(http.StatusUnauthorized, models.ErrorResponse{
				Code:    http.StatusUnauthorized,
				Message: "Invalid authorization token",
			})
			c.Abort()
			return
		}

		// Store capabilities in context
		c.Set("capabilities", capabilities)
		c.Next()
	}
}

// RequireCapability middleware checks if the user has the required capability
func (h *Handler) RequireCapability(capability string) gin.HandlerFunc {
	return func(c *gin.Context) {
		capabilities, exists := c.Get("capabilities")
		if !exists {
			c.JSON(http.StatusForbidden, models.ErrorResponse{
				Code:    http.StatusForbidden,
				Message: "No capabilities found",
			})
			c.Abort()
			return
		}

		caps, ok := capabilities.([]string)
		if !ok {
			c.JSON(http.StatusForbidden, models.ErrorResponse{
				Code:    http.StatusForbidden,
				Message: "Invalid capabilities format",
			})
			c.Abort()
			return
		}

		// Check if user has the required capability
		if !hasCapability(caps, capability) {
			c.JSON(http.StatusForbidden, models.ErrorResponse{
				Code:    http.StatusForbidden,
				Message: "Insufficient permissions",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// extractBearerToken extracts the token from "Bearer <token>" format
func extractBearerToken(authHeader string) string {
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		return authHeader[7:]
	}
	return ""
}

// validateToken validates a token and returns its capabilities
func (h *Handler) validateToken(token string) []string {
	for _, authToken := range h.config.Proxy.Auth.Tokens {
		if authToken.Token == token {
			return authToken.Capabilities
		}
	}
	return nil
}

// hasCapability checks if a capability exists in the capabilities list
func hasCapability(capabilities []string, required string) bool {
	for _, cap := range capabilities {
		if cap == required {
			return true
		}
	}
	return false
}
