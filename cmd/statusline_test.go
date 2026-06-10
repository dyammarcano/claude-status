package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/inovacc/claude-status/internal/usage"
)

type fakeNotifier struct{ count int }

func (f *fakeNotifier) Notify(_, _ string) error { f.count++; return nil }

func TestEmitAlerts_FiresOnCross(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	n := &fakeNotifier{}
	snap := usage.Snapshot{Session: usage.Window{UsedPct: 90, ResetsAt: time.Unix(1781956800, 0), Known: true}}

	if err := emitAlerts(n, snap, []float64{80}, statePath, time.Unix(1781950000, 0)); err != nil {
		t.Fatalf("emitAlerts: %v", err)
	}

	if n.count != 1 {
		t.Fatalf("want 1 toast, got %d", n.count)
	}

	// Second call, same window, should not re-fire.
	if err := emitAlerts(n, snap, []float64{80}, statePath, time.Unix(1781950000, 0)); err != nil {
		t.Fatalf("emitAlerts 2: %v", err)
	}

	if n.count != 1 {
		t.Fatalf("want still 1 toast, got %d", n.count)
	}
}

func TestStatuslineInstall_PrintOnly(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", dir)
	isolateCache(t) // keep config.json writes out of the real cache

	_ = os.WriteFile(filepath.Join(dir, "settings.json"), []byte(`{"statusLine":{"type":"command","command":"node gsd.js"}}`), 0o644)

	out := new(bytes.Buffer)
	statuslineInstall = true
	statuslineWrite = false

	t.Cleanup(func() { statuslineInstall = false })

	cmd := statuslineCmd
	cmd.SetOut(out)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("install run: %v", err)
	}

	if !bytes.Contains(out.Bytes(), []byte("claude-status statusline")) {
		t.Fatalf("snippet missing: %s", out.String())
	}

	// Print-only must not modify settings.json.
	got, _ := usage.ReadStatuslineCommand(filepath.Join(dir, "settings.json"))
	if got != "node gsd.js" {
		t.Fatalf("settings should be unchanged, got %q", got)
	}
}
