// Package usage captures and reports Claude Code subscription usage.
package usage

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Window is a usage window (5h session or 7d weekly): percent used + reset time.
type Window struct {
	UsedPct  float64   `json:"used_pct"`
	ResetsAt time.Time `json:"resets_at"`
	Known    bool      `json:"known"`
}

// Snapshot is the official usage state captured from the statusline payload.
type Snapshot struct {
	Session    Window    `json:"session"`
	Weekly     Window    `json:"weekly"`
	ContextPct float64   `json:"context_pct"`
	CostUSD    float64   `json:"cost_usd"`
	Model      string    `json:"model"`
	CapturedAt time.Time `json:"captured_at"`
}

// ModelUsage is estimated per-model token usage from local transcripts.
type ModelUsage struct {
	Model        string `json:"model"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	CacheTokens  int64  `json:"cache_tokens"`
}

// FormatCountdown renders the time until t (relative to now) as "47m", "1h47m",
// "4d", or "4d3h". Zero t → "unknown"; non-positive → "now".
func FormatCountdown(t, now time.Time) string {
	if t.IsZero() {
		return "unknown"
	}

	d := t.Sub(now)
	if d <= 0 {
		return "now"
	}

	if d >= 24*time.Hour {
		days := int(d / (24 * time.Hour))

		hours := int((d % (24 * time.Hour)) / time.Hour)
		if hours > 0 {
			return fmt.Sprintf("%dd%dh", days, hours)
		}

		return fmt.Sprintf("%dd", days)
	}

	h := int(d / time.Hour)
	m := int((d % time.Hour) / time.Minute)
	if h > 0 {
		return fmt.Sprintf("%dh%dm", h, m)
	}

	return fmt.Sprintf("%dm", m)
}

// ParseThresholds parses a comma list like "80,95" into ascending percents.
// Empty or all-invalid input falls back to the default {80, 95}.
func ParseThresholds(s string) []float64 {
	var out []float64

	for p := range strings.SplitSeq(s, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		if v, err := strconv.ParseFloat(p, 64); err == nil {
			out = append(out, v)
		}
	}

	if len(out) == 0 {
		return []float64{80, 95}
	}

	return out
}
