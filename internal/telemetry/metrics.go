package telemetry

import (
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// Metrics holds the OpenTelemetry instruments for the application
type Metrics struct {
	FlagsCreated      metric.Int64Counter
	FlagsUpdated      metric.Int64Counter
	FlagsDeleted      metric.Int64Counter
	ManifestRequests  metric.Int64Counter
	PostHogAPIErrors  metric.Int64Counter
}

// NewMetrics initializes and returns the application metrics
func NewMetrics() (*Metrics, error) {
	meter := otel.Meter("openfeature-posthog-proxy")

	flagsCreated, err := meter.Int64Counter("flags_created_total",
		metric.WithDescription("Total number of feature flags created"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create flags_created_total counter: %w", err)
	}

	flagsUpdated, err := meter.Int64Counter("flags_updated_total",
		metric.WithDescription("Total number of feature flags updated"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create flags_updated_total counter: %w", err)
	}

	flagsDeleted, err := meter.Int64Counter("flags_deleted_total",
		metric.WithDescription("Total number of feature flags deleted"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create flags_deleted_total counter: %w", err)
	}

	manifestRequests, err := meter.Int64Counter("manifest_requests_total",
		metric.WithDescription("Total number of manifest requests served"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create manifest_requests_total counter: %w", err)
	}

	posthogAPIErrors, err := meter.Int64Counter("posthog_api_errors_total",
		metric.WithDescription("Total number of errors from PostHog API"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create posthog_api_errors_total counter: %w", err)
	}

	return &Metrics{
		FlagsCreated:     flagsCreated,
		FlagsUpdated:     flagsUpdated,
		FlagsDeleted:     flagsDeleted,
		ManifestRequests: manifestRequests,
		PostHogAPIErrors: posthogAPIErrors,
	}, nil
}
