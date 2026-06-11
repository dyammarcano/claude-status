# Milestones

## v0.1.0 - Working Monitor
- **Target:** TBD
- **Status:** Complete (tagged `v0.1.0`, local-only)
- **Goals:**
  - [x] Project scaffolding
  - [x] Statuspage client
  - [x] Windows toast notifier
  - [x] Poll loop with transition detection

## v0.2.0 - Usage Tracking
- **Target:** TBD
- **Status:** Complete (tagged `v0.2.0`)
- **Goals:**
  - [x] statusLine wrapper capturing official rate_limits
  - [x] Milestone toast alerts (50–100%, session + weekly)
  - [x] `usage` readout (table + --json; opt-in per-model estimate)

## v0.3.0 - Windows Auto-start
- **Target:** TBD
- **Status:** Complete (tagged `v0.3.0`)
- **Goals:**
  - [x] `service install/uninstall/status` (HKCU Run key, logon, user session)
  - [x] `--background` hidden mode logging to `<cache>/monitor.log`
  - [x] Linux CI fix (windows-tagged notify test)

## v0.4.0 - Toast UX Polish
- **Target:** TBD
- **Status:** Complete (merged to main; ready to tag)
- **Goals:**
  - [x] No per-render console flash (`CREATE_NO_WINDOW` on passthrough child)
  - [x] "5h limit" relabel; title shows current % only
  - [x] Colored severity dot (🔵/🟡/🔴) with `·` separator

## v1.0.0 - First Stable Release
- **Target:** TBD
- **Status:** In Progress
- **Goals:**
  - [x] 80%+ test coverage (~83%)
  - [x] README usage docs
  - [ ] GoReleaser tagged build / published GitHub release
