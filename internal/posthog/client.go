package posthog

import (
"bytes"
"context"
"encoding/json"
"fmt"
"io"
"log/slog"
"net/http"
"strings"
"time"

"github.com/openfeature/posthog-proxy/internal/config"
"github.com/openfeature/posthog-proxy/internal/models"
"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// Client represents a PostHog API client
type Client struct {
config     config.PostHogConfig
httpClient *http.Client
baseURL    string
insecure   bool
retryConfig RetryConfig
}

// NewClient creates a new PostHog client
func NewClient(cfg config.PostHogConfig, insecureMode bool) *Client {
	timeout := 30 * time.Second
	if cfg.Timeout > 0 {
		timeout = time.Duration(cfg.Timeout) * time.Second
	}

	return &Client{
		config: cfg,
		httpClient: &http.Client{
			Transport: otelhttp.NewTransport(http.DefaultTransport),
			Timeout:   timeout,
		},
		baseURL:  fmt.Sprintf("%s/api/projects/%s", cfg.Host, cfg.ProjectID),
		insecure: insecureMode,
retryConfig: DefaultRetryConfig(),
	}
}

// GetFeatureFlags retrieves all feature flags from PostHog, traversing pagination when necessary.
func (c *Client) GetFeatureFlags(ctx context.Context) ([]models.PostHogFeatureFlag, error) {
nextURL := fmt.Sprintf("%s/feature_flags/", c.baseURL)
var allFlags []models.PostHogFeatureFlag

for nextURL != "" {
req, err := c.newRequest(ctx, http.MethodGet, nextURL, nil)
if err != nil {
slog.ErrorContext(ctx, "GetFeatureFlags - creating request", "error", err)
return nil, fmt.Errorf("creating request: %w", err)
}

c.logRequest(ctx, req)

resp, err := c.doWithRetry(ctx, req)
if err != nil {
slog.ErrorContext(ctx, "GetFeatureFlags - HTTP request", "error", err)
return nil, fmt.Errorf("making request: %w", err)
}

if err := func() error {
defer resp.Body.Close()
c.logResponse(ctx, resp)

if resp.StatusCode != http.StatusOK {
return c.parseErrorResponse(resp)
}

var page models.PostHogFeatureFlagsResponse
if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
slog.ErrorContext(ctx, "GetFeatureFlags - decoding response", "error", err)
return fmt.Errorf("decoding response: %w", err)
}

allFlags = append(allFlags, page.Results...)
if page.Next != nil && *page.Next != "" {
nextURL = c.resolveURL(*page.Next)
} else {
nextURL = ""
}

return nil
}(); err != nil {
return nil, err
}
}

slog.InfoContext(ctx, "GetFeatureFlags - Successfully retrieved flags", "count", len(allFlags))
return allFlags, nil
}

// GetFeatureFlag retrieves a specific feature flag by numeric ID.
func (c *Client) GetFeatureFlag(ctx context.Context, id int) (*models.PostHogFeatureFlag, error) {
return c.fetchFeatureFlag(ctx, fmt.Sprintf("%d", id), fmt.Sprintf("ID %d", id))
}

// GetFeatureFlagByKey retrieves a feature flag using its key directly from PostHog.
// The PostHog API supports /feature_flags/{key}/ endpoint which accepts either numeric IDs or string keys.
func (c *Client) GetFeatureFlagByKey(ctx context.Context, key string) (*models.PostHogFeatureFlag, error) {
	return c.fetchFeatureFlag(ctx, key, fmt.Sprintf("key %s", key))
}

func (c *Client) fetchFeatureFlag(ctx context.Context, identifier, label string) (*models.PostHogFeatureFlag, error) {
url := fmt.Sprintf("%s/feature_flags/%s/", c.baseURL, identifier)

req, err := c.newRequest(ctx, http.MethodGet, url, nil)
if err != nil {
slog.ErrorContext(ctx, "GetFeatureFlag - creating request", "error", err)
return nil, fmt.Errorf("creating request: %w", err)
}

c.logRequest(ctx, req)

resp, err := c.doWithRetry(ctx, req)
if err != nil {
slog.ErrorContext(ctx, "GetFeatureFlag - HTTP request", "error", err)
return nil, fmt.Errorf("making request: %w", err)
}
defer resp.Body.Close()

c.logResponse(ctx, resp)

if resp.StatusCode != http.StatusOK {
return nil, c.parseErrorResponse(resp)
}

var result models.PostHogFeatureFlag
if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
slog.ErrorContext(ctx, "GetFeatureFlag - decoding response", "error", err)
return nil, fmt.Errorf("decoding response: %w", err)
}

slog.InfoContext(ctx, "GetFeatureFlag - Successfully retrieved flag", "label", label)
return &result, nil
}

// CreateFeatureFlag creates a new feature flag in PostHog
func (c *Client) CreateFeatureFlag(ctx context.Context, req models.PostHogCreateFlagRequest) (*models.PostHogFeatureFlag, error) {
url := fmt.Sprintf("%s/feature_flags/", c.baseURL)

body, err := json.Marshal(req)
if err != nil {
slog.ErrorContext(ctx, "CreateFeatureFlag - marshaling request", "error", err)
return nil, fmt.Errorf("marshaling request: %w", err)
}

httpReq, err := c.newRequest(ctx, http.MethodPost, url, bytes.NewReader(body))
if err != nil {
slog.ErrorContext(ctx, "CreateFeatureFlag - creating request", "error", err)
return nil, fmt.Errorf("creating request: %w", err)
}

c.logRequest(ctx, httpReq)

resp, err := c.doWithRetry(ctx, httpReq)
if err != nil {
slog.ErrorContext(ctx, "CreateFeatureFlag - HTTP request", "error", err)
return nil, fmt.Errorf("making request: %w", err)
}
defer resp.Body.Close()

c.logResponse(ctx, resp)

if resp.StatusCode != http.StatusCreated {
return nil, c.parseErrorResponse(resp)
}

var result models.PostHogFeatureFlag
if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
slog.ErrorContext(ctx, "CreateFeatureFlag - decoding response", "error", err)
return nil, fmt.Errorf("decoding response: %w", err)
}

slog.InfoContext(ctx, "CreateFeatureFlag - Successfully created flag", "key", result.Key)
return &result, nil
}

// UpdateFeatureFlag updates an existing feature flag in PostHog
func (c *Client) UpdateFeatureFlag(ctx context.Context, id int, req models.PostHogUpdateFlagRequest) (*models.PostHogFeatureFlag, error) {
url := fmt.Sprintf("%s/feature_flags/%d/", c.baseURL, id)

body, err := json.Marshal(req)
if err != nil {
slog.ErrorContext(ctx, "UpdateFeatureFlag - marshaling request", "error", err)
return nil, fmt.Errorf("marshaling request: %w", err)
}

httpReq, err := c.newRequest(ctx, http.MethodPatch, url, bytes.NewReader(body))
if err != nil {
slog.ErrorContext(ctx, "UpdateFeatureFlag - creating request", "error", err)
return nil, fmt.Errorf("creating request: %w", err)
}

c.logRequest(ctx, httpReq)

resp, err := c.doWithRetry(ctx, httpReq)
if err != nil {
slog.ErrorContext(ctx, "UpdateFeatureFlag - HTTP request", "error", err)
return nil, fmt.Errorf("making request: %w", err)
}
defer resp.Body.Close()

c.logResponse(ctx, resp)

if resp.StatusCode != http.StatusOK {
return nil, c.parseErrorResponse(resp)
}

var result models.PostHogFeatureFlag
if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
slog.ErrorContext(ctx, "UpdateFeatureFlag - decoding response", "error", err)
return nil, fmt.Errorf("decoding response: %w", err)
}

slog.InfoContext(ctx, "UpdateFeatureFlag - Successfully updated flag", "id", id)
return &result, nil
}

// DeleteFeatureFlag deletes a feature flag in PostHog
func (c *Client) DeleteFeatureFlag(ctx context.Context, id int) error {
url := fmt.Sprintf("%s/feature_flags/%d/", c.baseURL, id)

req, err := c.newRequest(ctx, http.MethodDelete, url, nil)
if err != nil {
slog.ErrorContext(ctx, "DeleteFeatureFlag - creating request", "error", err)
return fmt.Errorf("creating request: %w", err)
}

c.logRequest(ctx, req)

resp, err := c.doWithRetry(ctx, req)
if err != nil {
slog.ErrorContext(ctx, "DeleteFeatureFlag - HTTP request", "error", err)
return fmt.Errorf("making request: %w", err)
}
defer resp.Body.Close()

c.logResponse(ctx, resp)

if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
return c.parseErrorResponse(resp)
}

slog.InfoContext(ctx, "DeleteFeatureFlag - Successfully deleted flag", "id", id)
return nil
}

func (c *Client) newRequest(ctx context.Context, method, url string, body io.Reader) (*http.Request, error) {
req, err := http.NewRequestWithContext(ctx, method, url, body)
if err != nil {
return nil, err
}

req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
req.Header.Set("Content-Type", "application/json")

return req, nil
}

func (c *Client) resolveURL(raw string) string {
if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
return raw
}

return fmt.Sprintf("%s%s", strings.TrimRight(c.config.Host, "/"), raw)
}

func (c *Client) logRequest(ctx context.Context, req *http.Request) {
if !c.insecure {
return
}
slog.InfoContext(ctx, "API Request",
"method", req.Method,
"url", req.URL.String(),
)
}

func (c *Client) logResponse(ctx context.Context, resp *http.Response) {
if !c.insecure {
return
}
slog.InfoContext(ctx, "API Response",
"status", resp.Status,
"status_code", resp.StatusCode,
)
}
