package main

import (
	"context"
	"os"
	"time"

	"github.com/jozefvalachovic/logger/v4"
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

	// Request-scoped logger via context
	// Simulate what middleware does: store an enriched logger in context
	child := logger.DefaultLogger().With("requestId", "req-abc123", "userId", "user456")
	ctx := logger.NewContext(context.Background(), child)

	// Downstream handler retrieves the enriched logger automatically
	l := logger.FromContext(ctx)
	l.LogInfo("User action", "action", "purchase")

	// Or use the package-level context-aware helpers
	logger.LogWarnWithContext(ctx, "Inventory low", "item", "widget")
	logger.LogErrorWithContext(ctx, "Payment failed", "reason", "timeout")

	// Simulate some load
	for i := range 100 {
		logger.LogDebug("Processing item", "item_id", i)
		logger.LogInfo("Item processed", "item_id", i, "status", "success")
		time.Sleep(10 * time.Millisecond)
	}

	// Get metrics
	stats := logger.GetMetrics()
	logger.LogInfo("Logger metrics", "stats", stats)

	logger.LogInfo("Advanced features demonstration complete")
}
