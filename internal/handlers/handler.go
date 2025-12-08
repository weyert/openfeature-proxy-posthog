package handlers

import (
	"github.com/openfeature/posthog-proxy/internal/config"
	"github.com/openfeature/posthog-proxy/internal/posthog"
	"github.com/openfeature/posthog-proxy/internal/telemetry"
)

// Handler handles HTTP requests for the OpenFeature API
type Handler struct {
	posthogClient posthog.ClientInterface
	config        *config.Config
	metrics       *telemetry.Metrics
}

// NewHandler creates a new handler instance
func NewHandler(posthogClient posthog.ClientInterface, cfg *config.Config, metrics *telemetry.Metrics) *Handler {
	return &Handler{
		posthogClient: posthogClient,
		config:        cfg,
		metrics:       metrics,
	}
}
