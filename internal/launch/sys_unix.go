//go:build unix

package launch

import (
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/unix"
)

// terminalSize reports the current terminal's columns and rows (0, 0 if unknown).
func terminalSize() (int, int) {
	ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return 0, 0
	}
	return int(ws.Col), int(ws.Row)
}

// detach starts the new terminal in its own session so closing this shell doesn't close it.
func detach(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }
