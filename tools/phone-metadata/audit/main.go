// Differential check against the pinned upstream library. Only aggregate counts
// are emitted; public example numbers and generated digit strings stay in RAM.
package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/AndriGitDev/dmz/packages/detect"
	"github.com/nyaruka/phonenumbers/v2"
)

func main() {
	fixturePath := flag.String("wasm-fixtures", "", "optional new private experiment JSONL for WASM parity")
	flag.Parse()
	var fixture *os.File
	if *fixturePath != "" {
		home, err := os.UserHomeDir()
		path, pathErr := filepath.Abs(*fixturePath)
		privateRoot := filepath.Join(home, ".local/share/dmz/experiments") + string(os.PathSeparator)
		if err != nil || pathErr != nil || !strings.HasPrefix(path, privateRoot) {
			fmt.Fprintln(os.Stderr, "fixtures require a private DMZ experiment path")
			os.Exit(1)
		}
		fixture, err = os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			fmt.Fprintln(os.Stderr, "cannot create new private fixtures")
			os.Exit(1)
		}
		defer fixture.Close()
	}
	counts := map[string]int{}
	conflicts := map[string]int{}
	check := func(raw string) {
		number, err := phonenumbers.Parse(raw, "ZZ")
		valid := err == nil && phonenumbers.IsValidNumber(number)
		if err == nil && phonenumbers.Format(number, phonenumbers.E164) != raw {
			counts["excluded_upstream_rewrites"]++
			return
		}
		input := "Tel: " + raw
		newPhone, other, anyPhone := false, false, false
		matches := detect.Detect(input)
		for _, m := range matches {
			if m.Kind == detect.Phone && m.Start == 5 && m.End == len(input) {
				anyPhone = true
				if m.Reason == "international-phone-numbering-plan" {
					newPhone = true
				}
			}
			if m.Kind != detect.Phone && m.Start < len(input) && m.End > 5 {
				other = true
				conflicts[string(m.Kind)]++
			}
		}
		counts["cases"]++
		if valid {
			counts["upstream_valid"]++
			if !anyPhone {
				if other {
					counts["protected_existing_type"]++
				} else {
					counts["unexpected_miss"]++
				}
			}
		} else if newPhone {
			counts["unexpected_false_positive"]++
		}
		if fixture != nil {
			predicted := make([][]any, 0, len(matches))
			for _, match := range matches {
				predicted = append(predicted, []any{match.Start, match.End, match.Kind})
			}
			row := map[string]any{"input": input, "predicted": predicted, "upstream_valid": valid}
			if err := json.NewEncoder(fixture).Encode(row); err != nil {
				fmt.Fprintln(os.Stderr, "writing private fixtures failed")
				os.Exit(1)
			}
		}
	}
	regions := []string{}
	for region := range phonenumbers.GetSupportedRegions() {
		regions = append(regions, region)
	}
	sort.Strings(regions)
	for _, region := range regions {
		for _, kind := range []phonenumbers.PhoneNumberType{phonenumbers.FIXED_LINE, phonenumbers.MOBILE, phonenumbers.TOLL_FREE, phonenumbers.PREMIUM_RATE, phonenumbers.SHARED_COST, phonenumbers.VOIP, phonenumbers.PERSONAL_NUMBER, phonenumbers.PAGER, phonenumbers.UAN, phonenumbers.VOICEMAIL} {
			if n := phonenumbers.GetExampleNumberForTypeInRegion(region, kind); n != nil {
				check(phonenumbers.Format(n, phonenumbers.E164))
			}
		}
	}
	rng := rand.New(rand.NewSource(20261005))
	for i := 0; i < 50000; i++ {
		region := regions[rng.Intn(len(regions))]
		code := phonenumbers.GetCountryCodeForRegion(region)
		var b strings.Builder
		b.WriteByte('+')
		b.WriteString(strconv.Itoa(code))
		n := 4 + rng.Intn(14)
		for j := 0; j < n; j++ {
			b.WriteByte(byte('0' + rng.Intn(10)))
		}
		check(b.String())
	}
	result := map[string]any{"schema": 1, "source": "nyaruka/phonenumbers/v2 v2.0.14, libphonenumber metadata v9.0.40", "scope": "canonical explicit-country-code validation parity; generated rule checks, not a PII accuracy corpus", "counts": counts, "protected_type_counts": conflicts}
	if fixture != nil {
		if err := fixture.Close(); err != nil {
			panic("closing private fixtures failed")
		}
		file, err := os.Open(*fixturePath)
		if err != nil {
			panic("reading fixture digest failed")
		}
		h := sha256.New()
		if _, err := io.Copy(h, file); err != nil {
			panic("hashing fixtures failed")
		}
		file.Close()
		result["fixture_sha256"] = fmt.Sprintf("%x", h.Sum(nil))
	}
	json.NewEncoder(os.Stdout).Encode(result)
	if counts["unexpected_miss"] != 0 || counts["unexpected_false_positive"] != 0 {
		fmt.Fprintln(os.Stderr, "phone rule parity failed")
		os.Exit(1)
	}
}
