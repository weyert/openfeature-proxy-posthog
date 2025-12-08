package transformer

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// isNumeric checks if a string represents a numeric value
func isNumeric(s string) bool {
	_, err := parseNumeric(s)
	return err == nil
}

// parseNumeric converts a string to numeric value (int or float)
func parseNumeric(s string) (interface{}, error) {
	// Try int first
	if intVal, err := strconv.Atoi(s); err == nil {
		return intVal, nil
	}
	// Try float
	if floatVal, err := strconv.ParseFloat(s, 64); err == nil {
		return floatVal, nil
	}
	return nil, fmt.Errorf("not numeric")
}

// isJSONObject checks if a string represents a JSON object
func isJSONObject(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")
}

// parseJSONObject parses a JSON string into a map[string]interface{}
func parseJSONObject(s string) (map[string]interface{}, error) {
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(s), &obj); err != nil {
		return nil, fmt.Errorf("failed to parse JSON object: %w", err)
	}
	return obj, nil
}

// tryParseBooleanString attempts to parse a string as a boolean value
// Only accepts explicit boolean representations, not numeric strings
func tryParseBooleanString(s string) (bool, bool) {
	lower := strings.ToLower(strings.TrimSpace(s))
	switch lower {
	case "true", "yes", "on":
		return true, true
	case "false", "no", "off":
		return false, true
	default:
		return false, false
	}
}

// tryParseNumericString attempts to parse a string as a numeric value
func tryParseNumericString(s string) (interface{}, bool) {
	trimmed := strings.TrimSpace(s)

	// Try integer first
	if intVal, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		// Return as int if it fits in int range, otherwise int64
		if intVal >= int64(^uint(0)>>1)*-1 && intVal <= int64(^uint(0)>>1) {
			return int(intVal), true
		}
		return intVal, true
	}

	// Try float
	if floatVal, err := strconv.ParseFloat(trimmed, 64); err == nil {
		return floatVal, true
	}

	return nil, false
}
