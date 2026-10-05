package logger

import (
	"bytes"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/jozefvalachovic/logger/v4/audit"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		in      string
		want    slog.Level
		wantErr bool
	}{
		{"trace", LevelTrace, false},
		{"DEBUG", slog.LevelDebug, false},
		{" info ", slog.LevelInfo, false},
		{"notice", LevelNotice, false},
		{"warn", slog.LevelWarn, false},
		{"Warning", slog.LevelWarn, false},
		{"error", slog.LevelError, false},
		{"audit", LevelAudit, false},
		{"inof", slog.LevelInfo, true},
		{"", slog.LevelInfo, true},
	}
	for _, tt := range tests {
		got, err := ParseLevel(tt.in)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseLevel(%q) error = %v, wantErr %v", tt.in, err, tt.wantErr)
		}
		if got != tt.want {
			t.Errorf("ParseLevel(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}

	// LOG_LEVEL parsing stays lenient.
	if got := parseLevelString("inof"); got != slog.LevelInfo {
		t.Errorf("parseLevelString fallback = %v, want Info", got)
	}
}

func TestSetLevelHonorsInfo(t *testing.T) {
	buf := &bytes.Buffer{}
	SetConfig(Config{
		Output:      buf,
		EnableColor: false,
		TimeFormat:  "15:04:05",
	})
	t.Cleanup(func() { SetConfig(defaultTestConfig) })

	if got := GetConfig().Level; got != LevelTrace {
		t.Fatalf("precondition: expected default Trace level, got %v", got)
	}

	SetLevel(slog.LevelInfo)

	cfg := GetConfig()
	if cfg.Level != slog.LevelInfo || !cfg.LevelSet {
		t.Fatalf("SetLevel(Info) = level %v, LevelSet %v", cfg.Level, cfg.LevelSet)
	}

	LogDebug("internal debug dropped")
	Slog().Debug("external debug dropped")
	LogInfo("internal info kept")
	Slog().Info("external info kept")

	out := buf.String()
	if strings.Contains(out, "dropped") {
		t.Errorf("debug records should be dropped after SetLevel(Info), got %q", out)
	}
	if !strings.Contains(out, "internal info kept") || !strings.Contains(out, "external info kept") {
		t.Errorf("info records should be written on both paths, got %q", out)
	}
}

func TestSetLevelLeavesAuditLoggerUntouched(t *testing.T) {
	auditCfg := audit.DefaultConfig()
	auditCfg.Output = io.Discard
	SetConfig(Config{
		Output:      io.Discard,
		EnableColor: false,
		TimeFormat:  "15:04:05",
		Audit:       &auditCfg,
	})
	t.Cleanup(func() { SetConfig(defaultTestConfig) })

	before := GetAuditLogger()
	if before == nil {
		t.Fatal("precondition: audit logger should be initialised")
	}

	SetLevel(slog.LevelWarn)

	if after := GetAuditLogger(); after != before {
		t.Error("SetLevel must not recreate the audit logger")
	}
}

func TestStdLogger(t *testing.T) {
	buf := &bytes.Buffer{}
	SetConfig(Config{
		Output:      buf,
		Level:       LevelInfo,
		LevelSet:    true,
		EnableColor: false,
		CompactJSON: true,
		TimeFormat:  "15:04:05",
	})
	t.Cleanup(func() { SetConfig(defaultTestConfig) })

	StdLogger(slog.LevelError).Printf("http: TLS handshake error from %s: EOF", "203.0.113.5:51234")
	StdLogger(slog.LevelDebug).Print("below configured level")

	out := buf.String()
	if !strings.Contains(out, "ERROR") || !strings.Contains(out, "TLS handshake error from 203.0.113.5:51234: EOF") {
		t.Errorf("std logger line should go through the pipeline at error level, got %q", out)
	}
	if strings.Contains(out, "below configured level") {
		t.Errorf("lines below the configured level should be dropped, got %q", out)
	}
	if strings.Count(out, "\n") != 1 {
		t.Errorf("expected exactly one output line, got %q", out)
	}
}
