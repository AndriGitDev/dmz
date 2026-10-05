// Generate the international-number plans consumed by the detector.
// The upstream protobuf loader runs here, never in the TinyGo/browser binary.
package main

import (
	"encoding/json"
	"fmt"
	"github.com/nyaruka/phonenumbers/v2/metadata"
	"os"
	"strconv"
)

type description struct {
	Pattern string  `json:"pattern"`
	Lengths []int32 `json:"lengths,omitempty"`
}
type plan struct {
	Region  string        `json:"region"`
	Leading string        `json:"leading,omitempty"`
	General description   `json:"general"`
	Types   []description `json:"types"`
}

func describe(d *metadata.PhoneNumberDesc) description {
	return description{d.GetNationalNumberPattern(), d.GetPossibleLength()}
}

func main() {
	if len(os.Args) != 2 {
		panic("usage: go run . OUTPUT_JSON")
	}
	collection, err := metadata.Collection()
	if err != nil {
		panic(err)
	}
	byRegion := map[string]plan{}
	for _, m := range collection.GetMetadata() {
		types := []description{}
		for _, d := range []*metadata.PhoneNumberDesc{m.GetFixedLine(), m.GetMobile(), m.GetTollFree(), m.GetPremiumRate(), m.GetSharedCost(), m.GetVoip(), m.GetPersonalNumber(), m.GetPager(), m.GetUan(), m.GetVoicemail()} {
			if d.GetNationalNumberPattern() != "" {
				types = append(types, describe(d))
			}
		}
		byRegion[strconv.Itoa(int(m.GetCountryCode()))+"/"+m.GetId()] = plan{m.GetId(), m.GetLeadingDigits(), describe(m.GetGeneralDesc()), types}
	}
	plans := map[string][]plan{}
	// Preserve upstream region priority within shared calling codes.
	for code, regions := range metadata.CountryCodeToRegion() {
		key := strconv.Itoa(code)
		for _, region := range regions {
			plans[key] = append(plans[key], byRegion[key+"/"+region])
		}
	}
	payload := struct {
		Schema  int               `json:"schema"`
		Source  string            `json:"source"`
		Version string            `json:"version"`
		Plans   map[string][]plan `json:"plans"`
	}{1, "google/libphonenumber via nyaruka/phonenumbers/v2 v2.0.14", metadata.Version, plans}
	bytes, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	bytes = append(bytes, '\n')
	if err = os.WriteFile(os.Args[1], bytes, 0644); err != nil {
		panic(err)
	}
	fmt.Printf("Generated %d calling-code plans, %d bytes, %s\n", len(plans), len(bytes), metadata.Version)
}
