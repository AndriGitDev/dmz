package agentgateway

import (
	"bytes"
	"strings"
)

// sseState restores markers in a server-sent event stream.
//
// Frames are released in their original order. A text frame whose
// accumulated channel text ends in a possible partial marker ("⟦" without a
// following "⟧", within replyMarkerCarry bytes) becomes the channel's
// anchor and is held, together with every later frame, until the marker
// closes or the channel ends. Text of later frames on the same channel is
// then coalesced: the earlier frame carries the restored text up to a safe
// cut and the newer frame carries the remainder (or an empty string). No
// frame is created, dropped or reordered, and only the selected JSON string
// bytes of a frame are rewritten.
//
// Channels are one content part (Responses output_index + content_index) or
// one content block (Messages index), so interleaved parts never join.
type sseState struct {
	c    *replyCore
	in   []byte // unconsumed input; a frame always starts at in[0]
	scan int    // next byte of in to scan for a line terminator
	line int    // start of the current line within in

	queue  []*sseFrame
	queued int64
	out    []byte
	pass   bool

	channels map[string]*sseChannel
	// Responses: output_index -> announced assistant message item id.
	items map[string]string
	// Messages: whether the current message is an assistant message, and
	// which content block indexes are text blocks.
	assistant  bool
	textBlocks map[string]bool
}

type sseChannel struct {
	group  string    // output_index (Responses) for item-wide finalization
	anchor *sseFrame // held frame
	buf    string    // unrestored channel text owned by anchor
}

type sseFrame struct {
	raw  []byte
	out  []byte // bytes to emit once not held
	held bool
	// Channel text frames: the raw span of the text string and its
	// original value.
	tStart, tEnd int
	orig         string
}

func newSSEState(c *replyCore) *sseState {
	return &sseState{
		c:          c,
		channels:   make(map[string]*sseChannel),
		items:      make(map[string]string),
		textBlocks: make(map[string]bool),
	}
}

// write consumes upstream bytes.
func (s *sseState) write(b []byte) {
	if s.pass {
		s.out = append(s.out, b...)
		return
	}
	s.in = append(s.in, b...)
	s.split(false)
	if !s.pass && int64(len(s.in)) > s.c.opts.MaxBytes {
		s.oversized()
	}
}

// oversized handles a frame larger than MaxBytes: parsing stops and
// everything from that frame on is forwarded unchanged.
func (s *sseState) oversized() {
	s.c.warn(WarnReplyTooLarge)
	s.drain()
	s.pass = true
}

// finish handles end of input or a read error: held text is released with
// any unclosed marker left as is, and an incomplete final frame is
// forwarded unchanged.
func (s *sseState) finish() {
	if !s.pass {
		s.split(true)
	}
	if !s.pass {
		s.drain()
	}
}

// drain finalizes all channels, releases the queue and forwards any
// unparsed input unchanged.
func (s *sseState) drain() {
	s.finalizeAll()
	s.release()
	s.out = append(s.out, s.in...)
	s.in, s.scan, s.line = nil, 0, 0
}

func (s *sseState) take() []byte {
	out := s.out
	s.out = nil
	return out
}

// split dispatches every complete frame in s.in. A frame ends with an empty
// line; lines end with CRLF, LF or CR.
func (s *sseState) split(eof bool) {
	consumed := 0
	for s.scan < len(s.in) {
		ch := s.in[s.scan]
		if ch != '\n' && ch != '\r' {
			s.scan++
			continue
		}
		end := s.scan + 1
		if ch == '\r' {
			if end == len(s.in) && !eof {
				break // CR may be the first half of CRLF
			}
			if end < len(s.in) && s.in[end] == '\n' {
				end++
			}
		}
		empty := s.scan == s.line
		s.scan, s.line = end, end
		if !empty {
			continue
		}
		if int64(end-consumed) > s.c.opts.MaxBytes {
			s.in = s.in[consumed:]
			s.oversized()
			return
		}
		s.dispatch(bytes.Clone(s.in[consumed:end]))
		consumed = end
	}
	if consumed > 0 {
		n := copy(s.in, s.in[consumed:])
		s.in = s.in[:n]
		s.scan -= consumed
		s.line -= consumed
	}
}

type sseDataLine struct{ raw, join, n int }

// parseSSEFrame extracts the event name and the joined data field of one
// raw frame, with the raw offset of each data line.
func parseSSEFrame(raw []byte) (event string, data []byte, lines []sseDataLine) {
	for i := 0; i < len(raw); {
		j := i
		for j < len(raw) && raw[j] != '\n' && raw[j] != '\r' {
			j++
		}
		line := raw[i:j]
		next := j
		if next < len(raw) {
			if raw[next] == '\r' && next+1 < len(raw) && raw[next+1] == '\n' {
				next += 2
			} else {
				next++
			}
		}
		if len(line) > 0 && line[0] != ':' {
			name, value, off := line, line[len(line):], j
			if k := bytes.IndexByte(line, ':'); k >= 0 {
				v := k + 1
				if v < len(line) && line[v] == ' ' {
					v++
				}
				name, value, off = line[:k], line[v:], i+v
			}
			switch string(name) {
			case "event":
				event = string(value)
			case "data":
				lines = append(lines, sseDataLine{raw: off, n: len(value)})
			}
		}
		i = next
	}
	switch len(lines) {
	case 0:
	case 1:
		data = raw[lines[0].raw : lines[0].raw+lines[0].n]
	default:
		for k := range lines {
			if k > 0 {
				data = append(data, '\n')
			}
			lines[k].join = len(data)
			data = append(data, raw[lines[k].raw:lines[k].raw+lines[k].n]...)
		}
	}
	return event, data, lines
}

// rawSpan maps a string node of the joined data to raw frame offsets. JSON
// strings cannot contain a raw line break, so a string never spans data
// lines.
func rawSpan(lines []sseDataLine, n *jsonNode) (int, int, bool) {
	for _, l := range lines {
		if n.start >= l.join && n.end <= l.join+l.n {
			start := l.raw + n.start - l.join
			return start, start + n.end - n.start, true
		}
	}
	return 0, 0, false
}

func (s *sseState) dispatch(raw []byte) {
	f := &sseFrame{raw: raw, out: raw}
	s.handle(f)
	s.enqueue(f)
}

// handle selects restorable text in one frame. Anything unrecognized keeps
// its raw bytes.
func (s *sseState) handle(f *sseFrame) {
	event, data, lines := parseSSEFrame(f.raw)
	if len(lines) == 0 || string(data) == "[DONE]" {
		return
	}
	root, perr := parseJSON(data)
	if perr != parseOK || root.kind != kindObject {
		s.c.warn(WarnReplyMalformedEvent)
		return
	}
	typ, ok := s.c.str(data, root.get("type"))
	if !ok {
		s.c.warn(WarnReplyMalformedEvent)
		return
	}
	if event != "" && event != typ {
		s.c.warn(WarnReplyEventMismatch)
		return
	}
	ev := sseEvent{s: s, f: f, doc: data, lines: lines, root: root}
	if s.c.opts.Protocol == Responses {
		ev.responses(typ)
	} else {
		ev.messages(typ)
	}
}

type sseEvent struct {
	s     *sseState
	f     *sseFrame
	doc   []byte
	lines []sseDataLine
	root  *jsonNode
}

// num returns the raw text of a numeric member, used as a channel key.
func (e *sseEvent) num(n *jsonNode, key string) (string, bool) {
	v := n.get(key)
	if v == nil || v.kind != kindNumber {
		return "", false
	}
	return string(e.doc[v.start:v.end]), true
}

// apply rewrites complete text fields of this frame immediately.
func (e *sseEvent) apply(edits []edit) {
	if len(edits) == 0 {
		return
	}
	raw := make([]edit, 0, len(edits))
	for _, ed := range edits {
		start, end, ok := rawSpan(e.lines, &jsonNode{start: ed.start, end: ed.end})
		if !ok {
			e.s.c.warn(WarnReplyMalformedEvent)
			return
		}
		raw = append(raw, edit{start: start, end: end, repl: ed.repl})
	}
	e.f.out = applyReplyEdits(e.f.raw, raw)
}

// stream feeds a text fragment that continues channel key.
func (e *sseEvent) stream(key, group string, n *jsonNode) {
	if n == nil || n.kind != kindString {
		e.s.c.warn(WarnReplyMalformedEvent)
		return
	}
	start, end, ok := rawSpan(e.lines, n)
	if !ok {
		e.s.c.warn(WarnReplyMalformedEvent)
		return
	}
	e.f.tStart, e.f.tEnd, e.f.orig = start, end, stringValue(e.doc, n)
	e.s.feed(key, group, e.f)
}

func (e *sseEvent) responses(typ string) {
	s, c, doc, root := e.s, e.s.c, e.doc, e.root
	switch typ {
	case "response.output_item.added":
		idx, ok := e.num(root, "output_index")
		if !ok {
			return
		}
		s.finalizeGroup(idx)
		item := root.get("item")
		if !c.isAssistantMessage(doc, item) {
			delete(s.items, idx)
			return
		}
		if _, seen := s.items[idx]; !seen && !s.remember(len(s.items)) {
			return
		}
		id, _ := c.str(doc, item.get("id"))
		s.items[idx] = id
		e.apply(c.responsesItem(doc, item, nil))
	case "response.output_text.delta", "response.output_text.done",
		"response.content_part.added", "response.content_part.done":
		idx, ok1 := e.num(root, "output_index")
		cidx, ok2 := e.num(root, "content_index")
		if !ok1 || !ok2 {
			return
		}
		key := idx + ":" + cidx
		id, announced := s.items[idx]
		if got, ok := c.str(doc, root.get("item_id")); announced && ok && id != "" && got != id {
			c.warn(WarnReplyMalformedEvent)
			announced = false
		}
		switch typ {
		case "response.output_text.delta":
			if announced {
				e.stream(key, idx, root.get("delta"))
			}
			return
		case "response.output_text.done", "response.content_part.done":
			s.finalize(key)
		}
		if !announced {
			return
		}
		switch typ {
		case "response.output_text.done":
			e.apply(c.restoreNode(doc, root.get("text"), nil))
		default:
			if part := root.get("part"); part != nil && part.kind == kindObject && c.is(doc, part, "type", "output_text") {
				e.apply(c.restoreNode(doc, part.get("text"), nil))
			}
		}
	case "response.output_item.done":
		idx, ok := e.num(root, "output_index")
		if !ok {
			return
		}
		s.finalizeGroup(idx)
		delete(s.items, idx)
		e.apply(c.responsesItem(doc, root.get("item"), nil))
	case "response.completed", "response.incomplete":
		s.finalizeAll()
		e.apply(c.responsesResponse(doc, root.get("response"), nil))
	case "response.failed", "error":
		s.finalizeAll()
	}
}

func (e *sseEvent) messages(typ string) {
	s, c, doc, root := e.s, e.s.c, e.doc, e.root
	switch typ {
	case "message_start":
		s.finalizeAll()
		clear(s.channels)
		clear(s.textBlocks)
		msg := root.get("message")
		s.assistant = c.isAssistantMessage(doc, msg)
		if s.assistant {
			e.apply(c.messagesMessage(doc, msg, nil))
		}
	case "content_block_start", "content_block_delta", "content_block_stop":
		idx, ok := e.num(root, "index")
		if !ok {
			return
		}
		switch typ {
		case "content_block_start":
			s.finalize(idx)
			block := root.get("content_block")
			if !s.assistant || block == nil || block.kind != kindObject || !c.is(doc, block, "type", "text") {
				delete(s.textBlocks, idx)
				return
			}
			if !s.textBlocks[idx] && !s.remember(len(s.textBlocks)) {
				return
			}
			s.textBlocks[idx] = true
			if text := block.get("text"); text != nil {
				// Clients append later deltas to the initial block text.
				e.stream(idx, idx, text)
			}
		case "content_block_delta":
			delta := root.get("delta")
			if s.textBlocks[idx] && delta != nil && delta.kind == kindObject && c.is(doc, delta, "type", "text_delta") {
				e.stream(idx, idx, delta.get("text"))
			}
		case "content_block_stop":
			s.finalize(idx)
			delete(s.textBlocks, idx)
		}
	case "message_delta", "message_stop", "error":
		s.finalizeAll()
	}
}

// partialMarkerLen returns the length of a suffix of t that may be the
// beginning of a marker: the last "⟦" with no "⟧" after it, if the suffix is
// shorter than replyMarkerCarry. Decoded JSON text is valid UTF-8, so "⟦"
// itself is never split.
func partialMarkerLen(t string) int {
	i := strings.LastIndex(t, replyMarkerOpen)
	if i < 0 || strings.Contains(t[i:], replyMarkerClose) || len(t)-i >= replyMarkerCarry {
		return 0
	}
	return len(t) - i
}

// feed adds frame f's text to channel key. A channel exists only while it
// holds a frame.
func (s *sseState) feed(key, group string, f *sseFrame) {
	ch := s.channels[key]
	if ch == nil {
		h := partialMarkerLen(f.orig)
		if h == 0 || len(s.channels) >= maxReplyStreams {
			if h != 0 {
				s.c.warn(WarnReplyBufferLimit)
			}
			s.setText(f, s.c.restore(f.orig))
			return
		}
		f.held = true
		s.channels[key] = &sseChannel{group: group, anchor: f, buf: f.orig}
		return
	}
	buf := ch.buf + f.orig
	h := partialMarkerLen(buf)
	cut := len(buf) - h
	s.setText(ch.anchor, s.c.restore(buf[:cut]))
	ch.anchor.held = false
	if h == 0 {
		s.setText(f, "")
		delete(s.channels, key)
		return
	}
	f.held = true
	ch.anchor, ch.buf = f, buf[cut:]
}

// maxReplyStreams bounds per-reply protocol state (announced items, text
// blocks and held channels). Real replies use a handful.
const maxReplyStreams = 1024

// remember records an announced Responses item or Messages text block
// within maxReplyStreams; beyond it, the item is not restored.
func (s *sseState) remember(n int) bool {
	if n >= maxReplyStreams {
		s.c.warn(WarnReplyBufferLimit)
		return false
	}
	return true
}

// finalize releases a channel's held text without waiting further. An
// unclosed marker stays as it is.
func (s *sseState) finalize(key string) {
	ch := s.channels[key]
	if ch == nil {
		return
	}
	s.setText(ch.anchor, s.c.restore(ch.buf))
	ch.anchor.held = false
	delete(s.channels, key)
}

func (s *sseState) finalizeGroup(group string) {
	for key, ch := range s.channels {
		if ch.group == group {
			s.finalize(key)
		}
	}
}

func (s *sseState) finalizeAll() {
	for key := range s.channels {
		s.finalize(key)
	}
}

func (s *sseState) setText(f *sseFrame, t string) {
	if t == f.orig {
		f.out = f.raw
		return
	}
	out := make([]byte, 0, len(f.raw)+len(t)-(f.tEnd-f.tStart)+2)
	out = append(out, f.raw[:f.tStart]...)
	out = appendJSONString(out, t)
	f.out = append(out, f.raw[f.tEnd:]...)
}

func (s *sseState) enqueue(f *sseFrame) {
	s.queue = append(s.queue, f)
	s.queued += int64(len(f.raw))
	s.release()
	if len(s.queue) > 0 && s.queued > s.c.opts.MaxBytes {
		// Too much is waiting behind a partial marker: give up on it.
		s.c.warn(WarnReplyBufferLimit)
		s.finalizeAll()
		s.release()
	}
}

// release emits frames from the head of the queue up to the first held
// frame.
func (s *sseState) release() {
	i := 0
	for ; i < len(s.queue) && !s.queue[i].held; i++ {
		s.out = append(s.out, s.queue[i].out...)
		s.queued -= int64(len(s.queue[i].raw))
		s.queue[i] = nil
	}
	if i > 0 {
		n := copy(s.queue, s.queue[i:])
		clear(s.queue[n:])
		s.queue = s.queue[:n]
	}
}
