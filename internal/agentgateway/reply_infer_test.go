package agentgateway

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestInferredReplyPreservesUnknownBytes(t *testing.T) {
	for _, body := range []string{"plain " + mEmail, "<html>" + mEmail + "</html>", strings.Repeat(" ", 4097) + mEmail, `{"bad":` + mEmail, "data: not-json\n\n", `{"error":"` + mEmail + `"}`} {
		var warnings []string
		r := RestoreReply(io.NopCloser(strings.NewReader(body)), "application/octet-stream", ReplyOptions{Protocol: Responses, Restore: testRestore, InferContentType: true, Warning: func(s string) { warnings = append(warnings, s) }})
		got, err := io.ReadAll(r)
		r.Close()
		if err != nil || string(got) != body {
			t.Fatal("unrecognized body changed")
		}
	}
}

func TestInferenceKeepsReadErrors(t *testing.T) {
	wantErr := errors.New("synthetic read failure")
	r := RestoreReply(&chunkBody{data: []byte("dat"), err: wantErr}, "", ReplyOptions{Protocol: Responses, Restore: testRestore, InferContentType: true})
	got, err := io.ReadAll(r)
	r.Close()
	if string(got) != "dat" || !errors.Is(err, wantErr) {
		t.Fatal("probe lost bytes or read error")
	}
}

func TestInferredStreamEmitsBeforeEOFAndCloseCancelsProbe(t *testing.T) {
	for _, probeOnly := range []bool{false, true} {
		upstream, write := io.Pipe()
		r := RestoreReply(upstream, "text/plain", ReplyOptions{Protocol: Responses, Restore: testRestore, InferContentType: true})
		done := make(chan error, 1)
		go func() { b := make([]byte, 128); _, err := r.Read(b); done <- err }()
		if !probeOnly {
			if _, err := io.WriteString(write, ": ping\n\n"); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("inferred SSE waited for EOF")
			}
		}
		r.Close()
		write.Close()
		if probeOnly {
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("Close did not cancel format probe")
			}
		}
	}
}
