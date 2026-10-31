// Package surrogates provides a device-local, in-memory prototype session.
// It is not a persistent or encrypted Portal/Gateway vault.
package surrogates

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/AndriGitDev/dmz/packages/detect"
)

type Finding struct {
	Kind       detect.Kind       `json:"kind"`
	Confidence detect.Confidence `json:"confidence"`
	Action     string            `json:"action"`
	Reason     string            `json:"reason"`
}

type Result struct {
	Allowed        bool         `json:"allowed"`
	Transformed    string       `json:"transformed,omitempty"`
	Findings       []Finding    `json:"findings"`
	SpansAvailable bool         `json:"spans_available"`
	DetectedSpans  []EntitySpan `json:"detected_spans"`
}

// EntitySpan describes the final non-overlapping detection set in original
// UTF-8 byte offsets. It contains no original value or reversible mapping.
type EntitySpan struct {
	Start int         `json:"start"`
	End   int         `json:"end"`
	Kind  detect.Kind `json:"kind"`
}

// Availability means that individual typed spans exist, not that detection is
// complete or correct. Whole-text masking and invalid-span guards leave it false.
func withDetectedSpans(result Result, matches []detect.Match) Result {
	for _, match := range matches {
		if match.Kind == detect.UnicodeReview {
			return result
		}
	}
	result.SpansAvailable = true
	result.DetectedSpans = make([]EntitySpan, 0, len(matches))
	for _, match := range matches {
		result.DetectedSpans = append(result.DetectedSpans, EntitySpan{match.Start, match.End, match.Kind})
	}
	return result
}

type Session struct {
	forward     map[string]string
	reverse     map[string]string
	serial      int
	generation  uint64
	namespace   string
	tenantTerms []string
}

// SetTenantTerms replaces the device-local exact-match dictionary and starts
// a fresh mapping session. Terms are deliberately bounded for browser use.
func (s *Session) SetTenantTerms(terms []string) error {
	if len(terms) > 32 {
		return fmt.Errorf("at most 32 tenant terms")
	}
	clean := make([]string, 0, len(terms))
	seen := make(map[string]bool)
	for _, term := range terms {
		term = strings.TrimSpace(term)
		if term == "" {
			continue
		}
		if utf8.RuneCountInString(term) < 3 || utf8.RuneCountInString(term) > 80 || strings.ContainsAny(term, "\x00\r\n⟦⟧") || !utf8.ValidString(term) || strings.IndexFunc(term, func(r rune) bool { return unicode.IsMark(r) || unicode.Is(unicode.Cf, r) }) >= 0 {
			return fmt.Errorf("tenant terms must be 3–80 characters without NULs, markers, newlines, combining marks or format characters")
		}
		if !seen[term] {
			clean = append(clean, term)
			seen[term] = true
		}
	}
	s.Reset()
	s.tenantTerms = clean
	return nil
}

func NewSession() *Session {
	return &Session{forward: make(map[string]string), reverse: make(map[string]string), namespace: newNamespace()}
}

func (s *Session) Reset() {
	s.forward = make(map[string]string)
	s.reverse = make(map[string]string)
	s.serial = 0
	s.generation++
	s.namespace = newNamespace()
	s.tenantTerms = nil
}

func newNamespace() string {
	var entropy [8]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		// A session without a unique namespace could restore an old answer to
		// a different real value. Refuse to run when entropy is unavailable.
		panic("dmz: session entropy unavailable")
	}
	return strings.ToUpper(hex.EncodeToString(entropy[:]))
}

// Process blocks the entire prompt if any match is uncertain or unsupported.
// No partially transformed text is returned in that case.
func (s *Session) Process(input string) Result {
	return s.ProcessWithNames(input, nil)
}

// NameSpan is a local model finding in UTF-8 byte offsets. Model findings may
// add masking, but must never override an existing review/block decision.
type NameSpan struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

func (s *Session) ProcessWithNames(input string, names []NameSpan) Result {
	return s.process(input, names, nil, false)
}

// ProcessBestEffort masks uncertain identifiers and secrets for dmz. The
// default policy remains available to other callers and baseline evaluations.
func (s *Session) ProcessBestEffort(input string, names []NameSpan) Result {
	return s.process(input, names, nil, true)
}

// ProcessBestEffortWithEntities adds typed local-model findings after the existing
// core and name pipeline. Additions never replace or extend an existing match.
// Callers must supply sorted, non-overlapping original UTF-8 spans; unsupported
// kinds or malformed boundaries reject the request without creating mappings.
// Existing whole-text safety fallbacks take precedence over model processing.
// This experimental adapter does not select or enable any external model.
func (s *Session) ProcessBestEffortWithEntities(input string, names []NameSpan, entities []EntitySpan) Result {
	return s.process(input, names, entities, true)
}

// When individual spans cannot safely represent an input, cover the whole
// input instead of releasing it or treating the content as a reason to block.
func (s *Session) maskWhole(input string) Result {
	key := "whole:" + input
	marker, exists := s.forward[key]
	if !exists {
		marker = s.makeSurrogate(detect.Kind("text"), input)
		s.forward[key] = marker
		s.reverse[marker] = input
	}
	return Result{Allowed: true, Transformed: marker, Findings: []Finding{{detect.Kind("text"), detect.Uncertain, "mask", "whole-text-coverage"}}}
}

func (s *Session) process(input string, names []NameSpan, entities []EntitySpan, bestEffort bool) Result {
	// Only exact markers issued by this live session may bypass detection.
	// Replace their bytes with opaque barriers so offsets still address input.
	// Whitespace would let context recognizers skip a protected value and
	// reinterpret a following token as its replacement on the next scan.
	// Unknown, edited and nested markers need review; accepting arbitrary marker
	// contents would let a caller hide real identifiers inside a fake marker.
	scan := []byte(input)
	var issuedMarkers []NameSpan
	for offset := 0; offset < len(input); {
		relative := strings.IndexAny(input[offset:], "⟦⟧")
		if relative < 0 {
			break
		}
		start := offset + relative
		end := strings.Index(input[start:], "⟧")
		if end < 0 || !strings.HasPrefix(input[start:], "⟦") {
			if bestEffort {
				return s.maskWhole(input)
			}
			return markerReview()
		}
		end += start + len("⟧")
		if _, known := s.reverse[input[start:end]]; !known {
			if bestEffort {
				return s.maskWhole(input)
			}
			return markerReview()
		}
		issuedMarkers = append(issuedMarkers, NameSpan{start, end})
		for i := start; i < end; i++ {
			scan[i] = 0
		}
		offset = end
	}
	matches := detect.DetectWithTerms(string(scan), s.tenantTerms)
	if bestEffort {
		for _, m := range matches {
			if m.Kind == detect.UnicodeReview || (m.Kind == detect.Secret && m.Reason != "private-key-block" && strings.HasPrefix(input[m.Start:m.End], "-----BEGIN")) {
				return s.maskWhole(input)
			}
		}
	}
	result := Result{Allowed: true, Findings: make([]Finding, 0, len(matches))}
	for _, m := range matches {
		action := "mask"
		if !bestEffort && m.Confidence == detect.Uncertain && m.Kind != detect.Kennitala {
			action = "review"
		}
		if !bestEffort && m.Kind == detect.Secret {
			action = "block"
		}
		if action != "mask" {
			result.Allowed = false
		}
		result.Findings = append(result.Findings, Finding{m.Kind, m.Confidence, action, m.Reason})
	}
	if !result.Allowed {
		return withDetectedSpans(result, matches)
	}
	// Only after the original protection rules allow the input, combine names.
	// Reject malformed/conflicting spans rather than silently losing coverage.
	previousEnd := 0
	if len(names) > 512 {
		return nameReview()
	}
	for _, name := range names {
		if name.Start < previousEnd || name.Start < 0 || name.End <= name.Start || name.End > len(input) ||
			!utf8.ValidString(input[:name.Start]) || !utf8.ValidString(input[:name.End]) ||
			splitsCombiningSequence(input, name.Start) || splitsCombiningSequence(input, name.End) ||
			strings.TrimSpace(input[name.Start:name.End]) == "" || strings.ContainsAny(input[name.Start:name.End], "⟦⟧") {
			return nameReview()
		}
		for _, marker := range issuedMarkers {
			if name.Start < marker.End && marker.Start < name.End {
				return nameReview()
			}
		}
		previousEnd = name.End
		// Keep the full existing mask when it already contains this name.
		// In particular, names inside emails must not replace the email mask.
		alreadyCovered := false
		for _, m := range matches {
			if bestEffort && m.Start <= name.Start && name.End <= m.End {
				alreadyCovered = true
				break
			}
		}
		if alreadyCovered {
			continue
		}
		start, end := name.Start, name.End
		kept := make([]detect.Match, 0, len(matches)+1)
		for _, m := range matches {
			if start < m.End && m.Start < end {
				if !bestEffort && m.Kind != detect.Name {
					return nameReview()
				}
				if m.Start < start {
					start = m.Start
				}
				if m.End > end {
					end = m.End
				}
			} else {
				kept = append(kept, m)
			}
		}
		matches = append(kept, detect.Match{Start: start, End: end, Kind: detect.Name, Confidence: detect.High, Reason: "local-trained-name"})
		result.Findings = append(result.Findings, Finding{detect.Name, detect.High, "mask", "local-trained-name"})
	}
	// Validate the complete supplemental response before creating any mappings.
	// Preserve existing typed matches, including names, on every overlap. This
	// avoids a broader or differently typed model span erasing a correct match.
	if len(names)+len(entities) > 512 {
		return entityReview()
	}
	previousEnd = 0
	for _, entity := range entities {
		if !supportedEntityKind(entity.Kind) || entity.Start < previousEnd || entity.Start < 0 ||
			entity.End <= entity.Start || entity.End > len(input) ||
			!utf8.ValidString(input[:entity.Start]) || !utf8.ValidString(input[:entity.End]) ||
			splitsCombiningSequence(input, entity.Start) || splitsCombiningSequence(input, entity.End) ||
			strings.TrimSpace(input[entity.Start:entity.End]) == "" ||
			strings.ContainsAny(input[entity.Start:entity.End], "⟦⟧") {
			return entityReview()
		}
		for _, marker := range issuedMarkers {
			if entity.Start < marker.End && marker.Start < entity.End {
				return entityReview()
			}
		}
		previousEnd = entity.End
		conflict := false
		for _, existing := range matches {
			if entity.Start < existing.End && existing.Start < entity.End {
				conflict = true
				break
			}
		}
		if conflict {
			continue
		}
		matches = append(matches, detect.Match{Start: entity.Start, End: entity.End,
			Kind: entity.Kind, Confidence: detect.Uncertain, Reason: "local-model-entity"})
		result.Findings = append(result.Findings, Finding{entity.Kind, detect.Uncertain, "mask", "local-model-entity"})
	}

	sort.Slice(matches, func(i, j int) bool { return matches[i].Start < matches[j].Start })
	for i := 1; i < len(matches); i++ {
		if matches[i].Start < matches[i-1].End {
			if bestEffort {
				return s.maskWhole(input)
			}
			return nameReview()
		}
	}
	var out strings.Builder
	previous := 0
	for _, m := range matches {
		out.WriteString(input[previous:m.Start])
		raw := input[m.Start:m.End]
		// A marker has one exact inverse. Canonical identifiers can have several
		// spellings in the same prompt, so sharing their marker would restore all
		// occurrences to whichever spelling was encountered first.
		key := string(m.Kind) + ":" + raw
		replacement, exists := s.forward[key]
		if !exists {
			replacement = s.makeSurrogate(m.Kind, raw)
			// The tiny prototype pools must never emit a surrogate already present
			// in the input or used by another mapping in this session.
			for strings.Contains(input, replacement) || s.reverse[replacement] != "" {
				replacement = s.makeSurrogate(m.Kind, raw)
			}
			s.forward[key] = replacement
			s.reverse[replacement] = raw
		}
		out.WriteString(replacement)
		previous = m.End
	}
	out.WriteString(input[previous:])
	result.Transformed = out.String()
	return withDetectedSpans(result, matches)
}

func nameReview() Result {
	return Result{Allowed: false, Findings: []Finding{{detect.Kind("name_review"), detect.Uncertain, "review", "invalid or conflicting model spans"}}}
}

func splitsCombiningSequence(input string, offset int) bool {
	if offset == len(input) {
		return false
	}
	r, _ := utf8.DecodeRuneInString(input[offset:])
	return unicode.IsMark(r)
}

func markerReview() Result {
	return Result{Allowed: false, Findings: []Finding{{detect.Kind("marker_review"), detect.Uncertain, "review", "unknown or changed session marker"}}}
}

// Restore rehydrates a complete response. NewStreamRestorer handles arbitrary
// chunk boundaries for integrations that consume a streamed response.
func (s *Session) Restore(response string) string {
	keys := make([]string, 0, len(s.reverse))
	for surrogate := range s.reverse {
		keys = append(keys, surrogate)
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
	pairs := make([]string, 0, len(keys)*2)
	for _, surrogate := range keys {
		pairs = append(pairs, surrogate, s.reverse[surrogate])
	}
	if len(pairs) == 0 {
		return response
	}
	return strings.NewReplacer(pairs...).Replace(response)
}

func (s *Session) makeSurrogate(kind detect.Kind, raw string) string {
	s.serial++
	n := s.serial
	switch kind {
	case detect.Kennitala:
		return fmt.Sprintf("⟦KT-%s-%d: XXXXX-XXXX⟧", s.namespace, n)
	case detect.Email:
		return fmt.Sprintf("⟦EMAIL-%s-%d: XXXXX@XXXXX.XXX⟧", s.namespace, n)
	case detect.IBAN:
		return fmt.Sprintf("⟦IBAN-%s-%d: ISXX XXXX XXXX XXXX XXXX XXXX XX⟧", s.namespace, n)
	case detect.Phone:
		return fmt.Sprintf("⟦PHONE-%s-%d: +354 XXX XXXX⟧", s.namespace, n)
	case detect.Card:
		return fmt.Sprintf("⟦CARD-%s-%d: XXXX XXXX XXXX XXXX⟧", s.namespace, n)
	case detect.Name:
		return fmt.Sprintf("⟦NAME-%s-%d: XXXXX XXXXX⟧", s.namespace, n)
	case detect.TenantDictionary:
		return fmt.Sprintf("⟦TERM-%s-%d: XXXXX⟧", s.namespace, n)
	}
	return fmt.Sprintf("⟦VALUE-%s-%d: XXXXX⟧", s.namespace, n)
}
