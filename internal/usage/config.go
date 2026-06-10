package usage

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config is claude-status's own config for the statusline wrapper.
type Config struct {
	Passthrough string `json:"passthrough"` // downstream statusline command
	Thresholds  string `json:"thresholds"`  // e.g. "80,95" (empty → default)
}

// LoadConfig reads config from path; a missing file yields a zero Config.
func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, nil
		}

		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}

	return c, nil
}

// SaveConfig atomically writes config to path.
func SaveConfig(path string, c Config) error {
	return writeJSONAtomic(path, c)
}

// ReadStatuslineCommand returns settings.json's statusLine.command, or "" if absent.
func ReadStatuslineCommand(settingsPath string) (string, error) {
	m, err := readSettings(settingsPath)
	if err != nil {
		return "", err
	}

	sl, ok := m["statusLine"].(map[string]any)
	if !ok {
		return "", nil
	}

	cmd, _ := sl["command"].(string)

	return cmd, nil
}

// WriteStatuslineCommand sets settings.json's statusLine.command, preserving all
// other keys.
func WriteStatuslineCommand(settingsPath, command string) error {
	m, err := readSettings(settingsPath)
	if err != nil {
		return err
	}

	sl, ok := m["statusLine"].(map[string]any)
	if !ok {
		sl = map[string]any{"type": "command"}
	}

	sl["command"] = command
	m["statusLine"] = sl

	return writeJSONAtomic(settingsPath, m)
}

func readSettings(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}

		return nil, fmt.Errorf("read settings: %w", err)
	}

	m := map[string]any{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("decode settings: %w", err)
	}

	return m, nil
}
