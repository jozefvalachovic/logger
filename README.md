# Logger Package

A beautiful, configurable logger for Go with colorized output and support for structured logging.

## Features

- 🌈 Colorized log levels (Debug, Info, Warn, Error)
- 📊 Structured logging with key-value pairs
- ⚙️ Configurable output, log level, colors, and time format
- 🎯 Support for multiple data types (string, int, uint, float, bool)
- 🚀 High performance with singleton logger instance

## Installation

```bash
go get github.com/jozefvalachovic/logger
```

## Quick Start

````go
package main

import "github.com/jozefvalachovic/logger"

func main() {
    // Simple logging with main Log function
    logger.Log(logger.Info, "Application started")

    // Structured logging with key-value pairs
    logger.Log(logger.Info, "User login",
        "username", "john",
        "id", 123,
        "rate", 3.14,
        "active", true)

    // Using convenience functions (shorter syntax)
    logger.LogInfo("Application started")
    logger.LogError("Database connection failed", "error", "timeout")
    logger.LogDebug("Processing data", "records", 250)
    logger.LogWarn("Memory usage high", "usage", 85.5)
}
```## Configuration

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
````

## Logging Methods

### Main Log Function

```go
logger.Log(logger.Info, "User action", "username", "john", "action", "login")
```

### Convenience Functions

```go
logger.LogDebug("Debug message", "key", "value")
logger.LogInfo("Info message", "key", "value")
logger.LogWarn("Warning message", "key", "value")
logger.LogError("Error message", "key", "value")
```

## Log Levels

- `logger.Debug` - Purple (detailed debugging information)
- `logger.Info` - Blue (general information)
- `logger.Warn` - Yellow (warning conditions)
- `logger.Error` - Red (error conditions)

Colors are automatically applied when `EnableColor` is `true` (default).## Supported Data Types

The logger automatically handles:

- `string`
- `int`, `int8`, `int16`, `int32`, `int64`
- `uint`, `uint8`, `uint16`, `uint32`, `uint64`
- `float32`, `float64`
- `bool`
- Any other type (converted to string)

## Example Output

```
2025-08-08 14:30:15 INFO User login {
  "username": "john",
  "id": 123,
  "rate": 3.14,
  "active": true
}
```

## API Reference

### Configuration

- `SetConfig(Config)` - Configure logger settings
- `GetConfig() Config` - Get current configuration

### Logging Functions

- `Log(LogLevel, string, ...any)` - Main logging function
- `LogDebug(string, ...any)` - Debug level convenience function
- `LogInfo(string, ...any)` - Info level convenience function
- `LogWarn(string, ...any)` - Warn level convenience function
- `LogError(string, ...any)` - Error level convenience function

### Types

- `LogLevel` - Debug, Info, Warn, Error constants
- `Config` - Logger configuration struct
