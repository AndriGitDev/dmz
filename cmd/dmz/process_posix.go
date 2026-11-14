//go:build linux || darwin

package main

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"
	"unsafe"
)

// Keep an interactive child in the terminal's foreground process group. For
// pipe/print mode give the launch its own group so cancellation and final
// cleanup also reach tool/helper subprocesses owned by this launch.
func configureChild(command *exec.Cmd, input io.Reader, out, errOut io.Writer) func() {
	group := !terminalStream(input) && !terminalStream(out) && !terminalStream(errOut)
	if group {
		command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}
	command.Cancel = func() error {
		if command.Process == nil {
			return os.ErrProcessDone
		}
		pid := command.Process.Pid
		if group {
			pid = -pid
		}
		err := syscall.Kill(pid, syscall.SIGTERM)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	command.WaitDelay = 5 * time.Second
	return func() {
		if group && command.Process != nil {
			_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		}
	}
}

func terminalStream(stream any) bool {
	f, ok := stream.(*os.File)
	if !ok {
		return false
	}
	var size [4]uint16
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&size)))
	return err == 0
}

func lifecycleSignals() []os.Signal {
	return []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP}
}
