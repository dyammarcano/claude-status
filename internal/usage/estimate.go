package usage

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type transcriptLine struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Message   struct {
		Model string `json:"model"`
		Usage struct {
			InputTokens         int64 `json:"input_tokens"`
			OutputTokens        int64 `json:"output_tokens"`
			CacheCreationTokens int64 `json:"cache_creation_input_tokens"`
			CacheReadTokens     int64 `json:"cache_read_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

// EstimateModels aggregates per-model token usage from *.jsonl transcripts under
// dir (one subdirectory level deep) for assistant messages at or after `since`.
// A missing dir returns (nil, nil). Results are sorted by model name.
func EstimateModels(dir string, since time.Time) ([]ModelUsage, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("read transcripts dir: %w", err)
	}

	agg := map[string]*ModelUsage{}

	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		if e.IsDir() {
			files, _ := os.ReadDir(path)
			for _, f := range files {
				if filepath.Ext(f.Name()) == ".jsonl" {
					accumulateFile(filepath.Join(path, f.Name()), since, agg)
				}
			}

			continue
		}

		if filepath.Ext(e.Name()) == ".jsonl" {
			accumulateFile(path, since, agg)
		}
	}

	out := make([]ModelUsage, 0, len(agg))
	for _, mu := range agg {
		out = append(out, *mu)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Model < out[j].Model })

	return out, nil
}

func accumulateFile(path string, since time.Time, agg map[string]*ModelUsage) {
	f, err := os.Open(path)
	if err != nil {
		return
	}

	defer func() { _ = f.Close() }()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	for sc.Scan() {
		var line transcriptLine
		if err := json.Unmarshal(sc.Bytes(), &line); err != nil {
			continue
		}

		if line.Type != "assistant" || line.Message.Model == "" {
			continue
		}

		if !line.Timestamp.IsZero() && line.Timestamp.Before(since) {
			continue
		}

		mu := agg[line.Message.Model]
		if mu == nil {
			mu = &ModelUsage{Model: line.Message.Model}
			agg[line.Message.Model] = mu
		}

		mu.InputTokens += line.Message.Usage.InputTokens
		mu.OutputTokens += line.Message.Usage.OutputTokens
		mu.CacheTokens += line.Message.Usage.CacheCreationTokens + line.Message.Usage.CacheReadTokens
	}
}
