package detect

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Lexicon is immutable after construction and may be shared between sessions.
// Corpus-derived matches are uncertain: policy must review/block them.
type Lexicon struct{ names, addresses *lexiconNode }
type lexiconNode struct {
	next     map[string]*lexiconNode
	terminal bool
}

var bundledLexicon *Lexicon
var lexicalTokenRE = regexp.MustCompile(`[\p{L}]+(?:[-’'][\p{L}]+)*|\.`)
var addressTailRE = regexp.MustCompile(`^[ \t]+[0-9]{1,3}[A-Za-z]?(?:/[0-9]{1,3}[A-Za-z]?)?(?:,[ \t]*|[ \t]+)[0-9]{3}(?:[ \t]+\p{Lu}\p{L}{1,40})?`)

// LoadLexicon accepts a bounded data pack. Errors never include source entries.
func LoadLexicon(data []byte) (*Lexicon, error) {
	if len(data) > 4*1024*1024 {
		return nil, fmt.Errorf("lexicon exceeds size limit")
	}
	var pack struct {
		Schema    int      `json:"schema"`
		Names     []string `json:"names"`
		Addresses []string `json:"addresses"`
	}
	if err := json.Unmarshal(data, &pack); err != nil || pack.Schema != 1 {
		return nil, fmt.Errorf("invalid lexicon schema")
	}
	if len(pack.Names) > 50000 || len(pack.Addresses) > 50000 {
		return nil, fmt.Errorf("too many lexicon entries")
	}
	l := &Lexicon{names: &lexiconNode{}, addresses: &lexiconNode{}}
	for i, forms := range [][]string{pack.Names, pack.Addresses} {
		root := l.names
		if i == 1 {
			root = l.addresses
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
		for mode, root := range []*lexiconNode{l.names, l.addresses} {
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
				if mode == 0 {
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
