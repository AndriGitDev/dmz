package detect

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Lexicon is immutable after construction and may be shared between sessions.
// Corpus-derived matches are uncertain: policy must review/block them.
type Lexicon struct{ names, addresses, nameParts, nameSeeds *lexiconNode }
type lexiconNode struct {
	next     map[string]*lexiconNode
	terminal bool
}

var bundledLexicon *Lexicon
var lexicalTokenRE = regexp.MustCompile(`[\p{L}]+(?:[-’'][\p{L}]+)*|\.`)
var patronymicEndRE = regexp.MustCompile(`(?:sonar|syni|son|dóttir|dóttur|burs|bur)$`)
var addressTailRE = regexp.MustCompile(`^[ \t]+[0-9]{1,3}[A-Za-z]?(?:/[0-9]{1,3}[A-Za-z]?)?(?:,[ \t]*|[ \t]+)[0-9]{3}(?:[ \t]+\p{Lu}\p{L}{1,40})?`)

// LoadLexicon accepts a bounded data pack. Errors never include source entries.
func LoadLexicon(data []byte) (*Lexicon, error) {
	if len(data) > 4*1024*1024 {
		return nil, fmt.Errorf("lexicon exceeds size limit")
	}
	var pack struct {
		NameSeeds []string `json:"name_seeds"`
		NameParts []string `json:"name_parts"`
		Schema    int      `json:"schema"`
		Names     []string `json:"names"`
		Addresses []string `json:"addresses"`
	}
	if err := json.Unmarshal(data, &pack); err != nil || pack.Schema != 1 {
		return nil, fmt.Errorf("invalid lexicon schema")
	}
	if len(pack.NameSeeds) > 50000 || len(pack.NameParts) > 50000 || len(pack.Names) > 50000 || len(pack.Addresses) > 50000 {
		return nil, fmt.Errorf("too many lexicon entries")
	}
	l := &Lexicon{names: &lexiconNode{}, addresses: &lexiconNode{}, nameParts: &lexiconNode{}, nameSeeds: &lexiconNode{}}
	for i, forms := range [][]string{pack.Names, pack.Addresses, pack.NameParts, pack.NameSeeds} {
		root := l.names
		if i == 1 {
			root = l.addresses
		}
		if i == 2 {
			root = l.nameParts
		}
		if i == 3 {
			root = l.nameSeeds
		}
		for _, form := range forms {
			if len(form) == 0 || len(form) > 256 || !utf8.ValidString(form) {
				return nil, fmt.Errorf("invalid lexicon entry")
			}
			for _, r := range form {
				if !unicode.IsLetter(r) && !strings.ContainsRune(" .-’'", r) {
					return nil, fmt.Errorf("invalid lexicon character")
				}
			}
			tokens := lexicalTokenRE.FindAllString(form, -1)
			if len(tokens) == 0 || len(tokens) > 8 || strings.Join(tokens, " ") != form {
				return nil, fmt.Errorf("noncanonical lexicon entry")
			}
			if i >= 2 && (len(tokens) != 1 || !unicode.IsUpper([]rune(form)[0]) || utf8.RuneCountInString(form) < 2) {
				return nil, fmt.Errorf("invalid name part")
			}
			node := root
			for _, token := range tokens {
				if i == 1 {
					token = strings.ToLower(token)
				}
				if node.next == nil {
					node.next = map[string]*lexiconNode{}
				}
				if node.next[token] == nil {
					node.next[token] = &lexiconNode{}
				}
				node = node.next[token]
			}
			node.terminal = true
		}
	}
	return l, nil
}

// Detect applies this lexicon as well as all baseline rules and tenant terms.
func (l *Lexicon) Detect(input string, terms []string) []Match {
	return detectWithLexicon(input, terms, l)
}

func (l *Lexicon) matches(input string) []Match {
	if l == nil {
		return nil
	}
	tokens := lexicalTokenRE.FindAllStringIndex(input, -1)
	var result []Match
	for i, loc := range tokens {
		if !bounded(input, loc[0], loc[1]) {
			continue
		}
		for mode, root := range []*lexiconNode{l.names, l.addresses, l.nameSeeds} {
			node := root
			for j := i; j < len(tokens) && j < i+8; j++ {
				part := tokens[j]
				if j > i && strings.Trim(input[tokens[j-1][1]:part[0]], " \t") != "" {
					break
				}
				token := input[part[0]:part[1]]
				if mode == 1 {
					token = strings.ToLower(token)
				}
				node = node.next[token]
				if node == nil {
					break
				}
				if !node.terminal || !bounded(input, loc[0], part[1]) {
					continue
				}
				if mode == 2 {
					result = append(result, Match{loc[0], part[1], Name, Uncertain, "corpus-name-seed"})
				} else if mode == 0 {
					result = append(result, Match{loc[0], part[1], Name, Uncertain, "corpus-name-lexicon"})
				} else if tail := addressTailRE.FindStringIndex(input[part[1]:]); tail != nil {
					end := part[1] + tail[1]
					if bounded(input, loc[0], end) && !currencyAfter(input, end) {
						result = append(result, Match{loc[0], end, Address, Uncertain, "gazetteer-number-postcode"})
					}
				}
			}
		}
	}
	return result
}

func currencyAfter(input string, end int) bool {
	rest := strings.TrimLeft(input[end:], " \t")
	for _, word := range []string{"kr", "krónur", "króna", "ISK"} {
		if end >= len(word) && strings.EqualFold(input[end-len(word):end], word) && bounded(input, end-len(word), end) {
			return true
		}
		if len(rest) >= len(word) && strings.EqualFold(rest[:len(word)], word) && bounded(rest, 0, len(word)) {
			return true
		}
	}
	return false
}

// extendNames joins only training-supported name components to existing matches.
// It never crosses punctuation, newlines, or an unrecognized word; components
// alone do not create detections. Older packs retain their original behavior.
func (l *Lexicon) extendNames(input string, matches []Match) {
	if l == nil || len(l.nameParts.next) == 0 {
		return
	}
	tokens := lexicalTokenRE.FindAllStringIndex(input, -1)
	known := func(i int) bool {
		n := l.nameParts.next[input[tokens[i][0]:tokens[i][1]]]
		return n != nil && n.terminal && bounded(input, tokens[i][0], tokens[i][1])
	}
	gap := func(a, b int) bool { return a < b && strings.Trim(input[a:b], " \t") == "" }
	for k := range matches {
		m := &matches[k]
		if m.Kind != Name {
			continue
		}
		first := sort.Search(len(tokens), func(i int) bool { return tokens[i][0] >= m.Start })
		last := sort.Search(len(tokens), func(i int) bool { return tokens[i][1] >= m.End })
		if first == len(tokens) || last == len(tokens) || tokens[first][0] != m.Start || tokens[last][1] != m.End {
			continue
		}
		for first > 0 && last-first < 7 && !patronymicEndRE.MatchString(input[tokens[first-1][0]:tokens[first-1][1]]) && known(first-1) && gap(tokens[first-1][1], tokens[first][0]) {
			first--
		}
		for last+1 < len(tokens) && last-first < 7 && !patronymicEndRE.MatchString(input[tokens[last][0]:tokens[last][1]]) && known(last+1) && gap(tokens[last][1], tokens[last+1][0]) {
			last++
		}
		if m.Start != tokens[first][0] || m.End != tokens[last][1] {
			m.Start = tokens[first][0]
			m.End = tokens[last][1]
			m.Confidence = Uncertain
			if m.Reason == "corpus-name-seed" {
				m.Reason = "corpus-name-seed-boundary"
			} else {
				m.Reason = "corpus-name-boundary"
			}
		}
	}
}

// A component with strong training evidence may be a standalone name, but an
// adjacent unknown capitalized word signals a possibly incomplete name span.
// Decline only the new seed candidate; baseline and full-name matches survive.
func (l *Lexicon) filterSeedBoundaries(input string, matches []Match) []Match {
	if l == nil || len(l.nameSeeds.next) == 0 {
		return matches
	}
	tokens := lexicalTokenRE.FindAllStringIndex(input, -1)
	gap := func(a, b int) bool { return a < b && strings.Trim(input[a:b], " \t") == "" }
	upper := func(i int) bool {
		r, _ := utf8.DecodeRuneInString(input[tokens[i][0]:tokens[i][1]])
		return unicode.IsUpper(r)
	}
	result := matches[:0]
	for _, m := range matches {
		open := false
		if strings.HasPrefix(m.Reason, "corpus-name-seed") {
			first := sort.Search(len(tokens), func(i int) bool { return tokens[i][0] >= m.Start })
			last := sort.Search(len(tokens), func(i int) bool { return tokens[i][1] >= m.End })
			if first > 0 && upper(first-1) && gap(tokens[first-1][1], m.Start) {
				open = true
			}
			if last+1 < len(tokens) && upper(last+1) && gap(m.End, tokens[last+1][0]) && !patronymicEndRE.MatchString(input[m.Start:m.End]) {
				open = true
			}
			// Do not mistake a surname after an initial for a complete name.
			if first >= 2 && input[tokens[first-1][0]:tokens[first-1][1]] == "." && upper(first-2) && utf8.RuneCountInString(input[tokens[first-2][0]:tokens[first-2][1]]) == 1 && strings.Trim(input[tokens[first-2][1]:tokens[first-1][0]], " \t") == "" && gap(tokens[first-1][1], m.Start) {
				open = true
			}
		}
		if !open {
			result = append(result, m)
		}
	}
	return result
}
