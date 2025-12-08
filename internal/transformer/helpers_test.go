package transformer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsNumeric(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Integer", "123", true},
		{"Negative integer", "-456", true},
		{"Float", "123.45", true},
		{"Negative float", "-123.45", true},
		{"Not numeric", "abc", false},
		{"Empty string", "", false},
		{"Mixed", "123abc", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNumeric(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseNumeric(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedVal interface{}
		expectError bool
	}{
		{"Integer", "123", 123, false},
		{"Negative integer", "-456", -456, false},
		{"Float", "123.45", 123.45, false},
		{"Not numeric", "abc", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseNumeric(tt.input)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedVal, result)
			}
		})
	}
}

func TestIsJSONObject(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Valid JSON object", `{"key": "value"}`, true},
		{"Empty object", `{}`, true},
		{"With whitespace", `  {"key": "value"}  `, true},
		{"Array", `[1, 2, 3]`, false},
		{"String", `"hello"`, false},
		{"Number", `123`, false},
		{"Invalid", `{key: value}`, true}, // Only checks format, not validity
		{"Empty", ``, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isJSONObject(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseJSONObject(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    map[string]interface{}
		expectError bool
	}{
		{
			name:  "Valid object",
			input: `{"key": "value", "number": 123}`,
			expected: map[string]interface{}{
				"key":    "value",
				"number": float64(123), // JSON numbers are float64
			},
			expectError: false,
		},
		{
			name:        "Invalid JSON",
			input:       `{key: value}`,
			expected:    nil,
			expectError: true,
		},
		{
			name:        "Array",
			input:       `[1, 2, 3]`,
			expected:    nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseJSONObject(tt.input)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestTryParseBooleanString(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedValue bool
		expectedFound bool
	}{
		{"true", "true", true, true},
		{"True", "True", true, true},
		{"TRUE", "TRUE", true, true},
		{"yes", "yes", true, true},
		{"on", "on", true, true},
		{"false", "false", false, true},
		{"False", "False", false, true},
		{"no", "no", false, true},
		{"off", "off", false, true},
		{"not boolean", "maybe", false, false},
		{"number", "1", false, false},
		{"empty", "", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, found := tryParseBooleanString(tt.input)
			assert.Equal(t, tt.expectedFound, found)
			if found {
				assert.Equal(t, tt.expectedValue, value)
			}
		})
	}
}

func TestTryParseNumericString(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedValue interface{}
		expectedFound bool
	}{
		{"integer", "123", 123, true},
		{"negative integer", "-456", -456, true},
		{"float", "123.45", 123.45, true},
		{"negative float", "-123.45", -123.45, true},
		{"with whitespace", "  789  ", 789, true},
		{"not numeric", "abc", nil, false},
		{"empty", "", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, found := tryParseNumericString(tt.input)
			assert.Equal(t, tt.expectedFound, found)
			if found {
				assert.Equal(t, tt.expectedValue, value)
			}
		})
	}
}
