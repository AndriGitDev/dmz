package routing

import (
	"bytes"
	"testing"
)

// The settings writer accepts only strictly parsed documents. A harmless
// comment added by another writer must survive selective route removal, while
// all pre-existing values and unrelated bytes remain intact.
func FuzzCodexRoutingText(f *testing.F) {
	for _, seed := range []string{"", "# synthetic\n", "model='synthetic'", codexCommentedFixture, codexRichFixture, "model_provider = 'openai' # keep\r\n[features]\nunified_exec=true\r\n"} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, original []byte) {
		written, err := CodexSettings(original, codexOrigin, codexToken)
		if err != nil {
			return
		}
		edit := []byte("\n# synthetic external edit\n")
		current := append(bytes.Clone(written), edit...)
		merged, err := MergeCodexSettings(original, written, current)
		if err != nil {
			t.Fatal("accepted document could not remove an unchanged route after a comment edit")
		}
		if !bytes.HasPrefix(merged, original) || !bytes.HasSuffix(merged, edit) {
			t.Fatal("selective removal changed unrelated bytes")
		}
		oldValues, oldOK := decodeTOML(original)
		values, ok := decodeTOML(merged)
		if !oldOK || !ok || !sameTOML(values, oldValues) {
			t.Fatal("selective removal changed original values")
		}
	})
}
