# Logger Package

A beautiful, high-performance logger for Go with colorized output, structured logging, and comprehensive data type support.

## Features

- 🌈 **Colorized log levels** — Debug, Info, Warn, Error with automatic color coding
- 📊 **Structured logging** — Key-value pairs with JSON-like output
- 🏗️ **Complex data structures** — Structs, arrays, maps, nested objects with JSON tag support
- 🌐 **HTTP middleware** — Clean request logging with panic recovery and colorized status codes
- 🔄 **Context support** — Distributed tracing with context-aware logging
- ⚙️ **Fully configurable** — Output destination, log levels, colors, time format
- 🎯 **Universal type support** — All Go primitive and complex types
- 🚀 **High performance** — Optimized with singleton pattern and efficient memory allocation
- 🔒 **Production ready** — Robust error handling with graceful degradation

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
    logger.LogInfo("Hello, world!", "user", "alice")
}
```

### Complex Data Structures

```go
type User struct {
    ID       int               `json:"id"`
    Name     string            `json:"name"`
    Roles    []string          `json:"roles"`
    Settings map[string]any    `json:"settings"`
}

user := User{
    ID:    123,
    Name:  "Alice",
    Roles: []string{"admin", "user"},
    Settings: map[string]any{
        "theme": "dark",
        "email_notifications": true,
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
    mux.HandleFunc("/api/test", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte(`{"result":"ok"}`))
    })
    loggedMux := logger.LogMiddleware(mux)
    http.ListenAndServe(":8080", loggedMux)
}

// Output example:
// 200 GET /api/test test-agent 15.234ms
```

### Context-Aware Logging

```go
import (
    "context"
    "github.com/jozefvalachovic/logger/v2"
)

func handleRequest(ctx context.Context) {
    ctx = context.WithValue(ctx, "trace_id", "abc123def456")
    logger.LogInfoWithContext(ctx, "Processing request", "step", 1)
}
```

## Configuration

```go
// Custom configuration
logger.SetConfig(logger.Config{
    Output:      os.Stderr,
    Level:       slog.LevelInfo,
    EnableColor: true,
    TimeFormat:  "15:04:05",
    RedactKeys:  []string{"password", "token", "secret"}, // Keys to redact in logs
    RedactMask:  "***",                                   // Mask value for redacted fields
})

// Get current configuration
config := logger.GetConfig()
```

- **RedactKeys**: List of keys whose values will be masked in all log output (case-insensitive).
- **RedactMask**: String used to replace the value of any redacted key.

### Example

```go
logger.SetConfig(logger.Config{
    Output:      os.Stdout,
    RedactKeys:  []string{"apiKey"},
    RedactMask:  "[HIDDEN]",
})

logger.LogInfo("User login", "username", "alice", "apiKey", "123456")
```

**Output:**

```
2025-11-12 10:04:12 INFO User login {
  "username": "alice",
  "apiKey": "[HIDDEN]"
}
```

## Logging Methods

### Core Functions

```go
// Explicit log level (v1 compatible)
logger.Log(logger.Info, "User action", "username", "john", "action", "login")

// Convenience functions
logger.LogDebug("Debug message", "key", "value")
logger.LogInfo("Info message", "key", "value")
logger.LogNotice("Notice message", "key", "value")
logger.LogTrace("Trace message", "key", "value")
logger.LogWarn("Warning message", "key", "value")
logger.LogError("Error message", "key", "value")

// Context-aware logging
logger.LogInfoWithContext(ctx, "Message", "key", "value")
```

### HTTP Request Logging

```go
req := &http.Request{ /* ... */ }
logger.LogHttpRequest(req)
```

- Logs status code, method, path, user agent, and request body (JSON or text).

### HTTP Middleware

```go
package main

import (
    "net/http"
    "github.com/jozefvalachovic/logger/v2"
)

func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/api/test", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte(`{"result":"ok"}`))
    })
    // Only logs requests (does not log request body for errors)
    loggedMux := logger.LogMiddleware(mux, false)
    http.ListenAndServe(":8080", loggedMux)

    // To also log request bodies for 4xx/5xx responses:
    // loggedMux := logger.LogMiddleware(mux, true)
}
```

**Features:**

- Clean, single-line request logs
- Colorized status codes (2xx=green, 3xx=blue, 4xx/5xx=red)
- Request duration tracking
- Full URL path with query parameters
- Panic recovery with structured error logging
- Method and status code logging
- Log request body for error responses (4xx/5xx) when `logBodyOnErrors` is `true`

## Log Levels

- `logger.Debug` — Purple (detailed debugging information)
- `logger.Info` — Blue (general information)
- `logger.Warn` — Yellow (warning conditions)
- `logger.Error` — Red (error conditions)

Colors are automatically applied when `EnableColor` is `true` (default).

## Supported Data Types

The logger automatically handles any Go data type:

### Primitive Types

- `string`, `int`, `int8`, `int16`, `int32`, `int64`
- `uint`, `uint8`, `uint16`, `uint32`, `uint64`
- `float32`, `float64`, `bool`, `nil`

### Complex Types

- **Structs** — Respects JSON tags, converts to structured objects
- **Arrays & Slices** — Any slice type including custom structs
- **Maps** — With any key/value types
- **Nested structures** — Deeply nested objects and arrays
- **Pointers** — Safe nil pointer handling

### JSON Tag Support

```go
type Product struct {
    ID    int    `json:"product_id"`
    Name  string `json:"product_name"`
    Price float64
}

logger.LogInfo("Product created", "product", product)
// Output uses "product_id" and "product_name" field names
```

## Example Output

### Structured Logging

```
2025-11-12 10:04:12 INFO User login {
  "username": "john",
  "active": true
}
```

### Complex Data Structures

```
2025-11-12 10:04:13 INFO User created {
  "user": {
    "id": 123,
    "name": "Alice",
    "roles": ["admin", "user"],
    "settings": {
      "theme": "dark",
      "email_notifications": true
    }
  }
}
```

### HTTP Middleware Output

```
200 GET /api/test test-agent 15.234ms
404 GET /api/notfound test-agent 2.456ms
500 POST /api/error test-agent 123.789ms
```

### Context-Aware Logging

```
2025-11-12 10:04:14 INFO Processing request {
  "trace_id": "abc123def456",
  "step": 1
}
```

## API Reference

### Configuration

- `SetConfig(Config)` — Configure logger settings (output, level, colors, time format)
- `GetConfig() Config` — Get current configuration

### Core Logging Functions

- `Log(LogLevel, string, ...any)` — Main logging function (v1 compatible)
- `LogDebug(string, ...any)` — Debug level convenience function
- `LogInfo(string, ...any)` — Info level convenience function
- `LogWarn(string, ...any)` — Warn level convenience function
- `LogError(string, ...any)` — Error level convenience function

### Context-Aware Functions

- `LogInfoWithContext(context.Context, string, ...any)` — Info with context

### HTTP Request Logging

- `LogHttpRequest(*http.Request)` — Logs HTTP request details

### HTTP Middleware

- `LogMiddleware(http.Handler) http.Handler` — Clean HTTP request logging with panic recovery

### Types

- `LogLevel` — Debug, Info, Warn, Error constants
- `Config` — Logger configuration struct with Output, Level, EnableColor, TimeFormat fields

## Performance

The logger is optimized for high-performance production use:

- **Singleton pattern** — Logger instance reused across calls
- **Pre-allocated memory** — Efficient slice allocation
- **Benchmarked** — Includes performance test suite
- **Zero-allocation** — String operations where possible
- **Smart type conversion** — Optimized for common types
- **Non-spammy HTTP logs** — Clean, single-line request logging

Run benchmarks:

```bash
go test -bench=. -benchmem
```

## Migration from v1

- The API remains compatible with v1 for basic logging.
- New features include structured logging, colorized output, context support, and HTTP middleware.

---

**For more examples and documentation, see the source code and tests.**
