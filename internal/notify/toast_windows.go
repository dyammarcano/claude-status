//go:build windows

package notify

import "github.com/go-toast/toast"

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

	return n.Push()
}
