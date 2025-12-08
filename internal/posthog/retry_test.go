package posthog

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/openfeature/posthog-proxy/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockRoundTripper for capturing requests and returning mocked responses
type MockRoundTripper struct {
	mock.Mock
}

func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	
	// Check if the first argument is a function
	if fn, ok := args.Get(0).(func(*http.Request) *http.Response); ok {
		// If first arg is func, second arg might be func too or just error
		if errFn, ok := args.Get(1).(func(*http.Request) error); ok {
			return fn(req), errFn(req)
		}
		return fn(req), args.Error(1)
	}

	var resp *http.Response
	if args.Get(0) != nil {
		resp = args.Get(0).(*http.Response)
	}
	return resp, args.Error(1)
}

func TestDoWithRetry(t *testing.T) {
	tests := []struct {
		name           string
		responses      []*http.Response // Sequence of responses to return
		errors         []error          // Sequence of errors to return
		expectedStatus int
		expectedError  string
		expectRetries  int
	}{
		{
			name: "Success on first attempt",
			responses: []*http.Response{
				{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString("{}"))},
			},
			expectedStatus: http.StatusOK,
			expectRetries:  0,
		},
		{
			name: "Success after one 500 error",
			responses: []*http.Response{
				{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(bytes.NewBufferString("error"))},
				{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString("{}"))},
			},
			expectedStatus: http.StatusOK,
			expectRetries:  1,
		},
		{
			name: "Success after one 429 error",
			responses: []*http.Response{
				{StatusCode: http.StatusTooManyRequests, Body: io.NopCloser(bytes.NewBufferString("rate limit"))},
				{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString("{}"))},
			},
			expectedStatus: http.StatusOK,
			expectRetries:  1,
		},
		{
			name: "Fail after max retries (500)",
			responses: []*http.Response{
				{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(bytes.NewBufferString("error"))},
				{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(bytes.NewBufferString("error"))},
				{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(bytes.NewBufferString("error"))},
				{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(bytes.NewBufferString("error"))},
			},
			expectedError: "max retries exceeded: server returned status 500",
			expectRetries: 3,
		},
		{
			name: "No retry on 400",
			responses: []*http.Response{
				{StatusCode: http.StatusBadRequest, Body: io.NopCloser(bytes.NewBufferString("bad request"))},
			},
			expectedStatus: http.StatusBadRequest,
			expectRetries:  0,
		},
		{
			name: "No retry on 404",
			responses: []*http.Response{
				{StatusCode: http.StatusNotFound, Body: io.NopCloser(bytes.NewBufferString("not found"))},
			},
			expectedStatus: http.StatusNotFound,
			expectRetries:  0,
		},
		{
			name: "Retry on network error",
			errors: []error{
				errors.New("connection refused"),
				nil,
			},
			responses: []*http.Response{
				nil,
				{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString("{}"))},
			},
			expectedStatus: http.StatusOK,
			expectRetries:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTransport := new(MockRoundTripper)
			
			// Setup mock expectations
			callCount := 0
			mockTransport.On("RoundTrip", mock.Anything).Run(func(args mock.Arguments) {
				// Just to track calls, logic is in Return
			}).Return(func(req *http.Request) *http.Response {
				idx := callCount
				callCount++
				
				if idx < len(tt.errors) && tt.errors[idx] != nil {
					return nil
				}
				if idx < len(tt.responses) {
					// Re-create body reader because it gets closed
					resp := tt.responses[idx]
					if resp != nil && resp.Body != nil {
						// We need a fresh body for each return if we want to be strict, 
						// but for this test the simple NopCloser is fine as long as we don't read it multiple times in test logic
						// However, the retry logic closes the body, so we should probably ensure it's safe.
						// For simplicity, we assume the test setup provides fresh bodies or we don't read them in the test verification deeply.
					}
					return resp
				}
				return nil
			}, func(req *http.Request) error {
				idx := callCount - 1 // callCount was incremented in the first func
				if idx < len(tt.errors) {
					return tt.errors[idx]
				}
				return nil
			})

			client := NewClient(config.PostHogConfig{Host: "http://localhost", ProjectID: "123"}, false)
			client.httpClient.Transport = mockTransport
			
			// Set fast retry config for tests
			client.retryConfig = RetryConfig{
				MaxRetries:     3,
				InitialBackoff: 1 * time.Millisecond,
				MaxBackoff:     10 * time.Millisecond,
			}

			req, _ := http.NewRequest("GET", "http://localhost/api", nil)
			resp, err := client.doWithRetry(context.Background(), req)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedStatus, resp.StatusCode)
				if resp != nil {
					resp.Body.Close()
				}
			}

			// Verify number of calls (initial + retries)
			// expectRetries is the number of *retries*, so total calls = 1 + expectRetries
			// But if we fail on max retries, we make 1 + MaxRetries calls.
			// Let's just check callCount matches what we expect.
			// Actually, checking mockTransport.AssertNumberOfCalls is better if we knew the exact count.
			// But our dynamic Return makes it slightly complex.
			// Let's just assert callCount.
			
			expectedCalls := tt.expectRetries + 1
			assert.Equal(t, expectedCalls, callCount, "Unexpected number of requests")
		})
	}
}

func TestDoWithRetry_ContextCancellation(t *testing.T) {
	mockTransport := new(MockRoundTripper)
	mockTransport.On("RoundTrip", mock.Anything).Return(
		&http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(bytes.NewBufferString("error"))},
		nil,
	)

	client := NewClient(config.PostHogConfig{Host: "http://localhost", ProjectID: "123"}, false)
	client.httpClient.Transport = mockTransport
	
	// Long backoff to ensure we can cancel during it
	client.retryConfig = RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     1 * time.Second,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	req, _ := http.NewRequest("GET", "http://localhost/api", nil)
	_, err := client.doWithRetry(ctx, req)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled))
}

func TestDoWithRetry_RetryAfterHeader(t *testing.T) {
	mockTransport := new(MockRoundTripper)
	
	// First response: 429 with Retry-After: 1 (second)
	// Second response: 200 OK
	
	start := time.Now()
	
	mockTransport.On("RoundTrip", mock.Anything).Return(func(req *http.Request) *http.Response {
		// If it's the first call (fast), return 429
		if time.Since(start) < 100*time.Millisecond {
			header := http.Header{}
			header.Set("Retry-After", "1") // 1 second
			return &http.Response{
				StatusCode: http.StatusTooManyRequests, 
				Header: header,
				Body: io.NopCloser(bytes.NewBufferString("rate limit")),
			}
		}
		// If enough time passed, return 200
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString("{}"))}
	}, nil)

	client := NewClient(config.PostHogConfig{Host: "http://localhost", ProjectID: "123"}, false)
	client.httpClient.Transport = mockTransport
	
	// Config with small backoff, so Retry-After (1s) should override it
	client.retryConfig = RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     5 * time.Second,
	}

	req, _ := http.NewRequest("GET", "http://localhost/api", nil)
	
	// This test might be flaky if system is very slow, but logic should hold
	resp, err := client.doWithRetry(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	
	// Verify that we waited at least ~1 second
	// We allow some buffer for execution time
	assert.True(t, time.Since(start) >= 1*time.Second, "Should have waited for Retry-After duration")
}
