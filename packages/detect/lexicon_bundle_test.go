//go:build dmz_lexicon

package detect

import "testing"

func TestBundledLexiconLoaded(t *testing.T) {
	if bundledLexicon == nil || len(bundledLexicon.names.next) == 0 || len(bundledLexicon.addresses.next) == 0 {
		t.Fatal("tagged build did not load its lexicon")
	}
}
