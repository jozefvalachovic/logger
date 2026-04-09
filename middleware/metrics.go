package middleware

import (
	"maps"
	"sync"
	"sync/atomic"
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
	mu               sync.RWMutex // protects maps only
	totalRequests    atomic.Int64
	totalErrors      atomic.Int64
	totalPanics      atomic.Int64
	totalDurationNs  atomic.Int64
	requestsByMethod map[string]int64
	requestsByStatus map[int]int64
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
	m.totalRequests.Add(1)
	m.totalDurationNs.Add(int64(duration))

	m.mu.Lock()
	m.requestsByMethod[method]++
	m.requestsByStatus[statusCode]++
	m.mu.Unlock()
}

// RecordError records an error metric
func (m *DefaultMetricsCollector) RecordError(method, path string, statusCode int) {
	m.totalErrors.Add(1)
}

// RecordPanic records a panic metric
func (m *DefaultMetricsCollector) RecordPanic(method, path string) {
	m.totalPanics.Add(1)
}

// GetMetrics returns the current metrics as a map
func (m *DefaultMetricsCollector) GetMetrics() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := m.totalRequests.Load()
	avgDuration := time.Duration(0)
	if total > 0 {
		avgDuration = time.Duration(m.totalDurationNs.Load()) / time.Duration(total)
	}

	// Copy maps to avoid race conditions
	methodsCopy := make(map[string]int64)
	maps.Copy(methodsCopy, m.requestsByMethod)

	statusCopy := make(map[int]int64)
	maps.Copy(statusCopy, m.requestsByStatus)

	return map[string]any{
		"total_requests":     total,
		"total_errors":       m.totalErrors.Load(),
		"total_panics":       m.totalPanics.Load(),
		"avg_duration":       avgDuration.String(),
		"requests_by_method": methodsCopy,
		"requests_by_status": statusCopy,
	}
}

// Reset resets all metrics
func (m *DefaultMetricsCollector) Reset() {
	m.totalRequests.Store(0)
	m.totalErrors.Store(0)
	m.totalPanics.Store(0)
	m.totalDurationNs.Store(0)

	m.mu.Lock()
	m.requestsByMethod = make(map[string]int64)
	m.requestsByStatus = make(map[int]int64)
	m.mu.Unlock()
}

// GetTotalRequests returns the total number of requests
func (m *DefaultMetricsCollector) GetTotalRequests() int64 {
	return m.totalRequests.Load()
}

// GetTotalErrors returns the total number of errors
func (m *DefaultMetricsCollector) GetTotalErrors() int64 {
	return m.totalErrors.Load()
}

// GetTotalPanics returns the total number of panics
func (m *DefaultMetricsCollector) GetTotalPanics() int64 {
	return m.totalPanics.Load()
}

// GetAverageDuration returns the average request duration
func (m *DefaultMetricsCollector) GetAverageDuration() time.Duration {
	total := m.totalRequests.Load()
	if total == 0 {
		return 0
	}
	return time.Duration(m.totalDurationNs.Load()) / time.Duration(total)
}

// GetErrorRate returns the error rate as a percentage
func (m *DefaultMetricsCollector) GetErrorRate() float64 {
	total := m.totalRequests.Load()
	if total == 0 {
		return 0
	}
	return float64(m.totalErrors.Load()) / float64(total) * 100
}
