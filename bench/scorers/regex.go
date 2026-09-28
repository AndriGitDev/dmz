package main

import (
	"regexp"
	"sort"

	"github.com/AndriGitDev/dmz/packages/detect"
)

// regexDetect is a deliberately plain pattern-only baseline. It has no
// date/checksum/Luhn/mod-97 validation, lexicon or tenant dictionary. It is
// not a production detector and should be scored on exactly the same corpus.
func regexDetect(input string) []detect.Match {
	patterns := []struct {
		kind detect.Kind
		re   *regexp.Regexp
	}{
		{detect.Secret, regexp.MustCompile(`(?:sk-[A-Za-z0-9_\-]{12,}|AKIA[0-9A-Z]{16}|(?:postgres|mysql)://[^\s]*[A-Za-z0-9/])`)},
		{detect.IBAN, regexp.MustCompile(`(?i)IS(?: ?[0-9]){24}`)},
		{detect.Email, regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)},
		{detect.Kennitala, regexp.MustCompile(`[0-9]{6}-?[0-9]{4}`)},
		{detect.Card, regexp.MustCompile(`[0-9](?:[0-9 \-]{11,25})[0-9]`)},
		{detect.Phone, regexp.MustCompile(`(?:\+354[ \-]?)?[0-9]{3}[ \-]?[0-9]{4}`)},
		{detect.Name, regexp.MustCompile(`\p{Lu}\p{L}+ \p{Lu}\p{L}+(?:son|dóttir|bur)`)},
	}
	var matches []detect.Match
	for _, p := range patterns {
		for _, loc := range p.re.FindAllStringIndex(input, -1) {
			matches = append(matches, detect.Match{Start: loc[0], End: loc[1], Kind: p.kind, Confidence: detect.High, Reason: "regex-only"})
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Start == matches[j].Start {
			return matches[i].End > matches[j].End
		}
		return matches[i].Start < matches[j].Start
	})
	return matches
}
