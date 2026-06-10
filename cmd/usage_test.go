package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/dyammarcano/claude-status/internal/usage"
)

func sampleSnapshot(now time.Time) usage.Snapshot {
	return usage.Snapshot{
		Session:    usage.Window{UsedPct: 61, ResetsAt: now.Add(time.Hour), Known: true},
		Weekly:     usage.Window{UsedPct: 38, ResetsAt: now.Add(48 * time.Hour), Known: true},
		ContextPct: 42, CostUSD: 1.23, Model: "Opus 4.8", CapturedAt: now,
	}
}

func TestRenderUsageTable(t *testing.T) {
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	out := new(bytes.Buffer)
	models := []usage.ModelUsage{{Model: "claude-opus-4-8", InputTokens: 1200000, OutputTokens: 89000}}
	renderUsageTable(out, sampleSnapshot(now), models, now)

	s := out.String()
	for _, want := range []string{"Session", "61%", "Weekly", "38%", "Opus 4.8", "claude-opus-4-8", "estimate"} {
		if !strings.Contains(s, want) {
			t.Fatalf("table missing %q:\n%s", want, s)
		}
	}
}

func TestRenderUsageJSON(t *testing.T) {
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)

	out := new(bytes.Buffer)
	if err := renderUsageJSON(out, sampleSnapshot(now), nil, now); err != nil {
		t.Fatalf("json: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("output not valid json: %v", err)
	}

	if _, ok := decoded["snapshot"]; !ok {
		t.Fatalf("missing snapshot key: %s", out.String())
	}
}

func TestRenderUsageTable_NoData(t *testing.T) {
	now := time.Now()
	out := new(bytes.Buffer)
	renderUsageTable(out, usage.Snapshot{}, nil, now)

	if !strings.Contains(out.String(), "unknown") {
		t.Fatalf("expected 'unknown' for empty snapshot:\n%s", out.String())
	}
}
