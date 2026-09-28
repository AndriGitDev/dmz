// Package surrogates provides a device-local, in-memory prototype session.
// It is not a persistent or encrypted Portal/Gateway vault.
package surrogates

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

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
	forward    map[string]string
	reverse    map[string]string
	serial     int
	generation uint64
	namespace  string
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
	matches := detect.Detect(input)
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
	}
	return fmt.Sprintf("⟦VALUE-%s-%d: XXXXX⟧", s.namespace, n)
}
