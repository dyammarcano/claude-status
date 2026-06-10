package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/inovacc/claude-status/internal/usage"
)

// isolateCache points os.UserCacheDir at a temp dir so config/state lookups in
// the cmd wiring don't read the real machine config (which could carry a real
// passthrough command and spawn the user's status line).
func isolateCache(t *testing.T) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmp)
	t.Setenv("LocalAppData", tmp)
}

func TestResolveCapturePath(t *testing.T) {
	t.Cleanup(func() { statuslineCaptureOvr = "" })

	statuslineCaptureOvr = "X/usage.json"

	if got := resolveCapturePath(); got != "X/usage.json" {
		t.Fatalf("flag override = %q", got)
	}

	statuslineCaptureOvr = ""

	t.Setenv("CLAUDE_STATUS_CAPTURE", "env.json")

	if got := resolveCapturePath(); got != "env.json" {
		t.Fatalf("env override = %q", got)
	}
}

func TestResolveExecAndThresholds(t *testing.T) {
	isolateCache(t) // no config.json present
	t.Cleanup(func() { statuslineExec = ""; statuslineThresholds = "" })

	statuslineExec = "node x.js"

	if got := resolveExec(); got != "node x.js" {
		t.Fatalf("exec flag = %q", got)
	}

	statuslineExec = ""

	if got := resolveExec(); got != "" {
		t.Fatalf("expected empty exec without config, got %q", got)
	}

	statuslineThresholds = "50,90"

	if got := resolveThresholds(); got != "50,90" {
		t.Fatalf("thresholds flag = %q", got)
	}

	statuslineThresholds = ""

	if got := resolveThresholds(); got != "50,60,70,80,90,100" {
		t.Fatalf("default thresholds = %q", got)
	}
}

func TestRunUsage_NoCapture(t *testing.T) {
	t.Cleanup(func() { usageCaptureOvr = ""; usageJSON = false; usageNoEstimate = false })

	usageCaptureOvr = filepath.Join(t.TempDir(), "missing.json")
	out := new(bytes.Buffer)
	usageCmd.SetOut(out)

	if err := runUsage(usageCmd, nil); err != nil {
		t.Fatalf("runUsage: %v", err)
	}

	if !strings.Contains(out.String(), "No usage captured") {
		t.Fatalf("expected no-capture message, got: %s", out.String())
	}
}

func TestRunUsage_TableAndJSON(t *testing.T) {
	t.Cleanup(func() { usageCaptureOvr = ""; usageJSON = false; usageNoEstimate = false })

	path := filepath.Join(t.TempDir(), "usage.json")
	now := time.Now()

	if err := usage.WriteSnapshot(path, usage.Snapshot{
		Session:    usage.Window{UsedPct: 61, ResetsAt: now.Add(time.Hour), Known: true},
		Model:      "Opus",
		CapturedAt: now,
	}); err != nil {
		t.Fatalf("seed snapshot: %v", err)
	}

	usageCaptureOvr = path
	usageNoEstimate = true

	out := new(bytes.Buffer)
	usageCmd.SetOut(out)

	if err := runUsage(usageCmd, nil); err != nil {
		t.Fatalf("runUsage table: %v", err)
	}

	if !strings.Contains(out.String(), "Session") {
		t.Fatalf("table not rendered: %s", out.String())
	}

	usageJSON = true

	out.Reset()
	usageCmd.SetOut(out)

	if err := runUsage(usageCmd, nil); err != nil {
		t.Fatalf("runUsage json: %v", err)
	}

	if !strings.Contains(out.String(), "\"snapshot\"") {
		t.Fatalf("json not rendered: %s", out.String())
	}
}

func TestRunStatusline_NoAlertCaptures(t *testing.T) {
	isolateCache(t)
	t.Cleanup(func() {
		statuslineNoAlert = false
		statuslineCaptureOvr = ""
		statuslineExec = ""
		statuslineInstall = false
	})

	capture := filepath.Join(t.TempDir(), "usage.json")
	statuslineCaptureOvr = capture
	statuslineNoAlert = true
	statuslineExec = "" // no downstream spawned

	out := new(bytes.Buffer)

	statuslineCmd.SetIn(strings.NewReader(`{"rate_limits":{"five_hour":{"used_percentage":61,"resets_at":1781956800}}}`))
	statuslineCmd.SetOut(out)

	if err := runStatusline(statuslineCmd, nil); err != nil {
		t.Fatalf("runStatusline: %v", err)
	}

	// With no downstream, a non-blank built-in line must be printed.
	if !strings.Contains(out.String(), "5h 61%") {
		t.Fatalf("expected built-in status line, got: %q", out.String())
	}

	snap, err := usage.ReadSnapshot(capture)
	if err != nil {
		t.Fatalf("capture not written: %v", err)
	}

	if !snap.Session.Known || snap.Session.UsedPct != 61 {
		t.Fatalf("snapshot wrong: %+v", snap.Session)
	}
}

func TestBuiltinStatusLine(t *testing.T) {
	line := builtinStatusLine(usage.Snapshot{
		Session: usage.Window{UsedPct: 61, Known: true},
		Weekly:  usage.Window{UsedPct: 38, Known: true},
		Model:   "Opus",
	})
	for _, want := range []string{"claude-status", "5h 61%", "7d 38%", "Opus"} {
		if !strings.Contains(line, want) {
			t.Fatalf("built-in line %q missing %q", line, want)
		}
	}

	if got := builtinStatusLine(usage.Snapshot{}); got != "claude-status" {
		t.Fatalf("empty built-in line = %q, want never-blank prefix", got)
	}
}

func TestCapturedAgo(t *testing.T) {
	now := time.Unix(1_000_000, 0)

	cases := []struct {
		at   time.Time
		want string
	}{
		{time.Time{}, "time unknown"},
		{now.Add(30 * time.Second), "just now"}, // future
		{now.Add(-30 * time.Second), "30s ago"},
		{now.Add(-5 * time.Minute), "5m ago"},
		{now.Add(-3 * time.Hour), "3h ago"},
	}
	for _, c := range cases {
		if got := capturedAgo(c.at, now); got != c.want {
			t.Fatalf("capturedAgo = %q, want %q", got, c.want)
		}
	}
}

func TestStatuslineInstall_Write(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", dir)
	isolateCache(t)
	t.Cleanup(func() { statuslineInstall = false; statuslineWrite = false })

	if err := os.WriteFile(filepath.Join(dir, "settings.json"),
		[]byte(`{"statusLine":{"type":"command","command":"node gsd.js"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	statuslineInstall = true
	statuslineWrite = true

	out := new(bytes.Buffer)
	statuslineCmd.SetOut(out)

	if err := runStatusline(statuslineCmd, nil); err != nil {
		t.Fatalf("install --write: %v", err)
	}

	got, _ := usage.ReadStatuslineCommand(filepath.Join(dir, "settings.json"))
	if got != "claude-status statusline" {
		t.Fatalf("settings not updated: %q", got)
	}
}
