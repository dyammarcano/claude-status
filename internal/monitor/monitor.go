// Package monitor polls a Statuspage client and notifies on indicator changes.
package monitor

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/inovacc/claude-status/internal/statuspage"
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
