package logger

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"
)

type sleepWriter struct{ d time.Duration }

func (w sleepWriter) Write(p []byte) (int, error) {
	time.Sleep(w.d)
	return len(p), nil
}

// Stopping async mode while other goroutines are logging must not race or
// panic with "send on closed channel"; late lines fall back to sync writes.
func TestAsyncStopWhileLogging(t *testing.T) {
	t.Cleanup(func() { SetConfig(defaultTestConfig) })
	for range 10 {
		SetConfig(Config{Output: sleepWriter{100 * time.Microsecond}, AsyncMode: true, BufferSize: 16, CompactJSON: true})
		var wg sync.WaitGroup
		stop := make(chan struct{})
		for range 8 {
			wg.Go(func() {
				for {
					select {
					case <-stop:
						return
					default:
						LogInfo("async line", "k", 1)
					}
				}
			})
		}
		time.Sleep(2 * time.Millisecond)
		SetConfig(Config{Output: io.Discard}) // stops async while writers run
		close(stop)
		wg.Wait()
	}
}

// Shutdown while the worker is still writing its last entry must not crash.
func TestAsyncShutdownWithEntryInFlight(t *testing.T) {
	t.Cleanup(func() { SetConfig(defaultTestConfig) })
	for range 10 {
		SetConfig(Config{Output: sleepWriter{3 * time.Millisecond}, AsyncMode: true, BufferSize: 8, CompactJSON: true})
		LogInfo("last entry")
		time.Sleep(time.Millisecond) // worker is now inside the slow Write
		if err := Shutdown(context.Background()); err != nil {
			t.Fatal(err)
		}
		SetConfig(Config{Output: io.Discard})
	}
}

// Every line queued before the async logger stops is written.
func TestAsyncStopDrainsQueue(t *testing.T) {
	t.Cleanup(func() { SetConfig(defaultTestConfig) })
	var mu sync.Mutex
	lines := 0
	w := writerFunc(func(p []byte) (int, error) {
		mu.Lock()
		lines++
		mu.Unlock()
		return len(p), nil
	})
	SetConfig(Config{Output: w, AsyncMode: true, BufferSize: 1000, CompactJSON: true})
	for range 500 {
		LogInfo("queued")
	}
	SetConfig(Config{Output: io.Discard})
	mu.Lock()
	defer mu.Unlock()
	if lines != 500 {
		t.Fatalf("want 500 lines written, got %d", lines)
	}
}

type writerFunc func(p []byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }
