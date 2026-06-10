package usage

import (
	"testing"
	"time"
)

func TestFormatCountdown(t *testing.T) {
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		at   time.Time
		want string
	}{
		{"zero", time.Time{}, "unknown"},
		{"past", now.Add(-time.Hour), "now"},
		{"minutes", now.Add(47 * time.Minute), "47m"},
		{"hours-minutes", now.Add(time.Hour + 47*time.Minute), "1h47m"},
		{"days", now.Add(4 * 24 * time.Hour), "4d"},
		{"days-hours", now.Add(4*24*time.Hour + 3*time.Hour), "4d3h"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := FormatCountdown(tc.at, now); got != tc.want {
				t.Fatalf("FormatCountdown = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseThresholds(t *testing.T) {
	tests := []struct {
		in   string
		want []float64
	}{
		{"80,95", []float64{80, 95}},
		{" 50 , 90 ", []float64{50, 90}},
		{"", []float64{80, 95}},
		{"garbage", []float64{80, 95}},
	}
	for _, tc := range tests {
		got := ParseThresholds(tc.in)
		if len(got) != len(tc.want) {
			t.Fatalf("ParseThresholds(%q) = %v, want %v", tc.in, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("ParseThresholds(%q) = %v, want %v", tc.in, got, tc.want)
			}
		}
	}
}
