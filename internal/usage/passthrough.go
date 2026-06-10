package usage

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"runtime"
	"time"
)

// ShellRunner builds an *exec.Cmd for a downstream command string.
// The execCmd string MUST come only from the user's own config (config.json
// passthrough) or the --exec flag — never from the statusline JSON payload,
// transcripts, or any network source. The payload is passed via stdin only.
type ShellRunner func(command string) *exec.Cmd

// DefaultShellRunner runs command through the platform shell.
func DefaultShellRunner(command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("cmd", "/c", command)
	}

	return exec.Command("sh", "-c", command)
}

// CaptureAndPassthrough reads the full statusline payload from in, writes a
// Snapshot to capturePath (best-effort), then — if execCmd is non-empty — runs it
// via runner, feeding it the same payload and streaming its stdout to out.
// Returns the parsed snapshot, the downstream exit code, and any capture error.
// A capture error never aborts passthrough; a downstream that fails to start
// yields exit 0 (the status line must not break).
func CaptureAndPassthrough(in io.Reader, out io.Writer, capturePath, execCmd string, runner ShellRunner, now time.Time) (Snapshot, int, error) {
	payload, _ := io.ReadAll(in)

	snap, capErr := ParseStatusline(payload, now)
	if capErr == nil && capturePath != "" {
		_ = WriteSnapshot(capturePath, snap)
	}

	if execCmd == "" {
		return snap, 0, capErr
	}

	if runner == nil {
		runner = DefaultShellRunner
	}

	cmd := runner(execCmd)
	cmd.Stdin = bytes.NewReader(payload)
	cmd.Stdout = out
	cmd.Stderr = os.Stderr

	exit := 0

	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			exit = ee.ExitCode()
		}
	}

	return snap, exit, capErr
}
