//go:build linux || darwin

package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

type signalOutput struct {
	mu    sync.Mutex
	body  bytes.Buffer
	ready chan struct{}
}

func (w *signalOutput) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.body.Write(p)
	if strings.Contains(w.body.String(), "synthetic-child-ready") {
		select {
		case w.ready <- struct{}{}:
		default:
		}
	}
	return n, err
}

func TestCancellationAllowsClientCleanup(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	command := exec.CommandContext(ctx, executable)
	command.Env = setEnv(os.Environ(), "DMZ_SYNTHETIC_CHILD", "signal")
	captured := &signalOutput{ready: make(chan struct{}, 1)}
	command.Stdout = captured
	cleanup := configureChild(command, strings.NewReader(""), io.Discard, io.Discard)
	defer cleanup()
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	select {
	case <-captured.ready:
	case err := <-done:
		t.Fatalf("helper exited before ready: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("helper did not start")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(7 * time.Second):
		t.Fatal("client cleanup exceeded bounded cancellation")
	}
	captured.mu.Lock()
	defer captured.mu.Unlock()
	if command.ProcessState.ExitCode() != 0 || !strings.Contains(captured.body.String(), "synthetic-child-graceful-exit") {
		t.Fatal("client was killed before graceful cleanup")
	}
}
