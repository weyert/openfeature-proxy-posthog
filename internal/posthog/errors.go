package posthog

import "fmt"

// APIError represents a structured error response from PostHog API
type APIError struct {
	Type       string `json:"type"`
	Code       string `json:"code"`
	Detail     string `json:"detail"`
	Attr       string `json:"attr,omitempty"`
	StatusCode int    `json:"-"`
}

func (e *APIError) Error() string {
	if e.Attr != "" {
		return fmt.Sprintf("PostHog API error [%s/%s] at %s: %s (status %d)",
			e.Type, e.Code, e.Attr, e.Detail, e.StatusCode)
	}
	return fmt.Sprintf("PostHog API error [%s/%s]: %s (status %d)",
		e.Type, e.Code, e.Detail, e.StatusCode)
}

// IsNotFound returns true if the error is a 404 not found error
func (e *APIError) IsNotFound() bool {
	return e.StatusCode == 404
}

// IsValidationError returns true if the error is a validation error
func (e *APIError) IsValidationError() bool {
	return e.Type == "validation_error"
}

// IsAuthError returns true if the error is an authentication error
func (e *APIError) IsAuthError() bool {
	return e.StatusCode == 401 || e.StatusCode == 403
}
