package logger

import (
	"context"
	"errors"
)

// Shutdown gracefully shuts down the logger, flushing all buffers and closing resources.
// It respects the context deadline for timeout control.
func Shutdown(ctx context.Context) error {
	var errs []error

	// Stop async logger and drain buffers
	done := make(chan struct{})
	go func() {
		stopAsyncLogger()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		errs = append(errs, ctx.Err())
	}

	// Flush dedup summaries
	if dedupMgr != nil {
		dedupMgr.Flush()
		dedupMgr.Stop()
		dedupMgr = nil
	}

	// Close audit logger
	if auditLogger != nil {
		if err := auditLogger.Close(); err != nil {
			errs = append(errs, err)
		}
		auditLogger = nil
	}

	return errors.Join(errs...)
}
