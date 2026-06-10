package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/inovacc/claude-status/internal/notify"
	"github.com/inovacc/claude-status/internal/usage"
	"github.com/spf13/cobra"
)

var (
	statuslineExec       string
	statuslineThresholds string
	statuslineCaptureOvr string
	statuslineNoAlert    bool
	statuslineInstall    bool
	statuslineWrite      bool
)

var statuslineCmd = &cobra.Command{
	Use:   "statusline",
	Short: "Claude Code statusLine wrapper: captures usage, alerts, and passes through",
	Long: "Run as your Claude Code statusLine command. It tees the official rate_limits\n" +
		"JSON to a capture file, fires toast alerts when usage crosses thresholds, and\n" +
		"passes stdin through to your existing status line (set via --install or --exec).",
	RunE: runStatusline,
}

func init() {
	rootCmd.AddCommand(statuslineCmd)
	statuslineCmd.Flags().StringVar(&statuslineExec, "exec", "", "downstream statusline command to run (overrides config)")
	statuslineCmd.Flags().StringVar(&statuslineThresholds, "thresholds", "", "comma list of alert percents (default 80,95)")
	statuslineCmd.Flags().StringVar(&statuslineCaptureOvr, "capture-file", "", "override capture file path")
	statuslineCmd.Flags().BoolVar(&statuslineNoAlert, "no-alert", false, "disable toast alerts")
	statuslineCmd.Flags().BoolVar(&statuslineInstall, "install", false, "print (or with --write, apply) the settings.json wiring")
	statuslineCmd.Flags().BoolVar(&statuslineWrite, "write", false, "with --install, modify settings.json in place")
}

func runStatusline(cmd *cobra.Command, _ []string) error {
	if statuslineInstall {
		return runStatuslineInstall(cmd)
	}

	now := time.Now()
	capturePath := resolveCapturePath()
	execCmd := resolveExec()

	snap, _, _ := usage.CaptureAndPassthrough(cmd.InOrStdin(), cmd.OutOrStdout(), capturePath, execCmd, usage.DefaultShellRunner, now)

	// With no downstream command, render a minimal built-in line so the status
	// line is never blank. The downstream exit code is intentionally ignored and
	// we always return nil: a status line must never fail the host, even when the
	// downstream errors.
	if execCmd == "" {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), builtinStatusLine(snap))
	}

	if !statuslineNoAlert {
		statePath, err := usage.StateFilePath()
		if err == nil {
			if aerr := emitAlerts(notify.New("claude-status"), snap, usage.ParseThresholds(resolveThresholds()), statePath, now); aerr != nil {
				_, _ = fmt.Fprintln(os.Stderr, "claude-status: alert error:", aerr)
			}
		}
	}

	return nil // never break the status line
}

func resolveCapturePath() string {
	if statuslineCaptureOvr != "" {
		return statuslineCaptureOvr
	}

	if env := os.Getenv("CLAUDE_STATUS_CAPTURE"); env != "" {
		return env
	}

	p, err := usage.CaptureFilePath()
	if err != nil {
		return ""
	}

	return p
}

func resolveExec() string {
	if statuslineExec != "" {
		return statuslineExec
	}

	if p, err := usage.ConfigPath(); err == nil {
		if c, err := usage.LoadConfig(p); err == nil {
			return c.Passthrough
		}
	}

	return ""
}

func resolveThresholds() string {
	if statuslineThresholds != "" {
		return statuslineThresholds
	}

	if p, err := usage.ConfigPath(); err == nil {
		if c, err := usage.LoadConfig(p); err == nil && c.Thresholds != "" {
			return c.Thresholds
		}
	}

	return "80,95"
}

// builtinStatusLine renders a minimal one-line status, used when no downstream
// statusline command is configured so the status line is never blank.
func builtinStatusLine(s usage.Snapshot) string {
	parts := []string{"claude-status"}

	if s.Session.Known {
		parts = append(parts, fmt.Sprintf("5h %.0f%%", s.Session.UsedPct))
	}

	if s.Weekly.Known {
		parts = append(parts, fmt.Sprintf("7d %.0f%%", s.Weekly.UsedPct))
	}

	if s.Model != "" {
		parts = append(parts, s.Model)
	}

	return strings.Join(parts, " · ")
}

// emitAlerts loads state, evaluates thresholds, fires toasts via n, and saves state.
func emitAlerts(n notify.Notifier, snap usage.Snapshot, thresholds []float64, statePath string, now time.Time) error {
	st, err := usage.LoadState(statePath)
	if err != nil {
		return err
	}

	for _, a := range usage.Evaluate(snap, thresholds, st, now) {
		_ = n.Notify(a.Title, a.Body)
	}

	return usage.SaveState(statePath, st)
}

func runStatuslineInstall(cmd *cobra.Command) error {
	settingsPath, err := usage.SettingsPath()
	if err != nil {
		return err
	}

	existing, _ := usage.ReadStatuslineCommand(settingsPath)
	if existing != "" && existing != "claude-status statusline" {
		if cfgPath, perr := usage.ConfigPath(); perr == nil {
			c, _ := usage.LoadConfig(cfgPath)
			c.Passthrough = existing
			_ = usage.SaveConfig(cfgPath, c)
		}
	}

	out := cmd.OutOrStdout()

	const newCmd = "claude-status statusline"

	if !statuslineWrite {
		_, _ = fmt.Fprintf(out, "Set statusLine.command in %s to:\n", settingsPath)
		_, _ = fmt.Fprintf(out, "  %s\n", strconv.Quote(newCmd))

		if existing != "" {
			_, _ = fmt.Fprintf(out, "Your current status line (%q) was saved as the passthrough target.\n", existing)
		}

		_, _ = fmt.Fprintln(out, "Re-run with --write to apply automatically.")

		return nil
	}

	if err := usage.WriteStatuslineCommand(settingsPath, newCmd); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(out, "Updated %s (statusLine.command = %q).\n", settingsPath, newCmd)

	return nil
}
