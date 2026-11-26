package logger

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// setupDedupTest routes output to a buffer with dedup on and a window long
// enough that the background cleanup never fires during the test. Dedup is
// switched off first so state from earlier tests (entries, window) is dropped,
// since SetConfig keeps an already-running dedup manager as is.
func setupDedupTest(t *testing.T, extra func(*Config)) *bytes.Buffer {
	t.Helper()
	stopDedup()
	var buf bytes.Buffer
	cfg := Config{
		Output:      &buf,
		Level:       LevelTrace,
		LevelSet:    true,
		EnableColor: false,
		CompactJSON: true,
		TimeFormat:  "15:04:05",
		EnableDedup: true,
		DedupWindow: time.Hour,
	}
	if extra != nil {
		extra(&cfg)
	}
	SetConfig(cfg)
	t.Cleanup(func() {
		stopDedup()
		SetConfig(defaultTestConfig)
	})
	return &buf
}

func outputLines(buf *bytes.Buffer) []string {
	return strings.Split(strings.TrimSpace(buf.String()), "\n")
}

func TestDedupNeverSuppressesAudit(t *testing.T) {
	buf := setupDedupTest(t, nil)

	LogAudit("event", "login_success", "userID", "a")
	LogAudit("event", "login_success", "userID", "b")
	// An exact repeat is still a distinct audit event and must be written.
	LogAudit("event", "login_success", "userID", "b")

	out := buf.String()
	if got := strings.Count(out, `"event":"login_success"`); got != 3 {
		t.Fatalf("want 3 audit lines, got %d:\n%s", got, out)
	}
	for _, want := range []string{`"userID":"a"`, `"userID":"b"`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s in output:\n%s", want, out)
		}
	}

	dedupMgr.Load().Flush()
	if strings.Contains(buf.String(), "repeated") {
		t.Errorf("audit lines must not produce a repeated summary:\n%s", buf.String())
	}
}

func TestDedupDistinctFieldsAreAllWritten(t *testing.T) {
	buf := setupDedupTest(t, nil)

	LogInfo("Email sent", "to", "alice@example.com")
	LogInfo("Email sent", "to", "bob@example.com")

	out := buf.String()
	if got := strings.Count(out, "Email sent"); got != 2 {
		t.Fatalf("want 2 lines, got %d:\n%s", got, out)
	}
	for _, want := range []string{"alice@example.com", "bob@example.com"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s in output:\n%s", want, out)
		}
	}
}

func TestDedupDistinctLevelsAreAllWritten(t *testing.T) {
	buf := setupDedupTest(t, nil)

	LogWarn("disk almost full", "pct", 91)
	LogError("disk almost full", "pct", 91)

	if got := strings.Count(buf.String(), "disk almost full"); got != 2 {
		t.Fatalf("want 2 lines, got %d:\n%s", got, buf.String())
	}
}

func TestDedupKeyValueBoundaries(t *testing.T) {
	buf := setupDedupTest(t, nil)

	// Same characters, split differently across keys and values.
	LogInfo("boundary", "ab", "c")
	LogInfo("boundary", "a", "bc")

	if got := strings.Count(buf.String(), "boundary"); got != 2 {
		t.Fatalf("want 2 lines, got %d:\n%s", got, buf.String())
	}
}

func TestDedupCollapsesExactRepeats(t *testing.T) {
	buf := setupDedupTest(t, nil)

	LogInfo("db timeout", "attempt", 1)
	LogInfo("db timeout", "attempt", 1)

	if got := strings.Count(buf.String(), "db timeout"); got != 1 {
		t.Fatalf("want the repeat suppressed (1 line), got %d:\n%s", got, buf.String())
	}

	dedupMgr.Load().Flush()

	lines := outputLines(buf)
	last := lines[len(lines)-1]
	if !strings.Contains(last, "db timeout (repeated 1 more times)") {
		t.Fatalf("want repeated summary as last line, got:\n%s", buf.String())
	}
	if !strings.Contains(last, "INFO") {
		t.Errorf("summary should keep the first line's level, got: %s", last)
	}
	if strings.Contains(last, "attempt") {
		t.Errorf("summary must not print fields, got: %s", last)
	}
}

func TestDedupUncomparableValues(t *testing.T) {
	buf := setupDedupTest(t, nil)

	// Slices and maps can't be map keys; hashing must handle them.
	LogInfo("batch", "ids", []int{1, 2}, "meta", map[string]any{"k": "v"})
	LogInfo("batch", "ids", []int{1, 2}, "meta", map[string]any{"k": "v"})
	LogInfo("batch", "ids", []int{1, 3}, "meta", map[string]any{"k": "v"})

	if got := strings.Count(buf.String(), "INFO batch"); got != 2 {
		t.Fatalf("want 2 lines (one repeat collapsed), got %d:\n%s", got, buf.String())
	}
}

func TestSamplingNeverDropsAudit(t *testing.T) {
	buf := setupDedupTest(t, func(c *Config) {
		c.EnableDedup = false
		c.SampleRate = 0
		c.SampleRateSet = true
	})

	LogInfo("sampled away")
	LogAudit("event", "login_success", "userID", "a")

	out := buf.String()
	if strings.Contains(out, "sampled away") {
		t.Errorf("SampleRate 0 should drop non-audit lines:\n%s", out)
	}
	if !strings.Contains(out, `"userID":"a"`) {
		t.Fatalf("audit line must bypass sampling, got:\n%s", out)
	}
}
