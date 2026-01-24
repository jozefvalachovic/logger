package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jozefvalachovic/logger/v4"
	"github.com/jozefvalachovic/logger/v4/audit"
)

func main() {
	fmt.Println("=== Logger Audit Examples ===")
	fmt.Println()

	// Example 1: Legacy audit logging (backward compatible)
	fmt.Println("1. Legacy Audit Logging (original API)")
	fmt.Println("----------------------------------------")
	legacyAuditExample()

	fmt.Println("\n2. Enterprise Audit Logging (new structured API)")
	fmt.Println("------------------------------------------------")
	enterpriseAuditExample()

	fmt.Println("\n3. Enterprise Audit with Compliance Preset")
	fmt.Println("-------------------------------------------")
	complianceAuditExample()
}

// legacyAuditExample demonstrates the original LogAudit API
// This continues to work exactly as before
func legacyAuditExample() {
	// User authentication - legacy style
	logger.LogAudit(
		"event", "user_authentication",
		"user_id", "user123",
		"method", "password",
		"success", true,
		"ip", "203.0.113.45",
		"timestamp", time.Now().Unix(),
	)

	// Data access - legacy style
	logger.LogAudit(
		"event", "data_access",
		"user_id", "user456",
		"resource", "customer_records",
		"action", "read",
		"record_count", 150,
		"timestamp", time.Now().Unix(),
	)

	// Permission change - legacy style
	logger.LogAudit(
		"event", "permission_change",
		"admin_id", "admin789",
		"target_user", "user123",
		"old_role", "user",
		"new_role", "admin",
		"reason", "promotion",
		"timestamp", time.Now().Unix(),
	)

	fmt.Println("✓ Legacy audit logs written successfully")
}

// enterpriseAuditExample demonstrates the new structured audit API
func enterpriseAuditExample() {
	ctx := context.Background()

	// Configure enterprise audit (optional - works without this too)
	auditConfig := audit.DefaultConfig()
	auditConfig.EnableStructured = true
	auditConfig.Service = audit.NewServiceContextFromEnv()
	auditConfig.Service.Name = "user-service"
	auditConfig.Service.Version = "1.0.0"
	auditConfig.Service.Environment = "development"

	// Note: You can configure the main logger with audit config
	// logger.SetConfig(logger.Config{
	//     Audit: &auditConfig,
	// })

	// Or use the audit package directly for more control
	auditLogger, err := audit.New(auditConfig)
	if err != nil {
		fmt.Printf("Failed to create audit logger: %v\n", err)
		return
	}
	defer auditLogger.Close()

	// Example 1: User authentication event
	authEvent := audit.AuditEvent{
		Type:    audit.AuditAuth,
		Action:  "login",
		Outcome: audit.OutcomeSuccess,
		Actor: audit.AuditActor{
			ID:        "user123",
			Type:      "user",
			Name:      "John Doe",
			IP:        "203.0.113.45",
			UserAgent: "Mozilla/5.0...",
			SessionID: "sess_abc123",
		},
		Description: "User successfully logged in via password",
		Metadata: map[string]any{
			"mfa_used":     true,
			"login_method": "password",
		},
	}

	if err := auditLogger.Log(ctx, authEvent); err != nil {
		fmt.Printf("Failed to log auth event: %v\n", err)
	}

	// Example 2: Data access event
	dataAccessEvent := audit.AuditEvent{
		Type:    audit.AuditDataAccess,
		Action:  "read",
		Outcome: audit.OutcomeSuccess,
		Actor: audit.AuditActor{
			ID:   "user123",
			Type: "user",
			IP:   "203.0.113.45",
		},
		Resource: &audit.AuditResource{
			ID:   "customer_records",
			Type: "database_table",
			Name: "Customer Records",
			Metadata: map[string]any{
				"database": "production",
				"schema":   "public",
			},
		},
		Description: "User accessed customer records",
		Metadata: map[string]any{
			"record_count": 150,
			"query_type":   "select",
		},
	}

	if err := auditLogger.Log(ctx, dataAccessEvent); err != nil {
		fmt.Printf("Failed to log data access event: %v\n", err)
	}

	// Example 3: Admin action with changes
	adminEvent := audit.AuditEvent{
		Type:    audit.AuditAdminAction,
		Action:  "role_change",
		Outcome: audit.OutcomeSuccess,
		Actor: audit.AuditActor{
			ID:   "admin789",
			Type: "admin",
			Name: "Admin User",
		},
		Resource: &audit.AuditResource{
			ID:   "user123",
			Type: "user",
			Name: "John Doe",
		},
		Changes: &audit.AuditChanges{
			Before: map[string]any{"role": "user"},
			After:  map[string]any{"role": "admin"},
		},
		Reason:      "Promotion to team lead",
		Description: "Changed user role from user to admin",
	}

	if err := auditLogger.Log(ctx, adminEvent); err != nil {
		fmt.Printf("Failed to log admin event: %v\n", err)
	}

	// Example 4: Failed authorization attempt
	authzFailEvent := audit.AuditEvent{
		Type:    audit.AuditAuthz,
		Action:  "access_denied",
		Outcome: audit.OutcomeDenied,
		Actor: audit.AuditActor{
			ID:   "user456",
			Type: "user",
			IP:   "192.168.1.100",
		},
		Resource: &audit.AuditResource{
			ID:   "admin_panel",
			Type: "page",
		},
		Description: "User attempted to access admin panel without permission",
		Metadata: map[string]any{
			"required_role": "admin",
			"user_role":     "user",
		},
	}

	if err := auditLogger.Log(ctx, authzFailEvent); err != nil {
		fmt.Printf("Failed to log authz event: %v\n", err)
	}

	fmt.Println("✓ Enterprise audit logs written successfully")

	// Print stats
	stats := auditLogger.GetStats()
	fmt.Printf("  - Sequence: %d\n", stats.Sequence)
	fmt.Printf("  - Buffer size: %d\n", stats.BufferSize)
}

// complianceAuditExample demonstrates using compliance presets
func complianceAuditExample() {
	ctx := context.Background()

	// Create SOC2-compliant audit logger
	soc2Config := audit.DefaultConfig()
	soc2Config.WithCompliance(audit.ComplianceSOC2)
	soc2Config.Service = &audit.ServiceContext{
		Name:        "payment-service",
		Version:     "2.1.0",
		Environment: "production",
		Region:      "us-east-1",
	}

	// In production, you'd configure file sinks, WAL paths, etc.
	// For this example, we'll use stdout
	soc2Config.WAL.Enabled = false // Disable WAL for demo

	auditLogger, err := audit.New(soc2Config)
	if err != nil {
		fmt.Printf("Failed to create SOC2 audit logger: %v\n", err)
		return
	}
	defer auditLogger.Close()

	// Log a PCI-DSS relevant event (payment processing)
	paymentEvent := audit.AuditEvent{
		Type:    audit.AuditDataModify,
		Action:  "process_payment",
		Outcome: audit.OutcomeSuccess,
		Actor: audit.AuditActor{
			ID:   "service_account",
			Type: "service",
			Name: "Payment Processor",
		},
		Resource: &audit.AuditResource{
			ID:   "txn_123456789",
			Type: "transaction",
			Metadata: map[string]any{
				"amount":   99.99,
				"currency": "USD",
				// Note: Never log full card numbers!
				"card_last_four": "4242",
			},
		},
		Description: "Payment transaction processed successfully",
		Metadata: map[string]any{
			"processor":   "stripe",
			"merchant_id": "merch_abc",
			"risk_score":  0.12,
		},
	}

	// Use sync logging for guaranteed delivery of sensitive events
	if err := auditLogger.LogSync(ctx, paymentEvent); err != nil {
		fmt.Printf("Failed to log payment event: %v\n", err)
	}

	fmt.Println("✓ SOC2-compliant audit logs written successfully")
	fmt.Printf("  - Hash chain enabled: %v\n", soc2Config.HashChain.Enabled)
	fmt.Printf("  - Structured format: %v\n", soc2Config.EnableStructured)
	if soc2Config.Retention != nil {
		fmt.Printf("  - Retention: %v\n", soc2Config.Retention.MaxAge)
	}
}

// Example: Using with the main logger (integrated approach)
func integratedExample() {
	ctx := context.Background()

	// Configure main logger with enterprise audit
	auditCfg := audit.DefaultConfig()
	auditCfg.EnableStructured = true
	auditCfg.Service = &audit.ServiceContext{
		Name:        "my-service",
		Version:     "1.0.0",
		Environment: os.Getenv("ENVIRONMENT"),
	}

	logger.SetConfig(logger.Config{
		Output:      os.Stdout,
		EnableColor: true,
		Audit:       &auditCfg,
	})

	// Now you can use LogAuditEvent through the main logger
	event := audit.AuditEvent{
		Type:    audit.AuditAPIAccess,
		Action:  "api_call",
		Outcome: audit.OutcomeSuccess,
		Actor: audit.AuditActor{
			ID:   "api_client_123",
			Type: "api_client",
		},
		Resource: &audit.AuditResource{
			ID:   "/api/v1/users",
			Type: "endpoint",
		},
	}

	// This will use enterprise audit if configured, or fallback to legacy
	logger.LogAuditEvent(ctx, event)

	// Legacy API still works
	logger.LogAudit("event", "simple_audit", "key", "value")

	// Regular logging is unaffected
	logger.LogInfo("This is a regular info log")
}
