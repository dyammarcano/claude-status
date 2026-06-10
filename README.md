# claude-status

claude-status is a CLI application

## Installation

```bash
go install github.com/inovacc/claude-status@latest
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

This sets `statusLine.command` in `~/.claude/settings.json` to `claude-status statusline`. On each render it captures the official `rate_limits` and toasts when session/weekly usage crosses 80% or 95% (re-arming after each reset).

### On-demand readout

```powershell
go run . usage           # human table
go run . usage --json    # machine-readable
```

Per-model figures are **estimates** computed from local transcripts; the session/weekly percentages and reset times are the official values from Claude Code.

## License

MIT
