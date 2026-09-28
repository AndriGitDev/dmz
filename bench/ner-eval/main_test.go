package main

import (
	"strings"
	"testing"
)

func TestExactPersonSpansAndSplit(t *testing.T) {
	input := `{"split":"dev","text":"Arna Jónsdóttir","entities":[{"kind":"Person","start":0,"end":17}]}`
	// UTF-8 byte boundaries, not rune counts.
	r, err := evaluate(strings.NewReader(input), "dev")
	if err != nil || r.TP != 1 || r.FP != 0 || r.FN != 0 {
		t.Fatalf("wrong exact result: %#v %v", r, err)
	}
	if _, err := evaluate(strings.NewReader(input), "test"); err == nil {
		t.Fatal("wrong split accepted")
	}
	negative := `{"split":"dev","text":"Arna Jónsdóttir","entities":[]}`
	r, err = evaluate(strings.NewReader(negative), "dev")
	if err != nil || r.FP != 1 {
		t.Fatalf("false positive not counted: %#v %v", r, err)
	}
}
