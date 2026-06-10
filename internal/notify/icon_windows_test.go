//go:build windows

package notify

import (
	"os"
	"sync"
	"testing"
)

func TestEmbeddedIcon(t *testing.T) {
	if len(iconPNG) < 8 || string(iconPNG[1:4]) != "PNG" {
		t.Fatalf("embedded icon is not a valid PNG (len=%d)", len(iconPNG))
	}
}

func TestIconFile_Materializes(t *testing.T) {
	iconOnce = sync.Once{}
	iconPath = ""

	t.Setenv("LocalAppData", t.TempDir()) // isolate os.UserCacheDir on Windows

	p := iconFile()
	if p == "" {
		t.Fatal("iconFile returned empty path")
	}

	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("icon not written: %v", err)
	}

	if info.Size() != int64(len(iconPNG)) {
		t.Fatalf("written icon size %d != embedded %d", info.Size(), len(iconPNG))
	}
}
