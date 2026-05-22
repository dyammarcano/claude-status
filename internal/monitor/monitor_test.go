package monitor

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/inovacc/claude-status/internal/statuspage"
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
