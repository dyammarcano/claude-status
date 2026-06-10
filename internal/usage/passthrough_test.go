package usage

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestHelperProcess is not a real test; it's the downstream "statusline" that
// CaptureAndPassthrough execs. It echoes stdin (prefixed) and exits per env.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(os.Stdin)
	_, _ = os.Stdout.WriteString("DOWNSTREAM:" + buf.String())

	if os.Getenv("HELPER_EXIT") == "2" {
		os.Exit(2)
	}

	os.Exit(0)
}

func helperRunner(t *testing.T, exitCode string) ShellRunner {
	t.Helper()

	return func(string) *exec.Cmd {
		c := exec.Command(os.Args[0], "-test.run=TestHelperProcess")

		c.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1", "HELPER_EXIT="+exitCode)

		return c
	}
}

func TestCaptureAndPassthrough_CapturesAndForwards(t *testing.T) {
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	capture := filepath.Join(t.TempDir(), "usage.json")
	payload := `{"model":{"display_name":"Opus"},"rate_limits":{"five_hour":{"used_percentage":61,"resets_at":1781956800}}}`
	out := new(bytes.Buffer)

	snap, exit, capErr := CaptureAndPassthrough(strings.NewReader(payload), out, capture, "downstream", helperRunner(t, "0"), now)
	if capErr != nil {
		t.Fatalf("capErr: %v", capErr)
	}

	if exit != 0 {
		t.Fatalf("exit = %d, want 0", exit)
	}

	if snap.Session.UsedPct != 61 {
		t.Fatalf("snapshot not parsed: %+v", snap)
	}

	if !strings.HasPrefix(out.String(), "DOWNSTREAM:") || !strings.Contains(out.String(), "Opus") {
		t.Fatalf("downstream output not forwarded: %q", out.String())
	}

	if _, err := ReadSnapshot(capture); err != nil {
		t.Fatalf("capture file not written: %v", err)
	}
}

func TestCaptureAndPassthrough_NoExec(t *testing.T) {
	capture := filepath.Join(t.TempDir(), "usage.json")
	out := new(bytes.Buffer)

	_, exit, err := CaptureAndPassthrough(strings.NewReader(`{}`), out, capture, "", nil, time.Now())
	if err != nil || exit != 0 {
		t.Fatalf("no-exec: err=%v exit=%d", err, exit)
	}
}

func TestCaptureAndPassthrough_DownstreamExitPropagates(t *testing.T) {
	out := new(bytes.Buffer)

	_, exit, _ := CaptureAndPassthrough(strings.NewReader(`{}`), out, filepath.Join(t.TempDir(), "u.json"), "downstream", helperRunner(t, "2"), time.Now())
	if exit != 2 {
		t.Fatalf("exit = %d, want 2", exit)
	}
}

func TestSplitArgs(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{`node "C:/Users/x/.claude/hooks/gsd-statusline.js"`, []string{"node", "C:/Users/x/.claude/hooks/gsd-statusline.js"}},
		{`echo hi`, []string{"echo", "hi"}},
		{`pwsh -File "C:/a b/s.ps1"`, []string{"pwsh", "-File", "C:/a b/s.ps1"}},
		{`  spaced   out  `, []string{"spaced", "out"}},
		{``, nil},
	}

	for _, tc := range tests {
		got := splitArgs(tc.in)
		if len(got) != len(tc.want) {
			t.Fatalf("splitArgs(%q) = %v, want %v", tc.in, got, tc.want)
		}

		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("splitArgs(%q) = %v, want %v", tc.in, got, tc.want)
			}
		}
	}
}
