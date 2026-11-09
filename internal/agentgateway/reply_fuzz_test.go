package agentgateway

import (
	"bytes"
	"io"
	"testing"
)

// FuzzRestoreReply checks that arbitrary replies never panic, that every
// read boundary gives the same output, and that a reply without "⟦" is
// forwarded byte-for-byte.
func FuzzRestoreReply(f *testing.F) {
	f.Add(responsesStream([]string{"Hæ ⟦EMAIL-t1-1: XX", "XXX@XXXXX.XXX⟧"}, splitText), uint8(7), false)
	f.Add(messagesStream([]string{"a " + mEmail[:5], mEmail[5:]}), uint8(3), true)
	f.Add([]byte("event: x\r\ndata: {\"type\":\"y\"}\r\r\n\n"), uint8(1), true)
	f.Fuzz(func(t *testing.T, in []byte, step uint8, messages bool) {
		proto := Responses
		if messages {
			proto = Messages
		}
		read := func(sizes []int) []byte {
			rc := RestoreReply(&chunkBody{data: bytes.Clone(in), sizes: sizes}, sseType, ReplyOptions{Protocol: proto, Restore: testRestore, MaxBytes: 4096})
			out, err := io.ReadAll(rc)
			if err != nil {
				t.Fatal(err)
			}
			return out
		}
		whole := read(nil)
		n := int(step%16) + 1
		var sizes []int
		for range len(in)/n + 1 {
			sizes = append(sizes, n)
		}
		if got := read(sizes); !bytes.Equal(got, whole) {
			t.Fatalf("read boundaries changed output:\n%q\n%q", got, whole)
		}
		if !bytes.Contains(in, []byte("⟦")) && !bytes.Contains(bytes.ToLower(in), []byte(`\u27e6`)) && !bytes.Equal(whole, in) {
			t.Fatalf("changed without markers:\n%q\n%q", in, whole)
		}
	})
}
