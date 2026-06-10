# ADR-0001: Project Scaffold and Tooling Choices

## Status
Accepted — 2026-05-22

## Context
`claude-status` is a small Go CLI that polls the Statuspage API for `status.claude.com` and fires Windows toasts on state changes. Needs a standard, low-friction Go project layout with linting, releases, and CI from day one.

## Decision
- **Structure:** Cobra + `internal/` packages (`statuspage`, `notify`, `monitor`).
- **CLI Framework:** Cobra via `omni scaffold cobra init --full --aicontext`.
- **Task Runner:** Taskfile.
- **Linting:** golangci-lint v2.
- **Releases:** GoReleaser.
- **Module Path:** `github.com/dyammarcano/claude-status`.
- **Notifications:** `github.com/go-toast/toast` behind a `Notifier` interface with build-tagged Windows / no-op stub implementations.
- **Data source:** Statuspage v2 JSON API (`/api/v2/status.json`) — no HTML scraping.

## Consequences

### Positive
- Consistent with other inovacc Go projects.
- Cross-platform build, Windows-only notification side-effect cleanly isolated.
- CI / release pipeline ready immediately.

### Negative
- Cobra is heavier than `flag` for a single-command tool, but worth it for `version`, `cmdtree`, `aicontext` subcommands the scaffold provides.
