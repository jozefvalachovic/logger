package middleware

import (
	"maps"
	"sync"
	"time"
)

// MetricsCollector interface for custom metrics implementations
type MetricsCollector interface {
	RecordRequest(method, path string, statusCode int, duration time.Duration)
	RecordError(method, path string, statusCode int)
	RecordPanic(method, path string)
}

// DefaultMetricsCollector provides basic metrics collection
type DefaultMetricsCollector struct {
	mu               sync.RWMutex
	totalRequests    int64
	totalErrors      int64
	totalPanics      int64
	requestsByMethod map[string]int64
	requestsByStatus map[int]int64
	totalDuration    time.Duration
}

// NewDefaultMetricsCollector creates a new default metrics collector
func NewDefaultMetricsCollector() *DefaultMetricsCollector {
	return &DefaultMetricsCollector{
		requestsByMethod: make(map[string]int64),
		requestsByStatus: make(map[int]int64),
	}
}

// RecordRequest records a request metric
func (m *DefaultMetricsCollector) RecordRequest(method, path string, statusCode int, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalRequests++
	m.requestsByMethod[method]++
	m.requestsByStatus[statusCode]++
	m.totalDuration += duration
}

// RecordError records an error metric
func (m *DefaultMetricsCollector) RecordError(method, path string, statusCode int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalErrors++
}

// RecordPanic records a panic metric
func (m *DefaultMetricsCollector) RecordPanic(method, path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalPanics++
}

// GetMetrics returns the current metrics as a map
func (m *DefaultMetricsCollector) GetMetrics() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	avgDuration := time.Duration(0)
	if m.totalRequests > 0 {
		avgDuration = m.totalDuration / time.Duration(m.totalRequests)
	}

	// Copy maps to avoid race conditions
	methodsCopy := make(map[string]int64)
	maps.Copy(methodsCopy, m.requestsByMethod)

	statusCopy := make(map[int]int64)
	maps.Copy(statusCopy, m.requestsByStatus)

	return map[string]any{
		"total_requests":     m.totalRequests,
		"total_errors":       m.totalErrors,
		"total_panics":       m.totalPanics,
		"avg_duration":       avgDuration.String(),
		"requests_by_method": methodsCopy,
		"requests_by_status": statusCopy,
	}
}

// Reset resets all metrics
func (m *DefaultMetricsCollector) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalRequests = 0
	m.totalErrors = 0
	m.totalPanics = 0
	m.totalDuration = 0
	m.requestsByMethod = make(map[string]int64)
	m.requestsByStatus = make(map[int]int64)
}

// GetTotalRequests returns the total number of requests
func (m *DefaultMetricsCollector) GetTotalRequests() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.totalRequests
}

// GetTotalErrors returns the total number of errors
func (m *DefaultMetricsCollector) GetTotalErrors() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.totalErrors
}

// GetTotalPanics returns the total number of panics
func (m *DefaultMetricsCollector) GetTotalPanics() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.totalPanics
}

// GetAverageDuration returns the average request duration
func (m *DefaultMetricsCollector) GetAverageDuration() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.totalRequests == 0 {
		return 0
	}
	return m.totalDuration / time.Duration(m.totalRequests)
}

// GetErrorRate returns the error rate as a percentage
func (m *DefaultMetricsCollector) GetErrorRate() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.totalRequests == 0 {
		return 0
	}
	return float64(m.totalErrors) / float64(m.totalRequests) * 100
}
