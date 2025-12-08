# PostHog OpenFeature Proxy

A Go-based proxy service that enables PostHog feature flag management through the OpenFeature CLI API specification. This proxy acts as a translation layer between OpenFeature's standardized manifest API and PostHog's feature flag system.

## Features

- ✅ Full OpenFeature CLI sync API v0.1.0 compliance
- ✅ PostHog API integration with CRUD operations
- ✅ Bidirectional data transformation (OpenFeature ↔ PostHog)
- ✅ Token-based authentication with capability management
- ✅ Docker support for easy deployment
- ✅ Environment-based configuration
- ✅ `direnv` support for local development

## Quick Start

### Prerequisites

- Go 1.21+
- PostHog account with API access
- (Optional) [direnv](https://direnv.net/) for environment management

### Local Development Setup

1. **Clone and setup environment:**
   ```bash
   git clone <repository-url>
   cd openfeature-cli-posthog
   ```

2. **Configure environment (choose one):**

   **Option A: Using direnv (recommended)**
   ```bash
   # Install direnv if you haven't already
   # macOS: brew install direnv
   # Ubuntu: sudo apt install direnv
   
   # Copy local environment template
   cp .env.local.example .env.local
   
   # Edit .env.local with your PostHog credentials
   nano .env.local
   
   # Allow direnv to load environment
   direnv allow .
   ```

   **Option B: Using traditional .env**
   ```bash
   cp .env.example .env
   # Edit .env with your PostHog credentials
   nano .env
   ```

3. **Required Configuration:**
   Set these values in your environment file:
   ```bash
   POSTHOG_API_KEY=phx_your_api_key_here
   POSTHOG_PROJECT_ID=12345
   ```

4. **Install dependencies and run the proxy:**
   ```bash
   # First time setup - download dependencies
   make deps
   
   # Run the proxy
   # Using direnv (if configured)
   proxy-run
   
   # Or using make directly
   make run
   ```

### Docker Deployment

1. **Build and run with Docker:**
   ```bash
   # Build the image
   make docker-build
   
   # Run with environment file
   make docker-run
   ```

2. **Using Docker Compose:**
   ```yaml
   version: '3.8'
   services:
     posthog-proxy:
       build: .
       ports:
         - "8080:8080"
       env_file:
         - .env.local
   ```

## API Endpoints

The proxy implements the OpenFeature CLI sync API:

- `GET /openfeature/v0/manifest` - Retrieve all feature flags
- `POST /openfeature/v0/manifest/flags` - Create new feature flag  
- `PUT /openfeature/v0/manifest/flags/{key}` - Update existing flag
- `DELETE /openfeature/v0/manifest/flags/{key}` - Delete/archive flag
- `GET /health` - Health check endpoint

## Configuration

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `POSTHOG_API_KEY` | ✅ | - | PostHog Personal API Key |
| `POSTHOG_PROJECT_ID` | ✅ | - | PostHog Project ID |
| `POSTHOG_HOST` | ❌ | `https://app.posthog.com` | PostHog instance URL |
| `PROXY_PORT` | ❌ | `8080` | Proxy server port |
| `READ_TOKEN` | ❌ | Auto-generated | Read-only access token |
| `WRITE_TOKEN` | ❌ | Auto-generated | Read/write access token |
| `ADMIN_TOKEN` | ❌ | Auto-generated | Full admin access token |
| `INSECURE_MODE` | ❌ | `false` | **⚠️ Dev only:** Disable authentication |
| `DEFAULT_ROLLOUT_PERCENTAGE` | ❌ | `0` | Default rollout for new flags |
| `ARCHIVE_INSTEAD_OF_DELETE` | ❌ | `true` | Archive vs hard delete flags |

### Authentication

The proxy supports two authentication modes:

#### Secure Mode (Default)
Uses Bearer token authentication:

```bash
curl -H "Authorization: Bearer $READ_TOKEN" \
  http://localhost:8080/openfeature/v0/manifest
```

**Token Capabilities:**
- `read` - Access to GET endpoints
- `write` - Access to POST/PUT endpoints  
- `delete` - Access to DELETE endpoints

#### Insecure Mode (Development Only)
⚠️ **WARNING**: Only use for development and testing!

```bash
# Enable insecure mode
export INSECURE_MODE=true

# No authentication required
curl http://localhost:8080/openfeature/v0/manifest
```

When `INSECURE_MODE=true`:
- No Bearer token required
- All endpoints accessible without authentication
- Full capabilities granted to all requests
- Clear warnings displayed in logs and health endpoint

### Custom Tokens

Add custom tokens using environment variables:
```bash
CUSTOM_TOKEN_1=my_integration_token:read,write
CUSTOM_TOKEN_2=external_service:read
```

## Development

### Available Commands

```bash
# Build and run
make build
make run

# Testing
make test
make test-unit
make test-integration
make coverage

# Development with hot reload
make dev

# Docker operations
make docker-build
make docker-run

# Cross-platform builds
make build-all
```

### With direnv

If you're using direnv, additional aliases are available:
- `proxy-run` - Run the proxy server
- `proxy-build` - Build the binary
- `proxy-test` - Run tests
- `proxy-docker` - Build and run with Docker

### Project Structure

```
├── cmd/server/           # Application entry point
├── internal/
│   ├── config/          # Configuration management
│   ├── handlers/        # HTTP request handlers
│   ├── models/          # Data models (OpenFeature & PostHog)
│   ├── posthog/         # PostHog API client
│   └── transformer/     # Data transformation logic
├── .envrc              # direnv configuration
├── .env.local.example  # Local development template
└── Dockerfile          # Container build
```

## PostHog API Integration

The proxy interacts with these PostHog endpoints:

- `GET /api/projects/{id}/feature_flags/` - List flags
- `POST /api/projects/{id}/feature_flags/` - Create flag
- `PATCH /api/projects/{id}/feature_flags/{id}/` - Update flag
- `DELETE /api/projects/{id}/feature_flags/{id}/` - Delete flag

### Required API Permissions

Your PostHog API key needs these scopes:
- `feature_flag:read` - Reading feature flag configurations
- `feature_flag:write` - Creating and updating feature flags
- `project:read` - Accessing project information

## Examples

### Creating a Feature Flag

```bash
curl -X POST \
  -H "Authorization: Bearer $WRITE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "key": "new-feature",
    "name": "New Feature",
    "type": "boolean",
    "defaultValue": false
  }' \
  http://localhost:8080/openfeature/v0/manifest/flags
```

### Updating a Feature Flag

```bash
curl -X PUT \
  -H "Authorization: Bearer $WRITE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "state": "ENABLED"
  }' \
  http://localhost:8080/openfeature/v0/manifest/flags/new-feature
```

## Release Automation

This repository uses [release-please](https://github.com/googleapis/release-please) to automate changelog and tag management. The workflow defined in `.github/workflows/release-please.yml` requires a token with permission to create pull requests. Add a classic Personal Access Token (or GitHub App token) with `repo` and `workflow` scopes as the `RELEASE_PLEASE_TOKEN` repository secret so the automation can open release PRs.

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Support

For issues and questions:
- Create an issue in this repository
- Check the [OpenFeature documentation](https://openfeature.dev)
- Review [PostHog API documentation](https://posthog.com/docs/api)