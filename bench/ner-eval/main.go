// Command ner-eval evaluates PERSON spans locally without printing corpus text.
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"unicode/utf8"

	"github.com/AndriGitDev/dmz/packages/detect"
)

type entity struct {
	Kind  string `json:"kind"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}
type sample struct {
	Split    string   `json:"split"`
	Text     string   `json:"text"`
	Entities []entity `json:"entities"`
}
type report struct {
	LexiconSHA256 string  `json:"lexicon_sha256,omitempty"`
	SHA256        string  `json:"input_sha256"`
	Split         string  `json:"split"`
	Sentences     int     `json:"sentences"`
	TP            int     `json:"tp"`
	FP            int     `json:"fp"`
	FN            int     `json:"fn"`
	Precision     float64 `json:"precision"`
	Recall        float64 `json:"recall"`
	Method        string  `json:"method"`
}

func evaluate(reader io.Reader, split string) (report, error) {
	return evaluateWithFinder(reader, split, detect.Detect)
}

func evaluateWithFinder(reader io.Reader, split string, finder func(string) []detect.Match) (report, error) {
	r := report{Split: split, Method: "exact UTF-8 span; detect.Name vs corpus Person; reconstructed token-spaced text; no outbound-exposure claim"}
	hash := sha256.New()
	scanner := bufio.NewScanner(io.TeeReader(reader, hash))
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		var s sample
		if err := json.Unmarshal(scanner.Bytes(), &s); err != nil {
			return r, fmt.Errorf("invalid JSON at row %d", r.Sentences+1)
		}
		if s.Split != split || s.Text == "" || !utf8.ValidString(s.Text) {
			return r, fmt.Errorf("invalid split/text at row %d", r.Sentences+1)
		}
		gold := map[[2]int]bool{}
		for _, e := range s.Entities {
			if e.Start < 0 || e.End <= e.Start || e.End > len(s.Text) || !utf8.ValidString(s.Text[e.Start:e.End]) {
				return r, fmt.Errorf("invalid span at row %d", r.Sentences+1)
			}
			if e.Kind == "Person" {
				gold[[2]int{e.Start, e.End}] = true
			}
		}
		for _, m := range finder(s.Text) {
			if m.Kind != detect.Name {
				continue
			}
			key := [2]int{m.Start, m.End}
			if gold[key] {
				r.TP++
				delete(gold, key)
			} else {
				r.FP++
			}
		}
		r.FN += len(gold)
		r.Sentences++
	}
	if err := scanner.Err(); err != nil {
		return r, fmt.Errorf("unable to read corpus")
	}
	if r.Sentences == 0 {
		return r, fmt.Errorf("empty corpus")
	}
	r.SHA256 = hex.EncodeToString(hash.Sum(nil))
	if r.TP+r.FP > 0 {
		r.Precision = float64(r.TP) / float64(r.TP+r.FP)
	}
	if r.TP+r.FN > 0 {
		r.Recall = float64(r.TP) / float64(r.TP+r.FN)
	}
	return r, nil
}

func main() {
	path := flag.String("input", "", "local prepared JSONL file")
	split := flag.String("split", "dev", "split to evaluate; defaults to dev to reserve test")
	lexiconPath := flag.String("lexicon", "", "optional local lexicon data pack")
	flag.Parse()
	if *split != "train" && *split != "dev" && *split != "test" {
		fmt.Fprintln(os.Stderr, "invalid split")
		os.Exit(2)
	}
	f, err := os.Open(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot open local corpus")
		os.Exit(1)
	}
	defer f.Close()
	finder := detect.Detect
	var lexiconHash string
	if *lexiconPath != "" {
		data, err := os.ReadFile(*lexiconPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "cannot read lexicon")
			os.Exit(1)
		}
		l, err := detect.LoadLexicon(data)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		sum := sha256.Sum256(data)
		lexiconHash = hex.EncodeToString(sum[:])
		finder = func(s string) []detect.Match { return l.Detect(s, nil) }
	}
	r, err := evaluateWithFinder(f, *split, finder)
	r.LexiconSHA256 = lexiconHash
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(r); err != nil {
		os.Exit(1)
	}
}
