# PostHog Feature Flags API Reference

This document outlines the PostHog API endpoints most relevant for our OpenFeature CLI PostHog proxy application.

## Overview

PostHog provides a comprehensive REST API for managing and evaluating feature flags. The API supports:
- Boolean and multivariate flags
- Property-based, cohort-based, and group-based targeting
- Gradual rollouts (percentage-based)
- Remote configuration payloads
- Experimentation support (A/B testing)

**Base URL:** `https://app.posthog.com` (US) or `https://eu.posthog.com` (EU)

**Authentication:** Personal API Key in header: `Authorization: Bearer <API_KEY>`

## Feature Flag Management Endpoints

### List Feature Flags

**Endpoint:** `GET /api/projects/:project_id/feature_flags/`

Retrieves all feature flags for a project with optional filtering.

**Query Parameters:**
- `active` (boolean) - Filter by active/inactive status
- `created_by_id` (integer) - Filter by creator
- `evaluation_runtime` (string) - Filter by evaluation runtime
- `limit` (integer) - Pagination limit
- `offset` (integer) - Pagination offset

**Response Example:**
```json
{
  "count": 10,
  "next": null,
  "previous": null,
  "results": [
    {
      "id": 123,
      "name": "New UI Feature",
      "key": "new-ui-feature",
      "active": true,
      "filters": {
        "groups": [
          {
            "properties": [],
            "rollout_percentage": 100
          }
        ]
      }
    }
  ]
}
```

### Create Feature Flag

**Endpoint:** `POST /api/projects/:project_id/feature_flags/`

Creates a new feature flag with targeting rules and rollout configuration.

**Request Body:**
```json
{
  "key": "new-feature",
  "name": "New Feature Description",
  "active": true,
  "filters": {
    "groups": [
      {
        "properties": [],
        "rollout_percentage": 50,
        "variant": null
      }
    ],
    "multivariate": null,
    "payloads": {}
  },
  "ensure_experience_continuity": false
}
```

**Key Fields:**
- `key` (string, required) - Unique identifier for the flag
- `name` (string) - Human-readable description
- `active` (boolean) - Whether the flag is active
- `filters.groups` (array) - Targeting rules and rollout configuration
  - `rollout_percentage` (number) - Percentage of users to include (0-100)
  - `properties` (array) - User/group property filters
  - `variant` (string|null) - For multivariate flags
- `filters.multivariate` (object|null) - Multivariate configuration
  - `variants` (array) - List of variant definitions
- `filters.payloads` (object) - Remote configuration payloads per variant
- `ensure_experience_continuity` (boolean) - Maintains consistent experience

**Multivariate Flag Example:**
```json
{
  "key": "button-color",
  "name": "Button Color Test",
  "active": true,
  "filters": {
    "groups": [
      {
        "properties": [],
        "rollout_percentage": 100
      }
    ],
    "multivariate": {
      "variants": [
        {
          "key": "control",
          "name": "Control",
          "rollout_percentage": 33
        },
        {
          "key": "blue",
          "name": "Blue Button",
          "rollout_percentage": 33
        },
        {
          "key": "green",
          "name": "Green Button",
          "rollout_percentage": 34
        }
      ]
    },
    "payloads": {
      "control": "{\"color\": \"#000000\"}",
      "blue": "{\"color\": \"#0000FF\"}",
      "green": "{\"color\": \"#00FF00\"}"
    }
  }
}
```

### Get Single Feature Flag

**Endpoint:** `GET /api/projects/:project_id/feature_flags/:id/`

Retrieves details for a specific feature flag.

**Response:** Same structure as individual flag in list endpoint.

### Update Feature Flag

**Endpoint:** `PATCH /api/projects/:project_id/feature_flags/:id/`

Updates an existing feature flag. Only provided fields are updated.

**Request Body:** Same structure as create endpoint (partial updates supported).

**Important Notes:**
- When updating `filters.groups`, the entire groups array must be provided
- Variant weights must sum to 100 for multivariate flags
- Use GET to fetch current state before updating to preserve existing configuration

### Delete Feature Flag

**Endpoint:** `DELETE /api/projects/:project_id/feature_flags/:id/`

Deletes a feature flag.

**Response:** `204 No Content` on success.

## Feature Flag Evaluation Endpoints

### Evaluate Feature Flags for a User

**Endpoint:** `POST /decide/?v=3`

Evaluates which flags are enabled for a specific user or group.

**Authentication:** Project API Key (different from Personal API Key)

**Request Body:**
```json
{
  "api_key": "<PROJECT_API_KEY>",
  "distinct_id": "user@example.com",
  "person_properties": {
    "email": "user@example.com",
    "plan": "premium"
  },
  "groups": {
    "company": "acme-corp"
  }
}
```

**Response Example:**
```json
{
  "featureFlags": {
    "new-ui": true,
    "button-color": "blue",
    "experimental-mode": false
  },
  "featureFlagPayloads": {
    "button-color": "{\"color\": \"#0000FF\"}"
  },
  "errorsWhileComputingFlags": false
}
```

**Parameters:**
- `distinct_id` (string, required) - User identifier
- `person_properties` (object) - User properties for targeting
- `groups` (object) - Group memberships for group-based targeting
- `$geoip_disable` (boolean) - Disable GeoIP lookups

**Response Fields:**
- `featureFlags` (object) - Map of flag keys to boolean or variant values
- `featureFlagPayloads` (object) - Remote configuration payloads
- `errorsWhileComputingFlags` (boolean) - Whether any errors occurred

### Local Evaluation

**Endpoint:** `GET /api/feature_flag/local_evaluation/`

Retrieves all feature flag definitions for local evaluation (SDK usage).

**Query Parameters:**
- `send_cohorts` (boolean) - Include cohort definitions

**Response:** Array of feature flag definitions with all targeting rules.

## Additional Utility Endpoints

### Feature Flag Activity

**Endpoint:** `GET /api/projects/:project_id/feature_flags/:id/activity/`

Retrieves audit log of changes to a feature flag.

### My Flags (Current User)

**Endpoint:** `GET /api/projects/:project_id/feature_flags/my_flags/`

Evaluates all flags for the current authenticated user.

### Create Static Cohort

**Endpoint:** `POST /api/projects/:project_id/feature_flags/:id/create_static_cohort_for_flag/`

Creates a cohort of users who match the flag's targeting rules.

## OpenFeature Manifest Integration

Our proxy translates between OpenFeature manifest format and PostHog API format:

### OpenFeature to PostHog Mapping

| OpenFeature Field | PostHog Field | Notes |
|------------------|---------------|-------|
| `flags.{key}` | `key` | Flag identifier |
| `description` | `name` | Human-readable name |
| `state` | `active` | ENABLED → true, DISABLED → false |
| `variants` | `filters.multivariate.variants` | For string/object flags |
| `defaultVariant` | Used to set rollout percentages | Control variant gets higher weight |
| `targeting` | `filters.groups[0].properties` | Property-based rules |

### Flag Type Mapping

| OpenFeature Type | PostHog Implementation |
|-----------------|------------------------|
| boolean | Simple flag with rollout_percentage |
| string | Multivariate flag with string variants |
| integer | Multivariate flag with payloads |
| float | Multivariate flag with payloads |
| object | Multivariate flag with JSON payloads |

## Important Constraints

### Variant Weight Validation

**PostHog Requirement:** For multivariate flags, variant rollout percentages must sum to exactly 100.

**Strategy:** 
- Equal distribution: `100 / variant_count`
- Remainder distributed to first variant(s)
- Example: 3 variants → [34, 33, 33]

### Groups Array Structure

**PostHog Requirement:** The `filters.groups` array defines targeting rules. Each group represents an OR condition.

**Structure:**
```json
{
  "groups": [
    {
      "properties": [
        {
          "key": "email",
          "value": "@example.com",
          "operator": "icontains",
          "type": "person"
        }
      ],
      "rollout_percentage": 100,
      "variant": null
    }
  ]
}
```

**Common Operators:**
- `exact` - Exact match
- `icontains` - Case-insensitive contains
- `regex` - Regular expression match
- `gt`, `gte`, `lt`, `lte` - Numeric comparisons
- `is_set` - Property exists

### Experience Continuity

Setting `ensure_experience_continuity: true` ensures users see consistent flag values across sessions using deterministic hashing.

## Error Handling

### Common Error Responses

**400 Bad Request:**
```json
{
  "type": "validation_error",
  "code": "invalid_input",
  "detail": "Variant rollout percentages must sum to 100",
  "attr": "filters.multivariate.variants"
}
```

**401 Unauthorized:**
```json
{
  "detail": "Invalid token."
}
```

**404 Not Found:**
```json
{
  "detail": "Not found."
}
```

## Rate Limits

- Management API: 480 requests/minute per project
- Evaluation API (/decide): Higher limits, suitable for client-side usage
- Local evaluation: Recommended for high-volume server-side usage

## Best Practices

1. **Use Local Evaluation for High Volume:** Download flag definitions and evaluate locally to reduce API calls
2. **Batch Updates:** When updating multiple flags, use PATCH to update only changed fields
3. **Version Control:** Store flag configurations in version control using OpenFeature manifest format
4. **Fetch Before Update:** Always GET current flag state before PATCH to preserve existing configuration
5. **Test Rollouts:** Use small rollout percentages initially, then gradually increase
6. **Monitor Errors:** Check `errorsWhileComputingFlags` in evaluation responses
7. **Use Cohorts:** For complex targeting, create cohorts in PostHog UI and reference them in flags

## References

- [PostHog Feature Flags API Reference](https://posthog.com/docs/api/feature-flags)
- [PostHog Evaluation API](https://posthog.com/docs/api/flags)
- [Feature Flags Tutorial](https://posthog.com/tutorials/api-feature-flags)
- [OpenFeature Specification](https://openfeature.dev/specification/sections/flag-evaluation)

## Implementation Notes for Our Proxy

### Current Implementation Status

✅ **Implemented:**
- GET manifest (converts PostHog flags to OpenFeature manifest)
- POST create flag (with basic boolean support)
- DELETE flag

🚧 **In Progress:**
- PATCH update flag (needs variant weight normalization)
- Groups array handling in updates

❌ **Not Implemented:**
- Cohort-based targeting
- Property-based targeting translation
- Remote configuration payloads
- A/B testing experiment integration

### Known Issues

1. **Variant Weight Sum:** PostHog requires weights to sum to exactly 100
   - **Solution:** Implement weight normalization in transformer
   
2. **Groups Array Preservation:** Updates may overwrite existing targeting rules
   - **Solution:** GET flag before PATCH to merge changes

3. **Type Mapping:** Not all OpenFeature types map cleanly to PostHog
   - **Solution:** Use payloads for complex types (integer, float, object)

### Future Enhancements

- Support for targeting rules in manifest
- Cohort integration
- Bulk operations
- Flag dependencies
- Audit log integration
