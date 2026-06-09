package cmd

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdout redirects os.Stdout for the duration of fn and returns what was written.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	orig := os.Stdout

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}

	os.Stdout = w

	done := make(chan string, 1)

	go func() {
		var sb strings.Builder

		_, _ = io.Copy(&sb, r)
		done <- sb.String()
	}()

	fn()

	_ = w.Close()
	os.Stdout = orig

	return <-done
}

func TestGetVersionInfo(t *testing.T) {
	info := GetVersionInfo()
	if info == nil {
		t.Fatal("GetVersionInfo returned nil")
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"version", info.Version, Version},
		{"git_hash", info.GitHash, GitHash},
		{"build_time", info.BuildTime, BuildTime},
		{"build_hash", info.BuildHash, BuildHash},
		{"go_version", info.GoVersion, GoVersion},
		{"goos", info.GoOS, GOOS},
		{"goarch", info.GoArch, GOARCH},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("field %s = %q, want %q", tt.name, tt.got, tt.want)
			}
		})
	}
}

func TestGetVersionJSON(t *testing.T) {
	out := GetVersionJSON()
	if out == "" || out == "{}" {
		t.Fatalf("GetVersionJSON returned empty/error result: %q", out)
	}

	var back VersionInfo
	if err := json.Unmarshal([]byte(out), &back); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	want := GetVersionInfo()
	if back != *want {
		t.Errorf("round-tripped VersionInfo = %+v, want %+v", back, *want)
	}
}

func TestRunVersion(t *testing.T) {
	t.Run("json", func(t *testing.T) {
		orig := jsonOutput
		jsonOutput = true

		defer func() { jsonOutput = orig }()

		out := captureStdout(t, func() {
			runVersion(versionCmd, nil)
		})

		var back VersionInfo
		if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &back); err != nil {
			t.Fatalf("output is not valid JSON: %v\noutput: %q", err, out)
		}

		if back.Version != Version {
			t.Errorf("Version = %q, want %q", back.Version, Version)
		}
	})

	t.Run("plain", func(t *testing.T) {
		orig := jsonOutput
		jsonOutput = false

		defer func() { jsonOutput = orig }()

		out := captureStdout(t, func() {
			runVersion(versionCmd, nil)
		})

		for _, want := range []string{"Version:", "Git Hash:", "Build Time:", "Build Hash:", "Go Version:", "OS/Arch:"} {
			if !strings.Contains(out, want) {
				t.Errorf("output missing %q\noutput: %q", want, out)
			}
		}

		if !strings.Contains(out, Version) {
			t.Errorf("output missing version value %q", Version)
		}
	})
}

func TestPrintVersion(t *testing.T) {
	out := captureStdout(t, printVersion)

	if !strings.Contains(out, "Version:") {
		t.Errorf("printVersion output missing 'Version:'\noutput: %q", out)
	}

	if !strings.Contains(out, GOARCH) {
		t.Errorf("printVersion output missing GOARCH %q", GOARCH)
	}
}
