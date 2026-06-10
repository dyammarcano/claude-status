package usage

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// rawStatusline mirrors the subset of the Claude Code statusLine JSON we read.
// Pointers distinguish "absent" from "zero value".
type rawStatusline struct {
	Model *struct {
		DisplayName string `json:"display_name"`
	} `json:"model"`
	ContextWindow *struct {
		UsedPercentage float64 `json:"used_percentage"`
	} `json:"context_window"`
	Cost *struct {
		TotalCostUSD float64 `json:"total_cost_usd"`
	} `json:"cost"`
	RateLimits *struct {
		FiveHour *rawWindow `json:"five_hour"`
		SevenDay *rawWindow `json:"seven_day"`
	} `json:"rate_limits"`
}

type rawWindow struct {
	UsedPercentage float64 `json:"used_percentage"`
	ResetsAt       int64   `json:"resets_at"`
}

func (rw *rawWindow) toWindow() Window {
	if rw == nil {
		return Window{}
	}

	w := Window{UsedPct: rw.UsedPercentage, Known: true}
	if rw.ResetsAt > 0 {
		w.ResetsAt = time.Unix(rw.ResetsAt, 0)
	}

	return w
}

// ParseStatusline decodes a raw statusline payload into a Snapshot stamped with capturedAt.
func ParseStatusline(data []byte, capturedAt time.Time) (Snapshot, error) {
	var raw rawStatusline
	if err := json.Unmarshal(data, &raw); err != nil {
		return Snapshot{}, fmt.Errorf("decode statusline: %w", err)
	}

	s := Snapshot{CapturedAt: capturedAt}
	if raw.Model != nil {
		s.Model = raw.Model.DisplayName
	}

	if raw.ContextWindow != nil {
		s.ContextPct = raw.ContextWindow.UsedPercentage
	}

	if raw.Cost != nil {
		s.CostUSD = raw.Cost.TotalCostUSD
	}

	if raw.RateLimits != nil {
		s.Session = raw.RateLimits.FiveHour.toWindow()
		s.Weekly = raw.RateLimits.SevenDay.toWindow()
	}

	return s, nil
}

// WriteSnapshot atomically writes the snapshot JSON to path.
func WriteSnapshot(path string, s Snapshot) error {
	return writeJSONAtomic(path, s)
}

// ReadSnapshot reads a previously written snapshot.
func ReadSnapshot(path string) (Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Snapshot{}, fmt.Errorf("read snapshot: %w", err)
	}

	var s Snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return Snapshot{}, fmt.Errorf("decode snapshot: %w", err)
	}

	return s, nil
}

// writeJSONAtomic marshals v and writes it to path via a temp file + rename.
func writeJSONAtomic(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write temp: %w", err)
	}

	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename: %w", err)
	}

	return nil
}
