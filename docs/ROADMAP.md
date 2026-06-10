# Roadmap

## Current Status
**Overall Progress:** 90% - Monitor implemented, lint clean, 88.9% coverage; release pending

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
- [x] 80%+ test coverage (88.9%)
- [ ] GitHub release via goreleaser

### Phase 2.5: Usage tracking [COMPLETE]
- [x] internal/usage: capture, alert, estimate, passthrough, config
- [x] `claude-status statusline` wrapper + `--install`
- [x] `claude-status usage` readout (table + --json)

### Phase 3: v1.0.0 [NOT STARTED]
- [ ] Tagged release
