package usage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")

	c, err := LoadConfig(path) // missing → zero
	if err != nil {
		t.Fatalf("load missing: %v", err)
	}

	if c.Passthrough != "" {
		t.Fatalf("expected empty config, got %+v", c)
	}

	c.Passthrough = `node "x.js"`
	if err := SaveConfig(path, c); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, _ := LoadConfig(path)
	if got.Passthrough != `node "x.js"` {
		t.Fatalf("round trip: %+v", got)
	}
}

func TestReadStatuslineCommand(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	_ = os.WriteFile(path, []byte(`{"model":"opus","statusLine":{"type":"command","command":"node a.js"}}`), 0o644)

	cmd, err := ReadStatuslineCommand(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if cmd != "node a.js" {
		t.Fatalf("command = %q", cmd)
	}
}

func TestReadStatuslineCommand_None(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	_ = os.WriteFile(path, []byte(`{"model":"opus"}`), 0o644)

	cmd, err := ReadStatuslineCommand(path)
	if err != nil || cmd != "" {
		t.Fatalf("expected empty, got %q err=%v", cmd, err)
	}
}

func TestWriteStatuslineCommand_PreservesKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	_ = os.WriteFile(path, []byte(`{"model":"opus","statusLine":{"type":"command","command":"old"}}`), 0o644)

	if err := WriteStatuslineCommand(path, "claude-status statusline"); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, _ := ReadStatuslineCommand(path)
	if got != "claude-status statusline" {
		t.Fatalf("command not updated: %q", got)
	}

	data, _ := os.ReadFile(path)
	if !containsStr(string(data), `"model"`) {
		t.Fatalf("other keys lost: %s", data)
	}
}

func containsStr(s, sub string) bool { return indexOfStr(s, sub) >= 0 }

func indexOfStr(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}

	return -1
}
