// Package monitor polls a Statuspage client and notifies on indicator changes.
package monitor

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/inovacc/claude-status/internal/statuspage"
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
