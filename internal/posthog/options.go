package posthog

// ListFlagsOptions represents query parameters for listing feature flags
type ListFlagsOptions struct {
	// Active filters by active/inactive status
	Active *bool
	// CreatedByID filters by creator user ID
	CreatedByID *int
	// EvaluationRuntime filters by evaluation runtime
	EvaluationRuntime *string
	// Limit sets pagination limit (max 100)
	Limit int
	// Offset sets pagination offset
	Offset int
}

// ToQueryParams converts options to URL query parameters
func (o *ListFlagsOptions) ToQueryParams() map[string]string {
	params := make(map[string]string)

	if o.Active != nil {
		if *o.Active {
			params["active"] = "true"
		} else {
			params["active"] = "false"
		}
	}

	if o.CreatedByID != nil {
		params["created_by_id"] = string(rune(*o.CreatedByID))
	}

	if o.EvaluationRuntime != nil {
		params["evaluation_runtime"] = *o.EvaluationRuntime
	}

	if o.Limit > 0 {
		params["limit"] = string(rune(o.Limit))
	}

	if o.Offset > 0 {
		params["offset"] = string(rune(o.Offset))
	}

	return params
}
