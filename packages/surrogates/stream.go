package surrogates

import (
	"strings"
	"unicode/utf8"
)

// StreamRestorer restores markers from arbitrarily split response chunks.
// It retains only a suffix that could begin a known marker. Call Flush at the
// end of the response. A reset disables restoration for the old stream.
type StreamRestorer struct {
	session    *Session
	generation uint64
	markers    []string
	buffer     string
}

func (s *Session) NewStreamRestorer() *StreamRestorer {
	markers := make([]string, 0, len(s.reverse))
	for marker := range s.reverse {
		markers = append(markers, marker)
	}
	return &StreamRestorer{session: s, generation: s.generation, markers: markers}
}

// Write returns currently available bytes. Incomplete UTF-8 runes are held;
// already-invalid bytes are emitted unchanged. Unknown or changed markers remain
// visible and are never guessed into a real value. After reset, this old stream
// only passes bytes through, including on subsequent writes and Flush.
func (r *StreamRestorer) Write(chunk string) string {
	if r.session.generation != r.generation {
		out := r.buffer + chunk
		r.buffer = ""
		r.markers = nil
		// Never bind an old response stream to a later mapping session.
		return out
	}
	r.buffer += chunk
	hold := r.partialMarkerSuffix()
	cut := len(r.buffer) - hold
	// A network chunk may divide a UTF-8 rune even when it divides no marker.
	// Keep those bytes until the next chunk, too.
	complete := 0
	for complete < cut && utf8.FullRuneInString(r.buffer[complete:cut]) {
		_, size := utf8.DecodeRuneInString(r.buffer[complete:cut])
		complete += size
	}
	// Invalid interior bytes are already complete decoding errors: preserve and
	// emit them unchanged instead of retaining all subsequent response text.
	cut = complete
	out := r.session.Restore(r.buffer[:cut])
	r.buffer = r.buffer[cut:]
	return out
}

func (r *StreamRestorer) Flush() string {
	out := r.buffer
	if r.session.generation == r.generation {
		out = r.session.Restore(out)
	} else {
		r.markers = nil
	}
	r.buffer = ""
	return out
}

func (r *StreamRestorer) partialMarkerSuffix() int {
	max := 0
	for _, marker := range r.markers {
		limit := min(len(marker)-1, len(r.buffer))
		for size := limit; size > max; size-- {
			if strings.HasPrefix(marker, r.buffer[len(r.buffer)-size:]) {
				max = size
				break
			}
		}
	}
	return max
}
