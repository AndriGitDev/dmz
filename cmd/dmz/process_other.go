//go:build !linux && !darwin

package main

import (
	"io"
	"os/exec"
	"time"
)

// Other platforms retain Go's direct-child cancellation. A Windows job-object
// implementation and native terminal lifecycle verification remain pending.
func configureChild(command *exec.Cmd, _ io.Reader, _ io.Writer, _ io.Writer) func() {
	command.WaitDelay = 5 * time.Second
	return func() {}
}
