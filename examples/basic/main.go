package main

import (
	"github.com/jozefvalachovic/logger/v3"
)

func main() {
	// Demonstrate all log levels
	logger.LogTrace("Application started", "module", "main")
	logger.LogDebug("Debug information", "config_loaded", true)
	logger.LogInfo("User action completed", "user_id", 123)
	logger.LogNotice("System event occurred", "event_type", "startup")
	logger.LogWarn("Resource usage high", "cpu_percent", 85)
	logger.LogError("Operation failed", "error", "connection timeout")

	// Audit logging (key-value pairs only, no message)
	logger.LogAudit(
		"action", "user_login",
		"user_id", "789",
		"ip_address", "192.168.1.100",
		"success", true,
	)
}
