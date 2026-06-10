//go:build !windows

package usage

import "os/exec"

// hideChildWindow is a no-op on non-Windows platforms (no console windows to hide).
func hideChildWindow(_ *exec.Cmd) {}
