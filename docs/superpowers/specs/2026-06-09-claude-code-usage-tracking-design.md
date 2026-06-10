# Claude Code Usage Tracking — Design

**Date:** 2026-06-09
**Status:** Approved (pending spec review)
**Related:** extends the existing status.claude.com monitor (`internal/{statuspage,notify,monitor}`)

## 1. Goal & Scope

Extend `claude-status` to capture and surface the current Claude Code **subscription usage**:

- **5-hour session window** — % used and reset time
- **7-day (weekly) limit** — % used and reset time
- **Context window** — % used (current session)
- **Cost** — total USD for the current session
- **Current model** — display name
- **Per-model usage** — token totals per model (Opus/Sonnet/Haiku) — **ESTIMATE**

Two surfaces:
1. **Background toast alerts** when session/weekly usage crosses configurable thresholds (and re-arm after reset).
2. **On-demand readout** — `claude-status usage` (human table + `--json`).

**In scope:** statusline capture (official), transcript parsing (estimate), alerts, readout, an install helper.

**Out of scope (YAGNI):** persistent history/trend log; the undocumented OAuth usage API; any GUI; modifying the existing status.claude.com monitor's behavior; per-model *limits* (Claude Code does not expose per-model caps — only aggregate windows).

## 2. Data Sources

### 2a. Official — statusLine JSON (primary)
Claude Code pipes JSON to the configured `statusLine` command on stdin on each render. Relevant fields (to be **pinned empirically** in Task 1 — see §11):

```json
{
  "model": { "display_name": "Opus 4.8" },
  "context_window": { "used_percentage": 42.0 },
  "cost": { "total_cost_usd": 1.23 },
  "rate_limits": {
    "five_hour": { "used_percentage": 61.0, "resets_at": 1782000000 },
    "seven_day": { "used_percentage": 38.0, "resets_at": 1782400000 }
  }
}
```

Notes:
- `resets_at` is Unix epoch **seconds**.
- This data **only flows while Claude Code is running** and rendering the statusline.
- The `rate_limits` / `context_window` blocks are newer additions; the parser MUST tolerate their absence (older CLI, or API-key sessions) and degrade to "unknown".

### 2b. Estimate — conversation transcripts (secondary, per-model)
`~/.claude/projects/**/*.jsonl`. Assistant messages carry:
```json
{ "type": "assistant",
  "message": { "model": "claude-opus-4-8",
    "usage": { "input_tokens": 0, "output_tokens": 0,
               "cache_creation_input_tokens": 0, "cache_read_input_tokens": 0 } } }
```
Aggregated **per model** over the trailing 5h and 7d windows using each line's timestamp. This is an **estimate** (local token tallies, not the server's accounting) and is labeled as such everywhere it appears.

## 3. Architecture

```
Claude Code ──stdin JSON──> `claude-status statusline --exec "<downstream>"`
                                  │  1. tee rate_limits → capture file (atomic)
                                  │  2. evaluate thresholds → toast (state-file debounced)
                                  └  3. exec <downstream>, feed same stdin, forward stdout/exit
                                                     │
  `claude-status usage` ──reads──> capture file (official) + transcripts (estimate) ──> table / --json
```

The existing status.claude.com monitor (`cmd/root.go` RunE) is **unchanged and independent**. Alerting is **wrapper-driven** (no new daemon): toasts fire from inside the statusline wrapper, exactly when fresh data arrives.

### New package: `internal/usage`
Each unit is small, single-purpose, interface-tested:

| File | Responsibility | Depends on |
|------|----------------|------------|
| `model.go` | Types: `Window`, `Snapshot`, `ModelUsage` | stdlib `time` |
| `capture.go` | Decode statusline JSON → `Snapshot`; atomic read/write of capture file | `model.go`, `encoding/json`, `os` |
| `passthrough.go` | Buffer stdin → capture → exec downstream with same stdin → forward stdout/exit | `capture.go`, `os/exec` |
| `estimate.go` | Walk transcripts, aggregate per-model tokens over 5h/7d | `model.go`, `encoding/json` |
| `alert.go` | Threshold-crossing + re-arm vs. a state file; produce toast messages | `model.go`, reads/writes state JSON |
| `paths.go` | Resolve cache/state/transcript paths cross-platform | `os` |

### New cmd files (package `cmd`)
| File | Command | Behavior |
|------|---------|----------|
| `cmd/statusline.go` | `claude-status statusline` | Flags: `--exec "<cmd>"` (downstream), `--thresholds 80,95`, `--no-alert`, `--install`. Runs the passthrough + alert path. |
| `cmd/usage.go` | `claude-status usage` | Reads capture file + transcripts; prints table; `--json` for machine output; `--estimate=false` to skip transcript scan. |

**Reuse:** `internal/notify.Notifier` for all toasts (same Windows/stderr split).

## 4. Data Model

```go
type Window struct {
    UsedPct  float64   // 0–100
    ResetsAt time.Time // zero value = unknown
    Known    bool      // false when the source omitted this window
}

type Snapshot struct {
    Session    Window
    Weekly     Window
    ContextPct float64
    CostUSD    float64
    Model      string
    CapturedAt time.Time
}

type ModelUsage struct {
    Model       string
    InputTokens, OutputTokens, CacheTokens int64
}
```

## 5. Statusline Wrapper (passthrough) — `cmd/statusline.go` + `passthrough.go`

1. Read **all** of stdin into a buffer (statusline payload is small).
2. Best-effort decode → `Snapshot`; atomic-write capture file. **Any failure here is swallowed** (logged to stderr only) — capturing must never break the user's status line.
3. If `--exec` given: spawn it, write the buffered stdin to its stdin, stream its stdout to our stdout, exit with its code. If `--exec` is empty: print a minimal built-in line (session% · weekly% · model).
4. If alerts enabled: run `alert.Evaluate(snapshot)` → if it returns messages, fire toasts via `notify.New`.

**Failure policy:** downstream exec error → log to stderr, exit 0 with a fallback line (never surface a broken statusline). Capture/alert errors are non-fatal.

## 6. Alerting — `alert.go`

- Default thresholds: **80% and 95%**, applied to both `Session` and `Weekly`.
- **State file** (`<cache>/claude-status/alert-state.json`) records, per window, the highest threshold already alerted and the `resets_at` it was armed against.
- Fire a toast only on an **upward crossing** of a threshold not yet alerted for the current window epoch.
- **Re-arm** automatically when `resets_at` changes (new window) → clears alerted thresholds.
- Toast copy: `"Claude session 80% — resets 3:45 PM"`, `"Claude weekly 95% — resets Mon 9:00 AM"`.
- `--no-alert` disables; `--thresholds a,b` overrides.

## 7. Transcript Estimate — `estimate.go`

- Resolve `~/.claude/projects/` (via `paths.go`); enumerate `*.jsonl`.
- **Performance:** only open files modified within the window; read line-by-line; skip non-assistant lines; stop early per file once timestamps fall before the 7d cutoff (files are append-ordered).
- Aggregate `usage.*` per `message.model` for the 5h and 7d windows.
- Always presented as **"estimated (local tokens)"**; never mixed with the official %.

## 8. `usage` Command Output

Table (example):
```
Claude Code usage                              (captured 12s ago)
  Session (5h)   ▕██████░░░░▏ 61%   resets in 1h47m (3:45 PM)
  Weekly  (7d)   ▕████░░░░░░▏ 38%   resets in 4d (Mon 9:00 AM)
  Context        42%        Cost  $1.23        Model  Opus 4.8

  Per-model (estimate, last 7d)
    claude-opus-4-8     in 1.2M  out 89k   cache 4.1M
    claude-sonnet-4-6   in 320k  out 21k   cache 900k
```
- If no capture file yet: explain the statusLine isn't wired and point to `--install`.
- `--json` emits `{snapshot, estimate, generatedAt}`.

## 9. Configuration & Defaults

| Setting | Default | Override |
|---------|---------|----------|
| Capture file | `<os.UserCacheDir>/claude-status/usage.json` | `--capture-file` / `CLAUDE_STATUS_CAPTURE` |
| State file | `<os.UserCacheDir>/claude-status/alert-state.json` | (derived) |
| Thresholds | `80,95` | `--thresholds` |
| Transcripts dir | `~/.claude/projects` | `CLAUDE_CONFIG_DIR` aware |

## 10. Install Helper — `claude-status statusline --install`

Prints (and optionally writes, behind a confirm flag) the `settings.json` snippet that wraps the user's existing status line:
```json
"statusLine": { "type": "command",
  "command": "claude-status statusline --exec \"node \\\"C:/Users/dyamm/.claude/hooks/gsd-statusline.js\\\"\"" }
```
Detects the current `statusLine.command` and embeds it as `--exec` so nothing is lost. Default is **print-only**; writing requires `--install --write`.

## 11. Error Handling Principles

- **The status line is sacred:** no code path in the wrapper may produce a non-zero exit or empty output due to a capture/alert/parse error.
- Missing/old data → fields render as `unknown`, not errors.
- **Task 1 of implementation: pin the real schema.** Use `claude-status statusline --exec ""` (or a `--dump` flag) to capture a live payload and assert the exact JSON keys before relying on them; adjust `capture.go` to match. Defensive decoding (pointers / `Known` flags) guards drift.

## 12. Testing (maintain ≥80% coverage)

- `capture.go`: decode full / partial / missing `rate_limits` payloads; atomic round-trip.
- `model.go`: reset-countdown + clock formatting (inject a fixed `now`).
- `alert.go`: no-cross, single-cross, re-arm-after-reset, both-windows, `--no-alert` — via a fake `Notifier` and an in-memory/temp state file.
- `estimate.go`: fixture `.jsonl` with mixed models + timestamps inside/outside windows; assert per-model sums and window boundaries.
- `passthrough.go`: fake downstream (a tiny echo command / script) — asserts capture file written, stdin forwarded, stdout/exit propagated, and that a failing downstream still exits 0.
- `cmd/usage.go`: table + `--json` against a seeded capture file (buffer output).

## 13. Future (explicitly deferred)

History/trend log (the "Readout + history" option), OAuth API for official per-model, cross-machine aggregation, configurable toast copy.
