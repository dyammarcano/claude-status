package usage

import (
	"path/filepath"
	"testing"
	"time"
)

func snap(sessionPct float64, resets int64) Snapshot {
	return Snapshot{Session: Window{UsedPct: sessionPct, ResetsAt: time.Unix(resets, 0), Known: true}}
}

func TestEvaluate_CrossesOnce(t *testing.T) {
	now := time.Unix(1781950000, 0)
	st := &AlertState{}
	th := []float64{80, 95}

	if a := Evaluate(snap(50, 1781956800), th, st, now); len(a) != 0 {
		t.Fatalf("50%% should not alert, got %v", a)
	}

	a := Evaluate(snap(82, 1781956800), th, st, now)
	if len(a) != 1 {
		t.Fatalf("82%% should alert once, got %d", len(a))
	}

	if a2 := Evaluate(snap(83, 1781956800), th, st, now); len(a2) != 0 {
		t.Fatalf("83%% (still in 80 band) should not re-alert, got %v", a2)
	}

	if a3 := Evaluate(snap(96, 1781956800), th, st, now); len(a3) != 1 {
		t.Fatalf("96%% should cross 95, got %d", len(a3))
	}
}

func TestEvaluate_ReArmsAfterReset(t *testing.T) {
	now := time.Unix(1781950000, 0)
	st := &AlertState{}
	th := []float64{80}

	if a := Evaluate(snap(90, 1781956800), th, st, now); len(a) != 1 {
		t.Fatalf("first crossing should alert, got %d", len(a))
	}
	// New window (different resets_at) → re-arm, alert again.
	if a := Evaluate(snap(90, 1782300000), th, st, now); len(a) != 1 {
		t.Fatalf("after reset should re-alert, got %d", len(a))
	}
}

func TestEvaluate_UnknownWindowIgnored(t *testing.T) {
	st := &AlertState{}
	if a := Evaluate(Snapshot{}, []float64{80}, st, time.Now()); len(a) != 0 {
		t.Fatalf("unknown window should not alert, got %v", a)
	}
}

func TestStateRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")

	st, err := LoadState(path) // missing → empty
	if err != nil {
		t.Fatalf("load missing: %v", err)
	}

	st.Windows["session"] = windowState{ResetsAtUnix: 99, MaxAlerted: 80}
	if err := SaveState(path, st); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := LoadState(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if got.Windows["session"].MaxAlerted != 80 {
		t.Fatalf("state not persisted: %+v", got)
	}
}
