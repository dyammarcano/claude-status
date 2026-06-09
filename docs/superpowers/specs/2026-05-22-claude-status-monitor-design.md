# claude-status — Windows Monitor for status.claude.com

**Date:** 2026-05-22
**Status:** Approved (pending user review of this document)

## Goal

A small Go CLI that runs as a long-running foreground process, polls the official Statuspage JSON API for `status.claude.com`, and fires a Windows toast notification whenever the overall status indicator changes (incident or recovery).

## Non-goals

- Configuration files, persistent state across restarts, retries with backoff.
- Per-component tracking (API vs. Console vs. claude.ai).
- Webhook / Slack / Discord output.
- Cross-platform notifications (Windows only; the binary will build on other OSes but notifications are no-ops there — out of scope for v1).

## Data source

`GET https://status.claude.com/api/v2/status.json`

Response shape (only fields we consume):

```json
{
  "page":   { "updated_at": "..." },
  "status": { "indicator": "none|minor|major|critical",
              "description": "All Systems Operational" }
}
```

The monitor treats `indicator` as the canonical state. `description` is used as the toast body.

## Behavior

1. **Startup**
   - Parse flags: `--interval` (Go `time.Duration`, default `60s`, min `10s`).
   - Configure `slog` JSON handler on stdout.
   - Install SIGINT/SIGTERM handler for clean shutdown.
2. **First poll**
   - Fetch status. Log the observed indicator + description as `info`.
   - **No toast** on first observation (avoids launch noise).
   - Store indicator as `previous`.
3. **Subsequent polls (every `--interval`)**
   - Fetch status.
   - If `indicator != previous`: fire toast, log transition, update `previous`.
   - If equal: log at `debug` only.
4. **Toast format**
   - Title: `Claude status: <indicator>` (e.g., `Claude status: minor`).
   - Body: `description` from API.
5. **Error handling**
   - HTTP error, non-2xx, or JSON decode error → log `warn` with error and URL. Do not update `previous`. Increment `consecutiveErrors`.
   - On exactly the 3rd consecutive error: fire a single toast `Monitor: cannot reach status page`. Reset the "already toasted" flag once a successful poll arrives.
   - Any successful poll resets `consecutiveErrors` to 0.
6. **Shutdown**
   - On signal: log `info` final line `shutdown`, exit 0.

## HTTP client

- `http.Client{ Timeout: 10 * time.Second }`.
- `User-Agent: claude-status-monitor/<version>`.
- No retries inside a single poll cycle — the next tick is the retry.

## Project layout

Bootstrapped via the `/scaffold:go` skill (standard Go project: Cobra-style CLI, `internal/` packages, table-driven tests, golangci-lint config, BSD 3-Clause `LICENSE`, `README.md`).

```
claude-status/
├── cmd/claude-status/main.go     # flags, signal handling, wires monitor
├── internal/
│   ├── statuspage/
│   │   ├── client.go             # HTTP client + Status type
│   │   └── client_test.go        # JSON decode + non-2xx tests (httptest)
│   ├── notify/
│   │   ├── toast_windows.go      # go-toast wrapper (build tag: windows)
│   │   └── toast_other.go        # no-op stub (build tag: !windows)
│   └── monitor/
│       ├── monitor.go            # poll loop, transition detection
│       └── monitor_test.go       # transition table + error-streak tests
├── go.mod
├── README.md
├── LICENSE                       # BSD 3-Clause
└── .golangci.yml
```

## Dependencies

- `github.com/spf13/cobra` — CLI (provided by scaffold).
- `github.com/go-toast/toast` — Windows toast notifications.
- Standard library: `log/slog`, `net/http`, `encoding/json`, `context`, `time`, `os/signal`.

## Interfaces

```go
// internal/statuspage
type Status struct {
    Indicator   string // "none" | "minor" | "major" | "critical"
    Description string
}
type Client interface {
    Fetch(ctx context.Context) (Status, error)
}

// internal/notify
type Notifier interface {
    Notify(title, body string) error
}

// internal/monitor
type Monitor struct {
    Client   statuspage.Client
    Notifier notify.Notifier
    Interval time.Duration
    Logger   *slog.Logger
}
func (m *Monitor) Run(ctx context.Context) error
```

Dependency injection via constructor — enables testing with fakes.

## Testing

- `statuspage` — `httptest.Server` returning fixture JSON for each indicator value + a 500 + malformed body. Asserts decoded fields.
- `monitor` — table-driven over a sequence of fake Client responses (`[]Status` + injected errors), records calls to a fake Notifier, asserts the exact toast titles/bodies produced and the order. Covers:
  - first poll silent
  - none → minor toast fired
  - minor → minor no toast
  - minor → none recovery toast fired
  - 3 consecutive errors → one "cannot reach" toast, 4th error does not re-toast
  - error → success resets streak
- Target coverage ≥ 80 %.

## Running

```powershell
go run ./cmd/claude-status
go run ./cmd/claude-status --interval=30s
```

Ctrl+C to stop.

## Open questions

None — all design decisions resolved during brainstorming.
