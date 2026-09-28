// Command scorer reports exact-span precision and recall on the public,
// synthetic corpus. It does not send text to a model or service.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/AndriGitDev/dmz/packages/detect"
)

type entity struct {
	Kind  detect.Kind `json:"kind"`
	Value string      `json:"value"`
}

type sample struct {
	ID       string   `json:"id"`
	Text     string   `json:"text"`
	Entities []entity `json:"entities"`
}

type span struct {
	Kind  detect.Kind
	Start int
	End   int
}

type score struct {
	TruePositive  int     `json:"tp"`
	FalsePositive int     `json:"fp"`
	FalseNegative int     `json:"fn"`
	Precision     float64 `json:"precision"`
	Recall        float64 `json:"recall"`
}

type report struct {
	Corpus      string           `json:"corpus"`
	Detector    string           `json:"detector"`
	Samples     int              `json:"samples"`
	Scoring     string           `json:"scoring"`
	PerKind     map[string]score `json:"per_kind"`
	TenantTerms []string         `json:"tenant_terms,omitempty"`
}

func main() {
	path := flag.String("corpus", "bench/corpus/v0.jsonl", "synthetic JSONL corpus")
	detectorName := flag.String("detector", "dmz", "detector: dmz or regex")
	baseline := flag.String("baseline", "", "published result to enforce no recall regression")
	flag.Parse()
	var finder func(string) []detect.Match
	var tenantTerms []string
	switch *detectorName {
	case "dmz":
		tenantTerms = []string{"Verkefni Gervi"}
		finder = func(text string) []detect.Match { return detect.DetectWithTerms(text, tenantTerms) }
	case "regex":
		finder = regexDetect
	default:
		fmt.Fprintln(os.Stderr, "unknown detector:", *detectorName)
		os.Exit(2)
	}
	result, err := evaluate(*path, *detectorName, finder)
	result.TenantTerms = tenantTerms
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *baseline != "" {
		published, err := os.ReadFile(*baseline)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		var prior report
		if err := json.Unmarshal(published, &prior); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if prior.Detector != result.Detector || !slices.Equal(prior.TenantTerms, result.TenantTerms) {
			fmt.Fprintln(os.Stderr, "baseline detector/configuration mismatch")
			os.Exit(1)
		}
		for kind, previous := range prior.PerKind {
			current, exists := result.PerKind[kind]
			if !exists || current.Recall < previous.Recall {
				fmt.Fprintf(os.Stderr, "recall regression for %s: %.3f -> %.3f\n", kind, previous.Recall, current.Recall)
				os.Exit(1)
			}
		}
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func evaluate(path, detectorName string, finder func(string) []detect.Match) (report, error) {
	f, err := os.Open(path)
	if err != nil {
		return report{}, err
	}
	defer f.Close()
	counts := map[string]score{}
	ids := map[string]bool{}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	n := 0
	for scanner.Scan() {
		n++
		var item sample
		if err := json.Unmarshal(scanner.Bytes(), &item); err != nil {
			return report{}, fmt.Errorf("line %d: %w", n, err)
		}
		if item.ID == "" || item.Text == "" || ids[item.ID] {
			return report{}, fmt.Errorf("line %d: missing text/id or duplicate id", n)
		}
		ids[item.ID] = true
		gold := map[span]bool{}
		for _, e := range item.Entities {
			if e.Kind == "" || e.Value == "" {
				return report{}, fmt.Errorf("line %d: empty annotation", n)
			}
			start := strings.Index(item.Text, e.Value)
			if start < 0 || strings.Count(item.Text, e.Value) != 1 {
				return report{}, fmt.Errorf("line %d: annotation must appear exactly once", n)
			}
			key := span{e.Kind, start, start + len(e.Value)}
			if gold[key] {
				return report{}, fmt.Errorf("line %d: duplicate annotation", n)
			}
			gold[key] = true
			if _, exists := counts[string(e.Kind)]; !exists {
				counts[string(e.Kind)] = score{}
			}
		}
		for _, found := range finder(item.Text) {
			key := span{found.Kind, found.Start, found.End}
			bucket := counts[string(found.Kind)]
			if gold[key] {
				bucket.TruePositive++
				delete(gold, key)
			} else {
				bucket.FalsePositive++
			}
			counts[string(found.Kind)] = bucket
		}
		for missed := range gold {
			bucket := counts[string(missed.Kind)]
			bucket.FalseNegative++
			counts[string(missed.Kind)] = bucket
		}
	}
	if err := scanner.Err(); err != nil {
		return report{}, err
	}
	if n == 0 {
		return report{}, errors.New("empty corpus")
	}
	for kind, bucket := range counts {
		if total := bucket.TruePositive + bucket.FalsePositive; total > 0 {
			bucket.Precision = float64(bucket.TruePositive) / float64(total)
		}
		if total := bucket.TruePositive + bucket.FalseNegative; total > 0 {
			bucket.Recall = float64(bucket.TruePositive) / float64(total)
		}
		counts[kind] = bucket
	}
	return report{Corpus: path, Detector: detectorName, Samples: n, Scoring: "exact span and entity type", PerKind: counts}, nil
}
