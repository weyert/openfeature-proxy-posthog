# PostHog OpenFeature Proxy API Reference

This document provides a complete API reference for the PostHog OpenFeature proxy, including endpoints, request/response formats, and authentication.

## Base URL

```
http://localhost:8080
```

## Authentication

The proxy supports bearer token authentication with capability-based access control.

### Capabilities

- `read`: Access to GET endpoints
- `write`: Access to POST and PUT endpoints  
- `delete`: Access to DELETE endpoints

### Authentication Header

```http
Authorization: Bearer <token>
```

### Insecure Mode

For development/testing, authentication can be disabled:

```bash
INSECURE_MODE=true
```

⚠️ **Warning**: Never use insecure mode in production!

## Endpoints

### Health Check

#### `GET /health`

Returns proxy health status and version information.

**Authentication**: None required

**Response**:
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "commit": "abc123",
  "date": "2023-12-07T18:00:00Z",
  "warning": "Running in INSECURE MODE - authentication disabled"
}
```

### Feature Flag Manifest

#### `GET /openfeature/v0/manifest`

Retrieves all feature flags in OpenFeature manifest format.

**Authentication**: Requires `read` capability

**Response**:
```json
{
  "flags": {
    "flag-key": {
      "key": "flag-key",
      "name": "Flag Display Name",
      "type": "boolean|string|number|object",
      "defaultValue": "any",
      "variants": {
        "variant-key": {
          "value": "any",
          "weight": 50
        }
      },
      "state": "ENABLED|DISABLED"
    }
  },
  "timestamp": "2023-12-07T18:00:00Z"
}
```

**Flag Types**:
- `boolean`: True/false flags
- `string`: Text-based flags  
- `number`: Numeric flags (int or float)
- `object`: Complex JSON object flags

**Flag States**:
- `ENABLED`: Flag is active in PostHog
- `DISABLED`: Flag is inactive in PostHog

### Create Feature Flag

#### `POST /openfeature/v0/manifest/flags`

Creates a new feature flag in PostHog.

**Authentication**: Requires `write` capability

**Request Body**:
```json
{
  "key": "new-flag-key",
  "name": "New Flag Display Name",
  "type": "boolean|string|number|object",
  "defaultValue": "any",
  "variants": {
    "variant-key": {
      "value": "any",
      "weight": 50
    }
  }
}
```

**Response**: Returns the created flag in OpenFeature format (same structure as manifest entry).

**Status Codes**:
- `201 Created`: Flag created successfully
- `400 Bad Request`: Invalid request body
- `500 Internal Server Error`: PostHog API error

### Update Feature Flag

#### `PUT /openfeature/v0/manifest/flags/{key}`

Updates an existing feature flag in PostHog.

**Authentication**: Requires `write` capability

**Path Parameters**:
- `key`: The feature flag key to update

**Request Body**:
```json
{
  "name": "Updated Flag Name",
  "state": "ENABLED|DISABLED",
  "variants": {
    "variant-key": {
      "value": "any",
      "weight": 50
    }
  }
}
```

**Response**: Returns the updated flag in OpenFeature format.

**Status Codes**:
- `200 OK`: Flag updated successfully
- `400 Bad Request`: Invalid request body
- `404 Not Found`: Flag not found
- `500 Internal Server Error`: PostHog API error

### Delete Feature Flag

#### `DELETE /openfeature/v0/manifest/flags/{key}`

Deletes a feature flag from PostHog.

**Authentication**: Requires `delete` capability

**Path Parameters**:
- `key`: The feature flag key to delete

**Response**: 
```json
{
  "message": "Flag deleted successfully"
}
```

**Status Codes**:
- `200 OK`: Flag deleted successfully
- `404 Not Found`: Flag not found
- `500 Internal Server Error`: PostHog API error

**Note**: Depending on configuration (`ARCHIVE_INSTEAD_OF_DELETE`), flags may be archived instead of permanently deleted.

## Error Response Format

All error responses follow this format:

```json
{
  "code": 400,
  "message": "Human readable error message",
  "details": "Detailed error information"
}
```

## Configuration Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `POSTHOG_API_KEY` | *required* | PostHog API key |
| `POSTHOG_PROJECT_ID` | *required* | PostHog project ID |
| `POSTHOG_HOST` | `https://app.posthog.com` | PostHog host URL |
| `PROXY_PORT` | `8080` | Proxy server port |
| `INSECURE_MODE` | `false` | Disable authentication (dev only) |
| `READ_TOKEN` | *auto-generated* | Token with read capability |
| `WRITE_TOKEN` | *auto-generated* | Token with read+write capabilities |
| `ADMIN_TOKEN` | *auto-generated* | Token with all capabilities |
| `COERCE_NUMERIC_STRINGS` | `false` | Enable numeric string coercion |
| `COERCE_BOOLEAN_STRINGS` | `false` | Enable boolean string coercion |
| `DEFAULT_ROLLOUT_PERCENTAGE` | `0` | Default rollout for new flags |
| `ARCHIVE_INSTEAD_OF_DELETE` | `true` | Archive flags instead of deleting |

## Type Coercion

When type coercion is enabled, the proxy automatically converts PostHog string payloads to appropriate types:

### Numeric Coercion (`COERCE_NUMERIC_STRINGS=true`)

```json
// PostHog payload: "42"
// OpenFeature value: 42

// PostHog payload: "3.14" 
// OpenFeature value: 3.14
```

### Boolean Coercion (`COERCE_BOOLEAN_STRINGS=true`)

```json
// PostHog payload: "true"
// OpenFeature value: true

// PostHog payload: "false"
// OpenFeature value: false

// Accepted values: "true", "false", "yes", "no", "on", "off"
```

### JSON Object Detection (Always Enabled)

```json
// PostHog payload: "{\"key\": \"value\"}"
// OpenFeature value: {"key": "value"}
// OpenFeature type: "object"
```

## OpenFeature Compliance

This proxy implements the OpenFeature CLI sync API v0.1.0 specification. For full specification details, see:
https://github.com/open-feature/cli/blob/main/api/v0/sync.yaml

## Examples

### Retrieve All Flags

```bash
curl -H "Authorization: Bearer your-read-token" \
  http://localhost:8080/openfeature/v0/manifest
```

### Create a Boolean Flag

```bash
curl -X POST \
  -H "Authorization: Bearer your-write-token" \
  -H "Content-Type: application/json" \
  -d '{
    "key": "new-feature",
    "name": "New Feature Toggle",
    "type": "boolean", 
    "defaultValue": false
  }' \
  http://localhost:8080/openfeature/v0/manifest/flags
```

### Update Flag State

```bash
curl -X PUT \
  -H "Authorization: Bearer your-write-token" \
  -H "Content-Type: application/json" \
  -d '{
    "state": "ENABLED"
  }' \
  http://localhost:8080/openfeature/v0/manifest/flags/new-feature
```

### Delete a Flag

```bash
curl -X DELETE \
  -H "Authorization: Bearer your-admin-token" \
  http://localhost:8080/openfeature/v0/manifest/flags/new-feature
```

## Debugging

### API Logging

When `INSECURE_MODE=true`, the proxy logs all PostHog API requests and responses to `./logs/` directory for debugging.

### Health Check with Configuration

```bash
curl http://localhost:8080/health
```

Returns configuration warnings and version information.

## Limitations

1. **Rate Limits**: Subject to PostHog API rate limits
2. **Real-time Updates**: Changes made directly in PostHog may not be immediately reflected (no webhooks)
3. **Complex Filters**: Some PostHog advanced filtering features are not exposed in the OpenFeature API
4. **Bulk Operations**: No bulk import/export operations currently supported

## Support

For issues related to the proxy implementation, refer to:
- `/docs/transformation-rules.md` for transformation logic
- Source code in `/internal/` directories
- PostHog API documentation: https://posthog.com/docs/api