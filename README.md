# claude-status

claude-status is a CLI application

## Installation

```bash
go install github.com/dyammarcano/claude-status@latest
```

## Usage

```bash
claude-status --help
```

## Commands

| Command | Description |
|---------|-------------|
| `version` | Print version information |

## Development

```bash
# Build
task build

# Run
task run

# Test
task test

# Lint
task lint
```

## Release

```bash
# Create a snapshot release
task release:snapshot

# Create a production release (requires git tag)
git tag v1.0.0
task release
```

## Monitor Usage

`claude-status` polls https://status.claude.com/api/v2/status.json and fires a Windows toast whenever the overall status indicator changes (incident or recovery).

```powershell
# Default: 60s interval
go run .

# Custom interval (minimum 10s recommended)
go run . --interval=30s

# Custom URL (e.g. point at another Statuspage)
go run . --url=https://status.anthropic.com/api/v2/status.json
```

On non-Windows platforms the notifier prints to stderr instead of firing a toast.

Press Ctrl+C to stop.

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

This sets `statusLine.command` in `~/.claude/settings.json` to `claude-status statusline`. On each render it captures the official `rate_limits` and toasts as session/weekly usage climbs past each milestone — **50, 60, 70, 80, 90, 100%** (once per milestone, re-arming after each reset). Override with `--thresholds "80,95"`.

### On-demand readout

```powershell
go run . usage              # human table (session/weekly %, resets, context, cost)
go run . usage --estimate   # + per-model token estimate (scans transcripts; slower)
go run . usage --json       # machine-readable
```

The session/weekly percentages and reset times are the **official** values from Claude Code. Per-model token counts (shown with `--estimate`) are **estimates** computed from local transcripts.

## Run at startup (Windows)

Start the status.claude.com monitor automatically at logon. It runs **hidden in your user session** (a true Windows service runs in session 0 and can't show toasts), logging to `<cache>/claude-status/monitor.log`:

```powershell
claude-status service install     # register to start hidden at next logon
claude-status service status      # show whether it's installed
claude-status service uninstall   # remove the auto-start entry
claude-status --background        # start now without re-logging in (self-hides)
```

> **Usage-limit toasts don't need this service** — they already fire from the statusLine wrapper installed via `claude-status statusline --install`. This auto-start is only for the **status.claude.com incident monitor**.

## License

BSD 3-Clause — see [LICENSE](LICENSE).
