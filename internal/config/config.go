package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config represents the application configuration
type Config struct {
	PostHog      PostHogConfig      `json:"posthog"`
	Proxy        ProxyConfig        `json:"proxy"`
	FeatureFlags FeatureFlagsConfig `json:"feature_flags"`
	Telemetry    TelemetryConfig    `json:"telemetry"`
}

// PostHogConfig represents PostHog-specific configuration
type PostHogConfig struct {
	APIKey    string `json:"api_key"`
	ProjectID string `json:"project_id"`
	Host      string `json:"host"`
	Timeout   int    `json:"timeout"` // Timeout in seconds
}

// ProxyConfig represents proxy server configuration
type ProxyConfig struct {
	Port         int        `json:"port"`
	Auth         AuthConfig `json:"auth"`
	InsecureMode bool       `json:"insecure_mode"`
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	Tokens []AuthToken `json:"tokens"`
}

// AuthToken represents an authentication token with capabilities
type AuthToken struct {
	Token        string   `json:"token"`
	Capabilities []string `json:"capabilities"`
}

// FeatureFlagsConfig represents feature flag-specific configuration
type FeatureFlagsConfig struct {
	DefaultRolloutPercentage int                   `json:"default_rollout_percentage"`
	ArchiveInsteadOfDelete   bool                  `json:"archive_instead_of_delete"`
	TypeCoercion             TypeCoercionConfig    `json:"type_coercion"`
}

// TypeCoercionConfig represents type coercion feature gates
type TypeCoercionConfig struct {
	// CoerceNumericStrings enables automatic conversion of numeric strings ("1", "200") to number type
	CoerceNumericStrings bool `json:"coerce_numeric_strings"`
	// CoerceBooleanStrings enables automatic conversion of boolean strings ("true", "false") to boolean type  
	CoerceBooleanStrings bool `json:"coerce_boolean_strings"`
}

// TelemetryConfig represents OpenTelemetry configuration
type TelemetryConfig struct {
	ServiceName  string `json:"service_name"`
	OTLPEndpoint string `json:"otlp_endpoint"`
	Protocol     string `json:"protocol"` // "grpc" or "http"
	Insecure     bool   `json:"insecure"`
	Prometheus   bool   `json:"prometheus"`
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{}

	// PostHog configuration
	cfg.PostHog.APIKey = getEnvOrError("POSTHOG_API_KEY")
	if cfg.PostHog.APIKey == "" {
		return nil, fmt.Errorf("POSTHOG_API_KEY environment variable is required")
	}

	cfg.PostHog.ProjectID = getEnvOrError("POSTHOG_PROJECT_ID")
	if cfg.PostHog.ProjectID == "" {
		return nil, fmt.Errorf("POSTHOG_PROJECT_ID environment variable is required")
	}

	cfg.PostHog.Host = getEnvOrDefault("POSTHOG_HOST", "https://app.posthog.com")

	timeoutStr := getEnvOrDefault("POSTHOG_TIMEOUT", "30")
	timeout, err := strconv.Atoi(timeoutStr)
	if err != nil {
		return nil, fmt.Errorf("invalid POSTHOG_TIMEOUT: %w", err)
	}
	cfg.PostHog.Timeout = timeout

	// Proxy configuration
	portStr := getEnvOrDefault("PROXY_PORT", "8080")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("invalid PROXY_PORT: %w", err)
	}
	cfg.Proxy.Port = port

	// Insecure mode configuration
	insecureStr := getEnvOrDefault("INSECURE_MODE", "false")
	insecure, err := strconv.ParseBool(insecureStr)
	if err != nil {
		return nil, fmt.Errorf("invalid INSECURE_MODE: %w", err)
	}
	cfg.Proxy.InsecureMode = insecure

	// Authentication configuration
	cfg.Proxy.Auth.Tokens = loadAuthTokens()

	// Feature flags configuration
	defaultRolloutStr := getEnvOrDefault("DEFAULT_ROLLOUT_PERCENTAGE", "0")
	defaultRollout, err := strconv.Atoi(defaultRolloutStr)
	if err != nil {
		return nil, fmt.Errorf("invalid DEFAULT_ROLLOUT_PERCENTAGE: %w", err)
	}
	cfg.FeatureFlags.DefaultRolloutPercentage = defaultRollout

	archiveStr := getEnvOrDefault("ARCHIVE_INSTEAD_OF_DELETE", "true")
	archive, err := strconv.ParseBool(archiveStr)
	if err != nil {
		return nil, fmt.Errorf("invalid ARCHIVE_INSTEAD_OF_DELETE: %w", err)
	}
	cfg.FeatureFlags.ArchiveInsteadOfDelete = archive

	// Type coercion configuration
	coerceNumericStr := getEnvOrDefault("COERCE_NUMERIC_STRINGS", "false")
	coerceNumeric, err := strconv.ParseBool(coerceNumericStr)
	if err != nil {
		return nil, fmt.Errorf("invalid COERCE_NUMERIC_STRINGS: %w", err)
	}
	cfg.FeatureFlags.TypeCoercion.CoerceNumericStrings = coerceNumeric

	coerceBooleanStr := getEnvOrDefault("COERCE_BOOLEAN_STRINGS", "false")
	coerceBoolean, err := strconv.ParseBool(coerceBooleanStr)
	if err != nil {
		return nil, fmt.Errorf("invalid COERCE_BOOLEAN_STRINGS: %w", err)
	}
	cfg.FeatureFlags.TypeCoercion.CoerceBooleanStrings = coerceBoolean

	// Telemetry configuration
	cfg.Telemetry.ServiceName = getEnvOrDefault("OTEL_SERVICE_NAME", "openfeature-posthog-proxy")
	cfg.Telemetry.OTLPEndpoint = getEnvOrDefault("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")
	cfg.Telemetry.Protocol = getEnvOrDefault("OTEL_EXPORTER_OTLP_PROTOCOL", "grpc")

	insecureOtelStr := getEnvOrDefault("OTEL_EXPORTER_OTLP_INSECURE", "false")
	insecureOtel, err := strconv.ParseBool(insecureOtelStr)
	if err != nil {
		// Default to false if invalid, but log it (or just ignore since we return error)
		// For now, let's just default to false if parsing fails, or return error like others
		return nil, fmt.Errorf("invalid OTEL_EXPORTER_OTLP_INSECURE: %w", err)
	}
	cfg.Telemetry.Insecure = insecureOtel

	promEnabledStr := getEnvOrDefault("OTEL_PROMETHEUS_ENABLED", "true")
	promEnabled, err := strconv.ParseBool(promEnabledStr)
	if err == nil {
		cfg.Telemetry.Prometheus = promEnabled
	} else {
		cfg.Telemetry.Prometheus = true // Default to true
	}

	return cfg, nil
}

// loadAuthTokens loads authentication tokens from environment variables
func loadAuthTokens() []AuthToken {
	var tokens []AuthToken

	// Add predefined tokens if they exist
	if readToken := os.Getenv("READ_TOKEN"); readToken != "" {
		tokens = append(tokens, AuthToken{
			Token:        readToken,
			Capabilities: []string{"read"},
		})
	}

	if writeToken := os.Getenv("WRITE_TOKEN"); writeToken != "" {
		tokens = append(tokens, AuthToken{
			Token:        writeToken,
			Capabilities: []string{"read", "write"},
		})
	}

	if adminToken := os.Getenv("ADMIN_TOKEN"); adminToken != "" {
		tokens = append(tokens, AuthToken{
			Token:        adminToken,
			Capabilities: []string{"read", "write", "delete"},
		})
	}

	// Load custom tokens from environment
	// Format: CUSTOM_TOKEN_1=token:capability1,capability2
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "CUSTOM_TOKEN_") {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				tokenParts := strings.SplitN(parts[1], ":", 2)
				if len(tokenParts) == 2 {
					token := tokenParts[0]
					capabilities := strings.Split(tokenParts[1], ",")
					
					// Trim whitespace from capabilities
					for i, cap := range capabilities {
						capabilities[i] = strings.TrimSpace(cap)
					}

					tokens = append(tokens, AuthToken{
						Token:        token,
						Capabilities: capabilities,
					})
				}
			}
		}
	}

	return tokens
}

// getEnvOrError returns the environment variable value or an empty string if not set
func getEnvOrError(key string) string {
	return os.Getenv(key)
}

// getEnvOrDefault returns the environment variable value or the default value if not set
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}