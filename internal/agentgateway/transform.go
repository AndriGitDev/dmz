package agentgateway

import (
	"fmt"
	"slices"
	"unicode/utf8"
)

// Protocol identifies the request schema a body is transformed with.
type Protocol string

const (
	// Responses is the OpenAI-compatible Responses request body
	// (POST /v1/responses).
	Responses Protocol = "responses"
	// Messages is the Anthropic-compatible Messages request body
	// (POST /v1/messages). Token-counting requests
	// (POST /v1/messages/count_tokens) use the same schema and protocol.
	Messages Protocol = "messages"
)

// MaskFunc replaces sensitive values in one user text field. It is called
// once per non-empty eligible field, in document order.
type MaskFunc func(string) (string, error)

// TransformReport is an aggregate summary of one transformation. It never
// contains request text.
type TransformReport struct {
	// TextParts counts eligible user text fields found (including empty
	// ones, which are not passed to the mask function).
	TextParts int
	// ChangedParts counts fields whose bytes were replaced.
	ChangedParts int
	// Warnings holds fixed Warn* codes, de-duplicated, in first-seen order.
	Warnings []string
}

// Warning codes. Warnings are fixed strings and never include request
// content or error text.
const (
	// Fail-open codes: the original body is returned unchanged.
	WarnUnsupportedProtocol = "unsupported_protocol"
	WarnInvalidJSON         = "invalid_json"
	WarnTrailingData        = "trailing_data"
	WarnLoneSurrogate       = "lone_surrogate"
	WarnDuplicateKey        = "duplicate_key"
	WarnNestingTooDeep      = "nesting_too_deep"
	WarnJSONTooComplex      = "json_too_complex"
	WarnUnsupportedShape    = "unsupported_shape"

	// Per-field codes: the affected field keeps its original bytes while
	// other eligible fields are still masked.
	WarnMaskError         = "mask_error"
	WarnMaskPanic         = "mask_panic"
	WarnMaskInvalidOutput = "mask_invalid_output"

	// Coverage codes: the request was valid, but some content is outside
	// the masking scope and is forwarded unchanged.
	WarnNoUserText                    = "no_user_text"
	WarnAttachmentUnchanged           = "attachment_unchanged"
	WarnToolResultUnchanged           = "tool_result_unchanged"
	WarnToolResultDetectionNotApplied = "tool_result_detection_not_applied"
	WarnInstructionsUnchanged         = "instructions_unchanged"
	WarnUnsupportedContentUnchanged   = "unsupported_content_unchanged"
)

// IsFailOpenWarning reports whether code means that Transform returned the
// original body without attempting any masking.
func IsFailOpenWarning(code string) bool {
	switch code {
	case WarnUnsupportedProtocol, WarnInvalidJSON, WarnTrailingData,
		WarnLoneSurrogate, WarnDuplicateKey, WarnNestingTooDeep,
		WarnUnsupportedShape, WarnJSONTooComplex:
		return true
	}
	return false
}

// Transform masks user-authored text fields of a model request body and
// returns the new body. Only the bytes of changed string values are
// replaced; every other byte of the request is preserved exactly.
//
// Masking scope (everything else is forwarded unchanged):
//
// Responses:
//   - a top-level "input" string;
//   - in a top-level "input" array, items whose "type" is absent or
//     "message" and whose "role" is "user": a "content" string, or the
//     string "text" of "content" parts with "type" "input_text".
//
// Not masked for Responses: "instructions", system/developer messages,
// assistant messages, tool calls and arguments, tool outputs
// (*_output items), reasoning/encrypted items, attachments (input_image,
// input_file, input_audio), tools/schemas and metadata.
//
// Messages:
//   - in the top-level "messages" array, entries with "role" "user": a
//     "content" string, or the string "text" of "content" blocks with
//     "type" "text".
//
// Not masked for Messages: "system", assistant messages, tool_result
// blocks (including string content), tool_use input, image/document
// blocks, thinking/signature data, tools/schemas and metadata.
//
// Malformed or ambiguous input (invalid JSON, invalid UTF-8, lone surrogate
// escapes, duplicate keys, excessive nesting, concatenated values, an
// unexpected top-level shape, or an unknown protocol) is returned unchanged
// with a fail-open warning. A mask error, panic or invalid-UTF-8 result for
// one field leaves that field unchanged with a warning; the remaining fields
// are still masked. Transform never panics because of the mask function.
func Transform(protocol Protocol, body []byte, mask MaskFunc) ([]byte, TransformReport) {
	t := &transformer{body: body, mask: mask}
	if protocol != Responses && protocol != Messages {
		t.warn(WarnUnsupportedProtocol)
		return body, t.report
	}
	root, perr := parseJSON(body)
	switch perr {
	case parseOK:
	case parseTrailing:
		t.warn(WarnTrailingData)
		return body, t.report
	case parseLoneSurrogate:
		t.warn(WarnLoneSurrogate)
		return body, t.report
	case parseDuplicateKey:
		t.warn(WarnDuplicateKey)
		return body, t.report
	case parseTooDeep:
		t.warn(WarnNestingTooDeep)
		return body, t.report
	case parseTooComplex:
		t.warn(WarnJSONTooComplex)
		return body, t.report
	default:
		t.warn(WarnInvalidJSON)
		return body, t.report
	}
	if root.kind != kindObject {
		t.warn(WarnUnsupportedShape)
		return body, t.report
	}

	var ok bool
	if protocol == Responses {
		ok = t.responses(root)
	} else {
		ok = t.messages(root)
	}
	if !ok {
		// A shape problem means the whole request is outside the known
		// schema; report it without partial masking.
		t.report.TextParts, t.report.ChangedParts = 0, 0
		t.report.Warnings = nil
		t.warn(WarnUnsupportedShape)
		return body, t.report
	}
	if t.report.TextParts == 0 {
		t.warn(WarnNoUserText)
	}
	if len(t.edits) == 0 {
		return body, t.report
	}
	return t.apply(), t.report
}

type edit struct {
	start, end int
	repl       []byte
}

type transformer struct {
	body   []byte
	mask   MaskFunc
	report TransformReport
	edits  []edit
	// pending holds string nodes selected for masking, in document order.
	pending []*jsonNode
}

func (t *transformer) warn(code string) {
	if !slices.Contains(t.report.Warnings, code) {
		t.report.Warnings = append(t.report.Warnings, code)
	}
}

func (t *transformer) str(n *jsonNode) (string, bool) {
	if n == nil || n.kind != kindString {
		return "", false
	}
	return stringValue(t.body, n), true
}

// responses selects eligible fields of a Responses request. It returns
// false when the top-level shape is unsupported.
func (t *transformer) responses(root *jsonNode) bool {
	if instr := root.get("instructions"); instr != nil && instr.kind != kindNull {
		if s, isStr := t.str(instr); !isStr || s != "" {
			t.warn(WarnInstructionsUnchanged)
		}
	}
	input := root.get("input")
	switch {
	case input == nil || input.kind == kindNull:
		// e.g. a continuation that only references stored state.
	case input.kind == kindString:
		t.pending = append(t.pending, input)
	case input.kind == kindArray:
		for _, item := range input.elems {
			t.responsesItem(item)
		}
	default:
		return false
	}
	t.run()
	return true
}

func (t *transformer) responsesItem(item *jsonNode) {
	if item.kind != kindObject {
		t.warn(WarnUnsupportedContentUnchanged)
		return
	}
	typ := item.get("type")
	if typ != nil {
		s, ok := t.str(typ)
		if !ok {
			t.warn(WarnUnsupportedContentUnchanged)
			return
		}
		if s != "message" {
			if isToolOutputType(s) {
				t.warn(WarnToolResultUnchanged)
			}
			// Tool calls, reasoning, references and other opaque
			// items are expected and forwarded without comment.
			return
		}
	}
	role, ok := t.str(item.get("role"))
	if !ok {
		t.warn(WarnUnsupportedContentUnchanged)
		return
	}
	switch role {
	case "user":
	case "assistant":
		return
	case "system", "developer":
		t.warn(WarnInstructionsUnchanged)
		return
	default:
		t.warn(WarnUnsupportedContentUnchanged)
		return
	}
	content := item.get("content")
	switch {
	case content == nil:
		t.warn(WarnUnsupportedContentUnchanged)
	case content.kind == kindString:
		t.pending = append(t.pending, content)
	case content.kind == kindArray:
		for _, part := range content.elems {
			t.responsesPart(part)
		}
	default:
		t.warn(WarnUnsupportedContentUnchanged)
	}
}

func (t *transformer) responsesPart(part *jsonNode) {
	typ, ok := t.str(part.get("type"))
	if !ok {
		t.warn(WarnUnsupportedContentUnchanged)
		return
	}
	switch typ {
	case "input_text":
		text := part.get("text")
		if text == nil || text.kind != kindString {
			t.warn(WarnUnsupportedContentUnchanged)
			return
		}
		t.pending = append(t.pending, text)
	case "input_image", "input_file", "input_audio":
		t.warn(WarnAttachmentUnchanged)
	default:
		t.warn(WarnUnsupportedContentUnchanged)
	}
}

// isToolOutputType matches Responses item types that carry tool results
// back to the model, such as function_call_output and
// custom_tool_call_output.
func isToolOutputType(s string) bool {
	const suffix = "_output"
	return len(s) > len(suffix) && s[len(s)-len(suffix):] == suffix
}

// messages selects eligible fields of a Messages or count_tokens request.
func (t *transformer) messages(root *jsonNode) bool {
	if sys := root.get("system"); sys != nil && sys.kind != kindNull {
		if s, isStr := t.str(sys); !isStr || s != "" {
			t.warn(WarnInstructionsUnchanged)
		}
	}
	msgs := root.get("messages")
	switch {
	case msgs == nil:
	case msgs.kind == kindArray:
		for _, m := range msgs.elems {
			t.messagesEntry(m)
		}
	default:
		return false
	}
	t.run()
	return true
}

func (t *transformer) messagesEntry(m *jsonNode) {
	role, ok := t.str(m.get("role"))
	if !ok {
		t.warn(WarnUnsupportedContentUnchanged)
		return
	}
	switch role {
	case "user":
	case "assistant":
		return
	default:
		t.warn(WarnUnsupportedContentUnchanged)
		return
	}
	content := m.get("content")
	switch {
	case content == nil:
		t.warn(WarnUnsupportedContentUnchanged)
	case content.kind == kindString:
		t.pending = append(t.pending, content)
	case content.kind == kindArray:
		for _, block := range content.elems {
			t.messagesBlock(block)
		}
	default:
		t.warn(WarnUnsupportedContentUnchanged)
	}
}

func (t *transformer) messagesBlock(block *jsonNode) {
	typ, ok := t.str(block.get("type"))
	if !ok {
		t.warn(WarnUnsupportedContentUnchanged)
		return
	}
	switch typ {
	case "text":
		text := block.get("text")
		if text == nil || text.kind != kindString {
			t.warn(WarnUnsupportedContentUnchanged)
			return
		}
		t.pending = append(t.pending, text)
	case "tool_result":
		t.warn(WarnToolResultUnchanged)
	case "image", "document", "file", "container_upload":
		t.warn(WarnAttachmentUnchanged)
	default:
		t.warn(WarnUnsupportedContentUnchanged)
	}
}

// run calls the mask function for every pending field in document order.
func (t *transformer) run() {
	slices.SortFunc(t.pending, func(a, b *jsonNode) int { return a.start - b.start })
	for _, n := range t.pending {
		t.report.TextParts++
		orig := stringValue(t.body, n)
		if orig == "" {
			continue
		}
		out, err, panicked := callMask(t.mask, orig)
		switch {
		case panicked:
			t.warn(WarnMaskPanic)
			continue
		case err != nil:
			t.warn(WarnMaskError)
			continue
		case !utf8.ValidString(out):
			t.warn(WarnMaskInvalidOutput)
			continue
		case out == orig:
			// Keep the original escaping byte-for-byte.
			continue
		}
		t.edits = append(t.edits, edit{start: n.start, end: n.end, repl: appendJSONString(nil, out)})
		t.report.ChangedParts++
	}
}

func callMask(mask MaskFunc, s string) (out string, err error, panicked bool) {
	if mask == nil {
		return "", fmt.Errorf("agentgateway: nil mask function"), false
	}
	defer func() {
		if recover() != nil {
			out, err, panicked = "", nil, true
		}
	}()
	out, err = mask(s)
	return out, err, false
}

func (t *transformer) apply() []byte {
	size := len(t.body)
	for _, e := range t.edits {
		size += len(e.repl) - (e.end - e.start)
	}
	out := make([]byte, 0, size)
	prev := 0
	for _, e := range t.edits {
		out = append(out, t.body[prev:e.start]...)
		out = append(out, e.repl...)
		prev = e.end
	}
	return append(out, t.body[prev:]...)
}
