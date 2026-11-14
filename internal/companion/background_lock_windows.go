//go:build windows

package companion

import (
	"errors"
	"os"
	"syscall"
	"unsafe"
)

func lockBackground(file *os.File) error {
	var overlapped syscall.Overlapped
	proc := platformKernel32.NewProc("LockFileEx")
	result, _, _ := proc.Call(file.Fd(), 3, 0, 1, 0, uintptr(unsafe.Pointer(&overlapped)))
	if result == 0 {
		return errors.New("background_already_running")
	}
	return nil
}
