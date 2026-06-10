//go:build windows

package notify

import "testing"

// TestNew verifies that New constructs a non-nil Notifier backed by a
// *WindowsToaster carrying the supplied AppID.
//
// Notify is intentionally NOT exercised here: on Windows it calls
// toast.Push(), which fires a REAL OS toast notification and has no
// injectable seam to stub out. There is no way to cover Notify without
// triggering a real desktop notification, so it is left uncovered.
func TestNew(t *testing.T) {
	tests := []struct {
		name  string
		appID string
	}{
		{name: "typical app id", appID: "claude-status"},
		{name: "empty app id", appID: ""},
		{name: "namespaced app id", appID: "com.inovacc.claude-status"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := New(tt.appID)
			if n == nil {
				t.Fatal("New() returned nil Notifier")
			}

			w, ok := n.(*WindowsToaster)
			if !ok {
				t.Fatalf("New() returned %T, want *WindowsToaster", n)
			}

			if w.AppID != tt.appID {
				t.Errorf("AppID = %q, want %q", w.AppID, tt.appID)
			}
		})
	}
}
