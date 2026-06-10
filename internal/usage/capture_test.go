package usage

import (
	"path/filepath"
	"testing"
	"time"
)

func TestParseStatusline_Full(t *testing.T) {
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	data := []byte(`{
		"model": {"display_name": "Opus 4.8"},
		"context_window": {"used_percentage": 42},
		"cost": {"total_cost_usd": 1.23},
		"rate_limits": {
			"five_hour": {"used_percentage": 61, "resets_at": 1781956800},
			"seven_day": {"used_percentage": 38, "resets_at": 1782345600}
		}
	}`)

	s, err := ParseStatusline(data, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.Model != "Opus 4.8" || s.ContextPct != 42 || s.CostUSD != 1.23 {
		t.Fatalf("scalar fields wrong: %+v", s)
	}

	if !s.Session.Known || s.Session.UsedPct != 61 || s.Session.ResetsAt.Unix() != 1781956800 {
		t.Fatalf("session wrong: %+v", s.Session)
	}

	if !s.Weekly.Known || s.Weekly.UsedPct != 38 {
		t.Fatalf("weekly wrong: %+v", s.Weekly)
	}

	if !s.CapturedAt.Equal(now) {
		t.Fatalf("capturedAt = %v, want %v", s.CapturedAt, now)
	}
}

func TestParseStatusline_MissingRateLimits(t *testing.T) {
	s, err := ParseStatusline([]byte(`{"model":{"display_name":"Sonnet"}}`), time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.Session.Known || s.Weekly.Known {
		t.Fatalf("windows should be unknown, got %+v / %+v", s.Session, s.Weekly)
	}

	if s.Model != "Sonnet" {
		t.Fatalf("model = %q", s.Model)
	}
}

func TestParseStatusline_BadJSON(t *testing.T) {
	if _, err := ParseStatusline([]byte("not json"), time.Now()); err == nil {
		t.Fatal("expected error for bad json")
	}
}

func TestSnapshotRoundTrip(t *testing.T) {
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), "usage.json")

	want := Snapshot{
		Session:    Window{UsedPct: 61, ResetsAt: time.Unix(1781956800, 0), Known: true},
		ContextPct: 42, CostUSD: 1.23, Model: "Opus 4.8", CapturedAt: now,
	}
	if err := WriteSnapshot(path, want); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := ReadSnapshot(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if got.Model != want.Model || got.Session.UsedPct != 61 || !got.Session.Known {
		t.Fatalf("round trip mismatch: %+v", got)
	}
}

func TestReadSnapshot_Missing(t *testing.T) {
	if _, err := ReadSnapshot(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Fatal("expected error for missing file")
	}
}
