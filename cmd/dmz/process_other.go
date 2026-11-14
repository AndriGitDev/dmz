//go:build !linux && !darwin

package main

import (
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// Other platforms retain Go's direct-child cancellation. A Windows job-object
// implementation and native terminal lifecycle verification remain pending.
func configureChild(command *exec.Cmd, _ io.Reader, _ io.Writer, _ io.Writer) func() {
	command.WaitDelay = 5 * time.Second
	return func() {}
}

func lifecycleSignals() []os.Signal { return []os.Signal{os.Interrupt, syscall.SIGTERM} }

func terminalStream(stream any) bool {
	f, ok := stream.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
