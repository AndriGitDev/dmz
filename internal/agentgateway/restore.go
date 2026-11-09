package agentgateway

import (
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
)

type replyScopeKey struct{}

// All core-issued marker formats use this bounded, non-sensitive shape. A
// candidate is still verified against the live session before restoration.
var issuedReplyMarker = regexp.MustCompile(`⟦[A-Z]+-[0-9A-F]{16}-[0-9]+: [^⟦⟧\r\n]{1,128}⟧`)

func (m *mapping) restoreAnswer(text string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.invalid.Load() {
		return text
	}
	m.rememberReplyValues(text)
	spans := issuedReplyMarker.FindAllStringIndex(text, -1)
	size := int64(len(text))
	for _, span := range spans {
		marker := text[span[0]:span[1]]
		if m.noRestore[marker] {
			if m.warning != nil {
				m.warning("reply_whole_text_unrestored")
			}
			continue
		}
		if original, known := m.core.Original(marker); known {
			size += int64(len(original) - (span[1] - span[0]))
			if size > DefaultReplyMaxBytes {
				if m.warning != nil {
					m.warning(WarnReplyExpansionLimit)
				}
				return text
			}
		}
	}
	var out strings.Builder
	at := 0
	for _, span := range spans {
		marker := text[span[0]:span[1]]
		if m.noRestore[marker] {
			continue
		}
		original, known := m.core.Original(marker)
		if !known {
			continue
		}
		out.WriteString(text[at:span[0]])
		out.WriteString(original)
		at = span[1]
	}
	if at == 0 {
		return text
	}
	out.WriteString(text[at:])
	return out.String()
}

// Also remember markers created while re-reading user history: this reestablishes
// exact-value replay protection when a client sends its original user messages
// after DMZ restarts. Older histories without those values still need review.
func (m *mapping) rememberReplyValues(text string) {
	for _, marker := range issuedReplyMarker.FindAllString(text, -1) {
		original, known := m.core.Original(marker)
		if !known {
			continue
		}
		if m.restored == nil {
			m.restored = make(map[string]string)
		}
		if _, known := m.restored[original]; !known {
			m.restored[original] = marker
			m.replay = nil
		}
	}
}

// remaskRestored is called with m.mu held by prepare. Its scope is only exact
// originals from this live session (including replayed user history). Issued
// markers already present in history remain opaque and byte-identical.
func (m *mapping) remaskRestored(text string) string {
	if len(m.restored) == 0 {
		return text
	}
	if m.replay == nil {
		keys := make([]string, 0, len(m.restored))
		for original := range m.restored {
			keys = append(keys, original)
		}
		sort.Slice(keys, func(i, j int) bool {
			if len(keys[i]) != len(keys[j]) {
				return len(keys[i]) > len(keys[j])
			}
			return keys[i] < keys[j]
		})
		pairs := make([]string, 0, len(keys)*2)
		for _, original := range keys {
			pairs = append(pairs, original, m.restored[original])
		}
		m.replay = strings.NewReplacer(pairs...)
	}
	var out strings.Builder
	at := 0
	for _, span := range issuedReplyMarker.FindAllStringIndex(text, -1) {
		marker := text[span[0]:span[1]]
		if _, known := m.core.Original(marker); !known {
			continue
		}
		out.WriteString(m.replay.Replace(text[at:span[0]]))
		out.WriteString(marker)
		at = span[1]
	}
	if at == 0 {
		return m.replay.Replace(text)
	}
	out.WriteString(m.replay.Replace(text[at:]))
	return out.String()
}

func (s *Server) restoreResponse(response *http.Response) {
	if response == nil || response.Request == nil {
		return
	}
	scope, _ := response.Request.Context().Value(replyScopeKey{}).(*mapping)
	if scope == nil || response.Body == nil || response.Request.Method != http.MethodPost || response.StatusCode < 200 || response.StatusCode >= 300 {
		return
	}
	suffix, _ := endpoint(s.cfg.Protocol, response.Request.URL.Path, response.Request.Method)
	// The upstream path can have the ChatGPT backend prefix instead of /v1.
	if suffix == "" {
		if strings.HasSuffix(response.Request.URL.Path, "/responses") {
			suffix = "/responses"
		}
		if strings.HasSuffix(response.Request.URL.Path, "/messages") {
			suffix = "/messages"
		}
	}
	if suffix != "/responses" && suffix != "/messages" {
		return
	}
	if encoding := strings.TrimSpace(response.Header.Get("Content-Encoding")); encoding != "" && !strings.EqualFold(encoding, "identity") || len(response.Header.Values("Content-Encoding")) > 1 {
		s.warn("reply_encoding_unrestored")
		return
	}
	if replyMediaKind(response.Header.Get("Content-Type")) == replyOther {
		s.warn(WarnReplyUnsupportedContentType)
		return
	}
	select {
	case s.replySlots <- struct{}{}:
	default:
		s.warn("reply_busy_unrestored")
		return
	}
	response.Body = &replySlotBody{ReadCloser: RestoreReply(response.Body, response.Header.Get("Content-Type"), ReplyOptions{Protocol: s.cfg.Protocol, Restore: scope.restoreAnswer, Warning: s.warn, MaxBytes: min(s.cfg.MaxBodyBytes, DefaultReplyMaxBytes)}), release: func() { <-s.replySlots }}
	response.ContentLength = -1
	response.TransferEncoding = nil
	// These describe the upstream bytes; restored responses use chunked framing.
	for _, name := range []string{"Content-Length", "Content-MD5", "Digest", "Content-Digest", "Repr-Digest", "ETag"} {
		response.Header.Del(name)
	}
}

type replySlotBody struct {
	io.ReadCloser
	once    sync.Once
	release func()
}

func (b *replySlotBody) Close() error {
	err := b.ReadCloser.Close()
	b.once.Do(b.release)
	return err
}
