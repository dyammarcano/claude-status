//go:build !windows

package notify

import (
	"fmt"
	"os"
)

// StderrNotifier prints notifications to stderr (non-Windows platforms).
type StderrNotifier struct{}

// New returns a Notifier appropriate for the current platform.
func New(_ string) Notifier { return &StderrNotifier{} }

// Notify implements Notifier.
func (StderrNotifier) Notify(title, body string) error {
	_, err := fmt.Fprintf(os.Stderr, "[notify] %s — %s\n", title, body)
	return err
}
