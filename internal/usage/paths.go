package usage

import (
	"fmt"
	"os"
	"path/filepath"
)

const appDir = "claude-status"

// CacheDir returns (and creates) the claude-status cache directory.
func CacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("user cache dir: %w", err)
	}

	dir := filepath.Join(base, appDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", dir, err)
	}

	return dir, nil
}

func cacheFile(name string) (string, error) {
	dir, err := CacheDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, name), nil
}

// CaptureFilePath is where the latest official Snapshot is persisted.
func CaptureFilePath() (string, error) { return cacheFile("usage.json") }

// StateFilePath is where alert debounce state is persisted.
func StateFilePath() (string, error) { return cacheFile("alert-state.json") }

// ConfigPath is claude-status's own config (passthrough command, thresholds).
func ConfigPath() (string, error) { return cacheFile("config.json") }

// claudeConfigDir returns the Claude Code config dir (CLAUDE_CONFIG_DIR or ~/.claude).
func claudeConfigDir() (string, error) {
	if cfg := os.Getenv("CLAUDE_CONFIG_DIR"); cfg != "" {
		return cfg, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home dir: %w", err)
	}

	return filepath.Join(home, ".claude"), nil
}

// TranscriptsDir returns the Claude Code projects transcript directory.
func TranscriptsDir() (string, error) {
	dir, err := claudeConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "projects"), nil
}

// SettingsPath returns the Claude Code settings.json path.
func SettingsPath() (string, error) {
	dir, err := claudeConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "settings.json"), nil
}
