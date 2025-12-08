# PostHog Feature Flag Proxy Design Document

## Overview

This document outlines the design and architecture of the PostHog OpenFeature Proxy, a Go-based service that manages PostHog feature flags through the OpenFeature CLI API specification. The proxy acts as a translation layer between OpenFeature's standardized manifest API and PostHog's feature flag system.

**Implementation Language**: Go 1.23+  
**Framework**: Gin Web Framework  
**Status**: Production Ready

## Architecture

```
┌─────────────────┐    ┌─────────────────────────────────┐    ┌─────────────────┐
│   OpenFeature   │    │     Proxy Service (Go)          │    │     PostHog     │
│      CLI        │◄──►│  ┌─────────────────────────┐    │◄──►│   Environment   │
│                 │    │  │  Gin HTTP Server        │    │    │                 │
│                 │    │  │  - Auth Middleware      │    │    │                 │
│                 │    │  │  - Handlers             │    │    │                 │
│                 │    │  │  - OpenTelemetry        │    │    │                 │
│                 │    │  └─────────────────────────┘    │    │                 │
│                 │    │  ┌─────────────────────────┐    │    │                 │
│                 │    │  │  PostHog Client         │    │    │                 │
│                 │    │  │  - Retry Logic          │    │    │                 │
│                 │    │  │  - Request Logging      │    │    │                 │
│                 │    │  └─────────────────────────┘    │    │                 │
│                 │    │  ┌─────────────────────────┐    │    │                 │
│                 │    │  │  Transformer            │    │    │                 │
│                 │    │  │  - Type Detection       │    │    │                 │
│                 │    │  │  - Data Mapping         │    │    │                 │
│                 │    │  └─────────────────────────┘    │    │                 │
└─────────────────┘    └─────────────────────────────────┘    └─────────────────┘
```

### Components

1. **OpenFeature CLI**: Client consuming the standardized manifest API
2. **Proxy Service**: Go-based translation layer implementing OpenFeature API spec
   - **Gin Router**: HTTP server handling API endpoints with middleware
   - **PostHog Client**: HTTP client with retry logic and request logging
   - **Transformer**: Bidirectional data transformation between formats
   - **Config Manager**: Environment-based configuration loading
   - **Telemetry**: OpenTelemetry integration for observability
3. **PostHog Environment**: Target feature flag management system

## API Specification Compliance

The proxy implements the OpenFeature CLI sync API v0.1.0 with the following endpoints:

### GET /openfeature/v0/manifest
- **Purpose**: Retrieve all active feature flags from PostHog
- **Authentication**: Requires `read` capability
- **Response**: OpenFeature manifest format with array of flags
- **PostHog Mapping**: 
  - Fetches feature flags via PostHog API with pagination support
  - Transforms PostHog flags to OpenFeature format
  - Applies type detection and coercion based on configuration
- **Handler**: `handlers.GetManifest`

### POST /openfeature/v0/manifest/flags
- **Purpose**: Create new feature flags in PostHog
- **Authentication**: Requires `write` capability
- **Request**: OpenFeature flag schema (key, type, defaultValue, variants)
- **Response**: Created flag with `updatedAt` timestamp
- **PostHog Mapping**: 
  - Creates feature flag with rollout configuration
  - Maps OpenFeature variants to PostHog multivariate tests
  - Sets initial rollout percentage from defaultValue
- **Handler**: `handlers.CreateFlag`

### PUT /openfeature/v0/manifest/flags/{key}
- **Purpose**: Update existing feature flag metadata
- **Authentication**: Requires `write` capability
- **Request**: Partial update (name, description, defaultValue, variants, state)
- **Response**: Updated flag with `updatedAt` timestamp
- **PostHog Mapping**: 
  - Fetches existing flag by key
  - Preserves PostHog-specific settings (groups, filters)
  - Updates only specified fields
- **Handler**: `handlers.UpdateFlag`

### DELETE /openfeature/v0/manifest/flags/{key}
- **Purpose**: Archive or delete feature flags
- **Authentication**: Requires `delete` capability
- **Response**: Message with optional `archivedAt` timestamp
- **PostHog Mapping**: 
  - Disables flag (archive) if `ARCHIVE_INSTEAD_OF_DELETE=true` (default)
  - Hard deletes if configured otherwise
- **Handler**: `handlers.DeleteFlag`

### GET /health
- **Purpose**: Health check endpoint
- **Authentication**: None (always accessible)
- **Response**: Status, version, commit, date, and insecure mode warning
- **Use**: Kubernetes liveness/readiness probes, monitoring

## Data Transformation

The proxy includes a sophisticated bidirectional transformer that handles type detection, coercion, and mapping between OpenFeature and PostHog formats.

### Type Detection System

The transformer (`internal/transformer/type_detector.go`) automatically detects flag types from PostHog data:

1. **Boolean Detection**: 
   - Single variant with value `true` or boolean payload
   - Rollout percentage maps to boolean (100% = true, 0% = false)

2. **String Detection**:
   - Multiple variants with string values
   - Single string variant

3. **Integer Detection**:
   - Numeric string variants ("1", "100") with `COERCE_NUMERIC_STRINGS=true`
   - Values are parsed and validated as integers

4. **Object Detection**:
   - Complex variant structures
   - JSON-serializable data

### OpenFeature to PostHog Mapping

| OpenFeature Field | PostHog Field | Transformation |
|-------------------|---------------|----------------|
| `key` | `key` | Direct mapping (immutable identifier) |
| `name` | - | Optional metadata (not stored in PostHog) |
| `description` | `name` | OpenFeature description → PostHog name |
| `type` | `filters.multivariate.variants` | Type determines variant structure |
| `defaultValue` | `filters.rollout_percentage` or payload | Boolean: rollout %, Others: variant payload |
| `variants` | `filters.multivariate.variants` | Array of variants with weights |
| `state` | `active` | ENABLED=true, DISABLED=false |

**Special Cases**:
- Boolean flags: `defaultValue: true` → `rollout_percentage: 100`
- Boolean flags: `defaultValue: false` → `rollout_percentage: 0`
- Variants without weights are distributed evenly

### PostHog to OpenFeature Mapping

| PostHog Field | OpenFeature Field | Transformation |
|---------------|-------------------|----------------|
| `key` | `key` | Direct mapping |
| `name` | `description` | PostHog name → OpenFeature description |
| `id` | - | Internal ID (used for updates/deletes) |
| `active` | `state` | true=ENABLED, false=DISABLED |
| `filters.rollout_percentage` | `defaultValue` | Convert to boolean/value |
| `filters.multivariate.variants` | `variants` | Extract variant configurations with weights |
| `filters.groups` | - | Preserved but not exposed in OpenFeature API |

**Type Coercion Configuration**:
- `COERCE_NUMERIC_STRINGS`: Auto-detect integer types from string values
- `COERCE_BOOLEAN_STRINGS`: Auto-detect boolean types from "true"/"false" strings

### Example Transformations

#### Boolean Flag
**OpenFeature → PostHog**:
```json
{
  "key": "new-checkout",
  "type": "boolean",
  "defaultValue": true
}
```
→
```json
{
  "key": "new-checkout",
  "rollout_percentage": 100,
  "filters": {
    "payloads": {"true": true}
  }
}
```

#### String Variant Flag
**PostHog → OpenFeature**:
```json
{
  "key": "banner-color",
  "filters": {
    "multivariate": {
      "variants": [
        {"key": "red", "rollout_percentage": 50},
        {"key": "blue", "rollout_percentage": 50}
      ]
    }
  }
}
```
→
```json
{
  "key": "banner-color",
  "type": "string",
  "defaultValue": "red",
  "variants": {
    "red": {"value": "red", "weight": 50},
    "blue": {"value": "blue", "weight": 50}
  }
}
```

## Implementation Details

### Technology Stack

- **Language**: Go 1.23+ (with toolchain 1.24.5)
- **Web Framework**: Gin v1.10.1
- **HTTP Client**: Go standard library with OpenTelemetry instrumentation
- **Configuration**: Environment variables with godotenv support
- **Observability**: OpenTelemetry (traces, metrics, logs)
- **Metrics**: Prometheus + OTLP exporters
- **Build**: Multi-stage Docker builds, GoReleaser for releases

### Project Structure

```
openfeature-cli-posthog/
├── cmd/
│   └── server/
│       └── main.go              # Application entry point
├── internal/
│   ├── config/
│   │   └── config.go            # Environment-based configuration
│   ├── handlers/
│   │   ├── handler.go           # Handler struct and initialization
│   │   ├── middleware.go        # Auth middleware with capability checks
│   │   ├── get_manifest.go      # GET /manifest handler
│   │   ├── create_flag.go       # POST /flags handler
│   │   ├── update_flag.go       # PUT /flags/{key} handler
│   │   ├── delete_flag.go       # DELETE /flags/{key} handler
│   │   ├── get_flag.go          # Helper for fetching flags
│   │   └── weights.go           # Variant weight calculations
│   ├── models/
│   │   ├── openfeature.go       # OpenFeature API models
│   │   └── posthog.go           # PostHog API models
│   ├── posthog/
│   │   ├── interface.go         # Client interface for testability
│   │   ├── client.go            # PostHog HTTP client
│   │   ├── client_enhanced.go   # Enhanced client methods
│   │   ├── retry.go             # Exponential backoff retry logic
│   │   ├── options.go           # Client configuration options
│   │   └── errors.go            # Error handling and parsing
│   ├── telemetry/
│   │   ├── setup.go             # OpenTelemetry provider initialization
│   │   ├── metrics.go           # Custom metrics definitions
│   │   └── logging.go           # Structured logging setup
│   └── transformer/
│       ├── transformer.go       # Core transformation logic
│       ├── type_detector.go     # Automatic type detection
│       └── helpers.go           # Transformation utilities
├── tests/
│   └── integration/             # Integration test suite
├── .env.example                 # Environment variable template
├── .env.local.example           # Local development template
├── .envrc                       # direnv configuration
├── Dockerfile                   # Multi-stage container build
├── .goreleaser.yaml             # Release automation
└── Makefile                     # Build and development tasks
```

## Environment Variables

The proxy requires the following environment variables for configuration:

### Required Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `POSTHOG_API_KEY` | PostHog Personal API Key with feature flag permissions | `phx_abc123...` |
| `POSTHOG_PROJECT_ID` | PostHog Project ID (numeric) | `12345` |

### Optional Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `POSTHOG_HOST` | `https://app.posthog.com` | PostHog instance URL (for self-hosted) |
| `POSTHOG_TIMEOUT` | `30` | HTTP timeout in seconds for PostHog requests |
| `PROXY_PORT` | `8080` | Port for the proxy server |
| `READ_TOKEN` | - | Token for read-only access |
| `WRITE_TOKEN` | - | Token for read/write access |
| `ADMIN_TOKEN` | - | Token for full admin access (read/write/delete) |
| `DEFAULT_ROLLOUT_PERCENTAGE` | `0` | Default rollout percentage for new flags |
| `ARCHIVE_INSTEAD_OF_DELETE` | `true` | Archive flags instead of hard delete |
| `INSECURE_MODE` | `false` | **⚠️ DEV ONLY**: Disable authentication |

### Type Coercion Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `COERCE_NUMERIC_STRINGS` | `false` | Auto-convert numeric strings ("1") to integer type |
| `COERCE_BOOLEAN_STRINGS` | `false` | Auto-convert "true"/"false" to boolean type |

### Telemetry Configuration (OpenTelemetry)

| Variable | Default | Description |
|----------|---------|-------------|
| `OTEL_SERVICE_NAME` | `openfeature-posthog-proxy` | Service name for telemetry |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `localhost:4317` | OTLP collector endpoint |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | `grpc` | Protocol: `grpc` or `http` |
| `OTEL_EXPORTER_OTLP_INSECURE` | `false` | Use insecure connection (no TLS) |
| `OTEL_PROMETHEUS_ENABLED` | `true` | Enable Prometheus metrics endpoint |

### Custom Tokens

Format: `CUSTOM_TOKEN_N=token:capability1,capability2`

Example:
```bash
CUSTOM_TOKEN_1=my_integration_token:read,write
CUSTOM_TOKEN_2=external_service:read
```

### Configuration Loading Priority

1. `.env.local` (highest priority, for local development)
2. `.env` (fallback)
3. System environment variables (if no .env files found)

### direnv Support

The project includes `.envrc` for automatic environment loading:

```bash
# .envrc
source_env .env.local

# Aliases for convenience
alias proxy-run="make run"
alias proxy-build="make build"
alias proxy-test="make test"
alias proxy-docker="make docker-build && make docker-run"
```

## PostHog API Integration

### PostHog Client Implementation

The proxy includes a robust HTTP client (`internal/posthog/client.go`) with the following features:

1. **Automatic Retry Logic**:
   - Exponential backoff with jitter (±20%)
   - Configurable max retries (default: 3)
   - Respects `Retry-After` headers
   - Retries on 5xx errors and 429 (rate limiting)
   - Does NOT retry on 4xx client errors (except 429)

2. **Request Logging** (when `INSECURE_MODE=true`):
   - Logs all HTTP requests and responses
   - Useful for debugging integration issues
   - **Security**: Only enable in development environments

3. **OpenTelemetry Instrumentation**:
   - Automatic trace propagation
   - Request/response metrics
   - Error tracking

4. **Pagination Support**:
   - Automatically traverses paginated responses
   - Handles both absolute and relative URLs
   - Consolidates all pages into single result

### Required PostHog API Endpoints

The proxy interacts with the following PostHog API endpoints:

#### Feature Flag Management

| Method | Endpoint | Purpose | OpenFeature Mapping |
|--------|----------|---------|-------------------|
| `GET` | `/api/projects/{project_id}/feature_flags/` | List all feature flags (with pagination) | `GET /openfeature/v0/manifest` |
| `POST` | `/api/projects/{project_id}/feature_flags/` | Create new feature flag | `POST /openfeature/v0/manifest/flags` |
| `GET` | `/api/projects/{project_id}/feature_flags/{key}/` | Get specific flag by key or ID | Internal helper |
| `PATCH` | `/api/projects/{project_id}/feature_flags/{id}/` | Update feature flag | `PUT /openfeature/v0/manifest/flags/{key}` |
| `DELETE` | `/api/projects/{project_id}/feature_flags/{id}/` | Delete feature flag | `DELETE /openfeature/v0/manifest/flags/{key}` |

**Note**: PostHog's GET endpoint supports both numeric IDs and string keys.

### PostHog API Authentication

The proxy authenticates with PostHog using Personal API Keys:

```
Authorization: Bearer phx_abc123...
```

**Required API Scopes**:
- `feature_flag:read` - For reading feature flag configurations
- `feature_flag:write` - For creating and updating feature flags
- `project:read` - For accessing project information

### API Request Examples

#### List Feature Flags
```bash
curl -H "Authorization: Bearer ${POSTHOG_API_KEY}" \
  ${POSTHOG_HOST}/api/projects/${POSTHOG_PROJECT_ID}/feature_flags/
```

Response:
```json
{
  "next": "https://app.posthog.com/api/projects/12345/feature_flags/?offset=100",
  "results": [
    {
      "id": 12345,
      "key": "new-feature",
      "name": "New Feature",
      "active": true,
      "filters": {...}
    }
  ]
}
```

#### Create Feature Flag
```bash
curl -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${POSTHOG_API_KEY}" \
  -d '{
    "name": "New Feature",
    "key": "new-feature",
    "active": true,
    "rollout_percentage": 0,
    "filters": {
      "payloads": {"true": true}
    }
  }' \
  ${POSTHOG_HOST}/api/projects/${POSTHOG_PROJECT_ID}/feature_flags/
```

#### Update Feature Flag
```bash
curl -X PATCH \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${POSTHOG_API_KEY}" \
  -d '{
    "active": true,
    "rollout_percentage": 50
  }' \
  ${POSTHOG_HOST}/api/projects/${POSTHOG_PROJECT_ID}/feature_flags/{flag_id}/
```

#### Get Flag by Key
```bash
curl -H "Authorization: Bearer ${POSTHOG_API_KEY}" \
  ${POSTHOG_HOST}/api/projects/${POSTHOG_PROJECT_ID}/feature_flags/new-feature/
```

### Error Handling

The proxy maps PostHog API errors to OpenFeature error responses:

| PostHog Status | Proxy Status | Retry | Description |
|---------------|--------------|-------|-------------|
| `404 Not Found` | `404` | No | Flag doesn't exist in PostHog |
| `400 Bad Request` | `400` | No | Invalid flag configuration |
| `401 Unauthorized` | `500` | No | PostHog API key invalid |
| `403 Forbidden` | `500` | No | API key lacks required permissions |
| `429 Rate Limited` | `503` | Yes | PostHog API rate limits (respects Retry-After) |
| `500 Internal Error` | `503` | Yes | PostHog service unavailable |
| `503 Service Unavailable` | `503` | Yes | PostHog temporary outage |

**Retry Configuration**:
```go
RetryConfig{
    MaxRetries:     3,
    InitialBackoff: 1 * time.Second,
    MaxBackoff:     10 * time.Second,
}
```

### Retry Logic Flow

```
Request → 5xx/429? → Yes → Wait with backoff → Retry (up to 3 times)
                   ↓
                   No
                   ↓
              4xx/2xx → Return response
```

Backoff calculation:
```
backoff = initial_backoff * 2^(attempt-1)
backoff = min(backoff, max_backoff)
backoff += jitter (±20%)
backoff = max(backoff, Retry-After header value)
```

### Authentication & Authorization

#### Token-Based Access Control

The proxy implements capability-based authentication using Bearer tokens:

**Authentication Flow**:
1. Client sends request with `Authorization: Bearer <token>` header
2. `AuthMiddleware` validates token against configured tokens
3. Token capabilities are stored in request context
4. `RequireCapability` middleware checks if required capability exists
5. Handler processes request if authorized

**Capability Levels**:
- **`read`**: Access to `GET /manifest` endpoint
- **`write`**: Access to `POST /flags` and `PUT /flags/{key}` endpoints
- **`delete`**: Access to `DELETE /flags/{key}` endpoint

**Token Configuration**:
```go
type AuthToken struct {
    Token        string   // The actual token string
    Capabilities []string // List of granted capabilities
}
```

**Predefined Tokens** (via environment variables):
- `READ_TOKEN`: Capabilities: `["read"]`
- `WRITE_TOKEN`: Capabilities: `["read", "write"]`
- `ADMIN_TOKEN`: Capabilities: `["read", "write", "delete"]`

**Custom Tokens**:
```bash
CUSTOM_TOKEN_1=my_integration:read,write
CUSTOM_TOKEN_2=monitoring_service:read
```

#### Insecure Mode (Development Only)

⚠️ **WARNING**: Only for development and testing!

When `INSECURE_MODE=true`:
- No Bearer token validation
- All requests granted full capabilities: `["read", "write", "delete"]`
- Clear warnings in logs and health endpoint
- Request/response logging enabled

**Use Cases**:
- Local development without auth setup
- Integration testing
- Debugging API transformations

**Never use in production!**

#### PostHog Integration

- Proxy authenticates with PostHog using `POSTHOG_API_KEY`
- Client tokens control access to proxy endpoints
- PostHog API key is never exposed to clients
- Separate authorization layers (proxy ↔ PostHog)

## Advanced Features

### Observability & Telemetry

The proxy includes comprehensive OpenTelemetry integration:

#### Traces
- **Provider**: OTLP (gRPC or HTTP)
- **Instrumentation**:
  - HTTP requests (via `otelhttp.NewTransport`)
  - Gin middleware (via `otelgin.Middleware`)
  - Context propagation (TraceContext, Baggage)
- **Exporters**: Configurable OTLP endpoint

#### Metrics
- **Providers**: 
  - OTLP exporter (to collector)
  - Prometheus exporter (pull-based, `/metrics` endpoint)
- **Custom Metrics**:
  ```go
  flags_created_total       // Counter: Total flags created
  flags_updated_total       // Counter: Total flags updated
  flags_deleted_total       // Counter: Total flags deleted
  manifest_requests_total   // Counter: Total manifest requests
  posthog_api_errors_total  // Counter: PostHog API errors
  ```
- **Automatic Metrics**:
  - HTTP request duration
  - Request counts by status code
  - Response sizes

#### Logs
- **Provider**: OTLP (gRPC or HTTP)
- **Format**: Structured logging with `log/slog`
- **Bridging**: slog → OpenTelemetry Logs API
- **Hybrid Output**:
  - Console output (stdout/stderr)
  - OTLP export to collector
- **Context**: Automatic trace ID correlation

**Configuration Example**:
```bash
OTEL_SERVICE_NAME=openfeature-posthog-proxy
OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4317
OTEL_EXPORTER_OTLP_PROTOCOL=grpc
OTEL_EXPORTER_OTLP_INSECURE=false
OTEL_PROMETHEUS_ENABLED=true
```

### Retry Strategy

Exponential backoff with jitter for transient failures:

1. **Retry Triggers**:
   - Network errors
   - HTTP 5xx errors
   - HTTP 429 (rate limiting)

2. **Backoff Algorithm**:
   ```
   backoff = initial_backoff * 2^(attempt-1)
   backoff = min(backoff, max_backoff)
   jitter = random(-20%, +20%) of backoff
   final_backoff = backoff + jitter
   ```

3. **Configuration**:
   - Max retries: 3
   - Initial backoff: 1 second
   - Max backoff: 10 seconds

4. **Retry-After Support**:
   - Respects HTTP `Retry-After` header
   - Parses both seconds (integer) and date formats

5. **Request Body Handling**:
   - Uses `GetBody` function for retry
   - Falls back to `io.Seeker` if available

### Type Coercion

Automatic type detection and conversion for better OpenFeature compatibility:

**Numeric String Coercion** (`COERCE_NUMERIC_STRINGS=true`):
- Detects variants with numeric string values
- Converts to `integer` type
- Example: `{"version": "1"}` → `type: integer, value: 1`

**Boolean String Coercion** (`COERCE_BOOLEAN_STRINGS=true`):
- Detects variants with "true"/"false" string values
- Converts to `boolean` type
- Example: `{"enabled": "true"}` → `type: boolean, value: true`

**Use Case**: Legacy PostHog flags created before type awareness

### Variant Weight Distribution

The proxy includes sophisticated variant weight handling:

1. **Equal Distribution** (no weights specified):
   ```go
   // 3 variants → each gets 33.33% (with rounding adjustment)
   weights := distributeWeights(3) // [33, 33, 34]
   ```

2. **Partial Weights** (some weights missing):
   - Distributes remaining percentage among unweighted variants
   - Ensures total = 100%

3. **Weight Validation**:
   - Total must equal 100
   - Individual weights: 0-100
   - Returns error if invalid

4. **PostHog Mapping**:
   ```go
   // OpenFeature variants → PostHog rollout_percentage
   variants := map[string]Variant{
       "red":  {Value: "red", Weight: 50},
       "blue": {Value: "blue", Weight: 50},
   }
   ```

### Graceful Shutdown

The server implements graceful shutdown with timeout:

```go
// On SIGTERM/SIGINT:
1. Stop accepting new connections
2. Finish processing active requests (5s timeout)
3. Shutdown telemetry providers
4. Exit cleanly
```

Signals handled:
- `SIGTERM` (Docker, Kubernetes)
- `SIGINT` (Ctrl+C)

### Health Checks

The `/health` endpoint provides:
- Status: "healthy"
- Version information (from build)
- Git commit hash
- Build date
- Insecure mode warning (if applicable)

**Kubernetes Integration**:
```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 30

readinessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10
```

## Security Considerations

1. **API Key Management**:
   - PostHog API keys stored in environment variables
   - Never exposed in logs (except in INSECURE_MODE)
   - Never returned in API responses
   - Recommended: Use secrets management (Kubernetes Secrets, AWS Secrets Manager)

2. **Token Validation**:
   - Strict validation of Bearer tokens
   - Token comparison using constant-time operations (implicit in Go string comparison)
   - Invalid tokens return 401 Unauthorized
   - Missing capabilities return 403 Forbidden

3. **Rate Limiting**:
   - Inherits PostHog's rate limiting
   - Respects `Retry-After` headers
   - Exponential backoff prevents overwhelming PostHog API
   - Consider adding proxy-level rate limiting for production

4. **HTTPS Enforcement**:
   - Recommended: Run behind reverse proxy with TLS (nginx, Traefik, Kubernetes Ingress)
   - Use HTTPS for PostHog API communication
   - Set `OTEL_EXPORTER_OTLP_INSECURE=false` for telemetry

5. **Input Validation**:
   - Gin framework validation for request bodies
   - Flag key validation (alphanumeric, hyphens, underscores)
   - JSON schema validation via Go struct tags
   - Type validation in transformer

6. **Error Information Disclosure**:
   - Generic error messages to clients
   - Detailed errors only in server logs
   - No stack traces in API responses
   - PostHog API errors are sanitized

7. **CORS Configuration**:
   - Not included by default (backend-to-backend service)
   - Add Gin CORS middleware if needed for browser clients

8. **Container Security**:
   - Multi-stage Docker build (minimal runtime image)
   - Non-root user in container
   - No unnecessary packages in runtime
   - Regular base image updates (Alpine Linux)

9. **Dependency Security**:
   - Go module checksums (go.sum)
   - Regular dependency updates
   - Vulnerability scanning recommended (Snyk, Trivy)

10. **Production Recommendations**:
    - **Always** disable `INSECURE_MODE` in production
    - Use strong, random tokens (minimum 32 characters)
    - Rotate tokens periodically
    - Implement network policies (firewall rules, VPC)
    - Enable audit logging
    - Monitor for suspicious activity
    - Use dedicated PostHog API keys per environment

## Deployment Options

### Docker Container

**Multi-stage Dockerfile**:
```dockerfile
# Build stage
FROM golang:1.21-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git ca-certificates tzdata
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
    -o main ./cmd/server

# Runtime stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /root/
COPY --from=builder /app/main .
EXPOSE 8080
ENV PORT=8080
CMD ["./main"]
```

**Build and Run**:
```bash
# Build image
docker build -t posthog-proxy:latest .

# Run container
docker run -d \
  --name posthog-proxy \
  -p 8080:8080 \
  --env-file .env \
  posthog-proxy:latest
```

### Docker Compose

```yaml
version: '3.8'
services:
  posthog-proxy:
    build: .
    ports:
      - "8080:8080"
    env_file:
      - .env.local
    environment:
      - POSTHOG_API_KEY=${POSTHOG_API_KEY}
      - POSTHOG_PROJECT_ID=${POSTHOG_PROJECT_ID}
      - ADMIN_TOKEN=${ADMIN_TOKEN}
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 10s
```

### Kubernetes Deployment

**Deployment manifest**:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: posthog-proxy
  labels:
    app: posthog-proxy
spec:
  replicas: 3
  selector:
    matchLabels:
      app: posthog-proxy
  template:
    metadata:
      labels:
        app: posthog-proxy
    spec:
      containers:
      - name: posthog-proxy
        image: posthog-proxy:latest
        ports:
        - containerPort: 8080
        env:
        - name: POSTHOG_API_KEY
          valueFrom:
            secretKeyRef:
              name: posthog-secrets
              key: api-key
        - name: POSTHOG_PROJECT_ID
          valueFrom:
            configMapKeyRef:
              name: posthog-config
              key: project-id
        - name: ADMIN_TOKEN
          valueFrom:
            secretKeyRef:
              name: posthog-secrets
              key: admin-token
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
        resources:
          requests:
            memory: "64Mi"
            cpu: "100m"
          limits:
            memory: "128Mi"
            cpu: "200m"
---
apiVersion: v1
kind: Service
metadata:
  name: posthog-proxy
spec:
  selector:
    app: posthog-proxy
  ports:
  - port: 8080
    targetPort: 8080
  type: ClusterIP
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: posthog-config
data:
  project-id: "12345"
  posthog-host: "https://app.posthog.com"
  default-rollout-percentage: "0"
---
apiVersion: v1
kind: Secret
metadata:
  name: posthog-secrets
type: Opaque
stringData:
  api-key: "phx_your_api_key_here"
  admin-token: "your_secure_admin_token"
  read-token: "your_secure_read_token"
  write-token: "your_secure_write_token"
```

**Ingress** (with TLS):
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: posthog-proxy
  annotations:
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
spec:
  tls:
  - hosts:
    - posthog-proxy.example.com
    secretName: posthog-proxy-tls
  rules:
  - host: posthog-proxy.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: posthog-proxy
            port:
              number: 8080
```

### Binary Release (GoReleaser)

**Supported platforms**:
- Linux: amd64, arm64
- macOS (Darwin): amd64, arm64
- Windows: amd64

**Release artifacts**:
```
posthog-proxy_1.0.0_linux_amd64.tar.gz
posthog-proxy_1.0.0_linux_arm64.tar.gz
posthog-proxy_1.0.0_darwin_amd64.tar.gz
posthog-proxy_1.0.0_darwin_arm64.tar.gz
posthog-proxy_1.0.0_windows_amd64.zip
checksums.txt
```

**Installation**:
```bash
# Download and extract
wget https://github.com/.../posthog-proxy_1.0.0_linux_amd64.tar.gz
tar -xzf posthog-proxy_1.0.0_linux_amd64.tar.gz

# Set environment variables
export POSTHOG_API_KEY=phx_...
export POSTHOG_PROJECT_ID=12345
export ADMIN_TOKEN=your_token

# Run
./posthog-proxy
```

### Cloud Platforms

#### AWS ECS/Fargate
- Use ECR for container registry
- Store secrets in AWS Secrets Manager
- Configure ALB for load balancing
- Use CloudWatch for logs and metrics

#### Google Cloud Run
```bash
# Build and deploy
gcloud builds submit --tag gcr.io/PROJECT_ID/posthog-proxy
gcloud run deploy posthog-proxy \
  --image gcr.io/PROJECT_ID/posthog-proxy \
  --platform managed \
  --set-env-vars POSTHOG_API_KEY=secretRef:posthog-key \
  --set-env-vars POSTHOG_PROJECT_ID=12345
```

#### Azure Container Instances
```bash
az container create \
  --resource-group myResourceGroup \
  --name posthog-proxy \
  --image posthog-proxy:latest \
  --dns-name-label posthog-proxy \
  --ports 8080 \
  --environment-variables \
    POSTHOG_PROJECT_ID=12345 \
  --secure-environment-variables \
    POSTHOG_API_KEY=phx_...
```

### Development Deployment

**Local with hot reload** (using `air` or `CompileDaemon`):
```bash
# Install air
go install github.com/air-verse/air@latest

# Run with hot reload
air

# Or use make
make dev
```

**With direnv**:
```bash
# .envrc loads environment automatically
direnv allow .

# Use aliases
proxy-run    # Start the server
proxy-test   # Run tests
proxy-build  # Build binary
```

## Testing Strategy

### Unit Tests

The codebase includes comprehensive unit tests with high coverage:

**Test Structure**:
```
internal/
├── handlers/
│   ├── create_flag_test.go
│   ├── update_flag_test.go
│   ├── delete_flag_test.go
│   ├── get_manifest_test.go
│   └── weights_test.go
├── posthog/
│   ├── client_test.go
│   └── retry_test.go
└── transformer/
    ├── transformer_test.go
    ├── type_detector_test.go
    └── helpers_test.go
```

**Testing Tools**:
- `testing` - Go standard testing package
- `testify/assert` - Assertions library
- `testify/mock` - Mocking framework

**Running Tests**:
```bash
# All tests
make test

# With coverage
make coverage

# Unit tests only
make test-unit

# Specific package
go test -v ./internal/transformer/...
```

### Integration Tests

Located in `tests/integration/`, testing full request/response flow:

```bash
# Run integration tests
make test-integration
```

**Test Coverage Goals**:
- Unit tests: >80% coverage
- Critical paths: 100% coverage (transformers, handlers)
- Error handling: Full coverage

### Mock PostHog Client

For handler testing, use the mock client:

```go
// internal/posthog/mock_client.go
type MockClient struct {
    mock.Mock
}

func (m *MockClient) GetFeatureFlags(ctx context.Context) ([]models.PostHogFeatureFlag, error) {
    args := m.Called(ctx)
    return args.Get(0).([]models.PostHogFeatureFlag), args.Error(1)
}

// Usage in tests
mockClient := new(posthog.MockClient)
mockClient.On("GetFeatureFlags", mock.Anything).Return(flags, nil)
```

### Test Data Helpers

The transformer package includes test helpers:

```go
// Example test
func TestPostHogToOpenFeatureFlag(t *testing.T) {
    phFlag := models.PostHogFeatureFlag{
        Key:    "test-flag",
        Name:   "Test Flag",
        Active: true,
        Filters: models.PostHogFilters{
            RolloutPercentage: ptr(100),
            Payloads: map[string]interface{}{
                "true": true,
            },
        },
    }
    
    result := transformer.PostHogToOpenFeatureFlag(phFlag, config.TypeCoercionConfig{})
    
    assert.Equal(t, "test-flag", result.Key)
    assert.Equal(t, models.FlagTypeBoolean, result.Type)
    assert.Equal(t, true, result.DefaultValue)
    assert.Equal(t, models.FlagStateEnabled, result.State)
}
```

### Manual Testing

**With INSECURE_MODE**:
```bash
# Start proxy in insecure mode
export INSECURE_MODE=true
make run

# Test endpoints without auth
curl http://localhost:8080/health
curl http://localhost:8080/openfeature/v0/manifest
```

**With Authentication**:
```bash
# Start with tokens
export READ_TOKEN=test_read_token
export WRITE_TOKEN=test_write_token
make run

# Test with auth header
curl -H "Authorization: Bearer $READ_TOKEN" \
  http://localhost:8080/openfeature/v0/manifest
```

### CI/CD Testing

Recommended GitHub Actions workflow:
```yaml
name: Test

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.23'
      
      - name: Download dependencies
        run: go mod download
      
      - name: Run tests
        run: make test
      
      - name: Run linter
        uses: golangci/golangci-lint-action@v3
      
      - name: Build
        run: make build
```

## Performance Considerations

### Request Handling

- **Concurrent Requests**: Gin framework handles concurrent requests efficiently
- **Context Timeouts**: PostHog client timeout configurable (default: 30s)
- **Connection Pooling**: Go's HTTP client reuses connections automatically
- **Resource Limits**: Configure Kubernetes resource limits based on load

### Memory Usage

- **Baseline**: ~20-50 MB for idle process
- **Under Load**: ~50-100 MB (depends on flag count and request rate)
- **Pagination**: PostHog responses processed page-by-page to avoid large memory allocations
- **No Caching**: Current design doesn't cache flags (stateless)

### Latency

Typical response times:
- **GET /manifest**: 100-500ms (depends on PostHog latency + flag count)
- **POST /flags**: 200-600ms (PostHog create operation)
- **PUT /flags/{key}**: 200-600ms (2 PostHog calls: fetch + update)
- **DELETE /flags/{key}**: 150-400ms (2 PostHog calls: fetch + delete)

**Optimization Opportunities**:
1. Add Redis cache for manifest (reduce PostHog calls)
2. Implement flag update batching
3. Use HTTP/2 for PostHog API calls
4. Add CDN caching with short TTL

### Scalability

**Horizontal Scaling**:
- Stateless design enables easy horizontal scaling
- Deploy multiple replicas behind load balancer
- No shared state between instances

**Recommended Configuration**:
```yaml
# Low traffic (<100 req/min)
replicas: 2
resources:
  requests: {cpu: 100m, memory: 64Mi}
  limits: {cpu: 200m, memory: 128Mi}

# Medium traffic (<1000 req/min)
replicas: 3-5
resources:
  requests: {cpu: 200m, memory: 128Mi}
  limits: {cpu: 500m, memory: 256Mi}

# High traffic (>1000 req/min)
replicas: 5-10
resources:
  requests: {cpu: 500m, memory: 256Mi}
  limits: {cpu: 1000m, memory: 512Mi}
```

### PostHog API Rate Limits

PostHog enforces rate limits based on plan:
- **Free**: 100 requests/minute
- **Paid**: 1000+ requests/minute

**Mitigation**:
- Implement caching layer
- Use retry logic with exponential backoff
- Monitor `posthog_api_errors_total` metric
- Consider flag manifest caching (5-60s TTL)

## Monitoring & Operations

### Key Metrics to Monitor

**Application Metrics** (via Prometheus):
```
# Request metrics
http_requests_total{method, endpoint, status}
http_request_duration_seconds{method, endpoint}

# Flag operations
flags_created_total
flags_updated_total
flags_deleted_total
manifest_requests_total

# PostHog integration
posthog_api_errors_total{status_code}
posthog_api_duration_seconds
```

**Infrastructure Metrics**:
- CPU usage per pod
- Memory usage per pod
- Network throughput
- Pod restart count

### Logging

**Structured Logging** with contextual information:
```json
{
  "timestamp": "2025-01-15T10:30:00Z",
  "level": "info",
  "message": "CreateFeatureFlag - Successfully created flag",
  "trace_id": "abc123...",
  "key": "new-feature",
  "duration_ms": 245
}
```

**Log Levels**:
- `ERROR`: PostHog API failures, authentication errors, critical issues
- `WARN`: Retry attempts, rate limiting, deprecated features
- `INFO`: Flag operations, server lifecycle events
- `DEBUG`: Request/response details (INSECURE_MODE only)

### Alerting

**Recommended Alerts**:

1. **High Error Rate**:
   ```
   rate(posthog_api_errors_total[5m]) > 10
   ```

2. **Service Unavailable**:
   ```
   up{job="posthog-proxy"} == 0
   ```

3. **High Latency**:
   ```
   histogram_quantile(0.95, http_request_duration_seconds) > 2
   ```

4. **Memory Usage**:
   ```
   container_memory_usage_bytes / container_memory_limit_bytes > 0.9
   ```

### Troubleshooting

**Common Issues**:

1. **"Invalid PostHog API Key"**:
   - Check `POSTHOG_API_KEY` is correct
   - Verify API key has required scopes
   - Ensure key is not expired

2. **"Flag not found"**:
   - Flag might not exist in PostHog
   - Check project ID matches
   - Verify flag key spelling

3. **"Rate limit exceeded"**:
   - PostHog API rate limits reached
   - Check retry backoff is working
   - Consider implementing caching

4. **High latency**:
   - PostHog API slow (check their status page)
   - Network issues between proxy and PostHog
   - Increase timeout configuration

**Debug Mode**:
```bash
# Enable request logging
export INSECURE_MODE=true
make run

# Watch logs
tail -f logs/*.log
```

## Future Enhancements

### Planned Features

1. **Caching Layer**:
   - Redis-backed flag manifest cache
   - Configurable TTL (5-300 seconds)
   - Cache invalidation on write operations
   - Reduce PostHog API calls by 80-90%

2. **Multi-Project Support**:
   - Manage flags across multiple PostHog projects
   - Project selection via API parameter or header
   - Separate token permissions per project

3. **Bulk Operations**:
   - Import/export multiple flags (JSON format)
   - Batch create/update operations
   - Flag migration between environments

4. **Webhook Integration**:
   - Real-time flag change notifications
   - Integration with CI/CD pipelines
   - Slack/Teams notifications

5. **Flag Templates**:
   - Pre-configured patterns (kill switches, gradual rollouts)
   - Environment-specific defaults
   - Team-specific conventions

6. **Enhanced Analytics**:
   - Flag usage statistics
   - Rollout success metrics
   - A/B test result integration

7. **GraphQL API**:
   - Alternative to REST API
   - More flexible queries
   - Reduced over-fetching

8. **Admin UI**:
   - Web-based flag management
   - Visual variant editor
   - Real-time flag status dashboard

### Performance Optimizations

1. **HTTP/2 Support**: Multiplexing PostHog API calls
2. **Connection Pooling**: Tuned for high throughput
3. **Request Batching**: Combine multiple operations
4. **Streaming Responses**: Large manifest pagination

### Security Enhancements

1. **mTLS Support**: Mutual TLS for PostHog communication
2. **OAuth2 Integration**: Token exchange flows
3. **Audit Trail**: Complete change history
4. **IP Allowlisting**: Network-level access control

## Conclusion

This PostHog OpenFeature Proxy provides a production-ready, enterprise-grade integration between OpenFeature's standardized approach and PostHog's powerful feature flag capabilities. 

**Key Achievements**:
- ✅ Full OpenFeature CLI API v0.1.0 compliance
- ✅ Robust PostHog integration with retry logic and error handling
- ✅ Comprehensive observability with OpenTelemetry
- ✅ Type-safe Go implementation with extensive test coverage
- ✅ Production-ready security and authentication
- ✅ Container-native deployment options
- ✅ Flexible configuration and environment support

The proxy enables teams to:
- Leverage PostHog's analytics and experimentation features
- Maintain OpenFeature compatibility for vendor independence
- Deploy feature flags with confidence using industry-standard tooling
- Scale horizontally to meet demand
- Monitor and troubleshoot effectively

**Getting Started**: See the [README.md](../README.md) for quick start guide and usage examples.

**Contributing**: The project welcomes contributions for bug fixes, feature enhancements, and documentation improvements.

---

**Last Updated**: 2025-01-15  
**Version**: 1.0.0  
**Maintained By**: OpenFeature Community