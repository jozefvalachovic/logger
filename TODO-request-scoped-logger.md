# TODO: Request-Scoped Logger Support

**Goal:** Add `context.Context`-based logger storage so middleware can enrich a logger once and handlers retrieve it with `FromContext(ctx)`.

---

## 1. Add `NewContext` and `FromContext` to `logger.go`

- Define a private typed context key: `type loggerCtxKey struct{}`
- Add `NewContext(ctx context.Context, l Logger) context.Context` — stores `l` in the context via `context.WithValue`
- Add `FromContext(ctx context.Context) Logger` — retrieves the `Logger` from context; if absent, returns `DefaultLogger()` (never nil)
- Both functions are package-level (not methods)

## 2. Expand `LogInfoWithContext` into a full context-aware set

- Add context-aware variants for every level:
  - `LogWithContext(ctx, level, msg, kv...)`
  - `LogDebugWithContext(ctx, msg, kv...)`
  - `LogTraceWithContext(ctx, msg, kv...)`
  - `LogNoticeWithContext(ctx, msg, kv...)`
  - `LogWarnWithContext(ctx, msg, kv...)`
  - `LogErrorWithContext(ctx, msg, kv...)`
- Each should: call `FromContext(ctx)` to get the enriched logger, then delegate to its corresponding method
- The existing `LogInfoWithContext` should be refactored to use the same pattern (currently it manually extracts `trace_id` — that extraction becomes unnecessary once the middleware pre-enriches the logger via `With()`)
- Keep the old `TraceIDContextKey` extraction as a backward-compatible fallback inside `LogInfoWithContext` only; deprecate it with a doc comment

## 3. Add context-aware methods to the `Logger` interface

- Add to the `Logger` interface:
  ```go
  LogWithContext(ctx context.Context, level LogLevel, message string, keyValues ...any)
  LogDebugWithContext(ctx context.Context, message string, keyValues ...any)
  LogTraceWithContext(ctx context.Context, message string, keyValues ...any)
  LogNoticeWithContext(ctx context.Context, message string, keyValues ...any)
  LogWarnWithContext(ctx context.Context, message string, keyValues ...any)
  LogErrorWithContext(ctx context.Context, message string, keyValues ...any)
  ```
- Implement on `defaultLoggerImpl` and `childLogger`
- Each implementation: extract the logger from ctx via `FromContext`, merge its fields with the receiver's fields, and call `logInternal`

## 4. Tests

- `TestNewContextFromContext` — store a `With("requestId", "abc")` logger, retrieve it, verify fields appear in output
- `TestFromContext_NilFallback` — `FromContext(context.Background())` returns `DefaultLogger()`, not nil
- `TestLogErrorWithContext` — verify an Error-level log through context carries enriched fields
- `TestLogInfoWithContext_BackwardCompat` — verify old `TraceIDContextKey` extraction still works
- Race test: concurrent `FromContext` + `NewContext` on shared context

## 5. Update `middleware/http.go` (in the logger package)

- After the existing `context.WithValue(r.Context(), RequestIDKey, requestID)` line, create an enriched child logger:
  ```go
  child := DefaultLogger().With("requestId", requestID)
  ctx = NewContext(ctx, child)
  ```
- This makes the request-scoped logger available to all downstream handlers automatically

## 6. Documentation

- Add a doc comment on `NewContext` and `FromContext` with a usage example showing the middleware → handler flow
- Update `examples/advanced/main.go` to demonstrate the pattern

---

## Not in scope

The server package's `RequestId` / `TraceContext` middleware calling `logger.NewContext` — that wiring will be done in the server repo after this lands.
