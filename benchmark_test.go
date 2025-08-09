package logger

import (
	"io"
	"log/slog"
	"testing"
)

func BenchmarkLogInfo(b *testing.B) {
	SetConfig(Config{
		Output:      io.Discard, // Don't actually write
		Level:       slog.LevelInfo,
		EnableColor: false,
		TimeFormat:  "15:04:05",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		LogInfo("Benchmark test", "iteration", i, "data", "test")
	}
}
