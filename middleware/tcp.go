package middleware

import (
	"net"
	"time"

	"github.com/jozefvalachovic/logger/v3"
)

// LogTCPMiddleware logs when a TCP connection is started and ended, and recovers from panics
func LogTCPMiddleware(next func(conn net.Conn)) func(conn net.Conn) {
	return func(conn net.Conn) {
		start := time.Now()

		remoteAddr := conn.RemoteAddr().String()
		logger.LogTrace("TCP Connection Started", "remote", remoteAddr)

		defer func() {
			duration := time.Since(start).String()

			// Handle connection close errors
			if err := conn.Close(); err != nil {
				logger.LogError("TCP Connection close error",
					"__error", err,
					"remote", remoteAddr,
					"duration", duration,
				)
			}

			// Recover from panics with stack trace
			if r := recover(); r != nil {
				stack := logger.GetStackTrace()
				logger.LogError("TCP Panic recovered",
					"__error", r,
					"remote", remoteAddr,
					"stack", stack,
				)
			}

			logger.LogTrace("TCP Connection Ended", "remote", remoteAddr, "duration", duration)
		}()

		next(conn)
	}
}
