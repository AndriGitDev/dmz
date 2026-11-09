package agentgateway

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"strings"
	"testing"
)

func TestReplyJSONResponses(t *testing.T) {
	in := `{"id":"resp_1","object":"response","instructions":"Write to ` + mEmail + `",` +
		`"output":[` +
		`{"type":"reasoning","id":"rs_1","encrypted_content":"enc ` + mEmail + `","summary":[{"type":"summary_text","text":"` + mEmail + `"}]},` +
		`{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[` +
		`{"type":"output_text","text":"Hæ ⟦EMAIL-t1-1: XXXXX@XXXXX.XXX⟧ \/ ` + mName + ` ` + mUnknown + `","annotations":[{"type":"url_citation","title":"` + mEmail + `"}],"logprobs":[]},` +
		`{"type":"refusal","refusal":"` + mEmail + `"},` +
		`{"type":"output_text","text":"plain \"quoted\" æ"}]},` +
		`{"type":"message","id":"msg_2","role":"user","content":[{"type":"output_text","text":"` + mEmail + `"}]},` +
		`{"type":"function_call","id":"fc_1","name":"lookup","arguments":"{\"q\":\"` + mEmail + `\"}"},` +
		`{"type":"function_call_output","call_id":"c","output":"` + mEmail + `"}` +
		`],"metadata":{"note":"` + mEmail + `"},"usage":{"input_tokens":1}}`
	want := strings.Replace(in, `"Hæ ⟦EMAIL-t1-1: XXXXX@XXXXX.XXX⟧ \/ `+mName+` `+mUnknown+`"`,
		`"Hæ `+oEmail+` / `+oName+` `+mUnknown+`"`, 1)
	r := runReply(t, Responses, "application/json; charset=utf-8", []byte(in), []int{7, 3}, testRestore)
	if r.err != nil {
		t.Fatal(r.err)
	}
	if string(r.out) != want {
		t.Fatalf("got\n%s\nwant\n%s", r.out, want)
	}
	assertWarnings(t, r.warnings)
}

func TestReplyJSONMessages(t *testing.T) {
	in := `{"id":"msg_1","type":"message","role":"assistant","model":"m",` +
		`"content":[` +
		`{"type":"thinking","thinking":"` + mEmail + `","signature":"c2lnOiDin6ZFTUFJTA=="},` +
		`{"type":"redacted_thinking","data":"` + mEmail + `"},` +
		`{"type":"text","text":"Til ` + mEmail + `","citations":[{"type":"char_location","cited_text":"` + mEmail + `"}]},` +
		`{"type":"tool_use","id":"toolu_1","name":"lookup","input":{"q":"` + mEmail + `"}},` +
		`{"type":"text","text":"` + mName + `\n"}],` +
		`"stop_reason":"stop_sequence","stop_sequence":"` + mEmail + `","usage":{"input_tokens":1}}`
	want := strings.Replace(in, `"text":"Til `+mEmail+`"`, `"text":"Til `+oEmail+`"`, 1)
	want = strings.Replace(want, `"text":"`+mName+`\n"`, `"text":"`+oName+`\n"`, 1)
	r := runReply(t, Messages, "application/json", []byte(in), nil, testRestore)
	if string(r.out) != want {
		t.Fatalf("got\n%s\nwant\n%s", r.out, want)
	}
	assertWarnings(t, r.warnings)

	// Non-assistant roots, error bodies and token counts are unchanged.
	for _, body := range []string{
		`{"type":"message","role":"user","content":[{"type":"text","text":"` + mEmail + `"}]}`,
		`{"type":"error","error":{"type":"invalid_request_error","message":"` + mEmail + `"}}`,
		`{"input_tokens":12}`,
	} {
		r := runReply(t, Messages, "application/json", []byte(body), nil, testRestore)
		if string(r.out) != body {
			t.Fatalf("changed %s", r.out)
		}
		assertWarnings(t, r.warnings)
	}
}

func TestReplyJSONFailOpen(t *testing.T) {
	text := `{"type":"output_text","text":"` + mEmail + `"}`
	msg := `{"output":[{"type":"message","role":"assistant","content":[` + text + `]}]}`
	for _, tc := range []struct{ name, body, warn string }{
		{"invalid", msg[:len(msg)-1], WarnReplyInvalidJSON},
		{"trailing", msg + " {}", WarnReplyInvalidJSON},
		{"duplicate", `{"output":[],"output":` + msg[10:], WarnReplyInvalidJSON},
		{"lone surrogate", strings.Replace(msg, mEmail, `\ud800`+mEmail, 1), WarnReplyInvalidJSON},
		{"invalid utf8", strings.Replace(msg, mEmail, "\xff"+mEmail, 1), WarnReplyInvalidJSON},
		{"too deep", strings.Repeat("[", 300) + strings.Repeat("]", 300), WarnReplyInvalidJSON},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := runReply(t, Responses, "application/json", []byte(tc.body), nil, testRestore)
			if string(r.out) != tc.body {
				t.Fatalf("changed")
			}
			assertWarnings(t, r.warnings, tc.warn)
		})
	}

	// Oversized bodies are forwarded unchanged without buffering the rest.
	big := msg + strings.Repeat(" ", 4096)
	var warnings []string
	body := &chunkBody{data: []byte(big), sizes: []int{100}}
	rc := RestoreReply(body, "application/json", ReplyOptions{Protocol: Responses, Restore: testRestore, MaxBytes: 1024, Warning: func(c string) { warnings = append(warnings, c) }})
	out, err := io.ReadAll(rc)
	if err != nil || string(out) != big {
		t.Fatalf("oversized: %v %d", err, len(out))
	}
	assertWarnings(t, warnings, WarnReplyTooLarge)
	rc.Close()
	if body.closed != 1 {
		t.Fatal("not closed")
	}

	// A read error forwards the bytes received, then the error.
	rc = RestoreReply(&chunkBody{data: []byte(msg[:20]), err: io.ErrUnexpectedEOF}, "application/json", ReplyOptions{Protocol: Responses, Restore: testRestore})
	out, err = io.ReadAll(rc)
	if err != io.ErrUnexpectedEOF || string(out) != msg[:20] {
		t.Fatalf("read error: %v %q", err, out)
	}
}

func TestReplyUnsupportedPassThrough(t *testing.T) {
	body := io.NopCloser(bytes.NewReader([]byte(mEmail)))
	for _, tc := range []struct {
		ct    string
		proto Protocol
		warn  string
	}{
		{"text/plain", Responses, WarnReplyUnsupportedContentType},
		{"", Messages, WarnReplyUnsupportedContentType},
		{"application/json", "chat", WarnReplyUnsupportedProtocol},
	} {
		var warnings []string
		rc := RestoreReply(body, tc.ct, ReplyOptions{Protocol: tc.proto, Restore: testRestore, Warning: func(c string) { warnings = append(warnings, c) }})
		if rc != body {
			t.Fatalf("%q: body was wrapped", tc.ct)
		}
		assertWarnings(t, warnings, tc.warn)
	}
	if RestoreReply(body, "application/json", ReplyOptions{Protocol: Responses}) != body {
		t.Fatal("nil Restore must not wrap")
	}
	if RestoreReply(nil, "application/json", ReplyOptions{Protocol: Responses, Restore: testRestore}) != nil {
		t.Fatal("nil body")
	}
}

func TestReplyJSONCallbackFailures(t *testing.T) {
	in := `{"type":"message","role":"assistant","content":[{"type":"text","text":"` + mEmail + `"},{"type":"text","text":"` + mName + `"}]}`
	calls := 0
	r := runReply(t, Messages, "application/json", []byte(in), nil, func(s string) string {
		calls++
		if calls == 1 {
			panic("synthetic")
		}
		return testRestore(s)
	})
	want := strings.Replace(in, mName, oName, 1)
	if string(r.out) != want {
		t.Fatalf("got %s", r.out)
	}
	assertWarnings(t, r.warnings, WarnReplyRestorePanic)
}

func TestPartialMarkerLen(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want int
	}{
		{"", 0},
		{"abc", 0},
		{"a ⟦", 3},
		{"a ⟦EM", 5},
		{mEmail, 0},
		{mEmail + " ⟦N", 4},
		{"⟦a⟦b", 4},
		{"⟦" + strings.Repeat("x", replyMarkerCarry-4), replyMarkerCarry - 1},
		{"⟦" + strings.Repeat("x", replyMarkerCarry-3), 0},
		{"⟧ ⟦", 3},
	} {
		if got := partialMarkerLen(tc.in); got != tc.want {
			t.Errorf("partialMarkerLen(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestReplyLimitsBoundState(t *testing.T) {
	// A huge MaxBytes must not overflow the JSON limit arithmetic.
	in := `{"type":"message","role":"assistant","content":[{"type":"text","text":"` + mEmail + `"}]}`
	rc := RestoreReply(io.NopCloser(strings.NewReader(in)), "application/json", ReplyOptions{Protocol: Messages, Restore: testRestore, MaxBytes: math.MaxInt64})
	if out, err := io.ReadAll(rc); err != nil || string(out) != strings.Replace(in, mEmail, oEmail, 1) {
		t.Fatalf("%v %s", err, out)
	}

	// Announced items beyond maxReplyStreams are forwarded unchanged.
	var b strings.Builder
	for i := range maxReplyStreams + 1 {
		b.WriteString(ev("response.output_item.added", "output_index", i, "item", obj{"type": "message", "id": fmt.Sprint("m", i), "role": "assistant"}))
	}
	last := ev("response.output_text.delta", "item_id", fmt.Sprint("m", maxReplyStreams), "output_index", maxReplyStreams, "content_index", 0, "delta", mEmail)
	first := ev("response.output_text.delta", "item_id", "m0", "output_index", 0, "content_index", 0, "delta", mEmail)
	b.WriteString(last + first)
	r := runReply(t, Responses, "text/event-stream", []byte(b.String()), nil, testRestore)
	if !strings.HasSuffix(string(r.out), last+strings.Replace(first, mEmail, oEmail, 1)) {
		t.Fatal("cap not applied")
	}
	assertWarnings(t, r.warnings, WarnReplyBufferLimit)
}
