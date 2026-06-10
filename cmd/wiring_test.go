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

	if got := resolveThresholds(); got != "80,95" {
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

	statuslineCmd.SetIn(strings.NewReader(`{"rate_limits":{"five_hour":{"used_percentage":61,"resets_at":1781956800}}}`))
	statuslineCmd.SetOut(new(bytes.Buffer))

	if err := runStatusline(statuslineCmd, nil); err != nil {
		t.Fatalf("runStatusline: %v", err)
	}

	snap, err := usage.ReadSnapshot(capture)
	if err != nil {
		t.Fatalf("capture not written: %v", err)
	}

	if !snap.Session.Known || snap.Session.UsedPct != 61 {
		t.Fatalf("snapshot wrong: %+v", snap.Session)
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
