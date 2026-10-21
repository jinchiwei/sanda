//go:build !unix

package launch

import "os/exec"

// Separate windows aren't supported here; Open refuses before these are reached.
func terminalSize() (int, int) { return 0, 0 }

func detach(*exec.Cmd) {}
