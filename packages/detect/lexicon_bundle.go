//go:build dmz_lexicon

package detect

import _ "embed"

// Generated locally from approved sources; never committed.
//
//go:embed lexicon.generated.json
var bundledLexiconJSON []byte

func init() {
	var err error
	bundledLexicon, err = LoadLexicon(bundledLexiconJSON)
	if err != nil {
		panic("invalid bundled detector lexicon")
	}
	bundledLexiconJSON = nil
}
