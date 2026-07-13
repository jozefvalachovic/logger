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
	if dm := dedupMgr.Swap(nil); dm != nil {
		dm.Flush()
		dm.Stop()
	}

	// Close audit logger with context deadline awareness
	if al := auditLogger.Swap(nil); al != nil {
		auditDone := make(chan error, 1)
		go func() {
			auditDone <- al.Close()
		}()

		select {
		case err := <-auditDone:
			if err != nil {
				errs = append(errs, err)
			}
		case <-ctx.Done():
			errs = append(errs, ctx.Err())
		}
	}

	return errors.Join(errs...)
}
