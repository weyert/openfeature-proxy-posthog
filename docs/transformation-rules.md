# PostHog to OpenFeature Transformation Rules

This document describes how the PostHog OpenFeature proxy transforms PostHog feature flags into OpenFeature-compliant manifest format.

## Table of Contents

- [Overview](#overview)
- [Type Detection Logic](#type-detection-logic)
- [Type Coercion Feature Gates](#type-coercion-feature-gates)
- [Variant Transformation](#variant-transformation)
- [Configuration](#configuration)
- [Examples](#examples)

## Overview

The transformation process converts PostHog feature flags to OpenFeature manifest format by:

1. **Type Detection**: Analyzing PostHog flag data to determine the appropriate OpenFeature type
2. **Value Transformation**: Converting PostHog values to OpenFeature-compatible values
3. **Variant Processing**: Transforming PostHog variants and payloads to OpenFeature variants
4. **State Mapping**: Converting PostHog active/inactive states to OpenFeature enabled/disabled states

## Type Detection Logic

The proxy determines OpenFeature flag types using the following priority order:

### 1. JSON Object Detection (Highest Priority)
- **Condition**: PostHog payload contains valid JSON objects (starts with `{`, ends with `}`)
- **OpenFeature Type**: `object`
- **Example**: 
  ```json
  // PostHog payload: {"key": 500}
  // OpenFeature type: "object"
  // OpenFeature value: {"key": 500}
  ```

### 2. Type Coercion (Optional, Feature Gated)
When type coercion is enabled, the proxy attempts to parse string payloads:

#### Boolean String Coercion
- **Feature Gate**: `COERCE_BOOLEAN_STRINGS=true`
- **Accepted Values**: 
  - `true`: `"true"`, `"yes"`, `"on"` (case-insensitive)
  - `false`: `"false"`, `"no"`, `"off"` (case-insensitive)
- **OpenFeature Type**: `boolean`
- **Note**: Numeric strings like `"1"` and `"0"` are NOT converted to boolean

#### Numeric String Coercion
- **Feature Gate**: `COERCE_NUMERIC_STRINGS=true`
- **Accepted Values**: Any valid numeric string (`"1"`, `"3.14"`, `"-5"`)
- **OpenFeature Type**: `number`
- **Value Types**: 
  - Integers: `"42"` → `42` (int)
  - Large integers: `"999999999999"` → `999999999999` (int64)
  - Floats: `"3.14"` → `3.14` (float64)

### 3. Multivariate Variant Analysis
- **Condition**: PostHog flag has multivariate configuration with variants
- **Logic**: 
  - If first variant key is numeric → `number` type
  - Otherwise → `string` type

### 4. Simple Flag Detection (Default)
- **Condition**: PostHog `is_simple_flag` is true or no other conditions match
- **OpenFeature Type**: `boolean`
- **Default Value**: `false`

## Type Coercion Feature Gates

Type coercion is controlled by environment variables and disabled by default for backward compatibility.

### Configuration Variables

```bash
# Enable numeric string coercion
COERCE_NUMERIC_STRINGS=true

# Enable boolean string coercion  
COERCE_BOOLEAN_STRINGS=true
```

### Coercion Priority

When both coercion types are enabled, the priority is:

1. **JSON Object** (always highest priority)
2. **Boolean String** (more specific than numeric)
3. **Numeric String** (fallback for non-boolean strings)
4. **Original String** (no coercion applied)

### Examples

```bash
# With COERCE_BOOLEAN_STRINGS=true
"true" → boolean: true
"false" → boolean: false  
"yes" → boolean: true
"1" → string: "1" (not coerced to boolean)

# With COERCE_NUMERIC_STRINGS=true
"1" → number: 1
"3.14" → number: 3.14
"true" → string: "true" (not coerced to number)

# With both enabled
"true" → boolean: true (boolean has priority)
"1" → number: 1
"hello" → string: "hello"
```

## Variant Transformation

### Multivariate Flags

For PostHog flags with multivariate configuration:

```json
{
  "filters": {
    "multivariate": {
      "variants": [
        {"key": "variant1", "rollout_flag": 30},
        {"key": "variant2", "rollout_flag": 70}
      ]
    },
    "payloads": {
      "variant1": "payload1",
      "variant2": "{\"key\": 500}"
    }
  }
}
```

Transforms to:

```json
{
  "variants": {
    "variant1": {
      "value": "payload1",
      "weight": 30
    },
    "variant2": {
      "value": {"key": 500},
      "weight": 70
    }
  }
}
```

### Payload Processing

For each variant payload, the transformation applies:

1. **JSON Object Detection**: `"{\"key\": 500}"` → `{"key": 500}`
2. **Type Coercion** (if enabled):
   - Boolean: `"true"` → `true`
   - Numeric: `"42"` → `42`
3. **Fallback**: Use original string value

### Simple Flags with Payloads

For non-multivariate flags with payloads:

```json
{
  "filters": {
    "payloads": {
      "true": "{\"config\": \"value\"}"
    }
  }
}
```

Transforms to:

```json
{
  "variants": {
    "true": {
      "value": {"config": "value"}
    }
  }
}
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `COERCE_NUMERIC_STRINGS` | `false` | Enable numeric string to number coercion |
| `COERCE_BOOLEAN_STRINGS` | `false` | Enable boolean string to boolean coercion |

### Application Configuration

The proxy loads configuration in this order:
1. `.env.local` file
2. `.env` file  
3. System environment variables

Example `.env.local`:
```bash
# Enable type coercion
COERCE_NUMERIC_STRINGS=true
COERCE_BOOLEAN_STRINGS=true
```

## Examples

### Example 1: Object Type Flag

**PostHog Flag:**
```json
{
  "key": "remote-config",
  "name": "Remote configuration", 
  "active": true,
  "filters": {
    "payloads": {
      "true": "{\"key\": 500}"
    }
  }
}
```

**OpenFeature Result:**
```json
{
  "key": "remote-config",
  "name": "Remote configuration",
  "type": "object",
  "defaultValue": {"key": 500},
  "variants": {
    "true": {
      "value": {"key": 500}
    }
  },
  "state": "ENABLED"
}
```

### Example 2: Numeric Flag with Coercion

**PostHog Flag:**
```json
{
  "key": "numeric-flag",
  "name": "Numeric feature flag",
  "active": true,
  "filters": {
    "multivariate": {
      "variants": [
        {"key": "number1", "rollout_flag": 0},
        {"key": "number2", "rollout_flag": 0}
      ]
    },
    "payloads": {
      "number1": "1",
      "number2": "2"
    }
  }
}
```

**OpenFeature Result (with `COERCE_NUMERIC_STRINGS=true`):**
```json
{
  "key": "numeric-flag",
  "name": "Numeric feature flag",
  "type": "number",
  "defaultValue": 1,
  "variants": {
    "number1": {
      "value": 1,
      "weight": 0
    },
    "number2": {
      "value": 2, 
      "weight": 0
    }
  },
  "state": "ENABLED"
}
```

### Example 3: Boolean Flag with Coercion

**PostHog Flag:**
```json
{
  "key": "feature-toggle",
  "name": "Feature Toggle",
  "active": true,
  "filters": {
    "payloads": {
      "true": "true",
      "false": "false"
    }
  }
}
```

**OpenFeature Result (with `COERCE_BOOLEAN_STRINGS=true`):**
```json
{
  "key": "feature-toggle", 
  "name": "Feature Toggle",
  "type": "boolean",
  "defaultValue": true,
  "variants": {
    "true": {
      "value": true
    },
    "false": {
      "value": false
    }
  },
  "state": "ENABLED"
}
```

## Implementation Details

The transformation logic is implemented in `/internal/transformer/transformer.go` with the following key functions:

- `PostHogToOpenFeatureManifest()`: Main transformation entry point
- `determineFlagTypeAndValue()`: Type detection logic
- `convertPostHogVariants()`: Variant transformation
- `tryParseBooleanString()`: Boolean string coercion
- `tryParseNumericString()`: Numeric string coercion
- `isJSONObject()` / `parseJSONObject()`: JSON object detection

For detailed implementation, refer to the source code and inline comments.