# Roadmap

## Current Status
**Overall Progress:** 97% - Monitor, usage tracking, Windows auto-start, and toast UX all shipped (lint clean, ~83% coverage); only the goreleaser public release remains

## Phases

### Phase 1: Foundation [COMPLETE]
- [x] Project scaffold (omni scaffold cobra init --full --aicontext)
- [x] Design spec (`docs/superpowers/specs/2026-05-22-claude-status-monitor-design.md`)
- [x] `internal/statuspage` HTTP client + tests
- [x] `internal/notify` Windows toast + non-windows stub
- [x] `internal/monitor` poll loop + transition detection + tests
- [x] Wire root cmd to start the monitor

### Phase 2: Polish [IN PROGRESS]
- [x] README usage section
- [x] golangci-lint clean
- [x] 80%+ test coverage (87.3%)
- [ ] GitHub release via goreleaser

### Phase 2.5: Usage tracking [COMPLETE]
- [x] internal/usage: capture, alert, estimate, passthrough, config
- [x] `claude-status statusline` wrapper + `--install`
- [x] `claude-status usage` readout (table + --json)

### Phase 2.6: Windows auto-start [COMPLETE] (v0.3.0)
- [x] `service install/uninstall/status` via HKCU Run key (logon, user session)
- [x] `--background` hidden mode (console hidden, logs to `<cache>/monitor.log`)
- [x] Non-windows stub + Linux CI fix (windows-tagged notify test)

### Phase 2.7: Toast UX polish [COMPLETE] (v0.4.0 candidate)
- [x] Spawn passthrough child with `CREATE_NO_WINDOW` (no per-render console flash)
- [x] Relabel "session" → "5h limit"; title shows current % only
- [x] Colored severity dot in title (🔵 <70% · 🟡 ≥70% · 🔴 ≥90%) with `·` separator

### Phase 3: v1.0.0 [NOT STARTED]
- [ ] GitHub release via goreleaser
- [ ] Tagged stable release
