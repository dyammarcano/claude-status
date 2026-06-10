package usage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeJSONL(t *testing.T, path string, lines []string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	var content strings.Builder
	for _, l := range lines {
		content.WriteString(l + "\n")
	}

	if err := os.WriteFile(path, []byte(content.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestEstimateModels(t *testing.T) {
	dir := t.TempDir()
	recent := "2026-06-09T11:00:00Z"
	old := "2026-05-01T00:00:00Z"
	writeJSONL(t, filepath.Join(dir, "projA", "s1.jsonl"), []string{
		`{"type":"assistant","timestamp":"` + recent + `","message":{"model":"claude-opus-4-8","usage":{"input_tokens":100,"output_tokens":10,"cache_read_input_tokens":5}}}`,
		`{"type":"assistant","timestamp":"` + recent + `","message":{"model":"claude-sonnet-4-6","usage":{"input_tokens":40,"output_tokens":4}}}`,
		`{"type":"user","timestamp":"` + recent + `","message":{"role":"user"}}`,
		`{"type":"assistant","timestamp":"` + old + `","message":{"model":"claude-opus-4-8","usage":{"input_tokens":9999,"output_tokens":9999}}}`,
	})

	since := time.Date(2026, 6, 9, 6, 0, 0, 0, time.UTC)

	got, err := EstimateModels(dir, since)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("want 2 models, got %d: %+v", len(got), got)
	}
	// Sorted by model name: opus then sonnet.
	if got[0].Model != "claude-opus-4-8" || got[0].InputTokens != 100 || got[0].CacheTokens != 5 {
		t.Fatalf("opus wrong: %+v", got[0])
	}

	if got[1].Model != "claude-sonnet-4-6" || got[1].OutputTokens != 4 {
		t.Fatalf("sonnet wrong: %+v", got[1])
	}
}

func TestEstimateModels_MissingDir(t *testing.T) {
	got, err := EstimateModels(filepath.Join(t.TempDir(), "nope"), time.Now())
	if err != nil {
		t.Fatalf("missing dir should be nil error, got %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("want empty, got %+v", got)
	}
}
