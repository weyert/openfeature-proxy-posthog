package posthog

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

const (
	defaultRetryCount     = 3
	defaultInitialBackoff = 1 * time.Second
	defaultMaxBackoff     = 10 * time.Second
)

// RetryConfig holds configuration for retry logic
type RetryConfig struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
}

// DefaultRetryConfig returns the default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:     defaultRetryCount,
		InitialBackoff: defaultInitialBackoff,
		MaxBackoff:     defaultMaxBackoff,
	}
}

// doWithRetry executes an HTTP request with exponential backoff retry logic
func (c *Client) doWithRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
	config := c.retryConfig

	var lastErr error
	var resp *http.Response

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Calculate backoff: initial * 2^(attempt-1)
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * config.InitialBackoff
			if backoff > config.MaxBackoff {
				backoff = config.MaxBackoff
			}

			// Add jitter: +/- 20%
			// Ensure backoff is large enough for jitter calculation
			if backoff > 0 {
				jitterRange := int64(backoff) / 5 // 20%
				if jitterRange > 0 {
					jitter := time.Duration(rand.Int63n(jitterRange*2) - jitterRange)
					backoff += jitter
				}
			}
			if backoff < 0 {
				backoff = 0
			}

			// Check for Retry-After header from previous response
			if resp != nil {
				if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
					if seconds, err := strconv.Atoi(retryAfter); err == nil {
						wait := time.Duration(seconds) * time.Second
						if wait > backoff {
							backoff = wait
						}
					} else if date, err := http.ParseTime(retryAfter); err == nil {
						wait := time.Until(date)
						if wait > backoff {
							backoff = wait
						}
					}
				}
			}

			slog.InfoContext(ctx, "Retrying request",
				"attempt", attempt,
				"backoff", backoff,
				"url", req.URL.String())

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}

			// Reset request body for retry if available
			if req.GetBody != nil {
				body, err := req.GetBody()
				if err != nil {
					return nil, fmt.Errorf("failed to get request body for retry: %w", err)
				}
				req.Body = body
			} else if req.Body != nil {
				// If GetBody is not set but Body is present, try to seek if possible
				if seeker, ok := req.Body.(io.Seeker); ok {
					if _, err := seeker.Seek(0, 0); err != nil {
						return nil, fmt.Errorf("failed to seek request body: %w", err)
					}
				}
			}
		}

		resp, lastErr = c.httpClient.Do(req)
		if lastErr != nil {
			// Network error, retry
			slog.WarnContext(ctx, "Request failed", "error", lastErr, "attempt", attempt)
			continue
		}

		// Check for 5xx errors or 429 Too Many Requests
		if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
			// Read and close body to ensure connection reuse
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()

			slog.WarnContext(ctx, "Server returned transient error", "status", resp.StatusCode, "attempt", attempt)
			lastErr = fmt.Errorf("server returned status %d", resp.StatusCode)
			continue
		}

		// Success or non-retriable error (4xx except 429)
		return resp, nil
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}
