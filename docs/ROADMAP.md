# Roadmap

## Current Status
**Overall Progress:** 70% - Monitor implemented and tested; polishing toward v0.1.0

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
- [ ] golangci-lint clean
- [ ] 80%+ test coverage
- [ ] GitHub release via goreleaser

### Phase 3: v1.0.0 [NOT STARTED]
- [ ] Tagged release
