package middleware

import (
	"fmt"
	"net"
	"time"

	"github.com/jozefvalachovic/logger/v4"
)

// LogTCPMiddleware logs when a TCP connection is started and ended, and recovers from panics
func LogTCPMiddleware(next func(conn net.Conn)) func(conn net.Conn) {
	return func(conn net.Conn) {
		start := time.Now()

		remoteAddr := conn.RemoteAddr().String()
		logger.LogTrace(fmt.Sprintf("TCP Connection Started %s", remoteAddr), "remote", remoteAddr)

		defer func() {
			duration := time.Since(start).String()

			// Recover from panics with stack trace (check this first)
			if r := recover(); r != nil {
				stack := logger.GetStackTrace()
				logger.LogError(fmt.Sprintf("TCP Panic recovered %s %s", remoteAddr, duration),
					"__error", r,
					"remote", remoteAddr,
					"duration", duration,
					"stack", stack,
				)
				// Still try to close connection after panic
				conn.Close()
				return
			}

			// Handle connection close errors
			if err := conn.Close(); err != nil {
				logger.LogError(fmt.Sprintf("TCP Connection close error %s %s", remoteAddr, duration),
					"__error", err,
					"remote", remoteAddr,
					"duration", duration,
				)
				return
			}

			logger.LogTrace(fmt.Sprintf("TCP Connection Ended %s %s", remoteAddr, duration), "remote", remoteAddr, "duration", duration)
		}()

		next(conn)
	}
}
