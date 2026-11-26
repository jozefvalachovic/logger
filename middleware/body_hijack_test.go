package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jozefvalachovic/logger/v4"
)

func setupMiddlewareLog(t *testing.T, maxBody int64) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	logger.SetConfig(logger.Config{
		Output: &buf, Level: logger.LevelTrace, LevelSet: true,
		CompactJSON: true, TimeFormat: "15:04:05", MaxBodySize: maxBody,
	})
	t.Cleanup(func() { logger.SetConfig(logger.Config{Output: io.Discard}) })
	return &buf
}

func TestBodyCaptureDoesNotTruncateHandlerBody(t *testing.T) {
	setupMiddlewareLog(t, 16)
	body := strings.Repeat("A", 1000)
	var seen string
	h := LogHTTPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		seen = string(b)
	}), WithLogBodyOnErrors(true))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/upload", strings.NewReader(body)))

	if seen != body {
		t.Fatalf("handler received %d of %d body bytes", len(seen), len(body))
	}
}

// The body is still logged on error when the handler never read it.
func TestBodyLoggedOnErrorWhenHandlerDidNotRead(t *testing.T) {
	buf := setupMiddlewareLog(t, 1024)
	h := LogHTTPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}), WithLogBodyOnErrors(true))

	req := httptest.NewRequest("POST", "/items", strings.NewReader(`{"name":"widget"}`))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(httptest.NewRecorder(), req)

	if !strings.Contains(buf.String(), `"body.name":"widget"`) {
		t.Fatalf("request body not logged:\n%s", buf.String())
	}
}

// A body larger than MaxBodySize is logged truncated with "...", as before.
func TestBodyLoggedTruncatedOnError(t *testing.T) {
	buf := setupMiddlewareLog(t, 8)
	h := LogHTTPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusInternalServerError)
	}), WithLogBodyOnErrors(true))

	req := httptest.NewRequest("POST", "/items", strings.NewReader("0123456789ABCDEF"))
	req.Header.Set("Content-Type", "text/plain")
	h.ServeHTTP(httptest.NewRecorder(), req)

	if !strings.Contains(buf.String(), `"request_body":"01234567..."`) {
		t.Fatalf("want truncated body in log:\n%s", buf.String())
	}
}

// syncBuffer is a log destination safe to read while a server goroutine
// writes to it. After a hijack, httptest.Server.Close no longer waits for the
// handler, so its final log line can arrive after the test's request returns.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// waitFor polls until s appears in b or the deadline passes.
func (b *syncBuffer) waitFor(s string, d time.Duration) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if strings.Contains(b.String(), s) {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

func TestMiddlewareSupportsHijack(t *testing.T) {
	buf := &syncBuffer{}
	logger.SetConfig(logger.Config{Output: buf, Level: logger.LevelTrace, LevelSet: true, CompactJSON: true, TimeFormat: "15:04:05"})
	t.Cleanup(func() { logger.SetConfig(logger.Config{Output: io.Discard}) })
	cases := map[string]func(http.Handler) http.Handler{
		"http":      func(h http.Handler) http.Handler { return LogHTTPMiddleware(h) },
		"websocket": func(h http.Handler) http.Handler { return LogWebSocketMiddleware(h) },
	}
	for name, mw := range cases {
		t.Run(name, func(t *testing.T) {
			var hijackErr error
			srv := httptest.NewServer(mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, rw, err := http.NewResponseController(w).Hijack()
				hijackErr = err
				if err == nil {
					_, _ = rw.WriteString("HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n")
					_ = rw.Flush()
					_ = conn.Close()
				}
			})))
			defer srv.Close()

			req, _ := http.NewRequest("GET", srv.URL+"/ws", nil)
			req.Header.Set("Connection", "Upgrade")
			req.Header.Set("Upgrade", "websocket")
			if resp, err := http.DefaultClient.Do(req); err == nil {
				_ = resp.Body.Close()
			}
			if hijackErr != nil {
				t.Fatalf("Hijack through %s middleware: %v", name, hijackErr)
			}
		})
	}
	if !buf.waitFor("GET /ws [101]", 2*time.Second) {
		t.Errorf("want hijacked request logged as 101:\n%s", buf.String())
	}
}

// Optional interfaces are reachable through http.ResponseController.
func TestMiddlewareUnwrap(t *testing.T) {
	setupMiddlewareLog(t, 1024)
	var unwrapped bool
	h := LogHTTPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, unwrapped = w.(interface{ Unwrap() http.ResponseWriter })
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	if !unwrapped {
		t.Fatal("wrapped writer does not implement Unwrap")
	}
}

// A handler that closes the body early must not cause a read-error log line.
func TestBodyClosedByHandlerIsNotReadAgain(t *testing.T) {
	buf := setupMiddlewareLog(t, 1024)
	h := LogHTTPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := make([]byte, 4)
		_, _ = r.Body.Read(p)
		_ = r.Body.Close()
		w.WriteHeader(http.StatusInternalServerError)
	}), WithLogBodyOnErrors(true))

	req := httptest.NewRequest("POST", "/items", strings.NewReader("abcdefgh"))
	req.Header.Set("Content-Type", "text/plain")
	h.ServeHTTP(httptest.NewRecorder(), req)

	out := buf.String()
	if strings.Contains(out, "Failed to read") {
		t.Fatalf("unexpected read error after handler closed the body:\n%s", out)
	}
	if !strings.Contains(out, `"request_body":"abcd"`) {
		t.Errorf("want the part the handler read to be logged:\n%s", out)
	}
}
