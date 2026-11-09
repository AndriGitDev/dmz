package surrogates

// Original returns the original of one exact marker issued by this session.
// Unknown and edited markers are never guessed. Like the other Session methods,
// callers must serialize access; no mapping is exported or persisted.
func (s *Session) Original(marker string) (string, bool) {
	value, ok := s.reverse[marker]
	return value, ok
}
