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
	Allowed     bool      `json:"allowed"`
	Transformed string    `json:"transformed,omitempty"`
	Findings    []Finding `json:"findings"`
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
		if utf8.RuneCountInString(term) < 3 || utf8.RuneCountInString(term) > 80 || strings.ContainsAny(term, "\r\n⟦⟧") || !utf8.ValidString(term) || strings.IndexFunc(term, func(r rune) bool { return unicode.IsMark(r) || unicode.Is(unicode.Cf, r) }) >= 0 {
			return fmt.Errorf("tenant terms must be 3–80 characters without markers, newlines, combining marks or format characters")
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
	// Only exact markers issued by this live session may bypass detection.
	// Blank their bytes in the scan copy so match offsets still address input.
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
			return markerReview()
		}
		end += start + len("⟧")
		if _, known := s.reverse[input[start:end]]; !known {
			return markerReview()
		}
		issuedMarkers = append(issuedMarkers, NameSpan{start, end})
		for i := start; i < end; i++ {
			scan[i] = ' '
		}
		offset = end
	}
	matches := detect.DetectWithTerms(string(scan), s.tenantTerms)
	result := Result{Allowed: true, Findings: make([]Finding, 0, len(matches))}
	for _, m := range matches {
		action := "mask"
		if m.Confidence == detect.Uncertain && m.Kind != detect.Kennitala {
			action = "review"
		}
		if m.Kind == detect.Secret {
			action = "block"
		}
		if action != "mask" {
			result.Allowed = false
		}
		result.Findings = append(result.Findings, Finding{m.Kind, m.Confidence, action, m.Reason})
	}
	if !result.Allowed {
		return result
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
			strings.TrimSpace(input[name.Start:name.End]) == "" || strings.ContainsAny(input[name.Start:name.End], "⟦⟧") {
			return nameReview()
		}
		for _, marker := range issuedMarkers {
			if name.Start < marker.End && marker.Start < name.End {
				return nameReview()
			}
		}
		previousEnd = name.End
		start, end := name.Start, name.End
		kept := make([]detect.Match, 0, len(matches)+1)
		for _, m := range matches {
			if start < m.End && m.Start < end {
				if m.Kind != detect.Name {
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
	sort.Slice(matches, func(i, j int) bool { return matches[i].Start < matches[j].Start })
	for i := 1; i < len(matches); i++ {
		if matches[i].Start < matches[i-1].End {
			return nameReview()
		}
	}
	var out strings.Builder
	previous := 0
	for _, m := range matches {
		out.WriteString(input[previous:m.Start])
		raw := input[m.Start:m.End]
		key := string(m.Kind) + ":" + detect.Canonical(m.Kind, raw)
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
	return result
}

func nameReview() Result {
	return Result{Allowed: false, Findings: []Finding{{detect.Kind("name_review"), detect.Uncertain, "review", "invalid or conflicting model spans"}}}
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
