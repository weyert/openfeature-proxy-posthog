package models

import (
	"encoding/json"
	"time"
)

// OpenFeature manifest and flag models based on the sync API specification

// Manifest represents the OpenFeature manifest response
// Following the OpenFeature CLI spec - flags should be an array
type Manifest struct {
	Flags []ManifestFlag `json:"flags"`
}

// ManifestFlag represents a feature flag in OpenFeature manifest format
type ManifestFlag struct {
	Key          string             `json:"key"`
	Name         string             `json:"name,omitempty"`
	Description  string             `json:"description,omitempty"`
	Type         FlagType           `json:"type"`
	DefaultValue interface{}        `json:"defaultValue"`
	Variants     map[string]Variant `json:"variants,omitempty"`
	State        FlagState          `json:"state"`
	Expiry       *time.Time         `json:"expiry,omitempty"`
}

// FlagType represents the type of a feature flag
type FlagType string

const (
	FlagTypeBoolean FlagType = "boolean"
	FlagTypeString  FlagType = "string"
	FlagTypeInteger FlagType = "integer"
	FlagTypeObject  FlagType = "object"
)

// FlagState represents the state of a feature flag
type FlagState string

const (
	FlagStateEnabled  FlagState = "ENABLED"
	FlagStateDisabled FlagState = "DISABLED"
)

// Variant represents a flag variant
type Variant struct {
	Value  interface{} `json:"value"`
	Weight *int        `json:"weight,omitempty"`
}

// CreateFlagRequest represents a request to create a feature flag
type CreateFlagRequest struct {
	Key          string             `json:"key" binding:"required"`
	Name         string             `json:"name,omitempty"`
	Description  string             `json:"description,omitempty"`
	Type         FlagType           `json:"type" binding:"required"`
	DefaultValue interface{}        `json:"defaultValue" binding:"required"`
	Variants     map[string]Variant `json:"variants,omitempty"`
	Expiry       *time.Time         `json:"expiry,omitempty"`
}

// UpdateFlagRequest represents a request to update a feature flag
type UpdateFlagRequest struct {
	Name         *string             `json:"name,omitempty"`
	Description  *string             `json:"description,omitempty"`
	Type         *FlagType           `json:"type,omitempty"`
	DefaultValue interface{}         `json:"defaultValue,omitempty"`
	Variants     *map[string]Variant `json:"variants,omitempty"`
	State        *FlagState          `json:"state,omitempty"`
	Expiry       *NullableTime       `json:"expiry,omitempty"`
}

// UnmarshalJSON allows distinguishing between missing and explicit null expiry values.
func (r *UpdateFlagRequest) UnmarshalJSON(data []byte) error {
	type alias struct {
		Name         *string             `json:"name,omitempty"`
		Description  *string             `json:"description,omitempty"`
		Type         *FlagType           `json:"type,omitempty"`
		DefaultValue interface{}         `json:"defaultValue,omitempty"`
		Variants     *map[string]Variant `json:"variants,omitempty"`
		State        *FlagState          `json:"state,omitempty"`
	}

	var aux struct {
		alias
		Expiry json.RawMessage `json:"expiry"`
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	r.Name = aux.Name
	r.Description = aux.Description
	r.Type = aux.Type
	r.DefaultValue = aux.DefaultValue
	r.Variants = aux.Variants
	r.State = aux.State

	if aux.Expiry != nil {
		if string(aux.Expiry) == "null" {
			r.Expiry = &NullableTime{}
			return nil
		}

		var parsed NullableTime
		if err := parsed.UnmarshalJSON(aux.Expiry); err != nil {
			return err
		}
		r.Expiry = &parsed
	} else {
		r.Expiry = nil
	}

	return nil
}

// ManifestFlagResponse represents the response when creating or updating a flag
// Following the OpenFeature CLI spec schema
type ManifestFlagResponse struct {
	Flag      ManifestFlag `json:"flag"`
	UpdatedAt time.Time    `json:"updatedAt"`
}

// ArchiveResponse represents the response when deleting/archiving a flag
// Following the OpenFeature CLI spec schema
type ArchiveResponse struct {
	Message    string     `json:"message"`
	ArchivedAt *time.Time `json:"archivedAt"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// NullableTime captures optional RFC3339 timestamps while preserving whether the
// field was explicitly provided (including explicit null).
type NullableTime struct {
	Value *time.Time
}

// TimePtr returns the parsed time pointer (may be nil).
func (nt *NullableTime) TimePtr() *time.Time {
	if nt == nil {
		return nil
	}
	return nt.Value
}

// MarshalJSON serializes the nullable time as an RFC3339 string or null.
func (nt NullableTime) MarshalJSON() ([]byte, error) {
	if nt.Value == nil {
		return []byte("null"), nil
	}

	return json.Marshal(nt.Value.UTC().Format(time.RFC3339))
}

// UnmarshalJSON parses RFC3339 timestamps or null values.
func (nt *NullableTime) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		nt.Value = nil
		return nil
	}

	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}

	if value == "" {
		nt.Value = nil
		return nil
	}

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return err
	}

	nt.Value = &parsed
	return nil
}
