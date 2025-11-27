package main

import (
	"time"

	"github.com/jozefvalachovic/logger/v3"
)

func main() {
	// Security audit logging examples

	// User authentication
	logger.LogAudit(
		"event", "user_authentication",
		"user_id", "user123",
		"method", "password",
		"success", true,
		"ip", "203.1.013.45",
		"timestamp", time.Now().Unix(),
	)

	// Data access
	logger.LogAudit(
		"event", "data_access",
		"user_id", "user456",
		"resource", "customer_records",
		"action", "read",
		"record_count", 150,
		"timestamp", time.Now().Unix(),
	)

	// Permission change
	logger.LogAudit(
		"event", "permission_change",
		"admin_id", "admin789",
		"target_user", "user123",
		"old_role", "user",
		"new_role", "admin",
		"reason", "promotion",
		"timestamp", time.Now().Unix(),
	)

	// System configuration change
	logger.LogAudit(
		"event", "config_change",
		"admin_id", "admin001",
		"setting", "max_login_attempts",
		"old_value", 3,
		"new_value", 5,
		"timestamp", time.Now().Unix(),
	)
}
