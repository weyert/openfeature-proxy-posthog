package models

import "time"

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
}

// UpdateFlagRequest represents a request to update a feature flag
type UpdateFlagRequest struct {
	Name         *string             `json:"name,omitempty"`
	Description  *string             `json:"description,omitempty"`
	Type         *FlagType           `json:"type,omitempty"`
	DefaultValue interface{}         `json:"defaultValue,omitempty"`
	Variants     *map[string]Variant `json:"variants,omitempty"`
	State        *FlagState          `json:"state,omitempty"`
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