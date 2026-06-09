package monitor

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/inovacc/claude-status/internal/statuspage"
)

// errNotifier always fails Notify so the "notify failed" warn branches in
// tick() are exercised.
type errNotifier struct {
	calls int
}

func (e *errNotifier) Notify(_, _ string) error {
	e.calls++
	return errors.New("notify boom")
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func TestRun_CancelledContextReturnsNil(t *testing.T) {
	f := &fakeFetcher{results: []result{
		{s: statuspage.Status{Indicator: "none", Description: "ok"}},
	}}
	n := &fakeNotifier{}
	m := newMonitor(f, n)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before Run so it does one tick then hits ctx.Done.

	if err := m.Run(ctx); err != nil {
		t.Fatalf("expected nil from Run, got %v", err)
	}

	// Exactly one initial tick should have consumed one fetch result.
	if f.idx != 1 {
		t.Fatalf("expected exactly 1 tick (idx=1), got idx=%d", f.idx)
	}
}

func TestRun_AppliesDefaults(t *testing.T) {
	f := &fakeFetcher{results: []result{
		{s: statuspage.Status{Indicator: "none", Description: "ok"}},
	}}
	m := &Monitor{
		Fetcher:     f,
		Notifier:    &fakeNotifier{},
		Logger:      discardLogger(),
		ErrorStreak: 0,
		Interval:    0,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := m.Run(ctx); err != nil {
		t.Fatalf("expected nil from Run, got %v", err)
	}

	if m.ErrorStreak != 3 {
		t.Fatalf("expected ErrorStreak default 3, got %d", m.ErrorStreak)
	}

	if m.Interval != 60*time.Second {
		t.Fatalf("expected Interval default 60s, got %v", m.Interval)
	}
}

func TestTick_NotifyFailureOnTransition(t *testing.T) {
	f := &fakeFetcher{results: []result{
		{s: statuspage.Status{Indicator: "none", Description: "ok"}},
		{s: statuspage.Status{Indicator: "minor", Description: "Partial outage"}},
	}}
	en := &errNotifier{}
	m := &Monitor{
		Fetcher:     f,
		Notifier:    en,
		Logger:      discardLogger(),
		ErrorStreak: 3,
	}

	// First tick establishes baseline, second triggers a transition that
	// calls Notify (which fails) and must hit the warn branch without panicking.
	m.tick(context.Background())
	m.tick(context.Background())

	if en.calls != 1 {
		t.Fatalf("expected exactly 1 Notify attempt on transition, got %d", en.calls)
	}

	// previous must still advance even though Notify failed.
	if m.previous != "minor" {
		t.Fatalf("expected previous to advance to minor, got %q", m.previous)
	}
}

func TestTick_NotifyFailureOnErrorStreak(t *testing.T) {
	boom := errors.New("boom")
	f := &fakeFetcher{results: []result{
		{err: boom}, {err: boom}, {err: boom}, {err: boom},
	}}
	en := &errNotifier{}
	m := &Monitor{
		Fetcher:     f,
		Notifier:    en,
		Logger:      discardLogger(),
		ErrorStreak: 3,
	}

	// Drive the error streak past the threshold so the unreachable Notify
	// fires once (and fails), exercising the warn branch.
	for range 4 {
		m.tick(context.Background())
	}

	if en.calls != 1 {
		t.Fatalf("expected exactly 1 unreachable Notify attempt, got %d", en.calls)
	}

	if !m.errToasted {
		t.Fatalf("expected errToasted to be set after streak Notify")
	}
}
