// SPDX-License-Identifier: GPL-3.0-only

//go:build !windows

package companion

import (
	"os/exec"
	"syscall"
)

func applyPlatformProcAttr(cmd *exec.Cmd, _ platformCommand) {
	// Detach helpers from the companion's process group so terminal or
	// browser processes are not signalled when the companion is stopped.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// EnsureConsole is a no-op outside Windows: the terminal emulator provides
// the controlling terminal.
func EnsureConsole() error { return nil }
