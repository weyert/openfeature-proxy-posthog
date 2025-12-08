package posthog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/openfeature/posthog-proxy/internal/models"
)

// GetFeatureFlagsWithOptions retrieves feature flags with filtering options
func (c *Client) GetFeatureFlagsWithOptions(ctx context.Context, opts *ListFlagsOptions) ([]models.PostHogFeatureFlag, error) {
	baseURL := fmt.Sprintf("%s/feature_flags/", c.baseURL)
	
	// Add query parameters if options provided
	if opts != nil {
		params := opts.ToQueryParams()
		if len(params) > 0 {
			query := url.Values{}
			for k, v := range params {
				query.Add(k, v)
			}
			baseURL = fmt.Sprintf("%s?%s", baseURL, query.Encode())
		}
	}

	nextURL := baseURL
	var allFlags []models.PostHogFeatureFlag

	for nextURL != "" {
		req, err := c.newRequest(ctx, http.MethodGet, nextURL, nil)
		if err != nil {
			slog.ErrorContext(ctx, "GetFeatureFlagsWithOptions - creating request", "error", err)
			return nil, fmt.Errorf("creating request: %w", err)
		}

		c.logRequest(ctx, req)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			slog.ErrorContext(ctx, "GetFeatureFlagsWithOptions - HTTP request", "error", err)
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
				slog.ErrorContext(ctx, "GetFeatureFlagsWithOptions - decoding response", "error", err)
				return fmt.Errorf("decoding response: %w", err)
			}

			// Filter out deleted flags
			for _, flag := range page.Results {
				if !flag.Deleted {
					allFlags = append(allFlags, flag)
				}
			}
			
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

	slog.InfoContext(ctx, "GetFeatureFlagsWithOptions - Successfully retrieved flags", "count", len(allFlags))
	return allFlags, nil
}

// GetFeatureFlagActivity retrieves the audit log for a feature flag
func (c *Client) GetFeatureFlagActivity(ctx context.Context, id int) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s/feature_flags/%d/activity/", c.baseURL, id)

	req, err := c.newRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		slog.ErrorContext(ctx, "GetFeatureFlagActivity - creating request", "error", err)
		return nil, fmt.Errorf("creating request: %w", err)
	}

	c.logRequest(ctx, req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.ErrorContext(ctx, "GetFeatureFlagActivity - HTTP request", "error", err)
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	c.logResponse(ctx, resp)

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseErrorResponse(resp)
	}

	var activity []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&activity); err != nil {
		slog.ErrorContext(ctx, "GetFeatureFlagActivity - decoding response", "error", err)
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	slog.InfoContext(ctx, "GetFeatureFlagActivity - Successfully retrieved activity", "id", id)
	return activity, nil
}

// parseErrorResponse attempts to parse a structured API error response
func (c *Client) parseErrorResponse(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("PostHog API error: status %d (failed to read body)", resp.StatusCode)
	}

	// Try to parse as structured error
	var apiErr APIError
	if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Detail != "" {
		apiErr.StatusCode = resp.StatusCode
		slog.Error("PostHog API error", "error", &apiErr)
		return &apiErr
	}

	// Fallback to raw error
	rawErr := fmt.Errorf("PostHog API error: status %d: %s", resp.StatusCode, string(body))
	slog.Error("PostHog API error", "error", rawErr)
	return rawErr
}
