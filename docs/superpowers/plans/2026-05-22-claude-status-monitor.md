# claude-status Monitor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** A Go CLI that polls `status.claude.com` every 60s and fires a Windows toast on state changes.

**Architecture:** Cobra binary (`claude-status`). `cmd/root.go` wires three `internal` packages: `statuspage` (HTTP client → `Status{Indicator,Description}`), `notify` (build-tagged Windows toast / no-op stub), and `monitor` (poll loop with transition + error-streak detection). All collaborators are interfaces so `monitor` is tested with fakes.

**Tech Stack:** Go 1.22+, Cobra, `github.com/go-toast/toast`, `log/slog`, `net/http/httptest`.

**Spec:** `docs/superpowers/specs/2026-05-22-claude-status-monitor-design.md`

---

## File Structure

| Action | Path | Responsibility |
|--------|------|----------------|
| Create | `internal/statuspage/client.go` | HTTP client + `Status` type + `Fetch` |
| Create | `internal/statuspage/client_test.go` | httptest-based decode + error tests |
| Create | `internal/notify/notifier.go` | `Notifier` interface |
| Create | `internal/notify/toast_windows.go` | Windows toast impl (build tag) |
| Create | `internal/notify/toast_other.go` | No-op stub (build tag) |
| Create | `internal/monitor/monitor.go` | Poll loop + transition + error streak |
| Create | `internal/monitor/monitor_test.go` | Table-driven transition tests |
| Modify | `cmd/root.go` | `--interval` flag, RunE wires & runs monitor |
| Modify | `README.md` | Usage section |

---

## Task 1: statuspage client

**Files:**
- Create: `internal/statuspage/client.go`
- Create: `internal/statuspage/client_test.go`

- [x] **Step 1: Write failing tests**

`internal/statuspage/client_test.go`:
```go
package statuspage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetch_Success(t *testing.T) {
	body := `{"page":{"updated_at":"2026-05-22T00:00:00Z"},"status":{"indicator":"minor","description":"Partial outage"}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-agent", 2*time.Second)
	got, err := c.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Indicator != "minor" || got.Description != "Partial outage" {
		t.Fatalf("unexpected status: %+v", got)
	}
}

func TestFetch_Non2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-agent", 2*time.Second)
	if _, err := c.Fetch(context.Background()); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFetch_BadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-agent", 2*time.Second)
	if _, err := c.Fetch(context.Background()); err == nil {
		t.Fatal("expected error, got nil")
	}
}
```

- [x] **Step 2: Run tests, confirm they fail**

```bash
go test ./internal/statuspage/...
```
Expected: compile error (`undefined: NewClient`, `undefined: Status`).

- [x] **Step 3: Implement client**

`internal/statuspage/client.go`:
```go
// Package statuspage fetches the overall status from a Statuspage v2 API.
package statuspage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Status is the subset of the Statuspage response we care about.
type Status struct {
	Indicator   string // "none" | "minor" | "major" | "critical"
	Description string
}

// Client fetches Status from a Statuspage v2 endpoint.
type Client struct {
	url       string
	userAgent string
	http      *http.Client
}

// NewClient builds a Client pointed at url (e.g. https://status.claude.com/api/v2/status.json).
func NewClient(url, userAgent string, timeout time.Duration) *Client {
	return &Client{
		url:       url,
		userAgent: userAgent,
		http:      &http.Client{Timeout: timeout},
	}
}

type apiResponse struct {
	Status struct {
		Indicator   string `json:"indicator"`
		Description string `json:"description"`
	} `json:"status"`
}

// Fetch retrieves the current overall Status.
func (c *Client) Fetch(ctx context.Context) (Status, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return Status{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return Status{}, fmt.Errorf("http get: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Status{}, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var body apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Status{}, fmt.Errorf("decode: %w", err)
	}
	return Status{Indicator: body.Status.Indicator, Description: body.Status.Description}, nil
}
```

- [x] **Step 4: Tests pass**

```bash
go test ./internal/statuspage/... -v
```
Expected: 3 PASS.

- [x] **Step 5: Commit**

```bash
git add internal/statuspage
git commit -m "feat(statuspage): add v2 API client with tests"
```

---

## Task 2: notify package (interface + build-tagged impls)

**Files:**
- Create: `internal/notify/notifier.go`
- Create: `internal/notify/toast_windows.go`
- Create: `internal/notify/toast_other.go`

- [x] **Step 1: Add go-toast dependency**

```bash
go get github.com/go-toast/toast
```

- [x] **Step 2: Write the interface**

`internal/notify/notifier.go`:
```go
// Package notify provides desktop notifications.
package notify

// Notifier sends a desktop notification.
type Notifier interface {
	Notify(title, body string) error
}
```

- [x] **Step 3: Windows implementation**

`internal/notify/toast_windows.go`:
```go
//go:build windows

package notify

import "github.com/go-toast/toast"

// WindowsToaster fires Windows toast notifications via go-toast.
type WindowsToaster struct {
	AppID string
}

// New returns a Notifier appropriate for the current platform.
func New(appID string) Notifier { return &WindowsToaster{AppID: appID} }

// Notify implements Notifier.
func (w *WindowsToaster) Notify(title, body string) error {
	n := toast.Notification{
		AppID:   w.AppID,
		Title:   title,
		Message: body,
	}
	return n.Push()
}
```

- [x] **Step 4: Non-Windows stub**

`internal/notify/toast_other.go`:
```go
//go:build !windows

package notify

import (
	"fmt"
	"os"
)

// StderrNotifier prints notifications to stderr (non-Windows platforms).
type StderrNotifier struct{}

// New returns a Notifier appropriate for the current platform.
func New(_ string) Notifier { return &StderrNotifier{} }

// Notify implements Notifier.
func (StderrNotifier) Notify(title, body string) error {
	_, err := fmt.Fprintf(os.Stderr, "[notify] %s — %s\n", title, body)
	return err
}
```

- [x] **Step 5: Build all targets**

```bash
go build ./...
GOOS=linux go build ./...
```
Expected: both succeed.

- [x] **Step 6: Commit**

```bash
git add internal/notify go.mod go.sum
git commit -m "feat(notify): add Notifier with Windows toast + stub"
```

---

## Task 3: monitor — write failing transition tests

**Files:**
- Create: `internal/monitor/monitor.go` (stub only at this step)
- Create: `internal/monitor/monitor_test.go`

- [x] **Step 1: Skeleton so tests compile**

`internal/monitor/monitor.go`:
```go
// Package monitor polls a Statuspage client and notifies on indicator changes.
package monitor

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/dyammarcano/claude-status/internal/statuspage"
)

// StatusFetcher is the dependency monitor needs from statuspage.
type StatusFetcher interface {
	Fetch(ctx context.Context) (statuspage.Status, error)
}

// Notifier is the dependency monitor needs from notify (kept local to avoid import cycles).
type Notifier interface {
	Notify(title, body string) error
}

// Monitor polls a StatusFetcher and fires notifications on transitions.
type Monitor struct {
	Fetcher       StatusFetcher
	Notifier      Notifier
	Interval      time.Duration
	Logger        *slog.Logger
	ErrorStreak   int // toast threshold; default 3
	previous      string
	hasPrev       bool
	errStreak     int
	errToasted    bool
}

// errStop is returned by tickFn to break the loop in tests.
var errStop = errors.New("stop")

// Run polls until ctx is cancelled.
func (m *Monitor) Run(ctx context.Context) error { return nil }

// tick performs one poll cycle. Exported for tests.
func (m *Monitor) tick(ctx context.Context) {}
```

- [x] **Step 2: Write tests**

`internal/monitor/monitor_test.go`:
```go
package monitor

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/dyammarcano/claude-status/internal/statuspage"
)

type fakeFetcher struct {
	results []result
	idx     int
}
type result struct {
	s   statuspage.Status
	err error
}

func (f *fakeFetcher) Fetch(_ context.Context) (statuspage.Status, error) {
	if f.idx >= len(f.results) {
		return statuspage.Status{}, errors.New("exhausted")
	}
	r := f.results[f.idx]
	f.idx++
	return r.s, r.err
}

type fakeNotifier struct {
	calls []struct{ title, body string }
}

func (n *fakeNotifier) Notify(title, body string) error {
	n.calls = append(n.calls, struct{ title, body string }{title, body})
	return nil
}

func newMonitor(f *fakeFetcher, n *fakeNotifier) *Monitor {
	return &Monitor{
		Fetcher:     f,
		Notifier:    n,
		Logger:      slog.New(slog.NewJSONHandler(io.Discard, nil)),
		ErrorStreak: 3,
	}
}

func TestTick_FirstObservationSilent(t *testing.T) {
	f := &fakeFetcher{results: []result{{s: statuspage.Status{Indicator: "none", Description: "All Systems Operational"}}}}
	n := &fakeNotifier{}
	m := newMonitor(f, n)
	m.tick(context.Background())
	if len(n.calls) != 0 {
		t.Fatalf("expected no toast on first observation, got %v", n.calls)
	}
}

func TestTick_TransitionFires(t *testing.T) {
	f := &fakeFetcher{results: []result{
		{s: statuspage.Status{Indicator: "none", Description: "ok"}},
		{s: statuspage.Status{Indicator: "minor", Description: "Partial outage"}},
	}}
	n := &fakeNotifier{}
	m := newMonitor(f, n)
	m.tick(context.Background())
	m.tick(context.Background())
	if len(n.calls) != 1 {
		t.Fatalf("expected 1 toast, got %d", len(n.calls))
	}
	if n.calls[0].title != "Claude status: minor" || n.calls[0].body != "Partial outage" {
		t.Fatalf("unexpected toast: %+v", n.calls[0])
	}
}

func TestTick_NoChangeSilent(t *testing.T) {
	f := &fakeFetcher{results: []result{
		{s: statuspage.Status{Indicator: "minor", Description: "x"}},
		{s: statuspage.Status{Indicator: "minor", Description: "x"}},
	}}
	n := &fakeNotifier{}
	m := newMonitor(f, n)
	m.tick(context.Background())
	m.tick(context.Background())
	if len(n.calls) != 0 {
		t.Fatalf("expected no toast, got %v", n.calls)
	}
}

func TestTick_RecoveryFires(t *testing.T) {
	f := &fakeFetcher{results: []result{
		{s: statuspage.Status{Indicator: "minor", Description: "x"}},
		{s: statuspage.Status{Indicator: "none", Description: "All Systems Operational"}},
	}}
	n := &fakeNotifier{}
	m := newMonitor(f, n)
	m.tick(context.Background())
	m.tick(context.Background())
	if len(n.calls) != 1 || n.calls[0].title != "Claude status: none" {
		t.Fatalf("unexpected calls: %+v", n.calls)
	}
}

func TestTick_ErrorStreakFiresOnce(t *testing.T) {
	boom := errors.New("boom")
	f := &fakeFetcher{results: []result{
		{err: boom}, {err: boom}, {err: boom}, {err: boom},
	}}
	n := &fakeNotifier{}
	m := newMonitor(f, n)
	for i := 0; i < 4; i++ {
		m.tick(context.Background())
	}
	if len(n.calls) != 1 {
		t.Fatalf("expected exactly 1 unreachable toast, got %d: %+v", len(n.calls), n.calls)
	}
	if n.calls[0].title != "claude-status" {
		t.Fatalf("unexpected title: %q", n.calls[0].title)
	}
}

func TestTick_ErrorThenSuccessResets(t *testing.T) {
	boom := errors.New("boom")
	f := &fakeFetcher{results: []result{
		{err: boom}, {err: boom},
		{s: statuspage.Status{Indicator: "none", Description: "ok"}},
		{err: boom}, {err: boom},
	}}
	n := &fakeNotifier{}
	m := newMonitor(f, n)
	for i := 0; i < 5; i++ {
		m.tick(context.Background())
	}
	if len(n.calls) != 0 {
		t.Fatalf("expected no toasts (streak reset), got %+v", n.calls)
	}
}
```

- [x] **Step 3: Run tests, confirm they fail**

```bash
go test ./internal/monitor/...
```
Expected: most tests FAIL (tick is a no-op).

- [x] **Step 4: Commit failing tests**

```bash
git add internal/monitor
git commit -m "test(monitor): add transition + error-streak tests (failing)"
```

---

## Task 4: monitor — implement tick + Run

**Files:**
- Modify: `internal/monitor/monitor.go`

- [x] **Step 1: Replace monitor.go with full implementation**

`internal/monitor/monitor.go`:
```go
// Package monitor polls a Statuspage client and notifies on indicator changes.
package monitor

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/dyammarcano/claude-status/internal/statuspage"
)

// StatusFetcher is the dependency monitor needs from statuspage.
type StatusFetcher interface {
	Fetch(ctx context.Context) (statuspage.Status, error)
}

// Notifier is the dependency monitor needs from notify.
type Notifier interface {
	Notify(title, body string) error
}

// Monitor polls a StatusFetcher and fires notifications on indicator transitions.
type Monitor struct {
	Fetcher     StatusFetcher
	Notifier    Notifier
	Interval    time.Duration
	Logger      *slog.Logger
	ErrorStreak int // consecutive failures before an "unreachable" toast; default 3

	previous   string
	hasPrev    bool
	errStreak  int
	errToasted bool
}

// Run polls every Interval until ctx is cancelled.
func (m *Monitor) Run(ctx context.Context) error {
	if m.ErrorStreak <= 0 {
		m.ErrorStreak = 3
	}
	if m.Interval <= 0 {
		m.Interval = 60 * time.Second
	}

	m.tick(ctx)

	t := time.NewTicker(m.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			m.Logger.Info("shutdown")
			return nil
		case <-t.C:
			m.tick(ctx)
		}
	}
}

func (m *Monitor) tick(ctx context.Context) {
	st, err := m.Fetcher.Fetch(ctx)
	if err != nil {
		m.errStreak++
		m.Logger.Warn("poll failed", "err", err, "streak", m.errStreak)
		if m.errStreak >= m.ErrorStreak && !m.errToasted {
			if nerr := m.Notifier.Notify("claude-status", "Cannot reach status page"); nerr != nil {
				m.Logger.Warn("notify failed", "err", nerr)
			}
			m.errToasted = true
		}
		return
	}

	// Success — reset error streak.
	m.errStreak = 0
	m.errToasted = false

	if !m.hasPrev {
		m.previous = st.Indicator
		m.hasPrev = true
		m.Logger.Info("initial status", "indicator", st.Indicator, "description", st.Description)
		return
	}

	if st.Indicator == m.previous {
		m.Logger.Debug("status unchanged", "indicator", st.Indicator)
		return
	}

	m.Logger.Info("status changed", "from", m.previous, "to", st.Indicator, "description", st.Description)
	title := fmt.Sprintf("Claude status: %s", st.Indicator)
	if err := m.Notifier.Notify(title, st.Description); err != nil {
		m.Logger.Warn("notify failed", "err", err)
	}
	m.previous = st.Indicator
}
```

- [x] **Step 2: Run tests**

```bash
go test ./internal/monitor/... -v
```
Expected: 6 PASS.

- [x] **Step 3: Commit**

```bash
git add internal/monitor/monitor.go
git commit -m "feat(monitor): implement poll loop with transition + error streak"
```

---

## Task 5: wire root command

**Files:**
- Modify: `cmd/root.go`

- [x] **Step 1: Read current root.go**

```bash
cat cmd/root.go
```

- [x] **Step 2: Replace root.go**

`cmd/root.go`:
```go
package cmd

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dyammarcano/claude-status/internal/monitor"
	"github.com/dyammarcano/claude-status/internal/notify"
	"github.com/dyammarcano/claude-status/internal/statuspage"
	"github.com/spf13/cobra"
)

const (
	defaultURL       = "https://status.claude.com/api/v2/status.json"
	defaultUserAgent = "claude-status-monitor/dev"
	httpTimeout      = 10 * time.Second
)

var (
	flagInterval time.Duration
	flagURL      string
)

var rootCmd = &cobra.Command{
	Use:   "claude-status",
	Short: "Monitor status.claude.com and toast on state changes",
	RunE: func(cmd *cobra.Command, _ []string) error {
		logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		client := statuspage.NewClient(flagURL, defaultUserAgent, httpTimeout)
		notifier := notify.New("claude-status")

		m := &monitor.Monitor{
			Fetcher:  client,
			Notifier: notifier,
			Interval: flagInterval,
			Logger:   logger,
		}

		logger.Info("starting monitor", "url", flagURL, "interval", flagInterval)
		return m.Run(ctx)
	},
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().DurationVar(&flagInterval, "interval", 60*time.Second, "polling interval (min 10s)")
	rootCmd.PersistentFlags().StringVar(&flagURL, "url", defaultURL, "Statuspage v2 status.json URL")
}
```

If `cmd/root.go` already has additional logic (other subcommands added by scaffold), preserve subcommand registrations (`rootCmd.AddCommand(...)`) — keep those lines from the original file and only replace the `rootCmd` definition + `init()` flag block and `Execute()`.

- [x] **Step 3: Build and run briefly**

```bash
go build ./...
go run . --interval=10s
```
Expected: prints `starting monitor` JSON line, then `initial status` JSON line within ~10s. Ctrl+C → `shutdown` line, exit 0.

- [x] **Step 4: Commit**

```bash
git add cmd/root.go
git commit -m "feat(cmd): wire root command to run monitor"
```

---

## Task 6: README usage + final checks

**Files:**
- Modify: `README.md`

- [x] **Step 1: Append usage section to README**

Add to `README.md` (after the existing scaffold content):
```markdown
## Usage

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
```

- [x] **Step 2: Run lint + full test suite**

```bash
go test ./...
golangci-lint run --fix ./... --timeout=5m
```
Expected: all tests pass, lint clean (or auto-fixed).

- [x] **Step 3: Final commit**

```bash
git add README.md
git commit -m "docs: add usage section to README"
```
