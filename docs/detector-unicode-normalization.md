# Canonical Unicode detection

The 4 October change recognizes canonically decomposed text without changing the
original prompt or restoring it to a different spelling. Previously, any combining
mark caused whole-text masking in dmz and prevented the trained scanner from
running, even for ordinary decomposed Icelandic accents.

## Detection and restoration

The rule detector and trained scanner each create an NFC detection copy. Go uses
the pinned `golang.org/x/text v0.42.0` normalization iterator; the scanner uses
JavaScript NFC normalization and grapheme segmentation. Both keep a map from
normalized UTF-8 boundaries to original UTF-8 boundaries. Unchanged segments
retain their rune boundaries; changed segments expose only boundaries that map
unambiguously to the original text. Model words use the normalized spelling,
while every returned detection span addresses the original input.

This follows [Go's normalization boundary contract](https://pkg.go.dev/golang.org/x/text@v0.42.0/unicode/norm#Iter.Next)
and [JavaScript's NFC operation](https://tc39.es/ecma262/multipage/text-processing.html#sec-string.prototype.normalize).
Compatibility normalization, accent removal and visual-confusable folding are not
performed. Masking and reverse mapping still use the exact original bytes. NFC and
NFD spellings therefore receive distinct markers when their original bytes differ.

The original text remains guarded when it has invalid encoding, invisible format
characters, combining marks left after NFC, or an ambiguous mapped boundary. The
basic detector returns a Unicode review finding; dmz's best-effort policy masks
the whole text. The scanner rejects unsupported input and the existing visible
fallback warning remains. A supplied model span that splits a base character from
its combining mark is rejected. None of this establishes complete evasion coverage.

Tenant matching remains exact and case-sensitive against the original bytes. The
lookup copy is normalized where needed, then checked against the configured raw
term before a detection is accepted. The existing tenant-entry validation limits
remain in place.

The Go dependency is pinned in `go.mod`/`go.sum`. Its licence is included in the
built extension under `licenses/golang-x-text-LICENSE.txt`.

## Evidence and limits

Go and WASM tests cover original-byte bounds for decomposed names, international
email and known/heuristic addresses; distinct original forms and streamed replies
restore exactly. Unsupported sequences retain their guard. Scanner tests cover
Icelandic letters, multiple accents, Hangul composition, compatibility characters,
emoji before an entity, and malformed input.

All 5,838 frozen preprocessing fixtures pass: 5,582 general excerpts with 96,055
words and 256 original-domain excerpts with 9,056 words. Tokenization, byte offsets,
CRF features and decisions, BPE IDs and pooling match their frozen references.
The source corpora contain no text changed by NFC. These are reused development
checks; weights, thresholds, training inputs and blind packets are unchanged.

The actual trained Chromium workflow masks names in decomposed Icelandic prose,
restores its original text in the isolated frame, and passes draft-edit, reset,
concurrent-tab and scanner-failure checks. It makes no external scanner requests.

The five-corpus packaged replay improves the trained exploratory set from 23/29
to 25/29 exact excerpts and from 20/25 to 21/25 exact entities. The later probe
improves from 23/30 to 25/30 exact excerpts and from 15/20 to 16/20 exact entities.
Each has one remaining typed abstention instead of three. The benign decomposed
excerpt is no longer masked. No set loses typed accuracy, precision, recall or
complete masking coverage, and every allowed result restores exactly.

[Aggregate replay](../bench/results/quality-99-v1/system-typed-after-nfc.json) and
the preserved pre-change reports record these results. They do not establish the
99% acceptance target or representative performance on real documents.

At the JavaScript/WASM boundary, a lone UTF-16 surrogate cannot be represented
losslessly as a Go UTF-8 string. The bridge now checks the original JavaScript
value before conversion and returns a fixed error for processing requests with
such input. dmz leaves that malformed draft unchanged and unsent. Valid
surrogate pairs, including emoji, remain supported. A malformed response is
returned unchanged, retaining any markers rather than guessing a restoration.
Tenant-term validation also checks decoded JavaScript JSON values so escaped lone
surrogates cannot silently become replacement characters or reset existing maps.
This transport-integrity guard is separate from the whole-text masking policy for
valid Unicode that the detector cannot safely segment.

WASM tests cover all three processing entry points, raw and JSON-escaped malformed
terms, unchanged malformed-response pass-through and preservation of existing maps.
Actual Chromium fixtures for both chat sites confirm that malformed drafts remain
unchanged and unsent, then valid emoji text sends and restores successfully. The
existing settings, restoration isolation and network/race workflows also pass.
