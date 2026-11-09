package agentgateway

import (
	"errors"
	"io"
	"mime"
	"slices"
	"strings"
	"sync/atomic"
	"unicode/utf8"
)

// This file contains the reply restoration engine. It rewrites only the
// assistant-visible text of a model reply, replacing exact markers issued
// for this request with their originals through a caller-supplied callback.
// Every other byte of the reply, including signatures, encrypted reasoning,
// tool calls, tool input, annotations, metadata and error strings, is
// forwarded unchanged. The engine never sees a mapping or a credential.
//
// Restoration scope (everything else is forwarded byte-for-byte):
//
// Responses JSON: output[] items with "type" "message" and "role"
// "assistant": the string "text" of "content" parts with "type"
// "output_text".
//
// Responses SSE: response.output_text.delta "delta" and
// response.output_text.done "text" of an output index that was announced
// as an assistant message by response.output_item.added;
// response.content_part.added/done "part.text" of output_text parts of such
// an output index; response.output_item.added/done assistant message items;
// response.completed and response.incomplete "response.output" assistant
// message items.
//
// Messages JSON: a root with "type" "message" and "role" "assistant": the
// string "text" of "content" blocks with "type" "text".
//
// Messages SSE: message_start "message" content (role verified as
// assistant); content_block_start text blocks; content_block_delta
// text_delta "text" of a block announced as text in an assistant message.

// ReplyOptions configures RestoreReply.
type ReplyOptions struct {
	// Protocol selects the reply schema.
	Protocol Protocol
	// Restore returns its argument with exact issued markers replaced by
	// their originals. It must restore only markers issued for this
	// request's mapping scope and leave unknown or edited markers
	// unchanged. It receives complete text segments that never end inside
	// a marker the engine is still waiting for. A panic or invalid UTF-8
	// result keeps the segment unchanged with a warning. Nil disables
	// restoration and returns the body unchanged.
	Restore func(string) string
	// Warning receives fixed WarnReply* codes, at most once per code per
	// reply. It never receives reply content. It may be nil.
	Warning func(string)
	// MaxBytes bounds a buffered JSON body, a single SSE frame and the SSE
	// frames queued behind a partial marker. Values <= 0 use
	// DefaultReplyMaxBytes.
	MaxBytes int64
}

// DefaultReplyMaxBytes is the MaxBytes used when ReplyOptions.MaxBytes is
// not positive.
const DefaultReplyMaxBytes int64 = 8 << 20

// maxReplyBytes clamps MaxBytes so limit arithmetic cannot overflow.
const maxReplyBytes int64 = 1 << 40

// replyMarkerCarry bounds the bytes held while waiting for a marker that
// began with "⟦" to close with "⟧". Issued markers are well under 128 bytes;
// a longer unclosed run is not a marker and is released unchanged.
const replyMarkerCarry = 256

const (
	replyMarkerOpen  = "⟦"
	replyMarkerClose = "⟧"
)

// Reply warning codes. They are fixed strings and never include reply text.
const (
	// The reply is forwarded unchanged.
	WarnReplyUnsupportedProtocol    = "reply_unsupported_protocol"
	WarnReplyUnsupportedContentType = "reply_unsupported_content_type"
	WarnReplyInvalidJSON            = "reply_invalid_json"
	// The JSON body or one SSE frame exceeded MaxBytes; the remaining
	// bytes are forwarded unchanged.
	WarnReplyTooLarge = "reply_too_large"
	// Frames queued behind a partial marker exceeded MaxBytes; held text
	// was released and a marker split at that point stays masked.
	WarnReplyBufferLimit = "reply_buffer_limit"
	// One SSE frame was malformed or inconsistent and forwarded unchanged.
	WarnReplyMalformedEvent = "reply_malformed_event"
	WarnReplyEventMismatch  = "reply_event_type_mismatch"
	// The Restore callback failed for one text segment, which was kept
	// unchanged.
	WarnReplyRestorePanic         = "reply_restore_panic"
	WarnReplyRestoreInvalidOutput = "reply_restore_invalid_output"
	// Restoration expansion is bounded as well as provider input bytes.
	WarnReplyExpansionLimit = "reply_restoration_expansion_limit"
)

var errReplyClosed = errors.New("agentgateway: read on closed reply body")

// RestoreReply wraps a model reply body and restores markers in
// assistant-visible text as the body is read. contentType is the reply
// Content-Type; application/json (or +json) and text/event-stream are
// supported, anything else is returned unchanged with a warning. The
// caller handles status, headers and content encoding; body must be
// identity-encoded.
//
// JSON bodies are buffered up to MaxBytes. SSE bodies are processed frame
// by frame without buffering the whole reply: a frame is released as soon
// as it is complete unless an earlier frame is held because its text ends
// in a possible partial marker. Malformed, unsupported or oversized input
// is forwarded unchanged with a warning; the reply is never aborted by the
// engine. Closing the returned body closes body. RestoreReply starts no
// goroutines.
func RestoreReply(body io.ReadCloser, contentType string, opts ReplyOptions) io.ReadCloser {
	if body == nil {
		return nil
	}
	c := &replyCore{opts: opts}
	if c.opts.MaxBytes <= 0 {
		c.opts.MaxBytes = DefaultReplyMaxBytes
	}
	c.opts.MaxBytes = min(c.opts.MaxBytes, maxReplyBytes)
	if opts.Protocol != Responses && opts.Protocol != Messages {
		c.warn(WarnReplyUnsupportedProtocol)
		return body
	}
	if opts.Restore == nil {
		return body
	}
	r := &replyReader{c: c, body: body}
	switch replyMediaKind(contentType) {
	case replyJSON:
	case replySSE:
		r.sse = newSSEState(c)
	default:
		c.warn(WarnReplyUnsupportedContentType)
		return body
	}
	return r
}

type replyKind uint8

const (
	replyOther replyKind = iota
	replyJSON
	replySSE
)

func replyMediaKind(contentType string) replyKind {
	mt, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return replyOther
	}
	switch {
	case mt == "text/event-stream":
		return replySSE
	case mt == "application/json" || strings.HasSuffix(mt, "+json"):
		return replyJSON
	}
	return replyOther
}

// replyCore holds per-reply options and warning de-duplication.
type replyCore struct {
	opts     ReplyOptions
	warned   []string
	expanded int64
}

func (c *replyCore) warn(code string) {
	if slices.Contains(c.warned, code) {
		return
	}
	c.warned = append(c.warned, code)
	if c.opts.Warning == nil {
		return
	}
	defer func() { _ = recover() }()
	c.opts.Warning(code)
}

// restore calls the Restore callback for one complete text segment. A
// panic or invalid UTF-8 result keeps the segment unchanged.
func (c *replyCore) restore(s string) string {
	if s == "" {
		return s
	}
	if c.expanded >= c.opts.MaxBytes {
		c.warn(WarnReplyExpansionLimit)
		return s
	}
	out, ok := callRestore(c.opts.Restore, s)
	switch {
	case !ok:
		c.warn(WarnReplyRestorePanic)
		return s
	case !utf8.ValidString(out):
		c.warn(WarnReplyRestoreInvalidOutput)
		return s
	}
	if out == s {
		return s
	}
	// Count the escaped JSON representation, not just decoded text: control
	// characters may expand sixfold when the selected string is serialized.
	if extra := replyJSONStringSize(out) - replyJSONStringSize(s); extra > 0 {
		if extra > c.opts.MaxBytes-c.expanded {
			c.expanded = c.opts.MaxBytes
			c.warn(WarnReplyExpansionLimit)
			return s
		}
		c.expanded += extra
	}
	return out
}

func replyJSONStringSize(s string) int64 {
	size := int64(len(s)) + 2
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"', '\\', '\n', '\r', '\t':
			size++
		default:
			if s[i] < 0x20 {
				size += 5
			}
		}
	}
	return size
}

func callRestore(f func(string) string, s string) (out string, ok bool) {
	defer func() {
		if recover() != nil {
			out, ok = "", false
		}
	}()
	return f(s), true
}

func (c *replyCore) str(doc []byte, n *jsonNode) (string, bool) {
	if n == nil || n.kind != kindString {
		return "", false
	}
	return stringValue(doc, n), true
}

func (c *replyCore) is(doc []byte, n *jsonNode, key, want string) bool {
	s, ok := c.str(doc, n.get(key))
	return ok && s == want
}

// restoreNode appends an edit for a string node whose restored value
// differs. Unchanged values keep their original escaping.
func (c *replyCore) restoreNode(doc []byte, n *jsonNode, edits []edit) []edit {
	orig, ok := c.str(doc, n)
	if !ok {
		return edits
	}
	out := c.restore(orig)
	if out == orig {
		return edits
	}
	return append(edits, edit{start: n.start, end: n.end, repl: appendJSONString(nil, out)})
}

func (c *replyCore) isAssistantMessage(doc []byte, item *jsonNode) bool {
	return item != nil && item.kind == kindObject &&
		c.is(doc, item, "type", "message") && c.is(doc, item, "role", "assistant")
}

// responsesItem restores output_text parts of an assistant message item.
func (c *replyCore) responsesItem(doc []byte, item *jsonNode, edits []edit) []edit {
	if !c.isAssistantMessage(doc, item) {
		return edits
	}
	content := item.get("content")
	if content == nil || content.kind != kindArray {
		return edits
	}
	for _, part := range content.elems {
		if part.kind == kindObject && c.is(doc, part, "type", "output_text") {
			edits = c.restoreNode(doc, part.get("text"), edits)
		}
	}
	return edits
}

// responsesResponse restores assistant messages in a response object.
func (c *replyCore) responsesResponse(doc []byte, resp *jsonNode, edits []edit) []edit {
	if resp == nil || resp.kind != kindObject {
		return edits
	}
	output := resp.get("output")
	if output == nil || output.kind != kindArray {
		return edits
	}
	for _, item := range output.elems {
		edits = c.responsesItem(doc, item, edits)
	}
	return edits
}

// messagesMessage restores text blocks of an assistant message object.
func (c *replyCore) messagesMessage(doc []byte, msg *jsonNode, edits []edit) []edit {
	if !c.isAssistantMessage(doc, msg) {
		return edits
	}
	content := msg.get("content")
	if content == nil || content.kind != kindArray {
		return edits
	}
	for _, block := range content.elems {
		if block.kind == kindObject && c.is(doc, block, "type", "text") {
			edits = c.restoreNode(doc, block.get("text"), edits)
		}
	}
	return edits
}

// restoreJSON restores a complete non-streaming reply body.
func (c *replyCore) restoreJSON(body []byte) []byte {
	root, perr := parseJSON(body)
	if perr != parseOK {
		c.warn(WarnReplyInvalidJSON)
		return body
	}
	var edits []edit
	if c.opts.Protocol == Responses {
		edits = c.responsesResponse(body, root, nil)
	} else {
		edits = c.messagesMessage(body, root, nil)
	}
	return applyReplyEdits(body, edits)
}

// applyReplyEdits splices non-overlapping edits into b.
func applyReplyEdits(b []byte, edits []edit) []byte {
	if len(edits) == 0 {
		return b
	}
	slices.SortFunc(edits, func(x, y edit) int { return x.start - y.start })
	size := len(b)
	for _, e := range edits {
		size += len(e.repl) - (e.end - e.start)
	}
	out := make([]byte, 0, size)
	prev := 0
	for _, e := range edits {
		out = append(out, b[prev:e.start]...)
		out = append(out, e.repl...)
		prev = e.end
	}
	return append(out, b[prev:]...)
}

// replyReader is a pull-based reader: all processing happens inside Read on
// the caller's goroutine.
type replyReader struct {
	c      *replyCore
	body   io.ReadCloser
	closed atomic.Bool
	sse    *sseState // nil for JSON
	pass   bool      // forward the remaining upstream bytes unchanged
	out    []byte    // processed bytes not yet returned
	err    error     // terminal error to return once out is drained
	chunk  []byte
}

const replyReadChunk = 32 << 10

func (r *replyReader) Read(p []byte) (int, error) {
	if r.closed.Load() {
		return 0, errReplyClosed
	}
	if len(p) == 0 {
		return 0, nil
	}
	for {
		if len(r.out) > 0 {
			n := copy(p, r.out)
			r.out = r.out[n:]
			if len(r.out) == 0 {
				r.out = nil
			}
			return n, nil
		}
		if r.err != nil {
			return 0, r.err
		}
		if r.pass {
			return r.body.Read(p)
		}
		if r.sse != nil {
			r.fillSSE()
		} else {
			r.fillJSON()
		}
	}
}

func (r *replyReader) fillJSON() {
	max := r.c.opts.MaxBytes
	buf, err := io.ReadAll(io.LimitReader(r.body, max+1))
	switch {
	case err != nil:
		// Forward what arrived unchanged, then surface the read error.
		r.out, r.err = buf, err
	case int64(len(buf)) > max:
		r.c.warn(WarnReplyTooLarge)
		r.out, r.pass = buf, true
	default:
		r.out, r.err = r.c.restoreJSON(buf), io.EOF
	}
}

func (r *replyReader) fillSSE() {
	if r.chunk == nil {
		r.chunk = make([]byte, replyReadChunk)
	}
	n, err := r.body.Read(r.chunk)
	if n > 0 {
		r.sse.write(r.chunk[:n])
	}
	if err != nil {
		r.sse.finish()
		r.err = err
	}
	r.out = append(r.out, r.sse.take()...)
	if r.sse.pass {
		r.pass = true
	}
}

// Close closes the upstream body. It is safe to call concurrently with
// Read; a Read after Close returns an error.
func (r *replyReader) Close() error {
	r.closed.Store(true)
	return r.body.Close()
}
