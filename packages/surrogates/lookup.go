package surrogates

// MappingCount reports issued values without exposing originals or markers.
// Callers must serialize access, as for the other Session methods.
func (s *Session) MappingCount() int { return len(s.reverse) }

// Original returns the original of one exact marker issued by this session.
// Unknown and edited markers are never guessed. Like the other Session methods,
// callers must serialize access; no mapping is exported or persisted.
func (s *Session) Original(marker string) (string, bool) {
	value, ok := s.reverse[marker]
	return value, ok
}
