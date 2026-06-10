package usage

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ShellRunner builds an *exec.Cmd for a downstream command string.
// The execCmd string MUST come only from the user's own config (config.json
// passthrough) or the --exec flag — never from the statusline JSON payload,
// transcripts, or any network source. The payload is passed via stdin only.
type ShellRunner func(command string) *exec.Cmd

// DefaultShellRunner builds the downstream command by tokenizing command into
// argv and exec'ing the program directly — NO shell. This keeps quoted paths
// intact across platforms (e.g. `node "C:/a b/x.js"`, which cmd /c would mangle)
// and avoids shell-injection. Shell operators (pipes, &&, redirects) in a
// passthrough command are intentionally NOT supported.
func DefaultShellRunner(command string) *exec.Cmd {
	args := splitArgs(command)
	if len(args) == 0 {
		return exec.Command(command) // degenerate; the caller handles execCmd==""
	}

	return exec.Command(args[0], args[1:]...)
}

// splitArgs tokenizes a command line into argv, honoring single/double quotes so
// arguments containing spaces survive intact. Quotes are stripped from tokens.
func splitArgs(s string) []string {
	var (
		args  []string
		cur   strings.Builder
		inArg bool
		quote rune
	)

	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}

			inArg = true
		case r == '"' || r == '\'':
			quote = r
			inArg = true
		case r == ' ' || r == '\t':
			if inArg {
				args = append(args, cur.String())
				cur.Reset()

				inArg = false
			}
		default:
			cur.WriteRune(r)

			inArg = true
		}
	}

	if inArg {
		args = append(args, cur.String())
	}

	return args
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
