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

## License

MIT
