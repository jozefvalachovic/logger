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

### New in v4.1.0

- 👶 **Child Loggers** — `With()` creates loggers with pre-set fields for request/module scoping
- 📍 **Caller Attribution** — Automatic `[file:line]` source location in log output
- 🛑 **Graceful Shutdown** — `Shutdown()` drains async buffers, flushes dedup, and closes audit
- 🔭 **OpenTelemetry Bridge** — `OTelBridgeHandler` maps custom levels for OTel-compatible collectors
- 🔏 **Regex Redaction** — Pattern-based value redaction (emails, credit cards, etc.)
- 🎚️ **Level Filtering** — `LevelFilterHandler` sets per-handler minimum log levels
- 🪵 **Error Logging with Stack** — `LogErrorWithStack()` captures error type, chain, and stack trace
- 🌍 **Environment-Aware Defaults** — `ConfigFromEnv()` reads `LOG_LEVEL`, `LOG_COLOR`, `LOG_CALLER`, etc.
- 📡 **gRPC Interceptor Helpers** — Zero-dependency `LogGRPCUnary` / `LogGRPCStream` wrappers
- 🔇 **Log Deduplication** — Suppress repeated messages within a configurable time window
- 💓 **Health Check** — `HealthCheck()` verifies output writer, buffer usage, and audit state
- 📊 **Prometheus Metrics Endpoint** — `MetricsHandler()` serves Prometheus text exposition format
- ✍️ **Ed25519 Audit Signing** — Cryptographic signatures on audit entries for non-repudiation
- 🗄️ **SQL Database Store** — `SQLStore` for PostgreSQL, MySQL, SQLite audit storage
- 🎯 **Body Sampling** — Probabilistic HTTP body capture via `WithBodySampleRate()`
- 🗜️ **Compact / Colorized JSON** — Single-line and color-highlighted JSON output modes
- ⏭️ **Conditional Evaluation** — `IfDebug()`, `IfTrace()`, etc. skip expensive computations
- 🌐 **WebSocket Middleware** — Logs upgrade, tracks messages/bytes, logs close with duration
- 📺 **SSE Audit Streaming** — Real-time Server-Sent Events stream for audit events
- 🔗 **Audit Correlation ID** — Link related audit events across services

### Enterprise Audit (v4)

- 🛡️ **Tamper Detection** — SHA-256/512 hash chain for audit log integrity
- 💾 **Guaranteed Delivery** — Write-ahead log (WAL) ensures no audit events are lost
- 🔗 **Distributed Tracing** — W3C, B3, and Jaeger trace context propagation
- 📤 **Multi-Sink Support** — Write to files, webhooks, and custom destinations simultaneously
- ⏰ **Retention Policies** — Automatic archival, compression, and cleanup
- 🏢 **Compliance Presets** — SOC2, HIPAA, PCI-DSS, GDPR, and FedRAMP configurations
- 🔍 **Query & Export API** — Search audit logs and export to JSON, JSONL, or CSV
- 🚦 **Rate Limiting** — Token bucket rate limiter to protect downstream systems

## Installation

```bash
go get github.com/jozefvalachovic/logger/v4
```

## Quick Start

### Basic Logging

```go
package main

import (
    "time"
    "github.com/jozefvalachovic/logger/v4"
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
    "github.com/jozefvalachovic/logger/v4"
    "github.com/jozefvalachovic/logger/v4/middleware"
)

func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/api/test", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte(`{"result":"ok"}`))
    })

    // Basic usage with functional options
    loggedMux := middleware.LogHTTPMiddleware(mux,
        middleware.WithLogBodyOnErrors(true),
        middleware.WithRequestID(true),
    )
    http.ListenAndServe(":8080", loggedMux)
}

// Output example:
// INFO GET /api/test [200] 15.234ms
```

### Context-Aware Logging

```go
import (
    "context"
    "github.com/jozefvalachovic/logger/v4"
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

## Enterprise Audit Logging (v4)

For production systems requiring compliance, tamper detection, and guaranteed delivery, use the enterprise audit package:

### Quick Start

```go
package main

import (
    "context"
    "github.com/jozefvalachovic/logger/v4"
    "github.com/jozefvalachovic/logger/v4/audit"
)

func main() {
    // Configure enterprise audit
    logger.SetConfig(logger.Config{
        Audit: &audit.Config{
            EnableStructured: true,
            HashChain: audit.HashChainConfig{
                Enabled:   true,
                Algorithm: "sha256",
            },
            Service: &audit.ServiceContext{
                Name:        "my-service",
                Version:     "1.0.0",
                Environment: "production",
            },
        },
    })

    // Log structured audit events
    ctx := context.Background()
    logger.LogAuditEvent(ctx, audit.AuditEvent{
        Type:    audit.AuditAuth,
        Action:  "user_login",
        Outcome: audit.OutcomeSuccess,
        Actor: audit.AuditActor{
            ID:   "user-123",
            Type: "user",
            IP:   "192.168.1.100",
        },
        Description: "User successfully logged in",
    })
}
```

### Compliance Presets

Apply industry-standard compliance configurations with a single call:

```go
// SOC2 compliance (1 year retention, hash chain, WAL)
cfg := audit.DefaultConfig()
cfg.WithCompliance(audit.ComplianceSOC2)
cfg.WAL.Path = "/var/log/audit/wal"

// Available presets:
// - audit.ComplianceSOC2    (1 year retention)
// - audit.ComplianceHIPAA   (6 year retention, signatures)
// - audit.CompliancePCIDSS  (1 year retention)
// - audit.ComplianceGDPR    (90 day retention, auto-delete)
// - audit.ComplianceFedRAMP (3 year retention, signatures)
```

### Multi-Sink Support

Write audit logs to multiple destinations:

```go
import (
    "github.com/jozefvalachovic/logger/v4/audit"
    "github.com/jozefvalachovic/logger/v4/audit/sink"
)

// File sink with rotation
fileSink, _ := sink.NewFileSink(sink.FileSinkConfig{
    Path:        "/var/log/audit/audit.jsonl",
    MaxSize:     100 << 20, // 100MB
    RotateDaily: true,
})

// Webhook sink for SIEM integration
webhookSink := sink.NewWebhookSink(sink.WebhookSinkConfig{
    Endpoint:   "https://siem.example.com/audit",
    Headers:    map[string]string{"Authorization": "Bearer token"},
    MaxRetries: 3,
    BatchSize:  100,
})

// Combine sinks
multiSink := sink.NewMultiSink(fileSink, webhookSink)

cfg := audit.DefaultConfig()
cfg.Sinks = []audit.Sink{multiSink}
```

### Typed Errors (Go 1.26+)

The audit package provides typed error wrappers for precise error handling with `errors.AsType`:

```go
import "errors"

err := auditLogger.LogSync(ctx, event)

// Extract specific error types
if se, ok := errors.AsType[*audit.SinkError](err); ok {
    log.Printf("sink %q failed: %v", se.SinkName, se.Err)
}
if we, ok := errors.AsType[*audit.WALError](err); ok {
    log.Printf("WAL %s failed: %v", we.Op, we.Err)
}
if se, ok := errors.AsType[*audit.StoreError](err); ok {
    log.Printf("store %s failed: %v", se.Op, se.Err)
}
```

Multi-sink errors now preserve all failures via `errors.Join` instead of returning only the last error.

### Query & Export API

Search and export audit logs:

```go
import (
    "github.com/jozefvalachovic/logger/v4/audit"
    "github.com/jozefvalachovic/logger/v4/audit/store"
)

// Configure with a store for querying
memStore := store.NewMemoryStore(store.MemoryStoreConfig{MaxSize: 10000})
cfg := audit.DefaultConfig()
cfg.Store = memStore

auditLogger, _ := audit.New(cfg)

// Query audit logs
query := audit.NewQuery().
    WithTimeRange(audit.LastDays(7)).
    WithEventTypes(audit.AuditAuth, audit.AuditAuthz).
    WithActorIDs("user-123").
    WithOutcomes(audit.OutcomeFailure).
    WithLimit(100)

result, _ := auditLogger.Query(query)
for _, entry := range result.Entries {
    fmt.Printf("%s: %s by %s\n", entry.Timestamp, entry.Event.Action, entry.Event.Actor.ID)
}

// Export to CSV
file, _ := os.Create("audit-export.csv")
store.Export(file, result.Entries, store.FormatCSV)
```

### Distributed Tracing

Automatically extract and propagate trace context:

```go
// Extract trace context from HTTP headers (W3C, B3, or Jaeger)
cfg := audit.DefaultConfig()
cfg.Tracing = audit.TracingConfig{
    Enabled:           true,
    PropagationFormat: "w3c", // or "b3", "b3-single", "jaeger"
}

// In your HTTP handler
func handler(w http.ResponseWriter, r *http.Request) {
    traceInfo := audit.ExtractTraceContext(cfg.Tracing, r.Header.Get)
    ctx := audit.WithTraceContext(r.Context(), traceInfo)

    // Audit events automatically include trace_id and span_id
    logger.LogAuditEvent(ctx, audit.AuditEvent{...})
}
```

### Event Types

The audit package provides standard event types for compliance:

| Type                 | Description                                      |
| -------------------- | ------------------------------------------------ |
| `AuditAuth`          | Authentication events (login, logout, MFA)       |
| `AuditAuthz`         | Authorization/permission checks                  |
| `AuditDataAccess`    | Data read operations                             |
| `AuditDataModify`    | Data create/update/delete operations             |
| `AuditConfigChange`  | System configuration changes                     |
| `AuditAdminAction`   | Administrative actions                           |
| `AuditSecurityEvent` | Security-related events                          |
| `AuditUserLifecycle` | User account lifecycle (create, delete, disable) |
| `AuditAPIAccess`     | API access events                                |
| `AuditSystem`        | System events                                    |
| `AuditCustom`        | Custom event types                               |

### Audit Entry Schema

Each audit entry contains:

```json
{
  "id": "1737734400000000000-hostname-1-a1b2c3d4",
  "timestamp": "2026-01-24T12:00:00Z",
  "event": {
    "type": "authentication",
    "action": "user_login",
    "outcome": "success",
    "actor": {
      "id": "user-123",
      "type": "user",
      "ip": "192.168.1.100"
    },
    "resource": {
      "id": "app-1",
      "type": "application"
    },
    "description": "User logged in successfully"
  },
  "service": {
    "name": "auth-service",
    "version": "2.1.0",
    "environment": "production"
  },
  "trace": {
    "trace_id": "abc123...",
    "span_id": "def456..."
  },
  "hash": "sha256:...",
  "previous_hash": "sha256:...",
  "signature": "ed25519:...",
  "sequence": 42,
  "schema_version": "1.0"
}
```

## v4.1.0 Features

### Child Loggers

Create scoped loggers with pre-set fields that are included in every subsequent log call:

```go
// Create a child logger for a specific request
reqLogger := logger.With("request_id", "abc-123", "user_id", "user-456")
reqLogger.LogInfo("Processing payment", "amount", 99.99)
reqLogger.LogError("Payment failed", "reason", "insufficient funds")
// All log lines include request_id and user_id automatically

// Child loggers can be nested
dbLogger := reqLogger.With("component", "database")
dbLogger.LogDebug("Query executed", "query", "SELECT ...")
```

### Caller Attribution

Include source file and line number in every log line:

```go
logger.SetConfig(logger.Config{
    Output:       os.Stdout,
    EnableCaller: true,
})

logger.LogInfo("Server started", "port", 8080)
// Output: 10:04:12 INFO [main.go:42] Server started {"port": 8080}
```

### Graceful Shutdown

Drain all async buffers, flush dedup summaries, and close audit resources:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

if err := logger.Shutdown(ctx); err != nil {
    log.Fatalf("logger shutdown: %v", err)
}
```

### OpenTelemetry Bridge

Map custom log levels (Trace, Notice, Audit) for OTel-compatible log collectors:

```go
jsonHandler := slog.NewJSONHandler(os.Stdout, nil)

bridge := logger.NewOTelBridgeHandler(jsonHandler, "my-service", "1.0.0")
// Enriches every record with service.name and service.version
// Maps Trace→DEBUG-4, Notice→INFO+1, Audit→ERROR+4
```

### Regex Redaction Patterns

Redact values matching regex patterns across all log output:

```go
logger.SetConfig(logger.Config{
    Output: os.Stdout,
    RedactPatterns: []string{
        `\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`, // emails
        `\b\d{4}[- ]?\d{4}[- ]?\d{4}[- ]?\d{4}\b`,              // credit cards
        `\b\d{3}-\d{2}-\d{4}\b`,                                  // SSN
    },
    RedactMask: "[REDACTED]",
})

logger.LogInfo("User", "email", "alice@example.com") // email → [REDACTED]
```

### Level Filtering per Handler

Send different log levels to different destinations:

```go
// Errors-only to a file, everything to stdout
errorFileHandler := slog.NewJSONHandler(errorFile, nil)
errorOnly := logger.NewLevelFilterHandler(slog.LevelError, errorFileHandler)

logger.SetConfig(logger.Config{
    Output:             os.Stdout,
    AdditionalHandlers: []slog.Handler{errorOnly},
})
```

### Structured Error Logging

Log errors with type information, unwrap chain, and stack trace:

```go
err := fmt.Errorf("db timeout: %w", sql.ErrConnDone)

logger.LogErrorWithStack(err, "Failed to save user",
    "user_id", "123",
    "operation", "insert",
)
// Logs: error, error_type, error_chain, stack, plus your key-values
```

### Environment-Aware Defaults

Configure the logger entirely from environment variables — no code changes needed:

```bash
export LOG_LEVEL=debug
export LOG_COLOR=true
export LOG_CALLER=true
export LOG_FORMAT=compact
export LOG_REDACT_KEYS=password,token,secret
```

```go
// Reads all LOG_* env vars automatically on init()
// Or explicitly:
cfg := logger.ConfigFromEnv()
logger.SetConfig(cfg)
```

| Variable          | Values                                         | Default    |
| ----------------- | ---------------------------------------------- | ---------- |
| `LOG_LEVEL`       | trace, debug, info, notice, warn, error, audit | info       |
| `LOG_COLOR`       | true, false, 1, 0                              | false      |
| `LOG_CALLER`      | true, false, 1, 0                              | false      |
| `LOG_FORMAT`      | compact, json                                  | (indented) |
| `LOG_REDACT_KEYS` | comma-separated key names                      | (none)     |

### gRPC Interceptor Helpers

Zero-dependency gRPC logging — use inside your own interceptors:

```go
import "github.com/jozefvalachovic/logger/v4/middleware"

// Unary interceptor
func loggingUnary(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
    return middleware.LogGRPCUnary(ctx, info.FullMethod, func(ctx context.Context) (any, error) {
        return handler(ctx, req)
    }, middleware.WithGRPCSkipMethods("/grpc.health.v1.Health/Check"))
}

// Stream interceptor
func loggingStream(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
    return middleware.LogGRPCStream(ss.Context(), info.FullMethod, func(ctx context.Context) error {
        return handler(srv, ss)
    })
}

server := grpc.NewServer(
    grpc.UnaryInterceptor(loggingUnary),
    grpc.StreamInterceptor(loggingStream),
)
```

### Log Deduplication

Suppress repeated identical messages and emit a summary when the window expires:

```go
logger.SetConfig(logger.Config{
    Output:      os.Stdout,
    EnableDedup: true,
    DedupWindow: 10 * time.Second, // Default: 5s
})

// Only the first message is logged immediately
for i := 0; i < 100; i++ {
    logger.LogWarn("Connection retry failed")
}
// After 10s: "Connection retry failed (repeated 99 more times)"
```

### Health Check

Verify the logger subsystem is healthy — suitable for `/healthz` endpoints:

```go
if err := logger.HealthCheck(); err != nil {
    // Possible issues:
    // - Output writer is nil
    // - Async buffer >90% full
    // - Audit logger is closed or buffer saturated
    http.Error(w, err.Error(), http.StatusServiceUnavailable)
    return
}
```

### Prometheus Metrics Endpoint

Expose log metrics in Prometheus text exposition format:

```go
logger.SetConfig(logger.Config{
    Output:        os.Stdout,
    EnableMetrics: true,
    MetricsPrefix: "myapp",
})

http.Handle("/metrics", logger.MetricsHandler())
```

Exposed metrics:

- `myapp_logs_total` — Total log entries (counter)
- `myapp_logs_by_level{level="info"}` — Entries per level (counter)
- `myapp_error_rate` — Errors per second (gauge)

### Ed25519 Audit Signing

Sign audit entries with Ed25519 for cryptographic non-repudiation:

```go
import "crypto/ed25519"

pub, priv, _ := ed25519.GenerateKey(nil)

cfg := audit.DefaultConfig()
cfg.HashChain = audit.HashChainConfig{
    Enabled:          true,
    Algorithm:        "sha256",
    EnableSignatures: true,
    PrivateKey:       priv,
}

// Each audit entry now includes a Signature field
// Verify later:
//   audit.VerifySignatureWithKey(entry, pub)
```

### SQL Database Store

Store audit logs in any SQL database (PostgreSQL, MySQL, SQLite):

```go
import (
    "database/sql"
    _ "github.com/mattn/go-sqlite3" // your driver

    "github.com/jozefvalachovic/logger/v4/audit/store"
)

db, _ := sql.Open("sqlite3", "audit.db")
sqlStore, _ := store.NewSQLStore(db, "audit_log")

cfg := audit.DefaultConfig()
cfg.Store = sqlStore
```

The store auto-creates the table and supports the full `Query` API (time range, event types, actor IDs, actions).

### Body Sampling Middleware

Capture HTTP request bodies on a percentage of requests (not just errors):

```go
loggedMux := middleware.LogHTTPMiddleware(mux,
    middleware.WithBodySampleRate(0.05), // Capture body on 5% of requests
    middleware.WithLogBodyOnErrors(true), // Always capture on errors
)
```

### Compact / Colorized JSON Output

```go
logger.SetConfig(logger.Config{
    Output:       os.Stdout,
    CompactJSON:  true,  // Single-line JSON instead of indented
    ColorizeJSON: true,  // Colorize JSON keys (requires EnableColor)
    EnableColor:  true,
})

logger.LogInfo("Request", "method", "GET", "path", "/api/users")
// Output: 10:04:12 INFO Request {"method":"GET","path":"/api/users"}
```

### Conditional / Lazy Evaluation

Skip expensive computations when the log level wouldn't output them:

```go
logger.IfDebug(func() {
    // This closure only runs if Debug level is enabled
    stats := gatherExpensiveDebugStats()
    logger.LogDebug("Debug stats", "stats", stats)
})

logger.IfTrace(func() {
    dump := dumpFullRequestContext(r)
    logger.LogTrace("Full request dump", "dump", dump)
})
```

Available: `IfTrace`, `IfDebug`, `IfInfo`, `IfWarn`, `IfError`.

### WebSocket Middleware

Log WebSocket connection lifecycle with message and byte tracking:

```go
import "github.com/jozefvalachovic/logger/v4/middleware"

http.Handle("/ws", middleware.LogWebSocketMiddleware(wsHandler))

// Inside your WebSocket handler, record message stats:
func wsHandler(w http.ResponseWriter, r *http.Request) {
    stats := middleware.WebSocketStatsFromWriter(w)
    // ... handle messages ...
    if stats != nil {
        stats.MessagesReceived.Add(1)
        stats.BytesReceived.Add(int64(len(msg)))
    }
}

// Log output:
//   INFO WebSocket upgrade /ws
//   INFO WebSocket closed /ws 45.2s {messages_received: 120, bytes_sent: 48000}
```

### SSE Audit Event Streaming

Stream audit events to dashboards in real-time via Server-Sent Events:

```go
import "github.com/jozefvalachovic/logger/v4/audit/sink"

sseSink := sink.NewSSESink()

cfg := audit.DefaultConfig()
cfg.Sinks = []audit.Sink{fileSink, sseSink}

// Expose as HTTP endpoint
http.Handle("/audit/stream", sseSink.Handler())

// Clients connect with: curl -N http://localhost:8080/audit/stream
// Each audit event is pushed as: data: {"id":"...","event":{...}}
```

### Audit Correlation ID

Link related audit events across services using a correlation ID:

```go
logger.LogAuditEvent(ctx, audit.AuditEvent{
    Type:          audit.AuditDataModify,
    Action:        "order_created",
    Outcome:       audit.OutcomeSuccess,
    CorrelationID: "order-flow-abc-123", // Links across services
    Actor:         audit.AuditActor{ID: "user-456", Type: "user"},
})

// Query by correlation ID to trace the full flow
```

## Configuration

```go
// Custom configuration
logger.SetConfig(logger.Config{
    Output:      os.Stderr,
    Level:       slog.LevelInfo,
    EnableColor: true,
    TimeFormat:  "15:04:05",

    // Key-based redaction
    RedactKeys:  []string{"password", "token", "secret"},
    RedactMask:  "***",

    // Regex-based value redaction (v4.1.0)
    RedactPatterns: []string{`\b\d{3}-\d{2}-\d{4}\b`}, // SSN pattern

    // Caller attribution (v4.1.0)
    EnableCaller: true,

    // Output format (v4.1.0)
    CompactJSON:  true,
    ColorizeJSON: true,

    // Deduplication (v4.1.0)
    EnableDedup: true,
    DedupWindow: 10 * time.Second,
})

// Get current configuration
config := logger.GetConfig()
```

- **RedactKeys**: List of keys whose values will be masked in all log output (case-insensitive).
- **RedactMask**: String used to replace the value of any redacted key.

### Multi-Handler Output (Go 1.26+)

Send log output to multiple destinations using `slog.NewMultiHandler`:

```go
// Log to both stdout (pretty) and a JSON file simultaneously
jsonHandler := slog.NewJSONHandler(logFile, &slog.HandlerOptions{Level: slog.LevelInfo})

logger.SetConfig(logger.Config{
    Output:             os.Stdout,
    AdditionalHandlers: []slog.Handler{jsonHandler},
})
```

The pretty handler is always included. Additional handlers receive the same log records.

## Advanced Features (v4.0+)

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
    EnableCaller:  true,             // Source file:line
    EnableDedup:   true,             // Suppress repeated messages
    DedupWindow:   10 * time.Second,
    CompactJSON:   true,             // Single-line JSON
    BufferSize:    2000,
    FlushTimeout:  time.Second,
})

// Graceful shutdown drains all buffers
defer logger.Shutdown(context.Background())
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

// Child loggers with pre-set fields (v4.1.0)
reqLogger := logger.With("request_id", "abc-123")
reqLogger.LogInfo("Handled", "status", 200)

// Error logging with stack trace (v4.1.0)
logger.LogErrorWithStack(err, "Operation failed", "op", "save")

// Conditional evaluation — closure runs only if level is active (v4.1.0)
logger.IfDebug(func() { logger.LogDebug("expensive", "data", compute()) })
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
    "time"

    "github.com/jozefvalachovic/logger/v4"
    "github.com/jozefvalachovic/logger/v4/middleware"
)

func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/api/test", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte(`{"result":"ok"}`))
    })

    // Full-featured middleware with functional options
    loggedMux := middleware.LogHTTPMiddleware(mux,
        middleware.WithLogBodyOnErrors(true),
        middleware.WithLogResponseBody(true),
        middleware.WithRequestID(true),
        middleware.WithRequestIDHeader("X-Correlation-ID"),
        middleware.WithSkipPaths("/health", "/ready"),
        middleware.WithSkipPathPrefixes("/metrics"),
        middleware.WithLogLevel(500, logger.Error),
        middleware.WithLogLevels(map[int]logger.LogLevel{
            400: logger.Warn,
            500: logger.Error,
        }),
        middleware.WithAudit(true),
        middleware.WithAuditMethods("POST", "PUT", "DELETE"),
        middleware.WithMetrics(true),
        middleware.WithCustomFields(map[string]any{"service": "api"}),
        middleware.WithOnRequestStart(func(r *http.Request) {
            // Pre-request hook
        }),
        middleware.WithOnRequestEnd(func(r *http.Request, status int, duration time.Duration) {
            // Post-request hook
        }),
    )
    http.ListenAndServe(":8080", loggedMux)
}
```

#### Middleware Options

| Option                                   | Description                                            |
| ---------------------------------------- | ------------------------------------------------------ |
| `WithLogBodyOnErrors(bool)`              | Log request body on 4xx/5xx errors                     |
| `WithLogResponseBody(bool)`              | Log response body on errors                            |
| `WithRequestID(bool)`                    | Generate/extract request IDs                           |
| `WithRequestIDHeader(string)`            | Custom header for request ID (default: `X-Request-ID`) |
| `WithSkipPaths(...string)`               | Exact paths to exclude from logging                    |
| `WithSkipPathPrefixes(...string)`        | Path prefixes to exclude from logging                  |
| `WithLogLevel(int, LogLevel)`            | Custom log level for specific status code              |
| `WithLogLevels(map[int]LogLevel)`        | Custom log levels for status code ranges               |
| `WithAudit(bool)`                        | Enable audit event emission                            |
| `WithAuditMethods(...string)`            | HTTP methods to audit (nil = all)                      |
| `WithMetrics(bool)`                      | Enable metrics collection                              |
| `WithMetricsCollector(MetricsCollector)` | Custom metrics collector                               |
| `WithCustomFields(map[string]any)`       | Add fields to every log entry                          |
| `WithOnRequestStart(func)`               | Callback before request processing                     |
| `WithOnRequestEnd(func)`                 | Callback after request processing                      |

#### Request ID Context

Access request ID and timing in your handlers:

```go
func handler(w http.ResponseWriter, r *http.Request) {
    requestID := middleware.GetRequestID(r.Context())
    startTime := middleware.GetRequestStart(r.Context())
    // ...
}
```

#### Metrics Collection

Use the built-in metrics collector or implement your own:

```go
// Built-in default collector
collector := middleware.NewDefaultMetricsCollector()

loggedMux := middleware.LogHTTPMiddleware(mux,
    middleware.WithMetricsCollector(collector),
)

// Access metrics
metrics := collector.GetMetrics()
fmt.Printf("Total requests: %d\n", collector.GetTotalRequests())
fmt.Printf("Error rate: %.2f%%\n", collector.GetErrorRate())
fmt.Printf("Avg duration: %s\n", collector.GetAverageDuration())

// Custom implementation
type MyCollector struct{}
func (c *MyCollector) RecordRequest(method, path string, status int, duration time.Duration) {}
func (c *MyCollector) RecordError(method, path string, status int) {}
func (c *MyCollector) RecordPanic(method, path string) {}
```

**Features:**

- Clean, single-line request logs
- Colorized status codes (2xx=green, 3xx=blue, 4xx/5xx=red)
- Request duration tracking
- Full URL path with query parameters
- Panic recovery with structured error logging
- Method and status code logging
- Request/response body logging on errors
- Request ID generation and propagation
- Path-based skip rules
- Audit integration
- Metrics collection
- Custom callbacks

### TCP Middleware

```go
package main

import (
    "net"
    "github.com/jozefvalachovic/logger/v4"
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
INFO GET /api/test [200] 15.234ms
WARN GET /api/notfound [404] 2.456ms
ERROR POST /api/error [500] 123.789ms
```

The middleware logs key request details (method, path, status, duration) in the log message for easy searching in log aggregation tools like GCP Cloud Logging and Grafana.

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
- `ConfigFromEnv() Config` — Config populated from environment variables
- `Shutdown(context.Context) error` — Graceful shutdown: drain buffers, flush, close
- `HealthCheck() error` — Verify logger subsystem health

### Core Logging Functions

- `Log(LogLevel, string, ...any)` — Main logging function (v1 compatible)
- `LogDebug(string, ...any)` — Debug level convenience function
- `LogInfo(string, ...any)` — Info level convenience function
- `LogWarn(string, ...any)` — Warn level convenience function
- `LogError(string, ...any)` — Error level convenience function
- `LogErrorWithStack(error, string, ...any)` — Error with type, chain, and stack trace
- `With(...any) Logger` — Create child logger with pre-set fields

### Conditional Functions

- `IfTrace(func())` — Run closure only if Trace level is active
- `IfDebug(func())` — Run closure only if Debug level is active
- `IfInfo(func())` — Run closure only if Info level is active
- `IfWarn(func())` — Run closure only if Warn level is active
- `IfError(func())` — Run closure only if Error level is active

### Context-Aware Functions

- `LogInfoWithContext(context.Context, string, ...any)` — Info with context

### HTTP Request Logging

- `LogHttpRequest(*http.Request)` — Logs HTTP request details

### HTTP / WebSocket Middleware

- `middleware.LogHTTPMiddleware(http.Handler, ...HTTPMiddlewareOption) http.Handler` — HTTP logging with panic recovery
- `middleware.LogWebSocketMiddleware(http.Handler, ...HTTPMiddlewareOption) http.Handler` — WebSocket lifecycle logging

### gRPC Helpers

- `middleware.LogGRPCUnary(ctx, fullMethod, handler, ...GRPCOption) (any, error)` — Unary interceptor
- `middleware.LogGRPCStream(ctx, fullMethod, handler, ...GRPCOption) error` — Stream interceptor

### Handlers

- `NewOTelBridgeHandler(slog.Handler, serviceName, version) *OTelBridgeHandler` — OTel level mapping
- `NewLevelFilterHandler(slog.Level, slog.Handler) *LevelFilterHandler` — Per-handler min level

### Metrics

- `MetricsHandler() http.Handler` — Prometheus text exposition endpoint

### Types

- `LogLevel` — Trace, Debug, Info, Notice, Warn, Error, Audit constants
- `Config` — Logger configuration struct
- `Logger` — Interface for dependency injection (includes `With`, `LogErrorWithStack`)

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
- **`examples/audit/`** — Security audit logging (legacy and enterprise)
- **`examples/http-middleware/`** — HTTP request logging middleware
- **`examples/advanced/`** — Sampling, async logging, metrics, and context

Run any example:

```bash
cd examples/basic && go run main.go
cd examples/audit && go run main.go  # Enterprise audit features
```

## Package Structure

```
logger/
├── main.go           # Core configuration and types
├── logger.go         # Logging functions, With(), IfDebug(), etc.
├── handler.go        # slog handler (redaction, caller, colorized JSON)
├── format.go         # Output formatting
├── convert.go        # Type conversion utilities
├── features.go       # Sampling, rotation, async, metrics, MetricsHandler
├── bridge.go         # OTelBridgeHandler, LevelFilterHandler
├── dedup.go          # Log deduplication manager
├── env.go            # Environment-aware defaults (ConfigFromEnv)
├── shutdown.go       # Graceful shutdown
├── health.go         # Health check
├── version.go        # Version information
├── audit/            # Enterprise audit package
│   ├── types.go      # Audit event types, schemas, CorrelationID
│   ├── config.go     # Audit configuration
│   ├── logger.go     # Audit logger implementation
│   ├── errors.go     # Typed errors (SinkError, WALError, StoreError)
│   ├── interfaces.go # Sink and Store interfaces
│   ├── chain.go      # Hash chain + Ed25519 signing
│   ├── wal.go        # Write-ahead log
│   ├── trace.go      # Distributed tracing
│   ├── ratelimit.go  # Token bucket rate limiter
│   ├── retention.go  # Retention policy management
│   ├── uuid.go       # UUID generation
│   ├── sink/         # Output sinks (file, webhook, multi, SSE)
│   └── store/        # Storage backends (memory, file, SQL, export)
├── middleware/        # HTTP/TCP/WebSocket/gRPC middleware
│   ├── http.go       # Core HTTP middleware (body sampling)
│   ├── websocket.go  # WebSocket lifecycle logging
│   ├── grpc.go       # gRPC interceptor helpers (zero-dep)
│   ├── options.go    # Functional options pattern
│   ├── metrics.go    # MetricsCollector interface
│   ├── helpers.go    # Internal helpers
│   └── tcp.go        # TCP middleware
└── examples/         # Runnable examples
```

## Migration from v3

v4 is fully backward compatible with v3. The new enterprise audit features are opt-in:

```go
// v3 code continues to work unchanged
logger.LogAudit("action", "login", "user", "alice")

// v4 adds optional enterprise audit
logger.SetConfig(logger.Config{
    Audit: &audit.Config{...}, // Enable enterprise features
})
logger.LogAuditEvent(ctx, audit.AuditEvent{...})

// v4 middleware now uses functional options (v3 boolean API still works)
// v3 style:
// loggedMux := middleware.LogHTTPMiddleware(mux, true)
// v4 style:
loggedMux := middleware.LogHTTPMiddleware(mux,
    middleware.WithLogBodyOnErrors(true),
    middleware.WithRequestID(true),
)
```

### v4.1.0 Changes

- **Go 1.26** — Requires Go 1.26+; leverages new standard library APIs throughout
- **Multi-handler output** — `AdditionalHandlers` config uses `slog.NewMultiHandler` for fan-out
- **Typed audit errors** — `SinkError`, `WALError`, `StoreError` with `errors.AsType[T]`
- **`errors.Join` multi-error** — Multi-sink and `Close()` preserve all errors
- **Child loggers** — `With()` creates scoped loggers with pre-set key-value fields
- **Caller attribution** — `EnableCaller` adds `[file:line]` to every log line
- **Graceful shutdown** — `Shutdown(ctx)` drains buffers, flushes dedup, closes audit
- **OTel bridge** — `OTelBridgeHandler` maps custom levels for OTel collectors
- **Regex redaction** — `RedactPatterns` redacts values matching regex patterns
- **Level filtering** — `LevelFilterHandler` gates per-handler minimum log levels
- **Error with stack** — `LogErrorWithStack()` logs error type, chain, and stack trace
- **Env-aware defaults** — `ConfigFromEnv()` reads `LOG_LEVEL`, `LOG_COLOR`, `LOG_CALLER`, etc.
- **gRPC helpers** — Zero-dep `LogGRPCUnary` / `LogGRPCStream` interceptor wrappers
- **Log deduplication** — `EnableDedup` / `DedupWindow` suppress repeated messages
- **Health check** — `HealthCheck()` verifies writer, buffer usage, audit state
- **Prometheus endpoint** — `MetricsHandler()` serves text exposition format
- **Ed25519 signing** — `EnableSignatures` / `PrivateKey` for audit entry non-repudiation
- **SQL store** — `SQLStore` for PostgreSQL, MySQL, SQLite audit storage
- **Body sampling** — `WithBodySampleRate()` for probabilistic HTTP body capture
- **Compact/colorized JSON** — `CompactJSON` / `ColorizeJSON` output modes
- **Conditional evaluation** — `IfDebug()`, `IfTrace()`, etc. skip expensive computations
- **WebSocket middleware** — `LogWebSocketMiddleware()` with message/byte tracking
- **SSE streaming** — `SSESink` with `Handler()` for real-time audit event streaming
- **Audit correlation ID** — `CorrelationID` field links related audit events
- **`io.LimitReader` safety** — `LogHttpRequest` respects `MaxBodySize`
- **`reflect.Value.Seq2`** — Map conversion uses Go 1.26 range-over iterator
- **`testing.T.ArtifactDir`** — File tests use `ArtifactDir()` with `-artifacts` flag
- **Goroutine leak detection** — Lifecycle test verifies background goroutine cleanup
- **CI improvements** — Concurrency control, minimal permissions, auto-tagging

### v4.0.3 Changes

- **CI simplified** — Now targets Go 1.26 with golangci-lint v2.8.0, auto-tags releases from `version.go`
- **golangci-lint v2 support** — Upgraded to golangci-lint-action@v8

### v4.0.1 Changes

- **Improved HTTP/TCP middleware log messages** — Key request details (method, path, status, duration) now appear in the log message for better searchability in GCP Cloud Logging, Grafana, and other log aggregation tools
- **Version alignment** — Package version now correctly reflects v4 module path

## Migration from v1

- The API remains compatible with v1 for basic logging.
- New features include structured logging, colorized output, context support, and HTTP middleware.

---

**For more examples and documentation, see the examples directory and tests.**
