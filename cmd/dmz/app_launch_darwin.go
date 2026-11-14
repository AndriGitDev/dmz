//go:build darwin

package main

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// Finder must see its short-lived launcher exit. A plain Go process has no
// Cocoa reopen event loop; retaining Finder's original process would make a
// second double-click do nothing after the browser window was closed.
func launchAppHost() (bool, error) {
	executable, err := os.Executable()
	if err != nil {
		return false, nil
	}
	if !strings.Contains(executable, ".app/Contents/MacOS/") {
		return false, nil
	}
	null, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return true, errors.New("app_window_unavailable")
	}
	defer null.Close()
	command := exec.Command(executable, "app")
	command.Stdin = null
	command.Stdout = null
	command.Stderr = null
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if home, err := os.UserHomeDir(); err == nil {
		command.Dir = home
	}
	if command.Start() != nil {
		return true, errors.New("app_window_unavailable")
	}
	command.Process.Release()
	return true, nil
}
