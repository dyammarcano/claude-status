# Claude Code Usage Tracking Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Capture Claude Code subscription usage (5h session %, 7d weekly %, both resets, context %, cost, current model, estimated per-model tokens) and surface it via threshold toast alerts and a `claude-status usage` readout.

**Architecture:** A new `internal/usage` package owns all logic (paths, capture, alerting, transcript estimate, passthrough, config). Claude Code invokes `claude-status statusline` as its statusLine command; that wrapper tees the official `rate_limits` JSON to a capture file, fires threshold toasts (debounced via a state file, re-armed on reset), and passes stdin through to the user's existing status line. `claude-status usage` reads the capture file + transcripts for an on-demand display. The existing status.claude.com monitor is untouched.

**Tech Stack:** Go 1.25, Cobra, `internal/notify` (existing toaster), `encoding/json`, `os/exec`, `bufio`. Tests: table-driven, `testing`, helper-process pattern for exec.

**Spec:** `docs/superpowers/specs/2026-06-09-claude-code-usage-tracking-design.md`

**Refinement vs spec §10:** install writes the downstream command to `<cache>/claude-status/config.json` and sets `settings.json` to plain `claude-status statusline` (avoids nested shell-quoting). `--exec` remains a runtime override.

---

## File Structure

| Action | Path | Responsibility |
|--------|------|----------------|
| Create | `internal/usage/paths.go` | Cross-platform cache/capture/state/config/settings/transcripts paths |
| Create | `internal/usage/model.go` | `Window`/`Snapshot`/`ModelUsage` types, `FormatCountdown`, `ParseThresholds` |
| Create | `internal/usage/capture.go` | Decode statusline JSON → `Snapshot`; atomic read/write |
| Create | `internal/usage/alert.go` | `AlertState`, `Evaluate` (crossing + re-arm), state load/save |
| Create | `internal/usage/estimate.go` | Per-model token aggregation from transcripts |
| Create | `internal/usage/passthrough.go` | `ShellRunner`, `CaptureAndPassthrough` |
| Create | `internal/usage/config.go` | claude-status `Config` + settings.json read/patch |
| Create | `internal/usage/*_test.go` | Tests for each unit above |
| Create | `cmd/statusline.go` | `claude-status statusline` wrapper + alerts + `--install` |
| Create | `cmd/usage.go` | `claude-status usage` readout (table + `--json`) |
| Create | `cmd/statusline_test.go`, `cmd/usage_test.go` | cmd tests |
| Modify | `README.md` | Usage-tracking section + statusline setup |

---

## Task 1: usage foundation — paths + model types

**Files:**
- Create: `internal/usage/paths.go`
- Create: `internal/usage/model.go`
- Create: `internal/usage/model_test.go`

- [ ] **Step 1: Write failing tests**

`internal/usage/model_test.go`:
```go
package usage

import (
	"testing"
	"time"
)

func TestFormatCountdown(t *testing.T) {
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		at   time.Time
		want string
	}{
		{"zero", time.Time{}, "unknown"},
		{"past", now.Add(-time.Hour), "now"},
		{"minutes", now.Add(47 * time.Minute), "47m"},
		{"hours-minutes", now.Add(time.Hour + 47*time.Minute), "1h47m"},
		{"days", now.Add(4 * 24 * time.Hour), "4d"},
		{"days-hours", now.Add(4*24*time.Hour + 3*time.Hour), "4d3h"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := FormatCountdown(tc.at, now); got != tc.want {
				t.Fatalf("FormatCountdown = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseThresholds(t *testing.T) {
	tests := []struct {
		in   string
		want []float64
	}{
		{"80,95", []float64{80, 95}},
		{" 50 , 90 ", []float64{50, 90}},
		{"", []float64{80, 95}},
		{"garbage", []float64{80, 95}},
	}
	for _, tc := range tests {
		got := ParseThresholds(tc.in)
		if len(got) != len(tc.want) {
			t.Fatalf("ParseThresholds(%q) = %v, want %v", tc.in, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("ParseThresholds(%q) = %v, want %v", tc.in, got, tc.want)
			}
		}
	}
}
```

- [ ] **Step 2: Run tests, confirm they fail**

Run: `go test ./internal/usage/... -run 'FormatCountdown|ParseThresholds' -v`
Expected: compile error (`undefined: FormatCountdown`, `undefined: ParseThresholds`).

- [ ] **Step 3: Implement model.go**

`internal/usage/model.go`:
```go
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

	for _, p := range strings.Split(s, ",") {
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
```

- [ ] **Step 4: Implement paths.go**

`internal/usage/paths.go`:
```go
package usage

import (
	"fmt"
	"os"
	"path/filepath"
)

const appDir = "claude-status"

// CacheDir returns (and creates) the claude-status cache directory.
func CacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("user cache dir: %w", err)
	}

	dir := filepath.Join(base, appDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", dir, err)
	}

	return dir, nil
}

func cacheFile(name string) (string, error) {
	dir, err := CacheDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, name), nil
}

// CaptureFilePath is where the latest official Snapshot is persisted.
func CaptureFilePath() (string, error) { return cacheFile("usage.json") }

// StateFilePath is where alert debounce state is persisted.
func StateFilePath() (string, error) { return cacheFile("alert-state.json") }

// ConfigPath is claude-status's own config (passthrough command, thresholds).
func ConfigPath() (string, error) { return cacheFile("config.json") }

// claudeConfigDir returns the Claude Code config dir (CLAUDE_CONFIG_DIR or ~/.claude).
func claudeConfigDir() (string, error) {
	if cfg := os.Getenv("CLAUDE_CONFIG_DIR"); cfg != "" {
		return cfg, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home dir: %w", err)
	}

	return filepath.Join(home, ".claude"), nil
}

// TranscriptsDir returns the Claude Code projects transcript directory.
func TranscriptsDir() (string, error) {
	dir, err := claudeConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "projects"), nil
}

// SettingsPath returns the Claude Code settings.json path.
func SettingsPath() (string, error) {
	dir, err := claudeConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "settings.json"), nil
}
```

- [ ] **Step 5: Run tests, confirm pass**

Run: `go test ./internal/usage/... -run 'FormatCountdown|ParseThresholds' -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/usage/paths.go internal/usage/model.go internal/usage/model_test.go
git commit -m "feat(usage): add types, countdown formatting, and path helpers"
```

---

## Task 2: capture — parse statusline JSON + atomic snapshot I/O

**Files:**
- Create: `internal/usage/capture.go`
- Create: `internal/usage/capture_test.go`

- [ ] **Step 1: Write failing tests**

`internal/usage/capture_test.go`:
```go
package usage

import (
	"path/filepath"
	"testing"
	"time"
)

func TestParseStatusline_Full(t *testing.T) {
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	data := []byte(`{
		"model": {"display_name": "Opus 4.8"},
		"context_window": {"used_percentage": 42},
		"cost": {"total_cost_usd": 1.23},
		"rate_limits": {
			"five_hour": {"used_percentage": 61, "resets_at": 1781956800},
			"seven_day": {"used_percentage": 38, "resets_at": 1782345600}
		}
	}`)

	s, err := ParseStatusline(data, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Model != "Opus 4.8" || s.ContextPct != 42 || s.CostUSD != 1.23 {
		t.Fatalf("scalar fields wrong: %+v", s)
	}
	if !s.Session.Known || s.Session.UsedPct != 61 || s.Session.ResetsAt.Unix() != 1781956800 {
		t.Fatalf("session wrong: %+v", s.Session)
	}
	if !s.Weekly.Known || s.Weekly.UsedPct != 38 {
		t.Fatalf("weekly wrong: %+v", s.Weekly)
	}
	if !s.CapturedAt.Equal(now) {
		t.Fatalf("capturedAt = %v, want %v", s.CapturedAt, now)
	}
}

func TestParseStatusline_MissingRateLimits(t *testing.T) {
	s, err := ParseStatusline([]byte(`{"model":{"display_name":"Sonnet"}}`), time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Session.Known || s.Weekly.Known {
		t.Fatalf("windows should be unknown, got %+v / %+v", s.Session, s.Weekly)
	}
	if s.Model != "Sonnet" {
		t.Fatalf("model = %q", s.Model)
	}
}

func TestParseStatusline_BadJSON(t *testing.T) {
	if _, err := ParseStatusline([]byte("not json"), time.Now()); err == nil {
		t.Fatal("expected error for bad json")
	}
}

func TestSnapshotRoundTrip(t *testing.T) {
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), "usage.json")
	want := Snapshot{
		Session:    Window{UsedPct: 61, ResetsAt: time.Unix(1781956800, 0), Known: true},
		ContextPct: 42, CostUSD: 1.23, Model: "Opus 4.8", CapturedAt: now,
	}
	if err := WriteSnapshot(path, want); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ReadSnapshot(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.Model != want.Model || got.Session.UsedPct != 61 || !got.Session.Known {
		t.Fatalf("round trip mismatch: %+v", got)
	}
}

func TestReadSnapshot_Missing(t *testing.T) {
	if _, err := ReadSnapshot(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Fatal("expected error for missing file")
	}
}
```

- [ ] **Step 2: Run tests, confirm they fail**

Run: `go test ./internal/usage/... -run 'Statusline|Snapshot' -v`
Expected: compile error (`undefined: ParseStatusline`, `WriteSnapshot`, `ReadSnapshot`).

- [ ] **Step 3: Implement capture.go**

`internal/usage/capture.go`:
```go
package usage

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// rawStatusline mirrors the subset of the Claude Code statusLine JSON we read.
// Pointers distinguish "absent" from "zero value".
type rawStatusline struct {
	Model *struct {
		DisplayName string `json:"display_name"`
	} `json:"model"`
	ContextWindow *struct {
		UsedPercentage float64 `json:"used_percentage"`
	} `json:"context_window"`
	Cost *struct {
		TotalCostUSD float64 `json:"total_cost_usd"`
	} `json:"cost"`
	RateLimits *struct {
		FiveHour *rawWindow `json:"five_hour"`
		SevenDay *rawWindow `json:"seven_day"`
	} `json:"rate_limits"`
}

type rawWindow struct {
	UsedPercentage float64 `json:"used_percentage"`
	ResetsAt       int64   `json:"resets_at"`
}

func (rw *rawWindow) toWindow() Window {
	if rw == nil {
		return Window{}
	}

	w := Window{UsedPct: rw.UsedPercentage, Known: true}
	if rw.ResetsAt > 0 {
		w.ResetsAt = time.Unix(rw.ResetsAt, 0)
	}

	return w
}

// ParseStatusline decodes a raw statusline payload into a Snapshot stamped with capturedAt.
func ParseStatusline(data []byte, capturedAt time.Time) (Snapshot, error) {
	var raw rawStatusline
	if err := json.Unmarshal(data, &raw); err != nil {
		return Snapshot{}, fmt.Errorf("decode statusline: %w", err)
	}

	s := Snapshot{CapturedAt: capturedAt}
	if raw.Model != nil {
		s.Model = raw.Model.DisplayName
	}

	if raw.ContextWindow != nil {
		s.ContextPct = raw.ContextWindow.UsedPercentage
	}

	if raw.Cost != nil {
		s.CostUSD = raw.Cost.TotalCostUSD
	}

	if raw.RateLimits != nil {
		s.Session = raw.RateLimits.FiveHour.toWindow()
		s.Weekly = raw.RateLimits.SevenDay.toWindow()
	}

	return s, nil
}

// WriteSnapshot atomically writes the snapshot JSON to path.
func WriteSnapshot(path string, s Snapshot) error {
	return writeJSONAtomic(path, s)
}

// ReadSnapshot reads a previously written snapshot.
func ReadSnapshot(path string) (Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Snapshot{}, fmt.Errorf("read snapshot: %w", err)
	}

	var s Snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return Snapshot{}, fmt.Errorf("decode snapshot: %w", err)
	}

	return s, nil
}

// writeJSONAtomic marshals v and writes it to path via a temp file + rename.
func writeJSONAtomic(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write temp: %w", err)
	}

	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename: %w", err)
	}

	return nil
}
```

- [ ] **Step 4: Run tests, confirm pass**

Run: `go test ./internal/usage/... -run 'Statusline|Snapshot' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/usage/capture.go internal/usage/capture_test.go
git commit -m "feat(usage): parse statusline rate_limits and persist snapshots"
```

---

## Task 3: alert — threshold crossing with re-arm

**Files:**
- Create: `internal/usage/alert.go`
- Create: `internal/usage/alert_test.go`

- [ ] **Step 1: Write failing tests**

`internal/usage/alert_test.go`:
```go
package usage

import (
	"path/filepath"
	"testing"
	"time"
)

func snap(sessionPct float64, resets int64) Snapshot {
	return Snapshot{Session: Window{UsedPct: sessionPct, ResetsAt: time.Unix(resets, 0), Known: true}}
}

func TestEvaluate_CrossesOnce(t *testing.T) {
	now := time.Unix(1781950000, 0)
	st := &AlertState{}
	th := []float64{80, 95}

	if a := Evaluate(snap(50, 1781956800), th, st, now); len(a) != 0 {
		t.Fatalf("50%% should not alert, got %v", a)
	}
	a := Evaluate(snap(82, 1781956800), th, st, now)
	if len(a) != 1 {
		t.Fatalf("82%% should alert once, got %d", len(a))
	}
	if a2 := Evaluate(snap(83, 1781956800), th, st, now); len(a2) != 0 {
		t.Fatalf("83%% (still in 80 band) should not re-alert, got %v", a2)
	}
	if a3 := Evaluate(snap(96, 1781956800), th, st, now); len(a3) != 1 {
		t.Fatalf("96%% should cross 95, got %d", len(a3))
	}
}

func TestEvaluate_ReArmsAfterReset(t *testing.T) {
	now := time.Unix(1781950000, 0)
	st := &AlertState{}
	th := []float64{80}

	if a := Evaluate(snap(90, 1781956800), th, st, now); len(a) != 1 {
		t.Fatalf("first crossing should alert, got %d", len(a))
	}
	// New window (different resets_at) → re-arm, alert again.
	if a := Evaluate(snap(90, 1782300000), th, st, now); len(a) != 1 {
		t.Fatalf("after reset should re-alert, got %d", len(a))
	}
}

func TestEvaluate_UnknownWindowIgnored(t *testing.T) {
	st := &AlertState{}
	if a := Evaluate(Snapshot{}, []float64{80}, st, time.Now()); len(a) != 0 {
		t.Fatalf("unknown window should not alert, got %v", a)
	}
}

func TestStateRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	st, err := LoadState(path) // missing → empty
	if err != nil {
		t.Fatalf("load missing: %v", err)
	}
	st.Windows["session"] = windowState{ResetsAtUnix: 99, MaxAlerted: 80}
	if err := SaveState(path, st); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := LoadState(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Windows["session"].MaxAlerted != 80 {
		t.Fatalf("state not persisted: %+v", got)
	}
}
```

- [ ] **Step 2: Run tests, confirm they fail**

Run: `go test ./internal/usage/... -run 'Evaluate|StateRoundTrip' -v`
Expected: compile error (`undefined: AlertState`, `Evaluate`, `LoadState`, `SaveState`, `windowState`).

- [ ] **Step 3: Implement alert.go**

`internal/usage/alert.go`:
```go
package usage

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"
)

// AlertState records, per window, the reset epoch armed against and the highest
// threshold already alerted for that epoch.
type AlertState struct {
	Windows map[string]windowState `json:"windows"`
}

type windowState struct {
	ResetsAtUnix int64   `json:"resets_at_unix"`
	MaxAlerted   float64 `json:"max_alerted"`
}

// Alert is a single notification to emit.
type Alert struct {
	Title string
	Body  string
}

// Evaluate compares the snapshot's windows against thresholds, mutates st, and
// returns alerts for newly-crossed thresholds. A window whose reset time changed
// is treated as a fresh window (re-armed).
func Evaluate(s Snapshot, thresholds []float64, st *AlertState, now time.Time) []Alert {
	if st.Windows == nil {
		st.Windows = map[string]windowState{}
	}

	ordered := append([]float64(nil), thresholds...)
	sort.Float64s(ordered)

	var alerts []Alert

	check := func(key, label string, w Window) {
		if !w.Known {
			return
		}

		prev := st.Windows[key]
		if prev.ResetsAtUnix != resetUnix(w) {
			prev = windowState{ResetsAtUnix: resetUnix(w)}
		}

		highest := prev.MaxAlerted
		for _, t := range ordered {
			if w.UsedPct >= t && t > highest {
				highest = t
			}
		}

		if highest > prev.MaxAlerted {
			alerts = append(alerts, Alert{
				Title: "claude-status",
				Body:  fmt.Sprintf("Claude %s %.0f%% — resets %s", label, highest, resetClock(w, now)),
			})
		}

		prev.MaxAlerted = highest
		st.Windows[key] = prev
	}

	check("session", "session", s.Session)
	check("weekly", "weekly", s.Weekly)

	return alerts
}

func resetUnix(w Window) int64 {
	if w.ResetsAt.IsZero() {
		return 0
	}

	return w.ResetsAt.Unix()
}

func resetClock(w Window, now time.Time) string {
	if w.ResetsAt.IsZero() {
		return "unknown"
	}

	return fmt.Sprintf("%s (%s)", FormatCountdown(w.ResetsAt, now), w.ResetsAt.Format("Mon 3:04 PM"))
}

// LoadState reads alert state; a missing file yields empty state.
func LoadState(path string) (*AlertState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &AlertState{Windows: map[string]windowState{}}, nil
		}

		return nil, fmt.Errorf("read state: %w", err)
	}

	var st AlertState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("decode state: %w", err)
	}

	if st.Windows == nil {
		st.Windows = map[string]windowState{}
	}

	return &st, nil
}

// SaveState atomically writes alert state to path.
func SaveState(path string, st *AlertState) error {
	return writeJSONAtomic(path, st)
}
```

- [ ] **Step 4: Run tests, confirm pass**

Run: `go test ./internal/usage/... -run 'Evaluate|StateRoundTrip' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/usage/alert.go internal/usage/alert_test.go
git commit -m "feat(usage): threshold-crossing alerts with reset re-arm"
```

---

## Task 4: estimate — per-model tokens from transcripts

**Files:**
- Create: `internal/usage/estimate.go`
- Create: `internal/usage/estimate_test.go`

- [ ] **Step 1: Write failing tests**

`internal/usage/estimate_test.go`:
```go
package usage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeJSONL(t *testing.T, path string, lines []string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestEstimateModels(t *testing.T) {
	dir := t.TempDir()
	recent := "2026-06-09T11:00:00Z"
	old := "2026-05-01T00:00:00Z"
	writeJSONL(t, filepath.Join(dir, "projA", "s1.jsonl"), []string{
		`{"type":"assistant","timestamp":"` + recent + `","message":{"model":"claude-opus-4-8","usage":{"input_tokens":100,"output_tokens":10,"cache_read_input_tokens":5}}}`,
		`{"type":"assistant","timestamp":"` + recent + `","message":{"model":"claude-sonnet-4-6","usage":{"input_tokens":40,"output_tokens":4}}}`,
		`{"type":"user","timestamp":"` + recent + `","message":{"role":"user"}}`,
		`{"type":"assistant","timestamp":"` + old + `","message":{"model":"claude-opus-4-8","usage":{"input_tokens":9999,"output_tokens":9999}}}`,
	})

	since := time.Date(2026, 6, 9, 6, 0, 0, 0, time.UTC)
	got, err := EstimateModels(dir, since)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 models, got %d: %+v", len(got), got)
	}
	// Sorted by model name: opus then sonnet.
	if got[0].Model != "claude-opus-4-8" || got[0].InputTokens != 100 || got[0].CacheTokens != 5 {
		t.Fatalf("opus wrong: %+v", got[0])
	}
	if got[1].Model != "claude-sonnet-4-6" || got[1].OutputTokens != 4 {
		t.Fatalf("sonnet wrong: %+v", got[1])
	}
}

func TestEstimateModels_MissingDir(t *testing.T) {
	got, err := EstimateModels(filepath.Join(t.TempDir(), "nope"), time.Now())
	if err != nil {
		t.Fatalf("missing dir should be nil error, got %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want empty, got %+v", got)
	}
}
```

- [ ] **Step 2: Run tests, confirm they fail**

Run: `go test ./internal/usage/... -run EstimateModels -v`
Expected: compile error (`undefined: EstimateModels`).

- [ ] **Step 3: Implement estimate.go**

`internal/usage/estimate.go`:
```go
package usage

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type transcriptLine struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Message   struct {
		Model string `json:"model"`
		Usage struct {
			InputTokens         int64 `json:"input_tokens"`
			OutputTokens        int64 `json:"output_tokens"`
			CacheCreationTokens int64 `json:"cache_creation_input_tokens"`
			CacheReadTokens     int64 `json:"cache_read_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

// EstimateModels aggregates per-model token usage from *.jsonl transcripts under
// dir (one subdirectory level deep) for assistant messages at or after `since`.
// A missing dir returns (nil, nil). Results are sorted by model name.
func EstimateModels(dir string, since time.Time) ([]ModelUsage, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("read transcripts dir: %w", err)
	}

	agg := map[string]*ModelUsage{}

	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		if e.IsDir() {
			files, _ := os.ReadDir(path)
			for _, f := range files {
				if filepath.Ext(f.Name()) == ".jsonl" {
					accumulateFile(filepath.Join(path, f.Name()), since, agg)
				}
			}

			continue
		}

		if filepath.Ext(e.Name()) == ".jsonl" {
			accumulateFile(path, since, agg)
		}
	}

	out := make([]ModelUsage, 0, len(agg))
	for _, mu := range agg {
		out = append(out, *mu)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Model < out[j].Model })

	return out, nil
}

func accumulateFile(path string, since time.Time, agg map[string]*ModelUsage) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	for sc.Scan() {
		var line transcriptLine
		if err := json.Unmarshal(sc.Bytes(), &line); err != nil {
			continue
		}

		if line.Type != "assistant" || line.Message.Model == "" {
			continue
		}

		if !line.Timestamp.IsZero() && line.Timestamp.Before(since) {
			continue
		}

		mu := agg[line.Message.Model]
		if mu == nil {
			mu = &ModelUsage{Model: line.Message.Model}
			agg[line.Message.Model] = mu
		}

		mu.InputTokens += line.Message.Usage.InputTokens
		mu.OutputTokens += line.Message.Usage.OutputTokens
		mu.CacheTokens += line.Message.Usage.CacheCreationTokens + line.Message.Usage.CacheReadTokens
	}
}
```

- [ ] **Step 4: Run tests, confirm pass**

Run: `go test ./internal/usage/... -run EstimateModels -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/usage/estimate.go internal/usage/estimate_test.go
git commit -m "feat(usage): estimate per-model token usage from transcripts"
```

---

## Task 5: passthrough — capture + forward to downstream statusline

**Files:**
- Create: `internal/usage/passthrough.go`
- Create: `internal/usage/passthrough_test.go`

- [ ] **Step 1: Write failing tests** (helper-process pattern — portable, no shell)

`internal/usage/passthrough_test.go`:
```go
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
	in, _ := os.ReadFile("/dev/stdin")
	_ = in
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
```

- [ ] **Step 2: Run tests, confirm they fail**

Run: `go test ./internal/usage/... -run CaptureAndPassthrough -v`
Expected: compile error (`undefined: CaptureAndPassthrough`, `ShellRunner`).

- [ ] **Step 3: Implement passthrough.go**

`internal/usage/passthrough.go`:
```go
package usage

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"runtime"
	"time"
)

// ShellRunner builds an *exec.Cmd for a downstream command string.
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
		if ok := asExitError(err, &ee); ok {
			exit = ee.ExitCode()
		}
	}

	return snap, exit, capErr
}

func asExitError(err error, target **exec.ExitError) bool {
	if ee, ok := err.(*exec.ExitError); ok {
		*target = ee
		return true
	}

	return false
}
```

> Note: `TestHelperProcess` reads `os.Stdin`; the stray `/dev/stdin` read in the test is harmless on Unix and ignored on Windows — remove it if it causes issues. The authoritative read is the `buf.ReadFrom(os.Stdin)` line.

- [ ] **Step 4: Run tests, confirm pass**

Run: `go test ./internal/usage/... -run CaptureAndPassthrough -v`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/usage/passthrough.go internal/usage/passthrough_test.go
git commit -m "feat(usage): capture + passthrough to downstream statusline"
```

---

## Task 6: config — claude-status config + settings.json read/patch

**Files:**
- Create: `internal/usage/config.go`
- Create: `internal/usage/config_test.go`

- [ ] **Step 1: Write failing tests**

`internal/usage/config_test.go`:
```go
package usage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	c, err := LoadConfig(path) // missing → zero
	if err != nil {
		t.Fatalf("load missing: %v", err)
	}
	if c.Passthrough != "" {
		t.Fatalf("expected empty config, got %+v", c)
	}
	c.Passthrough = `node "x.js"`
	if err := SaveConfig(path, c); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, _ := LoadConfig(path)
	if got.Passthrough != `node "x.js"` {
		t.Fatalf("round trip: %+v", got)
	}
}

func TestReadStatuslineCommand(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	_ = os.WriteFile(path, []byte(`{"model":"opus","statusLine":{"type":"command","command":"node a.js"}}`), 0o644)
	cmd, err := ReadStatuslineCommand(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if cmd != "node a.js" {
		t.Fatalf("command = %q", cmd)
	}
}

func TestReadStatuslineCommand_None(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	_ = os.WriteFile(path, []byte(`{"model":"opus"}`), 0o644)
	cmd, err := ReadStatuslineCommand(path)
	if err != nil || cmd != "" {
		t.Fatalf("expected empty, got %q err=%v", cmd, err)
	}
}

func TestWriteStatuslineCommand_PreservesKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	_ = os.WriteFile(path, []byte(`{"model":"opus","statusLine":{"type":"command","command":"old"}}`), 0o644)
	if err := WriteStatuslineCommand(path, "claude-status statusline"); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, _ := ReadStatuslineCommand(path)
	if got != "claude-status statusline" {
		t.Fatalf("command not updated: %q", got)
	}
	data, _ := os.ReadFile(path)
	if !contains(string(data), `"model"`) {
		t.Fatalf("other keys lost: %s", data)
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (indexOf(s, sub) >= 0) }
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
```

- [ ] **Step 2: Run tests, confirm they fail**

Run: `go test ./internal/usage/... -run 'Config|StatuslineCommand' -v`
Expected: compile error (`undefined: LoadConfig`, `SaveConfig`, `ReadStatuslineCommand`, `WriteStatuslineCommand`).

- [ ] **Step 3: Implement config.go**

`internal/usage/config.go`:
```go
package usage

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config is claude-status's own config for the statusline wrapper.
type Config struct {
	Passthrough string `json:"passthrough"` // downstream statusline command
	Thresholds  string `json:"thresholds"`  // e.g. "80,95" (empty → default)
}

// LoadConfig reads config from path; a missing file yields a zero Config.
func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, nil
		}

		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}

	return c, nil
}

// SaveConfig atomically writes config to path.
func SaveConfig(path string, c Config) error {
	return writeJSONAtomic(path, c)
}

// ReadStatuslineCommand returns settings.json's statusLine.command, or "" if absent.
func ReadStatuslineCommand(settingsPath string) (string, error) {
	m, err := readSettings(settingsPath)
	if err != nil {
		return "", err
	}

	sl, ok := m["statusLine"].(map[string]any)
	if !ok {
		return "", nil
	}

	cmd, _ := sl["command"].(string)

	return cmd, nil
}

// WriteStatuslineCommand sets settings.json's statusLine.command, preserving all
// other keys.
func WriteStatuslineCommand(settingsPath, command string) error {
	m, err := readSettings(settingsPath)
	if err != nil {
		return err
	}

	sl, ok := m["statusLine"].(map[string]any)
	if !ok {
		sl = map[string]any{"type": "command"}
	}

	sl["command"] = command
	m["statusLine"] = sl

	return writeJSONAtomic(settingsPath, m)
}

func readSettings(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}

		return nil, fmt.Errorf("read settings: %w", err)
	}

	m := map[string]any{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("decode settings: %w", err)
	}

	return m, nil
}
```

- [ ] **Step 4: Run tests, confirm pass**

Run: `go test ./internal/usage/... -run 'Config|StatuslineCommand' -v`
Expected: PASS.

- [ ] **Step 5: Run full usage package + commit**

```bash
go test ./internal/usage/... -cover
git add internal/usage/config.go internal/usage/config_test.go
git commit -m "feat(usage): config file and settings.json statusLine read/patch"
```

---

## Task 7: cmd statusline — wrapper command + alerts + install

**Files:**
- Create: `cmd/statusline.go`
- Create: `cmd/statusline_test.go`

- [ ] **Step 1: Write failing tests**

`cmd/statusline_test.go`:
```go
package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dyammarcano/claude-status/internal/usage"
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
```

- [ ] **Step 2: Run tests, confirm they fail**

Run: `go test ./cmd/... -run 'EmitAlerts|StatuslineInstall' -v`
Expected: compile error (`undefined: emitAlerts`, `statuslineCmd`, `statuslineInstall`, `statuslineWrite`).

- [ ] **Step 3: Implement statusline.go**

`cmd/statusline.go`:
```go
package cmd

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/dyammarcano/claude-status/internal/notify"
	"github.com/dyammarcano/claude-status/internal/usage"
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

	if !statuslineNoAlert {
		statePath, err := usage.StateFilePath()
		if err == nil {
			if aerr := emitAlerts(notify.New("claude-status"), snap, usage.ParseThresholds(resolveThresholds()), statePath, now); aerr != nil {
				fmt.Fprintln(os.Stderr, "claude-status: alert error:", aerr)
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
		fmt.Fprintf(out, "Set statusLine.command in %s to:\n", settingsPath)
		fmt.Fprintf(out, "  %s\n", strconv.Quote(newCmd))
		if existing != "" {
			fmt.Fprintf(out, "Your current status line (%q) was saved as the passthrough target.\n", existing)
		}

		fmt.Fprintln(out, "Re-run with --write to apply automatically.")

		return nil
	}

	if err := usage.WriteStatuslineCommand(settingsPath, newCmd); err != nil {
		return err
	}

	fmt.Fprintf(out, "Updated %s (statusLine.command = %q).\n", settingsPath, newCmd)

	return nil
}
```

- [ ] **Step 4: Run tests, confirm pass**

Run: `go test ./cmd/... -run 'EmitAlerts|StatuslineInstall' -v`
Expected: PASS.

> Note: `TestStatuslineInstall_PrintOnly` mutates the package global `statuslineInstall`; the `t.Cleanup` resets it. Run `go test ./cmd/...` (full package) afterward to confirm no cross-test leakage.

- [ ] **Step 5: Commit**

```bash
git add cmd/statusline.go cmd/statusline_test.go
git commit -m "feat(cmd): add statusline wrapper command with alerts and install"
```

---

## Task 8: cmd usage — on-demand readout

**Files:**
- Create: `cmd/usage.go`
- Create: `cmd/usage_test.go`

- [ ] **Step 1: Write failing tests**

`cmd/usage_test.go`:
```go
package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/dyammarcano/claude-status/internal/usage"
)

func sampleSnapshot(now time.Time) usage.Snapshot {
	return usage.Snapshot{
		Session:    usage.Window{UsedPct: 61, ResetsAt: now.Add(time.Hour), Known: true},
		Weekly:     usage.Window{UsedPct: 38, ResetsAt: now.Add(48 * time.Hour), Known: true},
		ContextPct: 42, CostUSD: 1.23, Model: "Opus 4.8", CapturedAt: now,
	}
}

func TestRenderUsageTable(t *testing.T) {
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	out := new(bytes.Buffer)
	models := []usage.ModelUsage{{Model: "claude-opus-4-8", InputTokens: 1200000, OutputTokens: 89000}}
	renderUsageTable(out, sampleSnapshot(now), models, now)
	s := out.String()
	for _, want := range []string{"Session", "61%", "Weekly", "38%", "Opus 4.8", "claude-opus-4-8", "estimate"} {
		if !strings.Contains(s, want) {
			t.Fatalf("table missing %q:\n%s", want, s)
		}
	}
}

func TestRenderUsageJSON(t *testing.T) {
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	out := new(bytes.Buffer)
	if err := renderUsageJSON(out, sampleSnapshot(now), nil, now); err != nil {
		t.Fatalf("json: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("output not valid json: %v", err)
	}
	if _, ok := decoded["snapshot"]; !ok {
		t.Fatalf("missing snapshot key: %s", out.String())
	}
}

func TestRenderUsageTable_NoData(t *testing.T) {
	now := time.Now()
	out := new(bytes.Buffer)
	renderUsageTable(out, usage.Snapshot{}, nil, now)
	if !strings.Contains(out.String(), "unknown") {
		t.Fatalf("expected 'unknown' for empty snapshot:\n%s", out.String())
	}
}
```

- [ ] **Step 2: Run tests, confirm they fail**

Run: `go test ./cmd/... -run RenderUsage -v`
Expected: compile error (`undefined: renderUsageTable`, `renderUsageJSON`).

- [ ] **Step 3: Implement usage.go**

`cmd/usage.go`:
```go
package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/dyammarcano/claude-status/internal/usage"
	"github.com/spf13/cobra"
)

var (
	usageJSON         bool
	usageNoEstimate   bool
	usageCaptureOvr   string
)

var usageCmd = &cobra.Command{
	Use:   "usage",
	Short: "Show current Claude Code usage (session, weekly, context, cost, per-model)",
	RunE:  runUsage,
}

func init() {
	rootCmd.AddCommand(usageCmd)
	usageCmd.Flags().BoolVar(&usageJSON, "json", false, "output as JSON")
	usageCmd.Flags().BoolVar(&usageNoEstimate, "no-estimate", false, "skip per-model transcript estimate")
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
		fmt.Fprintln(cmd.OutOrStdout(), "No usage captured yet. Wire the statusline with: claude-status statusline --install")

		return nil
	}

	var models []usage.ModelUsage
	if !usageNoEstimate {
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
	fmt.Fprintf(w, "Claude Code usage    (captured %s ago)\n", usage.FormatCountdown(now.Add(now.Sub(s.CapturedAt)), now))
	fmt.Fprintf(w, "  Session (5h)   %s   resets %s\n", pct(s.Session), reset(s.Session, now))
	fmt.Fprintf(w, "  Weekly  (7d)   %s   resets %s\n", pct(s.Weekly), reset(s.Weekly, now))
	fmt.Fprintf(w, "  Context %.0f%%    Cost $%.2f    Model %s\n", s.ContextPct, s.CostUSD, modelOrUnknown(s.Model))

	if len(models) > 0 {
		fmt.Fprintln(w, "\n  Per-model (estimate, last 7d)")
		for _, m := range models {
			fmt.Fprintf(w, "    %-22s in %d  out %d  cache %d\n", m.Model, m.InputTokens, m.OutputTokens, m.CacheTokens)
		}
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

var _ = os.Stdout // reserved for future flag wiring
```

> The `var _ = os.Stdout` line is a deliberate keep-import guard only if `os` is otherwise unused; remove it (and the `os` import) if the linter flags it. Prefer removing both over keeping the guard.

- [ ] **Step 4: Run tests, confirm pass**

Run: `go test ./cmd/... -run RenderUsage -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/usage.go cmd/usage_test.go
git commit -m "feat(cmd): add usage readout command (table + json)"
```

---

## Task 9: docs + final verification

**Files:**
- Modify: `README.md`
- Modify: `docs/ROADMAP.md`

- [ ] **Step 1: Append usage-tracking section to README**

Add to `README.md` after the existing Usage section:
````markdown
## Usage limits (Claude Code)

`claude-status` can also surface your Claude Code subscription usage — 5-hour session %, weekly %, both reset times, context %, cost, and an estimated per-model token breakdown.

### One-time setup

Wire claude-status into your Claude Code status line (it passes through to your existing one):

```powershell
# Print the wiring (saves your current status line as the passthrough target):
go run . statusline --install

# Or apply it automatically:
go run . statusline --install --write
```

This sets `statusLine.command` in `~/.claude/settings.json` to `claude-status statusline`. On each render it captures the official `rate_limits` and toasts when session/weekly usage crosses 80% or 95% (re-arming after each reset).

### On-demand readout

```powershell
go run . usage           # human table
go run . usage --json    # machine-readable
```

Per-model figures are **estimates** computed from local transcripts; the session/weekly percentages and reset times are the official values from Claude Code.
````

- [ ] **Step 2: Update ROADMAP**

In `docs/ROADMAP.md`, add under Phase 2 (or a new "Phase 2.5: Usage tracking" section):
```markdown
### Phase 2.5: Usage tracking [COMPLETE]
- [x] internal/usage: capture, alert, estimate, passthrough, config
- [x] `claude-status statusline` wrapper + `--install`
- [x] `claude-status usage` readout (table + --json)
```

- [ ] **Step 3: Full suite, lint, coverage**

```bash
go test ./... -count=1
golangci-lint run ./... --timeout=5m
go test ./... -coverprofile=coverage.out -covermode=atomic && go tool cover -func=coverage.out | tail -1
```
Expected: all tests pass; lint clean; total coverage ≥ 80%.

- [ ] **Step 4: Clean-clone build check**

```bash
go build ./...
```
Expected: BUILD OK.

- [ ] **Step 5: Commit**

```bash
git add README.md docs/ROADMAP.md
git commit -m "docs: document usage tracking; mark Phase 2.5 complete"
```

---

## Self-Review Notes (author)

- **Spec coverage:** §2a→Task 2; §2b→Task 4; §3 architecture→Tasks 5+7; §4 types→Task 1; §5 passthrough→Task 5; §6 alerts→Tasks 3+7; §7 estimate→Task 4; §8 usage cmd→Task 8; §9 config/paths→Tasks 1+6; §10 install→Tasks 6+7; §11 (never break statusline)→Task 5/7 (RunE always returns nil; capture errors swallowed); §11 schema-pinning→empirically validated when first run against a live payload (the `rate_limits` keys in Task 2 reflect the research; adjust `rawStatusline` if a live dump differs); §12 testing→every task.
- **Type consistency:** `Window/Snapshot/ModelUsage/Alert/AlertState/Config` names are stable across tasks; `Evaluate`, `CaptureAndPassthrough`, `emitAlerts`, `renderUsageTable/JSON` signatures match their call sites.
- **Coverage guard:** new `cmd` files are tested (`emitAlerts`, install print, render funcs); `internal/usage` is heavily covered. Re-confirm `cmd` package stays ≥80% in Task 9 Step 3.
- **Security — shell exec provenance (Task 5):** `DefaultShellRunner` runs the downstream via `sh -c` / `cmd /c`, which is intentional — the downstream is an *arbitrary user status-line command* (e.g. `node "…gsd-statusline.js"`) that we are explicitly asked to run. The exec string MUST come ONLY from the user's own config (`config.json` `passthrough`) or the `--exec` flag — **never** from the statusline JSON payload, transcripts, or any network source. The statusline payload is passed to the downstream's **stdin** (not its argv), so it cannot inject a command. Document this constraint in `passthrough.go`'s doc comment, and do not add any code path that derives `execCmd` from captured/parsed data.
- **Known follow-up:** Task 2's exact JSON keys (`rate_limits.five_hour.used_percentage`/`resets_at`, `context_window.used_percentage`) are from research, not a captured payload. First real run: `claude-status statusline --no-alert --exec ""` and inspect the capture file; if any field is empty, dump raw stdin and reconcile `rawStatusline`.
