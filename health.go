package logger

import (
	"errors"
	"fmt"
)

func HealthCheck() error {
	var errs []error

	cfg := *globalConfig.Load()

	if cfg.Output == nil {
		errs = append(errs, fmt.Errorf("health: output writer is nil"))
	}

	if cfg.AsyncMode && asyncRunning {
		asyncMu.Lock()
		chanLen := len(logChan)
		chanCap := cap(logChan)
		asyncMu.Unlock()
		if chanCap > 0 {
			usage := float64(chanLen) / float64(chanCap)
			if usage > 0.9 {
				errs = append(errs, fmt.Errorf("health: async buffer %.0f%% full (%d/%d)", usage*100, chanLen, chanCap))
			}
		}
	}

	if auditLogger != nil {
		stats := auditLogger.GetStats()
		if stats.Closed {
			errs = append(errs, fmt.Errorf("health: audit logger is closed"))
		}
		if stats.BufferSize > 0 {
			usage := float64(stats.BufferUsed) / float64(stats.BufferSize)
			if usage > 0.9 {
				errs = append(errs, fmt.Errorf("health: audit buffer %.0f%% full", usage*100))
			}
		}
	}

	return errors.Join(errs...)
}
