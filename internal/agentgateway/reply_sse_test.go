package agentgateway

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"runtime"
	"strings"
	"testing"
	"time"
)

const sseType = "text/event-stream"

// responsesStream builds a Responses stream with reasoning, one assistant
// message streamed as deltas, an annotation, and a function call. full is
// the text used in done/completed events.
func responsesStream(deltas []string, full string) []byte {
	reasoning := obj{"type": "reasoning", "id": "rs_1", "encrypted_content": "enc:" + mEmail, "summary": arr{obj{"type": "summary_text", "text": "Looked at " + mEmail}}}
	call := obj{"type": "function_call", "id": "fc_1", "call_id": "call_1", "name": "lookup", "arguments": `{"q":"` + mEmail + `"}`, "status": "completed"}
	msg := func(text string, status string) obj {
		content := arr{}
		if status == "completed" {
			content = arr{obj{"type": "output_text", "text": text, "annotations": arr{obj{"type": "url_citation", "title": mEmail, "url": "https://example.test/"}}}}
		}
		return obj{"type": "message", "id": "msg_1", "status": status, "role": "assistant", "content": content}
	}
	base := obj{"id": "resp_1", "object": "response", "status": "in_progress", "instructions": "Write to " + mEmail, "output": arr{}}
	seq := 0
	next := func() int { seq++; return seq }
	var b strings.Builder
	b.WriteString(ev("response.created", "response", base, "sequence_number", next()))
	b.WriteString(": keep-alive comment " + mEmail + "\n\n")
	b.WriteString(ev("response.output_item.added", "output_index", 0, "item", obj{"type": "reasoning", "id": "rs_1", "summary": arr{}}, "sequence_number", next()))
	b.WriteString(ev("response.reasoning_summary_text.delta", "item_id", "rs_1", "output_index", 0, "summary_index", 0, "delta", "Looked at "+mEmail, "sequence_number", next()))
	b.WriteString(ev("response.output_item.done", "output_index", 0, "item", reasoning, "sequence_number", next()))
	b.WriteString(ev("response.output_item.added", "output_index", 1, "item", msg("", "in_progress"), "sequence_number", next()))
	b.WriteString(ev("response.content_part.added", "item_id", "msg_1", "output_index", 1, "content_index", 0, "part", obj{"type": "output_text", "text": "", "annotations": arr{}}, "sequence_number", next()))
	for i, d := range deltas {
		b.WriteString(ev("response.output_text.delta", "item_id", "msg_1", "output_index", 1, "content_index", 0, "delta", d, "logprobs", arr{}, "obfuscation", fmt.Sprintf("pad%d", i), "sequence_number", next()))
		if i == 0 {
			b.WriteString(ev("response.refusal.delta", "item_id", "msg_1", "output_index", 1, "content_index", 1, "delta", mEmail, "sequence_number", next()))
		}
	}
	b.WriteString(ev("response.output_text.annotation.added", "item_id", "msg_1", "output_index", 1, "content_index", 0, "annotation_index", 0, "annotation", obj{"type": "url_citation", "title": mEmail}, "sequence_number", next()))
	b.WriteString(ev("response.output_text.done", "item_id", "msg_1", "output_index", 1, "content_index", 0, "text", full, "logprobs", arr{}, "sequence_number", next()))
	b.WriteString(ev("response.content_part.done", "item_id", "msg_1", "output_index", 1, "content_index", 0, "part", obj{"type": "output_text", "text": full, "annotations": arr{}}, "sequence_number", next()))
	b.WriteString(ev("response.output_item.done", "output_index", 1, "item", msg(full, "completed"), "sequence_number", next()))
	b.WriteString(ev("response.output_item.added", "output_index", 2, "item", obj{"type": "function_call", "id": "fc_1", "call_id": "call_1", "name": "lookup", "arguments": ""}, "sequence_number", next()))
	b.WriteString(ev("response.function_call_arguments.delta", "item_id", "fc_1", "output_index", 2, "delta", `{"q":"`+mEmail[:9], "sequence_number", next()))
	b.WriteString(ev("response.function_call_arguments.delta", "item_id", "fc_1", "output_index", 2, "delta", mEmail[9:]+`"}`, "sequence_number", next()))
	b.WriteString(ev("response.output_item.done", "output_index", 2, "item", call, "sequence_number", next()))
	done := obj{}
	for k, v := range base {
		done[k] = v
	}
	done["status"] = "completed"
	done["output"] = arr{reasoning, msg(full, "completed"), call}
	b.WriteString(ev("response.completed", "response", done, "sequence_number", next()))
	return []byte(b.String())
}

// messagesStream builds a Messages stream with thinking, one text block
// streamed as deltas, a tool_use block and trailing events.
func messagesStream(deltas []string) []byte {
	var b strings.Builder
	b.WriteString(ev("message_start", "message", obj{"id": "msg_1", "type": "message", "role": "assistant", "content": arr{}, "stop_reason": nil, "usage": obj{"input_tokens": 3, "output_tokens": 1}}))
	b.WriteString(ev("content_block_start", "index", 0, "content_block", obj{"type": "thinking", "thinking": "", "signature": ""}))
	b.WriteString(ev("content_block_delta", "index", 0, "delta", obj{"type": "thinking_delta", "thinking": "Consider " + mEmail[:7]}))
	b.WriteString(ev("content_block_delta", "index", 0, "delta", obj{"type": "thinking_delta", "thinking": mEmail[7:]}))
	b.WriteString(ev("content_block_delta", "index", 0, "delta", obj{"type": "signature_delta", "signature": "c2lnOiDin6ZFTUFJTA=="}))
	b.WriteString(ev("content_block_stop", "index", 0))
	b.WriteString(ev("content_block_start", "index", 1, "content_block", obj{"type": "text", "text": ""}))
	for i, d := range deltas {
		b.WriteString(ev("content_block_delta", "index", 1, "delta", obj{"type": "text_delta", "text": d}))
		if i == 0 {
			b.WriteString(ev("ping"))
		}
	}
	b.WriteString(ev("content_block_stop", "index", 1))
	b.WriteString(ev("content_block_start", "index", 2, "content_block", obj{"type": "tool_use", "id": "toolu_1", "name": "lookup", "input": obj{}}))
	b.WriteString(ev("content_block_delta", "index", 2, "delta", obj{"type": "input_json_delta", "partial_json": `{"q":"` + mEmail}))
	b.WriteString(ev("content_block_delta", "index", 2, "delta", obj{"type": "input_json_delta", "partial_json": `"}`}))
	b.WriteString(ev("content_block_stop", "index", 2))
	b.WriteString(ev("message_delta", "delta", obj{"stop_reason": "tool_use", "stop_sequence": mEmail}, "usage", obj{"output_tokens": 9}))
	b.WriteString(ev("message_stop"))
	return []byte(b.String())
}

// checkStream verifies out against the input stream and the expected
// stream (in with complete text fields restored). Delta frames may move
// text between frames of one channel but must keep every other byte.
func checkStream(t *testing.T, in, want, out []byte) {
	t.Helper()
	mustValidUTF8(t, out)
	assertSameFrames(t, in, out, true)
	fw, fo := splitFrames(want), splitFrames(out)
	for i := range fw {
		if fw[i] == fo[i] {
			continue
		}
		_, m := frameJSON(t, fw[i])
		if m != nil {
			switch m["type"] {
			case "response.output_text.delta", "content_block_delta", "content_block_start":
				continue
			}
		}
		if fw[i] != fo[i] {
			t.Fatalf("frame %d:\n got=%q\nwant=%q", i, fo[i], fw[i])
		}
	}
	gotText, inText := channelText(t, out), channelText(t, in)
	for k, v := range inText {
		if gotText[k] != testRestore(v) {
			t.Fatalf("channel %s text %q, want %q", k, gotText[k], testRestore(v))
		}
	}
}

const splitText = "Hæ " + mEmail + " og " + mName + "."

func TestReplySSEResponsesEverySplit(t *testing.T) {
	cuts := runeCuts(splitText)
	for a := 0; a < len(cuts); a++ {
		t.Run(fmt.Sprint(cuts[a]), func(t *testing.T) {
			t.Parallel()
			for b := a; b < len(cuts); b++ {
				i, j := cuts[a], cuts[b]
				deltas := []string{splitText[:i], splitText[i:j], splitText[j:]}
				in := responsesStream(deltas, splitText)
				wantN := responsesStream(deltas, testRestore(splitText))
				r := runReply(t, Responses, sseType, in, nil, testRestore)
				if r.err != nil {
					t.Fatal(r.err)
				}
				assertWarnings(t, r.warnings)
				checkStream(t, in, wantN, r.out)
			}
		})
	}
}

func TestReplySSEMessagesEverySplit(t *testing.T) {
	cuts := runeCuts(splitText)
	for a := 0; a < len(cuts); a++ {
		t.Run(fmt.Sprint(cuts[a]), func(t *testing.T) {
			t.Parallel()
			for b := a; b < len(cuts); b++ {
				i, j := cuts[a], cuts[b]
				in := messagesStream([]string{splitText[:i], splitText[i:j], splitText[j:]})
				r := runReply(t, Messages, sseType+"; charset=utf-8", in, nil, testRestore)
				if r.err != nil {
					t.Fatal(r.err)
				}
				assertWarnings(t, r.warnings)
				checkStream(t, in, in, r.out)
			}
		})
	}
}

func TestReplySSEEveryNetworkSplit(t *testing.T) {
	for _, tc := range []struct {
		proto Protocol
		in    []byte
		want  []byte
	}{
		{Responses, responsesStream([]string{"Hæ ⟦EMAIL-t1-1: XX", "XXX@XXXXX.XXX⟧ og ", mName + "."}, splitText), responsesStream([]string{"Hæ ⟦EMAIL-t1-1: XX", "XXX@XXXXX.XXX⟧ og ", mName + "."}, testRestore(splitText))},
		{Messages, messagesStream([]string{"Hæ ⟦EMAIL-t1-1: XX", "XXX@XXXXX.XXX⟧ og ", mName + "."}), nil},
	} {
		want := tc.want
		if want == nil {
			want = tc.in
		}
		// Every read boundary must give exactly the verified single-read
		// output.
		ref := runReply(t, tc.proto, sseType, tc.in, nil, testRestore)
		checkStream(t, tc.in, want, ref.out)
		for cut := 0; cut <= len(tc.in); cut++ {
			r := runReply(t, tc.proto, sseType, tc.in, []int{cut, len(tc.in)}, testRestore)
			if r.err != nil || !bytes.Equal(r.out, ref.out) {
				t.Fatalf("cut %d: %v\n%s", cut, r.err, r.out)
			}
		}
		ones := make([]int, len(tc.in))
		for i := range ones {
			ones[i] = 1
		}
		if r := runReply(t, tc.proto, sseType, tc.in, ones, testRestore); !bytes.Equal(r.out, ref.out) {
			t.Fatalf("one-byte reads:\n%s", r.out)
		}
	}
}

func TestReplySSEOpaqueEventsUnchanged(t *testing.T) {
	in := responsesStream([]string{mEmail}, mEmail)
	r := runReply(t, Responses, sseType, in, nil, testRestore)
	out := string(r.out)
	for _, keep := range []string{
		`"instructions":"Write to ` + mEmail,
		`"encrypted_content":"enc:` + mEmail,
		`"delta":"Looked at ` + mEmail,
		`"delta":"` + mEmail + `","item_id":"msg_1"`,
		`"title":"` + mEmail,
		`"arguments":"{\"q\":\"` + mEmail,
		": keep-alive comment " + mEmail,
	} {
		if !strings.Contains(out, keep) {
			t.Fatalf("missing unchanged %q in %s", keep, out)
		}
	}
	if !strings.Contains(out, `"delta":"`+oEmail+`"`) || !strings.Contains(out, `"text":"`+oEmail+`"`) {
		t.Fatalf("assistant text not restored: %s", out)
	}
	// Opaque Messages blocks stay byte-identical; only text deltas change.
	min := messagesStream([]string{mEmail})
	r = runReply(t, Messages, sseType, min, nil, testRestore)
	checkStream(t, min, min, r.out)
	for _, keep := range []string{`"thinking":"Consider ⟦EMAI"`, `"partial_json":"{\"q\":\"` + mEmail, `"stop_sequence":"` + mEmail} {
		if !strings.Contains(string(r.out), keep) {
			t.Fatalf("missing unchanged %q", keep)
		}
	}
	if !strings.Contains(string(r.out), `"text":"`+oEmail+`"`) {
		t.Fatalf("text delta not restored: %s", r.out)
	}
}

func TestReplySSEUnknownEditedTruncatedMarkers(t *testing.T) {
	text := "A " + mUnknown + " B ⟦EMAIL-t1-1: XXXXX@XXXXX.XX⟧ C " + mEmail + " D ⟦EMAIL-t1-1: XX"
	cuts := runeCuts(text)
	for a := 0; a < len(cuts); a++ {
		deltas := []string{text[:cuts[a]], text[cuts[a]:]}
		in := responsesStream(deltas, text)
		r := runReply(t, Responses, sseType, in, nil, testRestore)
		checkStream(t, in, responsesStream(deltas, testRestore(text)), r.out)
		got := channelText(t, r.out)["1:0"]
		if want := "A " + mUnknown + " B ⟦EMAIL-t1-1: XXXXX@XXXXX.XX⟧ C " + oEmail + " D ⟦EMAIL-t1-1: XX"; got != want {
			t.Fatalf("text %q, want %q", got, want)
		}
	}
}

func TestReplySSETwoPartialMarkersAcrossManyDeltas(t *testing.T) {
	// Every delta is one rune, so both markers stay partial for many frames
	// and the anchor moves repeatedly.
	var deltas []string
	for _, r := range splitText + " " + mEmail {
		deltas = append(deltas, string(r))
	}
	in := messagesStream(deltas)
	r := runReply(t, Messages, sseType, in, nil, testRestore)
	checkStream(t, in, in, r.out)
	if got := channelText(t, r.out)["1"]; got != testRestore(splitText+" "+mEmail) {
		t.Fatalf("got %q", got)
	}
}

func TestReplySSEInterleavedChannelsDoNotJoin(t *testing.T) {
	half := len("⟦EMAIL-t1-1: XX")
	var b strings.Builder
	b.WriteString(ev("response.output_item.added", "output_index", 0, "item", obj{"type": "message", "id": "m0", "role": "assistant", "content": arr{}}))
	b.WriteString(ev("response.output_item.added", "output_index", 1, "item", obj{"type": "message", "id": "m1", "role": "assistant", "content": arr{}}))
	// Channel 0:0 holds the first half of a marker; the second half arrives
	// on other channels first and must not complete it.
	b.WriteString(ev("response.output_text.delta", "item_id", "m0", "output_index", 0, "content_index", 0, "delta", "x "+mEmail[:half]))
	b.WriteString(ev("response.output_text.delta", "item_id", "m1", "output_index", 1, "content_index", 0, "delta", mEmail[half:]+" y "+mName[:5]))
	b.WriteString(ev("response.output_text.delta", "item_id", "m0", "output_index", 0, "content_index", 1, "delta", mEmail[half:]))
	b.WriteString(ev("response.output_text.delta", "item_id", "m1", "output_index", 1, "content_index", 0, "delta", mName[5:]))
	b.WriteString(ev("response.output_text.delta", "item_id", "m0", "output_index", 0, "content_index", 0, "delta", mEmail[half:]+" z"))
	b.WriteString(ev("response.output_text.done", "item_id", "m0", "output_index", 0, "content_index", 1, "text", mEmail[half:]))
	in := []byte(b.String())
	r := runReply(t, Responses, sseType, in, nil, testRestore)
	assertSameFrames(t, in, r.out)
	got := channelText(t, r.out)
	want := map[string]string{
		"0:0": "x " + oEmail + " z",
		"1:0": mEmail[half:] + " y " + oName,
		"0:1": mEmail[half:],
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("channel %s = %q, want %q", k, got[k], v)
		}
	}
}

func TestReplySSEOnlyAnnouncedAssistantItems(t *testing.T) {
	var b strings.Builder
	// No output_item.added for index 0; index 1 is not a message; index 2
	// is a message with a mismatched item id.
	b.WriteString(ev("response.output_text.delta", "item_id", "m0", "output_index", 0, "content_index", 0, "delta", mEmail))
	b.WriteString(ev("response.output_item.added", "output_index", 1, "item", obj{"type": "function_call", "id": "fc"}))
	b.WriteString(ev("response.output_text.delta", "item_id", "fc", "output_index", 1, "content_index", 0, "delta", mEmail))
	b.WriteString(ev("response.output_item.added", "output_index", 2, "item", obj{"type": "message", "id": "m2", "role": "assistant"}))
	b.WriteString(ev("response.output_text.delta", "item_id", "other", "output_index", 2, "content_index", 0, "delta", mEmail))
	b.WriteString(ev("response.output_item.added", "output_index", 3, "item", obj{"type": "message", "id": "m3", "role": "user"}))
	b.WriteString(ev("response.output_text.delta", "item_id", "m3", "output_index", 3, "content_index", 0, "delta", mEmail))
	b.WriteString(ev("response.failed", "response", obj{"output": arr{obj{"type": "message", "role": "assistant", "content": arr{obj{"type": "output_text", "text": mEmail}}}}, "error": obj{"message": mEmail}}))
	b.WriteString(ev("error", "message", mEmail, "code", "x"))
	in := []byte(b.String())
	r := runReply(t, Responses, sseType, in, nil, testRestore)
	if !bytes.Equal(r.out, in) {
		t.Fatalf("changed:\n%s", r.out)
	}
	assertWarnings(t, r.warnings, WarnReplyMalformedEvent)

	// Messages: a non-assistant message and a text_delta for an index not
	// announced as text are unchanged.
	b.Reset()
	b.WriteString(ev("message_start", "message", obj{"type": "message", "role": "user", "content": arr{obj{"type": "text", "text": mEmail}}}))
	b.WriteString(ev("content_block_start", "index", 0, "content_block", obj{"type": "text", "text": mEmail}))
	b.WriteString(ev("content_block_delta", "index", 0, "delta", obj{"type": "text_delta", "text": mEmail}))
	b.WriteString(ev("message_start", "message", obj{"type": "message", "role": "assistant", "content": arr{}}))
	b.WriteString(ev("content_block_delta", "index", 0, "delta", obj{"type": "text_delta", "text": mEmail}))
	b.WriteString(ev("content_block_start", "index", 1, "content_block", obj{"type": "tool_use", "input": obj{}}))
	b.WriteString(ev("content_block_delta", "index", 1, "delta", obj{"type": "text_delta", "text": mEmail}))
	b.WriteString(ev("content_block_delta", "index", 1, "delta", obj{"type": "citations_delta", "citation": obj{"cited_text": mEmail}}))
	b.WriteString(ev("error", "error", obj{"type": "overloaded_error", "message": mEmail}))
	in = []byte(b.String())
	r = runReply(t, Messages, sseType, in, nil, testRestore)
	if !bytes.Equal(r.out, in) {
		t.Fatalf("changed:\n%s", r.out)
	}
	assertWarnings(t, r.warnings)
}

func TestReplySSEMessageStartContent(t *testing.T) {
	in := ev("message_start", "message", obj{"type": "message", "role": "assistant", "content": arr{obj{"type": "text", "text": mEmail}, obj{"type": "thinking", "thinking": mEmail, "signature": "sig"}}}) +
		ev("content_block_start", "index", 0, "content_block", obj{"type": "text", "text": "pre " + mName[:4]}) +
		ev("content_block_delta", "index", 0, "delta", obj{"type": "text_delta", "text": mName[4:]})
	r := runReply(t, Messages, sseType, []byte(in), nil, testRestore)
	want := strings.Replace(in, `"text":"`+mEmail, `"text":"`+oEmail, 1)
	want = strings.Replace(want, `"text":"pre `+mName[:4]+`"`, `"text":"pre `+oName+`"`, 1)
	want = strings.Replace(want, `"text":"`+mName[4:]+`"`, `"text":""`, 1)
	if string(r.out) != want {
		t.Fatalf("got\n%s\nwant\n%s", r.out, want)
	}
}

func TestReplySSEFraming(t *testing.T) {
	data := `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"` + mEmail + `"}}`
	start := "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"type\":\"message\",\"role\":\"assistant\",\"content\":[]}}\n\n" +
		"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n"
	restored := strings.Replace(data, mEmail, oEmail, 1)
	for _, tc := range []struct{ name, in, want string }{
		{"crlf", strings.ReplaceAll(start, "\n", "\r\n") + "id: 7\r\nevent: content_block_delta\r\ndata: " + data + "\r\n\r\n",
			strings.ReplaceAll(start, "\n", "\r\n") + "id: 7\r\nevent: content_block_delta\r\ndata: " + restored + "\r\n\r\n"},
		{"cr", strings.ReplaceAll(start, "\n", "\r") + "event: content_block_delta\rdata:" + data + "\r\r",
			strings.ReplaceAll(start, "\n", "\r") + "event: content_block_delta\rdata:" + restored + "\r\r"},
		{"multiline data", start + ": comment\nretry: 10\ndata: {\"type\":\"content_block_delta\",\ndata: \"index\":0,\"delta\":{\"type\":\"text_delta\",\ndata:\"text\":\"" + mEmail + "\"}}\n\n",
			start + ": comment\nretry: 10\ndata: {\"type\":\"content_block_delta\",\ndata: \"index\":0,\"delta\":{\"type\":\"text_delta\",\ndata:\"text\":\"" + oEmail + "\"}}\n\n"},
		{"no event line, escaped", start + "data: " + strings.Replace(data, "⟦", `\u27e6`, 1) + "\n\n", start + "data: " + restored + "\n\n"},
		{"done and blank frames", start + "\n\ndata: [DONE]\n\n", start + "\n\ndata: [DONE]\n\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for cut := 0; cut <= len(tc.in); cut++ {
				r := runReply(t, Messages, sseType, []byte(tc.in), []int{cut, len(tc.in)}, testRestore)
				if string(r.out) != tc.want {
					t.Fatalf("cut %d:\n got %q\nwant %q", cut, r.out, tc.want)
				}
				assertWarnings(t, r.warnings)
			}
		})
	}
}

func TestReplySSEMalformedFrames(t *testing.T) {
	ok := ev("message_start", "message", obj{"type": "message", "role": "assistant", "content": arr{}}) +
		ev("content_block_start", "index", 0, "content_block", obj{"type": "text", "text": ""})
	delta := ev("content_block_delta", "index", 0, "delta", obj{"type": "text_delta", "text": mEmail})
	for _, tc := range []struct {
		name, frame, warn string
	}{
		{"invalid json", "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"text\":\"" + mEmail + "\"\n\n", WarnReplyMalformedEvent},
		{"duplicate key", "data: {\"type\":\"content_block_delta\",\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"" + mEmail + "\"}}\n\n", WarnReplyMalformedEvent},
		{"no type", "data: {\"index\":0}\n\n", WarnReplyMalformedEvent},
		{"array", "data: [1]\n\n", WarnReplyMalformedEvent},
		{"event mismatch", strings.Replace(delta, "event: content_block_delta", "event: content_block_stop", 1), WarnReplyEventMismatch},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := ok + tc.frame + delta
			r := runReply(t, Messages, sseType, []byte(in), nil, testRestore)
			want := ok + tc.frame + strings.Replace(delta, mEmail, oEmail, 1)
			if string(r.out) != want {
				t.Fatalf("got %q\nwant %q", r.out, want)
			}
			assertWarnings(t, r.warnings, tc.warn)
		})
	}
}

func TestReplySSEOversizedFramePassesThrough(t *testing.T) {
	head := ev("message_start", "message", obj{"type": "message", "role": "assistant", "content": arr{}}) +
		ev("content_block_start", "index", 0, "content_block", obj{"type": "text", "text": ""}) +
		ev("content_block_delta", "index", 0, "delta", obj{"type": "text_delta", "text": "a " + mEmail[:6]})
	big := ev("content_block_delta", "index", 0, "delta", obj{"type": "text_delta", "text": mEmail[6:] + strings.Repeat("z", 5000)})
	tail := ev("content_block_delta", "index", 0, "delta", obj{"type": "text_delta", "text": mEmail})
	in := head + big + tail
	hundreds := []int{len(head)}
	for range len(in) / 100 {
		hundreds = append(hundreds, 100)
	}
	// Incomplete oversized frame (small reads) and complete oversized frame
	// (one read).
	for _, sizes := range [][]int{hundreds, {len(head)}, nil} {
		body := &chunkBody{data: []byte(in), sizes: sizes}
		var warnings []string
		rc := RestoreReply(body, sseType, ReplyOptions{Protocol: Messages, Restore: testRestore, MaxBytes: 1024, Warning: func(c string) { warnings = append(warnings, c) }})
		out, err := io.ReadAll(rc)
		if err != nil {
			t.Fatal(err)
		}
		// Held text is released unchanged (the marker never closed before
		// the limit), and everything after is forwarded byte-for-byte.
		if string(out) != in {
			t.Fatalf("got %q", out)
		}
		assertWarnings(t, warnings, WarnReplyTooLarge)
	}
}

func TestReplySSEQueueLimit(t *testing.T) {
	head := ev("message_start", "message", obj{"type": "message", "role": "assistant", "content": arr{}}) +
		ev("content_block_start", "index", 0, "content_block", obj{"type": "text", "text": ""}) +
		ev("content_block_delta", "index", 0, "delta", obj{"type": "text_delta", "text": "a " + mEmail[:6]})
	var pings strings.Builder
	for range 40 {
		pings.WriteString(ev("ping"))
	}
	tail := ev("content_block_delta", "index", 0, "delta", obj{"type": "text_delta", "text": mEmail[6:] + " " + mName})
	in := head + pings.String() + tail
	var warnings []string
	rc := RestoreReply(&chunkBody{data: []byte(in)}, sseType, ReplyOptions{Protocol: Messages, Restore: testRestore, MaxBytes: 600, Warning: func(c string) { warnings = append(warnings, c) }})
	out, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	assertSameFrames(t, []byte(in), out)
	// The split marker stays masked; a later complete marker is restored.
	if got := channelText(t, out)["0"]; got != "a "+mEmail+" "+oName {
		t.Fatalf("got %q", got)
	}
	assertWarnings(t, warnings, WarnReplyBufferLimit)
}

func TestReplySSEIncrementalStreaming(t *testing.T) {
	pr, pw := io.Pipe()
	rc := RestoreReply(pr, sseType, ReplyOptions{Protocol: Messages, Restore: testRestore})
	type result struct {
		b   []byte
		err error
	}
	reads := make(chan result)
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := rc.Read(buf)
			reads <- result{bytes.Clone(buf[:n]), err}
			if err != nil {
				close(reads)
				return
			}
		}
	}()
	expect := func(want string) {
		t.Helper()
		var got []byte
		for len(got) < len(want) {
			select {
			case r := <-reads:
				if r.err != nil {
					t.Fatalf("read error %v after %q", r.err, got)
				}
				got = append(got, r.b...)
			case <-time.After(5 * time.Second):
				t.Fatalf("no output; have %q want %q", got, want)
			}
		}
		if string(got) != want {
			t.Fatalf("got %q want %q", got, want)
		}
	}
	expectNothing := func() {
		t.Helper()
		select {
		case r := <-reads:
			t.Fatalf("unexpected output %q", r.b)
		case <-time.After(50 * time.Millisecond):
		}
	}
	write := func(s string) {
		t.Helper()
		if _, err := pw.Write([]byte(s)); err != nil {
			t.Fatal(err)
		}
	}
	start := ev("message_start", "message", obj{"type": "message", "role": "assistant", "content": arr{}}) +
		ev("content_block_start", "index", 0, "content_block", obj{"type": "text", "text": ""})
	write(start)
	expect(start)
	plain := ev("content_block_delta", "index", 0, "delta", obj{"type": "text_delta", "text": "plain " + mEmail})
	// Half a frame produces nothing; the rest releases it at once.
	write(plain[:20])
	expectNothing()
	write(plain[20:])
	expect(strings.Replace(plain, mEmail, oEmail, 1))
	held := ev("content_block_delta", "index", 0, "delta", obj{"type": "text_delta", "text": "x " + mName[:8]})
	write(held)
	expectNothing()
	ping := ev("ping")
	write(ping)
	expectNothing()
	closing := ev("content_block_delta", "index", 0, "delta", obj{"type": "text_delta", "text": mName[8:] + "!"})
	write(closing)
	expect(strings.Replace(held, "x "+mName[:8], "x "+oName+"!", 1) + ping + strings.Replace(closing, mName[8:]+"!", "", 1))
	pw.Close()
	if r, ok := <-reads; !ok || r.err != io.EOF || len(r.b) != 0 {
		t.Fatalf("want EOF, got %v %q", r.err, r.b)
	}
}

func TestReplyCloseUnblocksAndPropagates(t *testing.T) {
	before := runtime.NumGoroutine()
	pr, pw := io.Pipe()
	rc := RestoreReply(pr, sseType, ReplyOptions{Protocol: Responses, Restore: testRestore})
	done := make(chan error)
	go func() {
		_, err := io.ReadAll(rc)
		done <- err
	}()
	if _, err := pw.Write([]byte("data: {\"type\":\"x\"")); err != nil {
		t.Fatal(err)
	}
	if err := rc.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("want error after close")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Read did not return after Close")
	}
	if _, err := pw.Write([]byte("x")); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("upstream not closed: %v", err)
	}
	if _, err := rc.Read(make([]byte, 1)); err == nil {
		t.Fatal("read after close succeeded")
	}
	deadline := time.Now().Add(2 * time.Second)
	for runtime.NumGoroutine() > before && time.Now().Before(deadline) {
		runtime.Gosched()
		time.Sleep(10 * time.Millisecond)
	}
	if n := runtime.NumGoroutine(); n > before {
		t.Fatalf("goroutines %d > %d", n, before)
	}
}

func TestReplySSEReadErrorFlushes(t *testing.T) {
	in := ev("message_start", "message", obj{"type": "message", "role": "assistant", "content": arr{}}) +
		ev("content_block_start", "index", 0, "content_block", obj{"type": "text", "text": ""}) +
		ev("content_block_delta", "index", 0, "delta", obj{"type": "text_delta", "text": "a " + mEmail[:6]}) +
		"event: content_block_delta\ndata: {\"partial"
	boom := errors.New("synthetic reset")
	rc := RestoreReply(&chunkBody{data: []byte(in), err: boom}, sseType, ReplyOptions{Protocol: Messages, Restore: testRestore})
	out, err := io.ReadAll(rc)
	if !errors.Is(err, boom) {
		t.Fatalf("err %v", err)
	}
	if string(out) != in {
		t.Fatalf("got %q", out)
	}
}

func TestReplySSEEOFFlushesIncompleteMarker(t *testing.T) {
	in := ev("message_start", "message", obj{"type": "message", "role": "assistant", "content": arr{}}) +
		ev("content_block_start", "index", 0, "content_block", obj{"type": "text", "text": ""}) +
		ev("content_block_delta", "index", 0, "delta", obj{"type": "text_delta", "text": mEmail + " then ⟦EMAIL-t1"})
	r := runReply(t, Messages, sseType, []byte(in), nil, testRestore)
	if want := strings.Replace(in, mEmail, oEmail, 1); string(r.out) != want {
		t.Fatalf("got %q", r.out)
	}
	// A long "⟦" run without "⟧" is not held past the carry bound.
	long := "⟦" + strings.Repeat("y", replyMarkerCarry)
	in = ev("message_start", "message", obj{"type": "message", "role": "assistant", "content": arr{}}) +
		ev("content_block_start", "index", 0, "content_block", obj{"type": "text", "text": ""})
	body := in + ev("content_block_delta", "index", 0, "delta", obj{"type": "text_delta", "text": long})
	pr, pw := io.Pipe()
	rc := RestoreReply(pr, sseType, ReplyOptions{Protocol: Messages, Restore: testRestore})
	go func() { pw.Write([]byte(body)) }()
	buf := make([]byte, len(body))
	if _, err := io.ReadFull(rc, buf); err != nil || string(buf) != body {
		t.Fatalf("long run not released promptly: %v", err)
	}
	pw.Close()
	rc.Close()
}

func TestReplyRestoreCallbackFailures(t *testing.T) {
	deltas := []string{"a " + mEmail[:5], mEmail[5:]}
	in := messagesStream(deltas)
	r := runReply(t, Messages, sseType, in, nil, func(string) string { panic("synthetic") })
	if !bytes.Equal(r.out, messagesStream([]string{"a " + mEmail, ""})) {
		t.Fatalf("panic changed text: %s", r.out)
	}
	assertWarnings(t, r.warnings, WarnReplyRestorePanic)

	r = runReply(t, Messages, sseType, in, nil, func(s string) string { return s + "\xff" })
	if !bytes.Equal(r.out, messagesStream([]string{"a " + mEmail, ""})) {
		t.Fatalf("invalid output used: %q", r.out)
	}
	assertWarnings(t, r.warnings, WarnReplyRestoreInvalidOutput)

	// A panicking warning sink does not break the reply.
	rc := RestoreReply(&chunkBody{data: in}, sseType, ReplyOptions{Protocol: Messages, Restore: func(string) string { panic(1) }, Warning: func(string) { panic(2) }})
	if out, err := io.ReadAll(rc); err != nil || len(out) == 0 {
		t.Fatalf("%v %d", err, len(out))
	}
}
