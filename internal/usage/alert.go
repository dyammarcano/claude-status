package usage

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// AlertState records, per window, the reset epoch armed against and the highest
// threshold already alerted for that epoch.
type AlertState struct {
	Windows map[string]windowState `json:"windows"`
}

type windowState struct {
	ResetsAtUnix int64   `json:"resets_at_unix"`
	MaxAlerted   float64 `json:"max_alerted"`
}

// Alert is a single notification to emit.
type Alert struct {
	Title string
	Body  string
}

// Evaluate compares the snapshot's windows against thresholds, mutates st, and
// returns alerts for newly-crossed thresholds. A window whose reset time changed
// is treated as a fresh window (re-armed).
func Evaluate(s Snapshot, thresholds []float64, st *AlertState, now time.Time) []Alert {
	if st.Windows == nil {
		st.Windows = map[string]windowState{}
	}

	ordered := append([]float64(nil), thresholds...)
	sort.Float64s(ordered)

	var alerts []Alert

	check := func(key, label, otherLabel string, w, other Window) {
		if !w.Known {
			return
		}

		prev := st.Windows[key]
		if prev.ResetsAtUnix != resetUnix(w) {
			prev = windowState{ResetsAtUnix: resetUnix(w)}
		}

		highest := prev.MaxAlerted
		for _, t := range ordered {
			if w.UsedPct >= t && t > highest {
				highest = t
			}
		}

		if highest > prev.MaxAlerted {
			alerts = append(alerts, Alert{
				Title: fmt.Sprintf("Claude %s %.0f%%", label, highest),
				Body:  alertBody(w, other, otherLabel, s, now),
			})
		}

		prev.MaxAlerted = highest
		st.Windows[key] = prev
	}

	check("session", "session", "Weekly", s.Session, s.Weekly)
	check("weekly", "weekly", "Session", s.Weekly, s.Session)

	return alerts
}

func resetUnix(w Window) int64 {
	if w.ResetsAt.IsZero() {
		return 0
	}

	return w.ResetsAt.Unix()
}

// alertBody builds the multi-line toast body: the crossed window's reset time,
// then the complementary window's usage + context, then cost + model — a compact
// usage snapshot. Newlines render as line breaks on Windows toasts.
func alertBody(w, other Window, otherLabel string, s Snapshot, now time.Time) string {
	var b strings.Builder

	if w.ResetsAt.IsZero() {
		b.WriteString("Reset time unknown")
	} else {
		fmt.Fprintf(&b, "Resets in %s · %s", FormatCountdown(w.ResetsAt, now), w.ResetsAt.Format("Mon 3:04 PM"))
	}

	b.WriteByte('\n')

	if other.Known {
		fmt.Fprintf(&b, "%s %.0f%% · Context %.0f%%", otherLabel, other.UsedPct, s.ContextPct)
	} else {
		fmt.Fprintf(&b, "Context %.0f%%", s.ContextPct)
	}

	b.WriteByte('\n')
	fmt.Fprintf(&b, "$%.2f", s.CostUSD)

	if s.Model != "" {
		fmt.Fprintf(&b, " · %s", s.Model)
	}

	return b.String()
}

// LoadState reads alert state; a missing file yields empty state.
func LoadState(path string) (*AlertState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &AlertState{Windows: map[string]windowState{}}, nil
		}

		return nil, fmt.Errorf("read state: %w", err)
	}

	var st AlertState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("decode state: %w", err)
	}

	if st.Windows == nil {
		st.Windows = map[string]windowState{}
	}

	return &st, nil
}

// SaveState atomically writes alert state to path.
func SaveState(path string, st *AlertState) error {
	return writeJSONAtomic(path, st)
}
