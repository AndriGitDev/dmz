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

// Write returns text that can safely be shown now. Unknown or changed markers
// remain visible and are never guessed into a real value.
func (r *StreamRestorer) Write(chunk string) string {
	if r.session.generation != r.generation {
		out := r.buffer + chunk
		r.buffer = ""
		r.markers = nil
		r.generation = r.session.generation
		return out
	}
	r.buffer += chunk
	hold := r.partialMarkerSuffix()
	cut := len(r.buffer) - hold
	// A network chunk may divide a UTF-8 rune even when it divides no marker.
	// Keep those bytes until the next chunk, too.
	for cut > 0 && !utf8.ValidString(r.buffer[:cut]) {
		cut--
	}
	out := r.session.Restore(r.buffer[:cut])
	r.buffer = r.buffer[cut:]
	return out
}

func (r *StreamRestorer) Flush() string {
	out := r.session.Restore(r.buffer)
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
