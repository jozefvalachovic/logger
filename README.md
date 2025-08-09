# Logger Package

A beautiful, high-performance logger for Go with colorized output, structured logging, and comprehensive data type support.

## Features

- 🌈 **Colorized log levels** - Debug, Info, Warn, Error with automatic color coding
- 📊 **Structured logging** - Key-value pairs with JSON-like output
- 🏗️ **Complex data structures** - Structs, arrays, maps, nested objects with JSON tag support
- 🌐 **HTTP middleware** - Clean request logging with panic recovery and colorized status codes
- 🔄 **Context support** - Distributed tracing with context-aware logging
- ⚙️ **Fully configurable** - Output destination, log levels, colors, time format
- 🎯 **Universal type support** - All Go primitive and complex types
- 🚀 **High performance** - Optimized with singleton pattern and efficient memory allocation
- 🔒 **Production ready** - Robust error handling with graceful degradation

## Installation

```bash
go get github.com/jozefvalachovic/logger/v2
```

## Quick Start

### Basic Logging

```go
package main

import "github.com/jozefvalachovic/logger/v2"

func main() {
    // Simple logging
    logger.Log(logger.Info, "Application started")
    logger.LogInfo("Application started")  // Convenience function

    // Structured logging with key-value pairs
    logger.LogInfo("User login",
        "username", "john",
        "id", 123,
        "rate", 3.14,
        "active", true)
}
```

### Complex Data Structures

```go
type User struct {
    ID       int      `json:"id"`
    Name     string   `json:"name"`
    Email    string   `json:"email"`
    Tags     []string `json:"tags"`
    Settings map[string]any `json:"settings"`
}

user := User{
    ID:    123,
    Name:  "John Doe",
    Email: "john@example.com",
    Tags:  []string{"admin", "active"},
    Settings: map[string]any{
        "theme": "dark",
        "notifications": true,
        "timeout": 30,
    },
}

// Automatically handles complex nested structures
logger.LogInfo("User created", "user", user)
```

### HTTP Middleware

```go
package main

import (
    "net/http"
    "github.com/jozefvalachovic/logger/v2"
)

func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/api/users", handleUsers)

    // Add clean HTTP logging middleware with panic recovery
    loggedMux := logger.LogMiddleware(mux)

    http.ListenAndServe(":8080", loggedMux)
}

// Clean, non-spammy output: 200 GET /api/users?page=1&limit=10 15.234ms
```

### Context-Aware Logging

```go
import "context"

func handleRequest(ctx context.Context) {
    // Extract trace ID from context if available
    logger.LogInfoWithContext(ctx, "Processing request",
        "operation", "user_lookup",
        "step", 1)
}
```

## Configuration

```go
// Custom configuration
logger.SetConfig(logger.Config{
    Output:      os.Stderr,
    Level:       slog.LevelWarn,
    EnableColor: false,
    TimeFormat:  "15:04:05",
})

// Get current configuration
config := logger.GetConfig()
```

## Logging Methods

### Core Functions

```go
// Explicit log level (v1 compatible)
logger.Log(logger.Info, "User action", "username", "john", "action", "login")

// Convenience functions
logger.LogDebug("Debug message", "key", "value")
logger.LogInfo("Info message", "key", "value")
logger.LogWarn("Warning message", "key", "value")
logger.LogError("Error message", "key", "value")

// Context-aware logging
logger.LogInfoWithContext(ctx, "Message", "key", "value")
logger.LogDebugWithContext(ctx, "Debug info", "trace_id", "abc123")
logger.LogWarnWithContext(ctx, "Warning", "component", "auth")
logger.LogErrorWithContext(ctx, "Error occurred", "error", err)
```

### HTTP Middleware

```go
// Add to any HTTP handler for clean request logging
mux := logger.LogMiddleware(http.NewServeMux())

// Features:
// - Clean, single-line request logs (prevents spam)
// - Colorized status codes (2xx=green, 3xx=blue, 4xx/5xx=red)
// - Request duration tracking
// - Full URL path with query parameters
// - Panic recovery with structured error logging
// - Method and status code logging
```

## Log Levels

- `logger.Debug` - Purple (detailed debugging information)
- `logger.Info` - Blue (general information)
- `logger.Warn` - Yellow (warning conditions)
- `logger.Error` - Red (error conditions)

Colors are automatically applied when `EnableColor` is `true` (default).

## Supported Data Types

The logger automatically handles any Go data type:

### Primitive Types

- `string`, `int`, `int8`, `int16`, `int32`, `int64`
- `uint`, `uint8`, `uint16`, `uint32`, `uint64`
- `float32`, `float64`, `bool`, `nil`

### Complex Types

- **Structs** - Respects JSON tags, converts to structured objects
- **Arrays & Slices** - Any slice type including custom structs
- **Maps** - With any key/value types
- **Nested structures** - Deeply nested objects and arrays
- **Pointers** - Safe nil pointer handling

### JSON Tag Support

```go
type Product struct {
    ID       int    `json:"product_id"`      // Uses JSON tag name
    Name     string `json:"product_name"`    // Uses JSON tag name
    Internal string `json:"-"`               // Excluded from output
    Price    float64                         // Uses field name
}

logger.LogInfo("Product created", "product", product)
// Output uses "product_id" and "product_name" field names
```

## Example Output

### Structured Logging

```
2025-08-08 14:30:15 INFO User login {
  "username": "john",
  "id": 123,
  "rate": 3.14,
  "active": true
}
```

### Complex Data Structures

```
2025-08-08 14:30:16 INFO User created {
  "user": {
    "id": 123,
    "name": "John Doe",
    "email": "john@example.com",
    "tags": ["admin", "active"],
    "settings": {
      "theme": "dark",
      "notifications": true,
      "timeout": 30
    }
  }
}
```

### HTTP Middleware Output (Clean & Concise)

```
200 GET /api/users?page=1&limit=10 15.234ms
201 POST /api/users 45.123ms
404 GET /api/nonexistent 2.456ms
500 POST /api/error 123.789ms
```

### Context-Aware Logging

```
2025-08-08 14:30:18 INFO Processing request {
  "trace_id": "abc123def456",
  "operation": "user_lookup",
  "step": 1
}
```

## API Reference

### Configuration

- `SetConfig(Config)` - Configure logger settings (output, level, colors, time format)
- `GetConfig() Config` - Get current configuration

### Core Logging Functions

- `Log(LogLevel, string, ...any)` - Main logging function (v1 compatible)
- `LogDebug(string, ...any)` - Debug level convenience function
- `LogInfo(string, ...any)` - Info level convenience function
- `LogWarn(string, ...any)` - Warn level convenience function
- `LogError(string, ...any)` - Error level convenience function

### Context-Aware Functions

- `LogDebugWithContext(context.Context, string, ...any)` - Debug with context
- `LogInfoWithContext(context.Context, string, ...any)` - Info with context
- `LogWarnWithContext(context.Context, string, ...any)` - Warn with context
- `LogErrorWithContext(context.Context, string, ...any)` - Error with context

### HTTP Middleware

- `LogMiddleware(http.Handler) http.Handler` - Clean HTTP request logging with panic recovery

### Types

- `LogLevel` - Debug, Info, Warn, Error constants
- `Config` - Logger configuration struct with Output, Level, EnableColor, TimeFormat fields

## Performance

The logger is optimized for high-performance production use:

- **Singleton pattern** - Logger instance reused across calls
- **Pre-allocated memory** - Efficient slice allocation
- **Benchmarked** - Includes performance test suite
- **Zero-allocation** - String operations where possible
- **Smart type conversion** - Optimized for common types
- **Non-spammy HTTP logs** - Clean, single-line request logging

Run benchmarks:

```bash
go test -bench=. -benchmem
```

## Migration from v1

v2 is **100% backward compatible** with v1. No code changes required:

```go
// v1 code continues to work unchanged
logger.Log(logger.Info, "message", "key", "value")

// v2 adds new convenience functions (optional)
logger.LogInfo("message", "key", "value")

// v2 adds new features (optional)
logger.LogMiddleware(handler)
logger.LogInfoWithContext(ctx, "message")
```
