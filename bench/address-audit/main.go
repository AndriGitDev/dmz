// Address vocabulary format audit. Source forms and individual failures stay
// outside the checkout; stderr contains aggregate progress only.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/AndriGitDev/dmz/packages/detect"
	"golang.org/x/text/unicode/norm"
)

type format struct {
	name     string
	negative bool
	make     func(string) (string, bool)
}

func formats() []format {
	positive := func(suffix string) func(string) (string, bool) {
		return func(form string) (string, bool) { return form + suffix, true }
	}
	return []format{
		{"number", false, positive(" 99")},
		{"letter", false, positive(" 99b")},
		{"postcode", false, positive(" 99, 999 Sýnibær")},
		{"postcode_no_comma", false, positive(" 99 999 Sýnibær")},
		{"range", false, positive(" 97 - 99")},
		{"unit", false, positive(" 99 / 2")},
		{"range_nonbreaking_hyphen", false, positive(" 97\u201199")},
		{"range_unicode_hyphen", false, positive(" 97\u201099")},
		{"lowercase", false, func(s string) (string, bool) { return strings.ToLower(s) + " 99", true }},
		{"uppercase", false, func(s string) (string, bool) { return strings.ToUpper(s) + " 99", true }},
		{"decomposed", false, func(s string) (string, bool) { return norm.NFD.String(s) + " 99", true }},
		{"nbsp", false, func(s string) (string, bool) { return strings.ReplaceAll(s, " ", "\u00a0") + "\u00a099", true }},
		{"narrow_nbsp", false, func(s string) (string, bool) { return strings.ReplaceAll(s, " ", "\u202f") + "\u202f99", true }},
		{"street_nonbreaking_hyphen", false, func(s string) (string, bool) {
			return strings.ReplaceAll(s, "-", "\u2011") + " 99", strings.Contains(s, "-")
		}},
		{"street_unicode_hyphen", false, func(s string) (string, bool) {
			return strings.ReplaceAll(s, "-", "\u2010") + " 99", strings.Contains(s, "-")
		}},
		{"street_apostrophe", false, func(s string) (string, bool) {
			return strings.ReplaceAll(s, "'", "’") + " 99", strings.Contains(s, "'")
		}},
		{"currency", true, positive(" 99 kr.")},
		{"currency_nbsp", true, positive(" 99\u00a0ISK")},
		{"decimal_currency", true, positive(" 99,50 kr.")},
		{"overlong_number", true, positive(" 9999")},
		{"overlong_unit", true, positive(" 99 / 2222")},
		{"no_number", true, positive("")},
	}
}

type counts struct {
	Cases        int `json:"cases"`
	Passed       int `json:"passed"`
	Partial      int `json:"partial"`
	Missed       int `json:"missed"`
	FalseAddress int `json:"false_address"`
}

func classify(input string, negative bool, matches []detect.Match) string {
	var addresses []detect.Match
	for _, match := range matches {
		if match.Kind == detect.Address {
			addresses = append(addresses, match)
		}
	}
	if negative {
		if len(addresses) == 0 {
			return "passed"
		}
		return "false_address"
	}
	if len(addresses) == 1 && addresses[0].Start == 0 && addresses[0].End == len(input) {
		return "passed"
	}
	if len(addresses) > 0 {
		return "partial"
	}
	return "missed"
}

func external(path string, exists bool) (string, error) {
	_, source, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(source), "../.."))
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid external path")
	}
	if exists {
		abs, err = filepath.EvalSymlinks(abs)
	} else {
		var parent string
		parent, err = filepath.EvalSymlinks(filepath.Dir(abs))
		abs = filepath.Join(parent, filepath.Base(abs))
	}
	if err != nil {
		return "", fmt.Errorf("external path is unavailable")
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
		return "", fmt.Errorf("source and outputs must stay outside checkout")
	}
	return abs, nil
}

func digest(data []byte) string { h := sha256.Sum256(data); return hex.EncodeToString(h[:]) }

func run(inputPath, outputPath, failurePath string, failureLimit int) error {
	inputPath, err := external(inputPath, true)
	if err != nil {
		return err
	}
	outputPath, err = external(outputPath, false)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("unable to read source")
	}
	lexicon, err := detect.LoadLexicon(data)
	if err != nil {
		return err
	}
	var pack struct {
		Addresses []string `json:"addresses"`
	}
	if err = json.Unmarshal(data, &pack); err != nil || len(pack.Addresses) == 0 {
		return fmt.Errorf("empty or invalid address pack")
	}
	var failures *os.File
	if failurePath != "" {
		failurePath, err = external(failurePath, false)
		if err != nil {
			return err
		}
		failures, err = os.OpenFile(failurePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return fmt.Errorf("cannot create new private failure file")
		}
		defer failures.Close()
	}
	output, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("cannot create new aggregate output")
	}
	defer output.Close()
	report := map[string]any{
		"schema": 1, "scope": "Mechanical formats of approved vocabulary forms; Address only; not representative or independent validation",
		"source_sha256": digest(data), "forms": len(pack.Addresses), "independent_validation": false,
		"metric": "One complete Address span for each positive generated value; no Address span for generated negatives",
	}
	results := map[string]counts{}
	written := 0
	for _, format := range formats() {
		var totals counts
		for _, form := range pack.Addresses {
			text, applicable := format.make(form)
			if !applicable {
				continue
			}
			matches := lexicon.Detect(text, nil)
			status := classify(text, format.negative, matches)
			totals.Cases++
			switch status {
			case "passed":
				totals.Passed++
			case "partial":
				totals.Partial++
			case "missed":
				totals.Missed++
			case "false_address":
				totals.FalseAddress++
			}
			if status != "passed" && failures != nil && written < failureLimit {
				if err := json.NewEncoder(failures).Encode(map[string]any{"format": format.name, "form_sha256": digest([]byte(form)), "input": text, "status": status, "matches": matches}); err != nil {
					return err
				}
				written++
			}
		}
		results[format.name] = totals
		fmt.Fprintf(os.Stderr, "completed %s: %d/%d passed\n", format.name, totals.Passed, totals.Cases)
	}
	report["formats"], report["private_failures_saved"] = results, written
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func main() {
	input := flag.String("lexicon", "", "private approved address pack")
	output := flag.String("output", "", "new private aggregate report")
	failures := flag.String("failures", "", "optional new private failure examples")
	limit := flag.Int("failure-limit", 100, "maximum saved individual failures")
	flag.Parse()
	if *input == "" || *output == "" || *limit < 0 {
		fmt.Fprintln(os.Stderr, "lexicon/output and nonnegative failure limit required")
		os.Exit(1)
	}
	if err := run(*input, *output, *failures, *limit); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
