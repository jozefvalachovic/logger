package logger

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// ConfigFromEnv returns a Config populated from environment variables.
// Recognized variables:
//   - LOG_LEVEL: trace, debug, info, notice, warn, error, audit
//   - LOG_COLOR: true, false, 1, 0
//   - LOG_CALLER: true, false, 1, 0
//   - LOG_FORMAT: compact (sets CompactJSON)
//   - LOG_REDACT_KEYS: comma-separated additional keys to redact
func ConfigFromEnv() Config {
	cfg := defaultConfig
	applyEnvOverrides(&cfg)
	return cfg
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.Level = parseLevelString(v)
		cfg.LevelSet = true
	}
	if v := os.Getenv("LOG_COLOR"); v != "" {
		cfg.EnableColor = parseBoolEnv(v)
	}
	if v := os.Getenv("LOG_CALLER"); v != "" {
		cfg.EnableCaller = parseBoolEnv(v)
	}
	if v := os.Getenv("LOG_FORMAT"); v != "" {
		switch strings.ToLower(v) {
		case "compact", "json":
			cfg.CompactJSON = true
		}
	}
	if v := os.Getenv("LOG_REDACT_KEYS"); v != "" {
		keys := strings.Split(v, ",")
		for i := range keys {
			keys[i] = strings.TrimSpace(keys[i])
		}
		cfg.RedactKeys = append(cfg.RedactKeys, keys...)
	}
}

// ParseLevel converts a level name (trace, debug, info, notice, warn, warning,
// error, audit; case-insensitive) to its slog.Level and reports unknown names.
func ParseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "trace":
		return LevelTrace, nil
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "notice":
		return LevelNotice, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	case "audit":
		return LevelAudit, nil
	default:
		return slog.LevelInfo, fmt.Errorf("logger: unknown level %q", s)
	}
}

// parseLevelString keeps the lenient LOG_LEVEL behavior: unknown names fall back to Info.
func parseLevelString(s string) slog.Level {
	level, _ := ParseLevel(s)
	return level
}

func parseBoolEnv(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "1", "yes", "on":
		return true
	default:
		return false
	}
}
