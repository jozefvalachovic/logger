package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

func setupRenderTest(t *testing.T, mut func(*Config)) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	cfg := Config{Output: &buf, Level: LevelTrace, LevelSet: true, CompactJSON: true, TimeFormat: "15:04:05"}
	if mut != nil {
		mut(&cfg)
	}
	SetConfig(cfg)
	t.Cleanup(func() { SetConfig(defaultTestConfig) })
	return &buf
}

// fieldsOf returns the JSON fields of the last output line.
func fieldsOf(t *testing.T, buf *bytes.Buffer) map[string]json.RawMessage {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	last := lines[len(lines)-1]
	i := strings.IndexByte(last, '{')
	if i < 0 {
		t.Fatalf("no JSON fields in %q", last)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(last[i:]), &m); err != nil {
		t.Fatalf("bad JSON in %q: %v", last, err)
	}
	return m
}

// jsonEqual compares two JSON documents by value, keeping number precision.
func jsonEqual(t *testing.T, a, b []byte) bool {
	t.Helper()
	decode := func(data []byte) any {
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.UseNumber()
		var v any
		if err := dec.Decode(&v); err != nil {
			t.Fatalf("bad JSON %s: %v", data, err)
		}
		return v
	}
	return reflect.DeepEqual(decode(a), decode(b))
}

type accountStatus string
type priority int

type colorEnum int

func (c colorEnum) MarshalJSON() ([]byte, error) {
	return json.Marshal(fmt.Sprintf("color-%d", int(c)))
}

type nilPtrErr struct{}

func (*nilPtrErr) Error() string { return "nilPtrErr" }

func TestValueRendering(t *testing.T) {
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	var typedNil *nilPtrErr

	tests := []struct {
		name  string
		value any
		want  string
	}{
		{"error", errors.New("boom"), `"boom"`},
		{"wrapped error", fmt.Errorf("ctx: %w", errors.New("boom")), `"ctx: boom"`},
		{"typed nil error", error(typedNil), `"<nil>"`},
		{"time", at, `"2026-01-02T03:04:05Z"`},
		{"time pointer", &at, `"2026-01-02T03:04:05Z"`},
		{"duration", 1500 * time.Millisecond, `"1.5s"`},
		{"named string", accountStatus("active"), `"active"`},
		{"named int", priority(3), `3`},
		{"custom MarshalJSON", colorEnum(2), `"color-2"`},
		{"struct big int", struct{ N int64 }{1<<60 + 1}, `{"N":1152921504606846977}`},
		{"struct with time", struct {
			At time.Time `json:"at"`
		}{at}, `{"at":"2026-01-02T03:04:05Z"}`},
		// Unchanged from v4.5.0:
		{"string", "s", `"s"`},
		{"int", 7, `7`},
		{"bool", true, `true`},
		{"nil", nil, `"<nil>"`},
		{"map", map[string]int{"a": 1}, `{"a":1}`},
		{"slice", []string{"a", "b"}, `["a","b"]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := setupRenderTest(t, nil)
			LogInfo("render", "v", tt.value)
			got := fieldsOf(t, buf)["v"]
			if !jsonEqual(t, got, []byte(tt.want)) {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

// Pattern redaction still applies to named string types.
func TestNamedStringStillPatternRedacted(t *testing.T) {
	buf := setupRenderTest(t, func(c *Config) { c.RedactPatterns = []string{`^sk_live_`} })
	LogInfo("key", "v", accountStatus("sk_live_123"))
	if got := string(fieldsOf(t, buf)["v"]); got != `"***"` {
		t.Fatalf("got %s, want masked", got)
	}
}

func TestMessageControlCharsEscaped(t *testing.T) {
	buf := setupRenderTest(t, nil)
	LogInfo("GET /a\n00:00:00 AUDIT {\"userID\":\"admin\"}\r")
	LogInfo("ansi \x1b[2Jcleared")
	LogInfo("tab\tkept")

	out := buf.String()
	if got := strings.Count(out, "\n"); got != 3 {
		t.Fatalf("want exactly 3 output lines, got %d:\n%q", got, out)
	}
	for _, want := range []string{`GET /a\n00:00:00 AUDIT`, `{"userID":"admin"}\r`, `ansi \x1b[2Jcleared`, "tab\tkept"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%q", want, out)
		}
	}
	if strings.ContainsRune(out, '\x1b') {
		t.Errorf("raw ESC in output: %q", out)
	}
}

// The library's own colouring keeps working when colours are on.
func TestMessageEscapingKeepsOwnColors(t *testing.T) {
	buf := setupRenderTest(t, func(c *Config) { c.EnableColor = true })
	LogInfo("hello\nworld")
	out := buf.String()
	if !strings.Contains(out, "\x1b[36mhello\\nworld\x1b[0m") {
		t.Fatalf("want coloured, escaped message, got %q", out)
	}
}

func TestLogErrorWithStackNilError(t *testing.T) {
	buf := setupRenderTest(t, nil)
	LogErrorWithStack(nil, "nil err")
	f := fieldsOf(t, buf)
	if !jsonEqual(t, f["error"], []byte(`"<nil>"`)) {
		t.Fatalf("want error <nil>, got %s", f["error"])
	}
	if _, ok := f["stack"]; !ok {
		t.Error("stack missing")
	}
}

func TestOTelBridgeSeverity(t *testing.T) {
	var bridged, sibling bytes.Buffer
	opts := &slog.HandlerOptions{Level: slog.Level(-100)}
	l := slog.New(slog.NewMultiHandler(
		NewOTelBridgeHandler(slog.NewJSONHandler(&bridged, opts), "svc", "1.0"),
		slog.NewJSONHandler(&sibling, opts),
	))
	for _, lvl := range []slog.Level{LevelTrace, LevelNotice, LevelAudit} {
		l.Log(context.Background(), lvl, "m")
	}

	got := strings.Split(strings.TrimSpace(bridged.String()), "\n")
	for i, want := range []string{"TRACE", "NOTICE", "AUDIT"} {
		var rec map[string]any
		_ = json.Unmarshal([]byte(got[i]), &rec)
		if rec["severity"] != want {
			t.Errorf("record %d: severity = %v, want %s", i, rec["severity"], want)
		}
	}
	if strings.Contains(sibling.String(), "service.name") || strings.Contains(sibling.String(), "severity") {
		t.Errorf("bridge attrs leaked into sibling handler:\n%s", sibling.String())
	}
}

func TestLogHttpRequestKeepsFullBody(t *testing.T) {
	setupRenderTest(t, func(c *Config) { c.MaxBodySize = 10; c.Output = io.Discard })
	body := strings.Repeat("A", 100)
	closed := false
	r, _ := http.NewRequest("POST", "http://x/y", strings.NewReader(body))
	r.Body = closeTracker{Reader: strings.NewReader(body), closed: &closed}

	LogHttpRequest(r)

	rest, _ := io.ReadAll(r.Body)
	if string(rest) != body {
		t.Fatalf("handler sees %d of %d body bytes", len(rest), len(body))
	}
	_ = r.Body.Close()
	if !closed {
		t.Error("Close was not passed through to the original body")
	}
}

type closeTracker struct {
	io.Reader
	closed *bool
}

func (c closeTracker) Close() error { *c.closed = true; return nil }
