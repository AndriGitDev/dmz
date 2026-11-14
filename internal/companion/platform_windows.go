// SPDX-License-Identifier: GPL-3.0-only

//go:build windows

package companion

import (
	"os"
	"os/exec"
	"syscall"
)

const (
	platformCreateNewConsole    = 0x00000010
	platformCreateNoWindow      = 0x08000000
	platformAttachParentProcess = ^uint32(0)
	platformStdInputHandle      = ^uint32(10 - 1)
	platformStdOutputHandle     = ^uint32(11 - 1)
	platformStdErrorHandle      = ^uint32(12 - 1)
)

func applyPlatformProcAttr(cmd *exec.Cmd, c platformCommand) {
	attr := &syscall.SysProcAttr{}
	switch {
	case c.NewConsole:
		// No CREATE_NEW_PROCESS_GROUP: it would disable Ctrl+C for the
		// interactive coding client.
		attr.CreationFlags = platformCreateNewConsole
	case c.NoWindow:
		attr.CreationFlags = platformCreateNoWindow
		attr.HideWindow = true
	}
	cmd.SysProcAttr = attr
}

var (
	platformKernel32         = syscall.NewLazyDLL("kernel32.dll")
	platformAllocConsole     = platformKernel32.NewProc("AllocConsole")
	platformAttachConsole    = platformKernel32.NewProc("AttachConsole")
	platformGetConsoleWindow = platformKernel32.NewProc("GetConsoleWindow")
	platformSetStdHandle     = platformKernel32.NewProc("SetStdHandle")
)

// EnsureConsole gives a GUI-subsystem DMZ.exe a console for the connect
// session: it attaches to the parent's console when one exists (for example
// when run from a terminal), otherwise it allocates a new console window.
// Standard streams are then reopened on that console so the coding client
// inherits it.
func EnsureConsole() error {
	if hwnd, _, _ := platformGetConsoleWindow.Call(); hwnd == 0 {
		if r, _, _ := platformAttachConsole.Call(uintptr(platformAttachParentProcess)); r == 0 {
			if r, _, _ := platformAllocConsole.Call(); r == 0 {
				return ErrConsoleUnavailable
			}
		}
	}
	// A GUI process can have a console window while inheriting NUL handles.
	// Always reopen the streams on the attached console before client launch.
	in, err := os.OpenFile("CONIN$", os.O_RDWR, 0)
	if err != nil {
		return ErrConsoleUnavailable
	}
	out, err := os.OpenFile("CONOUT$", os.O_RDWR, 0)
	if err != nil {
		in.Close()
		return ErrConsoleUnavailable
	}
	platformSetStdHandle.Call(uintptr(platformStdInputHandle), in.Fd())
	platformSetStdHandle.Call(uintptr(platformStdOutputHandle), out.Fd())
	platformSetStdHandle.Call(uintptr(platformStdErrorHandle), out.Fd())
	os.Stdin, os.Stdout, os.Stderr = in, out, out
	return nil
}

func instanceProcessAlive(pid int) bool {
	// PROCESS_QUERY_LIMITED_INFORMATION works for the current user's GUI host.
	handle, err := syscall.OpenProcess(0x1000, false, uint32(pid))
	if err != nil {
		return err != syscall.Errno(87)
	} // ERROR_INVALID_PARAMETER: absent PID
	defer syscall.CloseHandle(handle)
	var code uint32
	if syscall.GetExitCodeProcess(handle, &code) != nil {
		return true
	}
	return code == 259 // STILL_ACTIVE
}
