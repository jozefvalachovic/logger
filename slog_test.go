package logger

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
)

// captureHandler records the attributes each handler in the fan-out receives.
type captureHandler struct {
	mu      sync.Mutex
	records []map[string]any
	bound   []slog.Attr
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *captureHandler) Handle(_ context.Context, record slog.Record) error {
	fields := make(map[string]any)
	mergeAttrsInto(fields, h.bound)
	attrs := make([]slog.Attr, 0, record.NumAttrs())
	record.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, a)
		return true
	})
	mergeAttrsInto(fields, attrs)

	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, fields)
	return nil
}

func (h *captureHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &captureHandler{bound: append(append([]slog.Attr{}, h.bound...), attrs...)}
}

func (h *captureHandler) WithGroup(string) slog.Handler { return h }

func (h *captureHandler) snapshot() []map[string]any {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]map[string]any(nil), h.records...)
}

func TestSlogWritesThroughPipeline(t *testing.T) {
	buf := &bytes.Buffer{}
	SetConfig(Config{
		Output:      buf,
		Level:       LevelInfo,
		EnableColor: false,
		CompactJSON: true,
		TimeFormat:  "15:04:05",
	})
	t.Cleanup(func() { SetConfig(defaultTestConfig) })

	Slog().Info("external write", "service", "model-router")

	output := buf.String()
	if !strings.Contains(output, "external write") {
		t.Fatalf("expected message in pipeline output, got %q", output)
	}
	if !strings.Contains(output, `"service":"model-router"`) {
		t.Errorf("expected attribute in pipeline output, got %q", output)
	}
}

func TestSlogRespectsLevel(t *testing.T) {
	buf := &bytes.Buffer{}
	SetConfig(Config{
		Output:      buf,
		Level:       LevelWarn,
		LevelSet:    true,
		EnableColor: false,
		TimeFormat:  "15:04:05",
	})
	t.Cleanup(func() { SetConfig(defaultTestConfig) })

	log := Slog()
	log.Debug("should be dropped")
	log.Info("should be dropped too")
	log.Warn("should be kept")

	output := buf.String()
	if strings.Contains(output, "should be dropped") {
		t.Errorf("records below the configured level must be dropped, got %q", output)
	}
	if !strings.Contains(output, "should be kept") {
		t.Errorf("records at the configured level must be written, got %q", output)
	}
}

func TestSlogTracksSetConfig(t *testing.T) {
	first := &bytes.Buffer{}
	SetConfig(Config{
		Output:      first,
		Level:       LevelInfo,
		EnableColor: false,
		CompactJSON: true,
		TimeFormat:  "15:04:05",
	})
	t.Cleanup(func() { SetConfig(defaultTestConfig) })

	// Capture once, the way a consumer wires a dependency at startup.
	log := Slog().With("component", "router")
	log.Info("before reconfigure")

	second := &bytes.Buffer{}
	SetConfig(Config{
		Output:      second,
		Level:       LevelInfo,
		EnableColor: false,
		CompactJSON: true,
		TimeFormat:  "15:04:05",
	})
	log.Info("after reconfigure")

	if !strings.Contains(first.String(), "before reconfigure") {
		t.Errorf("first write should land in the original output, got %q", first.String())
	}
	if strings.Contains(first.String(), "after reconfigure") {
		t.Errorf("cached logger must not keep writing to the replaced output")
	}
	out := second.String()
	if !strings.Contains(out, "after reconfigure") {
		t.Errorf("cached logger should follow SetConfig, got %q", out)
	}
	if !strings.Contains(out, `"component":"router"`) {
		t.Errorf("bound attrs should be replayed onto the new handler, got %q", out)
	}
}

func TestSlogRedactsKeysAndPatterns(t *testing.T) {
	buf := &bytes.Buffer{}
	SetConfig(Config{
		Output:         buf,
		Level:          LevelInfo,
		EnableColor:    false,
		TimeFormat:     "15:04:05",
		RedactKeys:     []string{"password", "token"},
		RedactMask:     "[REDACTED]",
		RedactPatterns: []string{`sk-[a-z0-9]+`},
	})
	t.Cleanup(func() { SetConfig(defaultTestConfig) })

	log := Slog().With("token", "bound-secret")
	log.Info("secrets",
		"password", "hunter2",
		"api", "sk-livekey123",
		slog.Group("auth", "token", "grouped-secret", "user", "john"),
	)

	output := buf.String()
	for _, secret := range []string{"bound-secret", "hunter2", "sk-livekey123", "grouped-secret"} {
		if strings.Contains(output, secret) {
			t.Errorf("%q should have been redacted, got %q", secret, output)
		}
	}
	if !strings.Contains(output, "john") {
		t.Errorf("non-sensitive group fields should survive, got %q", output)
	}
}

func TestRedactionAppliesToAdditionalHandlers(t *testing.T) {
	capture := &captureHandler{}
	buf := &bytes.Buffer{}
	SetConfig(Config{
		Output:             buf,
		Level:              LevelInfo,
		EnableColor:        false,
		TimeFormat:         "15:04:05",
		RedactKeys:         []string{"password"},
		RedactMask:         "[REDACTED]",
		RedactPatterns:     []string{`sk-[a-z0-9]+`},
		AdditionalHandlers: []slog.Handler{capture},
	})
	t.Cleanup(func() { SetConfig(defaultTestConfig) })

	LogInfo("internal path", "password", "internal-secret", "api", "sk-internal1")
	Slog().Info("external path", "password", "external-secret", "api", "sk-external1")

	records := capture.snapshot()
	if len(records) != 2 {
		t.Fatalf("expected 2 records to reach the additional handler, got %d", len(records))
	}
	for _, fields := range records {
		if fields["password"] != "[REDACTED]" {
			t.Errorf("additional handler received unredacted key: %v", fields["password"])
		}
		if fields["api"] != "[REDACTED]" {
			t.Errorf("additional handler received unredacted pattern match: %v", fields["api"])
		}
	}
}

func TestRedactionMatchesNamespacedKeys(t *testing.T) {
	buf := &bytes.Buffer{}
	SetConfig(Config{
		Output:      buf,
		Level:       LevelInfo,
		EnableColor: false,
		TimeFormat:  "15:04:05",
		RedactKeys:  []string{"password"},
		RedactMask:  "[REDACTED]",
	})
	t.Cleanup(func() { SetConfig(defaultTestConfig) })

	LogInfo("body capture", "body.password", "body-secret", "body.username", "john")

	output := buf.String()
	if strings.Contains(output, "body-secret") {
		t.Errorf("namespaced sensitive keys should be redacted, got %q", output)
	}
	if !strings.Contains(output, "john") {
		t.Errorf("non-sensitive body fields should survive, got %q", output)
	}
}

func TestPrettyHandlerWithAttrsAndGroups(t *testing.T) {
	buf := &bytes.Buffer{}
	SetConfig(Config{
		Output:      buf,
		Level:       LevelInfo,
		EnableColor: false,
		CompactJSON: true,
		TimeFormat:  "15:04:05",
	})
	t.Cleanup(func() { SetConfig(defaultTestConfig) })

	Slog().With("service", "router").WithGroup("request").Info("handled", "method", "GET")

	output := buf.String()
	if !strings.Contains(output, `"service":"router"`) {
		t.Errorf("bound attrs should be formatted by the pretty handler, got %q", output)
	}
	if !strings.Contains(output, `"request":{"method":"GET"}`) {
		t.Errorf("groups should nest in the pretty output, got %q", output)
	}
}

func TestSetAsSlogDefault(t *testing.T) {
	buf := &bytes.Buffer{}
	SetConfig(Config{
		Output:      buf,
		Level:       LevelInfo,
		EnableColor: false,
		TimeFormat:  "15:04:05",
	})
	previous := slog.Default()
	t.Cleanup(func() {
		slog.SetDefault(previous)
		SetConfig(defaultTestConfig)
	})

	SetAsSlogDefault()
	slog.Info("via slog default", "key", "value")

	if !strings.Contains(buf.String(), "via slog default") {
		t.Errorf("slog.Default() should route through the pipeline, got %q", buf.String())
	}
}

func TestConcurrentInternalAndExternalLogging(t *testing.T) {
	SetConfig(Config{
		Output:      &syncBuffer{},
		Level:       LevelInfo,
		EnableColor: false,
		TimeFormat:  "15:04:05",
	})
	t.Cleanup(func() { SetConfig(defaultTestConfig) })

	log := Slog()
	var wg sync.WaitGroup
	for i := range 25 {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			LogInfo("internal", "n", n)
		}(i)
		go func(n int) {
			defer wg.Done()
			log.Info("external", "n", n)
		}(i)
	}
	wg.Wait()
}

// syncBuffer is a writer safe for concurrent use by racing loggers.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}
