//go:build windows

package usage

import (
	"os/exec"
	"syscall"
)

// createNoWindow is the Windows CREATE_NO_WINDOW process creation flag. A child
// process spawned with it gets no console window, so the status line wrapper can
// run `node …gsd-statusline.js` without flashing a terminal on every render.
const createNoWindow = 0x08000000

// hideChildWindow ensures cmd's child process spawns without a visible console
// window (Windows only).
func hideChildWindow(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}

	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}

	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= createNoWindow
}
