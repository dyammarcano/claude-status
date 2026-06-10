package usage

import (
	"path/filepath"
	"strings"
	"testing"
)

// isolateCache points os.UserCacheDir at a temp dir on both Unix (XDG_CACHE_HOME)
// and Windows (LocalAppData) so cache paths are deterministic and side-effect free.
func isolateCache(t *testing.T) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmp)
	t.Setenv("LocalAppData", tmp)
}

func TestCacheAndCacheFiles(t *testing.T) {
	isolateCache(t)

	dir, err := CacheDir()
	if err != nil {
		t.Fatalf("CacheDir: %v", err)
	}

	if filepath.Base(dir) != appDir {
		t.Fatalf("CacheDir base = %q, want %q", filepath.Base(dir), appDir)
	}

	cases := []struct {
		name string
		fn   func() (string, error)
		want string
	}{
		{"capture", CaptureFilePath, "usage.json"},
		{"state", StateFilePath, "alert-state.json"},
		{"config", ConfigPath, "config.json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := tc.fn()
			if err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}

			if filepath.Base(p) != tc.want {
				t.Fatalf("%s = %q, want base %q", tc.name, p, tc.want)
			}
		})
	}
}

func TestConfigDirPaths_EnvOverride(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", cfg)

	tdir, err := TranscriptsDir()
	if err != nil {
		t.Fatalf("TranscriptsDir: %v", err)
	}

	if tdir != filepath.Join(cfg, "projects") {
		t.Fatalf("TranscriptsDir = %q", tdir)
	}

	sp, err := SettingsPath()
	if err != nil {
		t.Fatalf("SettingsPath: %v", err)
	}

	if sp != filepath.Join(cfg, "settings.json") {
		t.Fatalf("SettingsPath = %q", sp)
	}
}

func TestConfigDir_HomeFallback(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "") // empty → falls back to ~/.claude

	sp, err := SettingsPath()
	if err != nil {
		t.Fatalf("SettingsPath: %v", err)
	}

	if filepath.Base(sp) != "settings.json" {
		t.Fatalf("SettingsPath base = %q", filepath.Base(sp))
	}
}

func TestDefaultShellRunner(t *testing.T) {
	cmd := DefaultShellRunner("echo hi")
	if cmd == nil || len(cmd.Args) == 0 {
		t.Fatal("expected a non-empty command")
	}

	if !strings.Contains(strings.Join(cmd.Args, " "), "echo hi") {
		t.Fatalf("args missing command: %v", cmd.Args)
	}
}
