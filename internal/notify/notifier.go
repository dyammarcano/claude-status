// Package notify provides desktop notifications.
package notify

// Notifier sends a desktop notification.
type Notifier interface {
	Notify(title, body string) error
}
