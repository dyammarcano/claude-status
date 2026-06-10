//go:build windows

package notify

import (
	_ "embed"
	"os"
	"path/filepath"
	"sync"

	"github.com/go-toast/toast"
)

// iconPNG is the claude-status toast icon, embedded into the binary.
//
//go:embed claude.png
var iconPNG []byte

var (
	iconOnce sync.Once
	iconPath string
)

// iconFile lazily materializes the embedded icon to a stable on-disk path. The
// Windows toast runtime loads images by file path (not bytes), so we write the
// embedded PNG once per process. Returns "" if it cannot be written, in which
// case callers simply toast without an icon.
func iconFile() string {
	iconOnce.Do(func() {
		base, err := os.UserCacheDir()
		if err != nil {
			return
		}

		dir := filepath.Join(base, "claude-status")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return
		}

		p := filepath.Join(dir, "icon.png")
		if err := os.WriteFile(p, iconPNG, 0o644); err != nil {
			return
		}

		iconPath = p
	})

	return iconPath
}

// WindowsToaster fires Windows toast notifications via go-toast.
type WindowsToaster struct {
	AppID string
}

// New returns a Notifier appropriate for the current platform.
func New(appID string) Notifier { return &WindowsToaster{AppID: appID} }

// Notify implements Notifier.
func (w *WindowsToaster) Notify(title, body string) error {
	n := toast.Notification{
		AppID:   w.AppID,
		Title:   title,
		Message: body,
	}

	if icon := iconFile(); icon != "" {
		n.Icon = icon
	}

	return n.Push()
}
