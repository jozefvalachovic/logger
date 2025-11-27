# Logger Package

A beautiful, high-performance logger for Go with colorized output, structured logging, and comprehensive data type support.

## Features

- 🌈 **Colorized log levels** — Trace, Debug, Info, Notice, Warn, Error, Audit with automatic color coding
- 📊 **Structured logging** — Key-value pairs with JSON-like output
- 🔐 **Audit logging** — Dedicated audit log level for security and compliance events
- 🏗️ **Complex data structures** — Structs, arrays, maps, nested objects with JSON tag support
- 🌐 **HTTP middleware** — Clean request logging with panic recovery and colorized status codes
- 🔄 **Context support** — Distributed tracing with context-aware logging
- ⚙️ **Fully configurable** — Output destination, log levels, colors, time format
- 🎯 **Universal type support** — All Go primitive and complex types
- 🚀 **High performance** — Optimized with singleton pattern and efficient memory allocation
- 🔒 **Production ready** — Robust error handling with graceful degradation
- 🎲 **Log Sampling** — Reduce log volume by sampling a percentage of messages
- 🔄 **Log Rotation** — Automatic log file rotation based on size or age
- ⚡ **Async Logging** — Non-blocking log writes for high-throughput applications
- 📈 **Metrics** — Built-in log metrics collection and reporting

## Installation

```bash
go get github.com/jozefvalachovic/logger/v3
```

## Quick Start

### Basic Logging

```go
package main

import (
    "time"
    "github.com/jozefvalachovic/logger/v3"
)

func main() {
    // Regular log messages
    logger.LogInfo("Hello, world!", "user", "alice")
    logger.LogError("Something failed", "error", "timeout")

    // Audit logs - no message, just structured data
    logger.LogAudit(
        "action", "user_login",
        "user_id", "123",
        "ip_address", "192.168.1.1",
        "timestamp", time.Now().Unix(),
        "success", true,
    )
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
    "github.com/jozefvalachovic/logger/v3"
    "github.com/jozefvalachovic/logger/v3/middleware"
)

func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/api/test", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte(`{"result":"ok"}`))
    })

    // Use middleware package for HTTP logging
    // Second parameter: log request body on errors (4xx/5xx)
    loggedMux := middleware.LogHTTPMiddleware(mux, true)
    http.ListenAndServe(":8080", loggedMux)
}

// Output example:
// 200 GET /api/test test-agent 15.234ms
```

### Context-Aware Logging

```go
import (
    "context"
    "github.com/jozefvalachovic/logger/v3"
)

func handleRequest(ctx context.Context) {
    ctx = context.WithValue(ctx, "trace_id", "abc123def456")
    logger.LogInfoWithContext(ctx, "Processing request", "step", 1)
}
```

## Log Levels

The logger supports the following log levels with distinct colors:

- `logger.Trace` — Gray (very detailed tracing)
- `logger.Debug` — Purple (detailed debugging information)
- `logger.Info` — Blue (general information)
- `logger.Notice` — Green (normal but significant events)
- `logger.Warn` — Yellow (warning conditions)
- `logger.Error` — Red (error conditions)
- `logger.Audit` — Bold Bright Cyan (security and compliance audit events)

**Audit Logging**: The `LogAudit` function is special - it only accepts key-value pairs and logs without a message, making it ideal for structured security audit trails:

```go
logger.LogAudit(
    "action", "password_reset",
    "user_id", "456",
    "performed_by", "admin",
    "timestamp", time.Now().Unix(),
)

// Output:
// 19:54:32 AUDIT {
//   "action": "password_reset",
//   "performed_by": "admin",
//   "timestamp": 1732740872,
//   "user_id": "456"
// }
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

## Advanced Features (v3.1.0+)

### Log Sampling

Reduce log volume by logging only a percentage of messages. Useful for high-traffic applications where you need to sample logs without losing observability.

```go
logger.SetConfig(logger.Config{
    Output:     os.Stdout,
    SampleRate: 0.1,  // Log only 10% of messages
    SampleSeed: 42,   // Optional: deterministic sampling with seed
})

// Only 10% of these messages will be logged
for i := 0; i < 1000; i++ {
    logger.LogInfo("High volume event", "id", i)
}
```

- **SampleRate**: Float between 0.0 and 1.0 (default: 1.0 = log everything)
- **SampleSeed**: Optional seed for deterministic sampling

### Log Rotation

Automatically rotate log files based on size or age, with optional compression and backup retention.

```go
// Create a rotating writer
rotatingWriter, err := logger.NewRotatingWriter("app.log", &logger.RotationConfig{
    MaxSize:    100 << 20,          // 100MB
    MaxAge:     24 * time.Hour,     // 24 hours
    MaxBackups: 7,                  // Keep 7 old files
    Compress:   true,               // Compress rotated files
})
if err != nil {
    panic(err)
}
defer rotatingWriter.Close()

// Use the rotating writer
logger.SetConfig(logger.Config{
    Output: rotatingWriter,
})

logger.LogInfo("This will be written to a rotating log file")
```

- **MaxSize**: Maximum file size before rotation (in bytes)
- **MaxAge**: Maximum age before rotation
- **MaxBackups**: Number of old files to keep (0 = keep all)
- **Compress**: Whether to compress rotated files

### Async Logging

Enable non-blocking log writes for high-throughput applications. Logs are queued and written asynchronously.

```go
logger.SetConfig(logger.Config{
    Output:       os.Stdout,
    AsyncMode:    true,
    BufferSize:   1000,                    // Queue size
    FlushTimeout: 500 * time.Millisecond, // Flush interval
})

// These log calls return immediately without blocking
for i := 0; i < 10000; i++ {
    logger.LogInfo("High throughput message", "id", i)
}
```

- **AsyncMode**: Enable async logging (default: false)
- **BufferSize**: Channel buffer size (default: 1000)
- **FlushTimeout**: How often to flush buffered logs (default: 1s)

**Note**: When the buffer is full, logs automatically fall back to synchronous writes to prevent data loss.

### Metrics Collection

Track logging statistics including total logs, logs by level, and error rates.

```go
logger.SetConfig(logger.Config{
    Output:        os.Stdout,
    EnableMetrics: true,
    MetricsPrefix: "myapp", // Optional prefix for metric names
})

// Log some messages
logger.LogInfo("Info message")
logger.LogWarn("Warning message")
logger.LogError("Error message")

// Get metrics
metrics := logger.GetMetrics()
fmt.Printf("Total logs: %v\n", metrics["total_logs"])
fmt.Printf("Info logs: %v\n", metrics["logs_info"])
fmt.Printf("Error logs: %v\n", metrics["logs_error"])
fmt.Printf("Error rate: %v\n", metrics["error_rate"])
```

- **EnableMetrics**: Enable metrics collection (default: false)
- **MetricsPrefix**: Prefix for metric names (default: "logger")

Available metrics:

- `total_logs`: Total number of logs
- `logs_<level>`: Count per log level (trace, debug, info, notice, warn, error)
- `error_rate`: Errors per second

### Combining Features

You can combine multiple advanced features:

```go
logger.SetConfig(logger.Config{
    Output:        rotatingWriter,
    SampleRate:    0.5,              // Sample 50% of logs
    AsyncMode:     true,             // Non-blocking writes
    EnableMetrics: true,             // Track statistics
    BufferSize:    2000,
    FlushTimeout:  time.Second,
})
```

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
    "github.com/jozefvalachovic/logger/v3"
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

### TCP Middleware

```go
package main

import (
    "net"
    "github.com/jozefvalachovic/logger/v3"
)

func main() {
    // Your actual connection handler
    handler := func(conn net.Conn) {
        // Handle the connection
        conn.Close()
    }

    // Apply middleware (outermost to innermost)
    wrappedHandler := LogTCPMiddleware(handler)

    // Pass the wrapped handler to your TCP server
    server := NewTCPServer(
        "MyApp",
        "1.0.0",
        0, 0,
        wrappedHandler,
        nil, // tlsConfig
    )

    // Start the server
    server.Start()
}
```

**Features:**

- Logs when a TCP connection is started and ended
- Recovers from panics and logs errors
- Easy to compose with other middleware

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

## Examples

The `examples/` directory contains complete, runnable examples:

- **`examples/basic/`** — All log levels demonstration
- **`examples/audit/`** — Security audit logging examples
- **`examples/http-middleware/`** — HTTP request logging middleware
- **`examples/advanced/`** — Sampling, async logging, metrics, and context

Run any example:

```bash
cd examples/basic && go run main.go
```

## Migration from v1

- The API remains compatible with v1 for basic logging.
- New features include structured logging, colorized output, context support, and HTTP middleware.

---

**For more examples and documentation, see the examples directory and tests.**
