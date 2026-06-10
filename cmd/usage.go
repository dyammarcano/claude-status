package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/dyammarcano/claude-status/internal/usage"
	"github.com/spf13/cobra"
)

var (
	usageJSON       bool
	usageEstimate   bool
	usageCaptureOvr string
)

var usageCmd = &cobra.Command{
	Use:   "usage",
	Short: "Show current Claude Code usage (session, weekly, context, cost, per-model)",
	RunE:  runUsage,
}

func init() {
	rootCmd.AddCommand(usageCmd)
	usageCmd.Flags().BoolVar(&usageJSON, "json", false, "output as JSON")
	usageCmd.Flags().BoolVar(&usageEstimate, "estimate", false, "include per-model token estimate from transcripts (slower; scans ~/.claude/projects)")
	usageCmd.Flags().StringVar(&usageCaptureOvr, "capture-file", "", "override capture file path")
}

func runUsage(cmd *cobra.Command, _ []string) error {
	now := time.Now()

	capturePath := usageCaptureOvr
	if capturePath == "" {
		p, err := usage.CaptureFilePath()
		if err != nil {
			return err
		}

		capturePath = p
	}

	snap, err := usage.ReadSnapshot(capturePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.As(err, new(*os.PathError)) {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No usage captured yet. Wire the statusline with: claude-status statusline --install")

			return nil
		}

		return err
	}

	var models []usage.ModelUsage

	if usageEstimate {
		if dir, derr := usage.TranscriptsDir(); derr == nil {
			models, _ = usage.EstimateModels(dir, now.Add(-7*24*time.Hour))
		}
	}

	if usageJSON {
		return renderUsageJSON(cmd.OutOrStdout(), snap, models, now)
	}

	renderUsageTable(cmd.OutOrStdout(), snap, models, now)

	return nil
}

func renderUsageTable(w io.Writer, s usage.Snapshot, models []usage.ModelUsage, now time.Time) {
	_, _ = fmt.Fprintf(w, "Claude Code usage    (captured %s)\n", capturedAgo(s.CapturedAt, now))
	_, _ = fmt.Fprintf(w, "  Session (5h)   %s   resets %s\n", pct(s.Session), reset(s.Session, now))
	_, _ = fmt.Fprintf(w, "  Weekly  (7d)   %s   resets %s\n", pct(s.Weekly), reset(s.Weekly, now))
	_, _ = fmt.Fprintf(w, "  Context %.0f%%    Cost $%.2f    Model %s\n", s.ContextPct, s.CostUSD, modelOrUnknown(s.Model))

	if len(models) > 0 {
		_, _ = fmt.Fprintln(w, "\n  Per-model (estimate, last 7d)")

		for _, m := range models {
			_, _ = fmt.Fprintf(w, "    %-22s in %s  out %s  cache %s\n",
				m.Model, humanize.Comma(m.InputTokens), humanize.Comma(m.OutputTokens), humanize.Comma(m.CacheTokens))
		}
	}
}

// capturedAgo renders how long ago the snapshot was captured.
func capturedAgo(at, now time.Time) string {
	if at.IsZero() {
		return "time unknown"
	}

	d := now.Sub(at)

	switch {
	case d < 0:
		return "just now"
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
}

func pct(win usage.Window) string {
	if !win.Known {
		return "unknown"
	}

	return fmt.Sprintf("%.0f%%", win.UsedPct)
}

func reset(win usage.Window, now time.Time) string {
	if !win.Known {
		return "unknown"
	}

	return usage.FormatCountdown(win.ResetsAt, now)
}

func modelOrUnknown(m string) string {
	if m == "" {
		return "unknown"
	}

	return m
}

type usageJSONDoc struct {
	Snapshot    usage.Snapshot     `json:"snapshot"`
	Estimate    []usage.ModelUsage `json:"estimate"`
	GeneratedAt time.Time          `json:"generated_at"`
}

func renderUsageJSON(w io.Writer, s usage.Snapshot, models []usage.ModelUsage, now time.Time) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")

	if err := enc.Encode(usageJSONDoc{Snapshot: s, Estimate: models, GeneratedAt: now}); err != nil {
		return fmt.Errorf("encode usage json: %w", err)
	}

	return nil
}
