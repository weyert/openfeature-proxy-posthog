# PostHog OpenFeature Proxy Documentation

Welcome to the documentation for the PostHog OpenFeature proxy. This proxy enables you to manage PostHog feature flags through the standardized OpenFeature CLI API.

## Documentation Overview

### 📖 [Transformation Rules](./transformation-rules.md)
Comprehensive guide to how PostHog feature flags are transformed into OpenFeature-compliant format:
- Type detection logic and priority
- Type coercion feature gates  
- Variant transformation rules
- JSON object detection
- Configuration examples

### 🔌 [API Reference](./api-reference.md) 
Complete API documentation covering:
- Authentication and capabilities
- All available endpoints
- Request/response formats
- Error handling
- Configuration options
- Usage examples

## Quick Start

1. **Setup Environment Variables**
   ```bash
   cp .env.local.example .env.local
   # Edit .env.local with your PostHog credentials
   ```

2. **Build and Run**
   ```bash
   make build
   ./posthog-proxy
   ```

3. **Test the API**
   ```bash
   curl http://localhost:8080/health
   curl http://localhost:8080/openfeature/v0/manifest
   ```

## Key Features

### 🔄 **Intelligent Type Detection**
- **JSON Objects**: Automatic detection and parsing of JSON payloads
- **Numeric Coercion**: Convert numeric strings ("1", "3.14") to numbers
- **Boolean Coercion**: Convert boolean strings ("true", "false") to booleans
- **Multivariate Support**: Full support for PostHog multivariate flags

### 🔒 **Secure by Default**
- Bearer token authentication with capability-based access control
- Configurable tokens for read, write, and delete operations
- Insecure mode available for development/testing only

### ⚡ **OpenFeature Compliant**
- Implements OpenFeature CLI sync API v0.1.0 specification
- Compatible with OpenFeature SDKs and tooling
- Standardized flag manifest format

## Configuration

### Required Environment Variables
```bash
POSTHOG_API_KEY=phx_your_api_key
POSTHOG_PROJECT_ID=your_project_id
```

### Optional Type Coercion
```bash
# Enable intelligent type conversion
COERCE_NUMERIC_STRINGS=true
COERCE_BOOLEAN_STRINGS=true
```

### Development Mode
```bash
# Disable authentication for testing
INSECURE_MODE=true
```

## Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│                 │    │                  │    │                 │
│ OpenFeature CLI │◄──►│ PostHog Proxy    │◄──►│ PostHog API     │
│                 │    │                  │    │                 │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                              │
                              ▼
                       ┌──────────────────┐
                       │  Transformation  │
                       │     Engine       │
                       └──────────────────┘
```

The proxy acts as a translation layer between OpenFeature and PostHog, providing:
- **Protocol Translation**: OpenFeature REST API ↔ PostHog REST API
- **Data Transformation**: PostHog flags → OpenFeature manifest format  
- **Type Intelligence**: Automatic type detection and coercion
- **Authentication**: Secure token-based access control

## Type Transformation Examples

### Before (PostHog)
```json
{
  "key": "config-flag",
  "filters": {
    "payloads": {
      "true": "{\"timeout\": 5000, \"enabled\": true}"
    }
  }
}
```

### After (OpenFeature)
```json
{
  "key": "config-flag",
  "type": "object", 
  "defaultValue": {"timeout": 5000, "enabled": true},
  "variants": {
    "true": {
      "value": {"timeout": 5000, "enabled": true}
    }
  }
}
```

## Common Use Cases

### 🎯 **Feature Toggle Management**
- Centralized feature flag management through OpenFeature CLI
- Consistent flag format across different feature flag providers
- Team collaboration through standardized tooling

### 🔧 **Configuration Management**  
- Dynamic configuration delivery through feature flags
- JSON object support for complex configuration
- Environment-specific configuration rollouts

### 📊 **A/B Testing**
- Multivariate flag support for A/B testing scenarios
- Weighted variant distribution
- Gradual feature rollouts

### 🚀 **CI/CD Integration**
- Automated flag management in deployment pipelines
- Infrastructure as code for feature flags
- GitOps workflows with OpenFeature CLI

## Development

### Project Structure
```
├── cmd/server/          # Application entry point
├── internal/
│   ├── config/         # Configuration management
│   ├── handlers/       # HTTP request handlers
│   ├── models/         # Data models
│   ├── posthog/        # PostHog API client  
│   ├── transformer/    # Flag transformation logic
│   └── debug/          # API logging utilities
└── docs/               # Documentation
```

### Building
```bash
make build
make test
make docker
```

### Debugging
Enable comprehensive API logging:
```bash
INSECURE_MODE=true ./posthog-proxy
# Check ./logs/ directory for API request/response logs
```

## Contributing

1. Review the transformation rules documentation
2. Understand the type detection logic
3. Add tests for any new transformation rules
4. Update documentation for API changes

## Support

- **Issues**: Report bugs and feature requests in the project repository
- **API Questions**: Refer to the [API Reference](./api-reference.md)
- **Transformation Logic**: See [Transformation Rules](./transformation-rules.md)
- **PostHog Documentation**: https://posthog.com/docs

## License

This project is licensed under the MIT License.