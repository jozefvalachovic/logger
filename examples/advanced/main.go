package main

import (
	"context"
	"os"
	"time"

	"github.com/jozefvalachovic/logger/v3"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	requestIDKey contextKey = "request_id"
	userIDKey    contextKey = "user_id"
)

func main() {
	// Configure logger with advanced settings
	logger.SetConfig(logger.Config{
		Output:      os.Stdout,
		Level:       logger.LevelTrace,
		EnableColor: true,
		TimeFormat:  "15:04:05",

		// Enable sampling (log 50% of messages)
		SampleRate: 0.5,
		SampleSeed: 12345,

		// Enable async logging for better performance
		AsyncMode:    true,
		BufferSize:   1000,
		FlushTimeout: time.Second,

		// Enable metrics tracking
		EnableMetrics: true,
		MetricsPrefix: "myapp",
	})

	logger.LogInfo("Advanced features configured")

	// Context-aware logging
	ctx := context.WithValue(context.Background(), requestIDKey, "req-abc123")
	ctx = context.WithValue(ctx, userIDKey, "user456")

	logger.LogInfoWithContext(ctx, "User action", "action", "purchase")

	// Simulate some load
	for i := 0; i < 100; i++ {
		logger.LogDebug("Processing item", "item_id", i)
		logger.LogInfo("Item processed", "item_id", i, "status", "success")
		time.Sleep(10 * time.Millisecond)
	}

	// Get metrics
	stats := logger.GetMetrics()
	logger.LogInfo("Logger metrics", "stats", stats)

	logger.LogInfo("Advanced features demonstration complete")
}
