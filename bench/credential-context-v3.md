# Credential assignment experiment

Procedure fixed on 5 October 2026 before scoring the new cases. The documented
detector backlog calls for contextual credential detection with benign code and
configuration negatives. This trial adds a bounded local heuristic; it does not
change the frozen name model, existing corpus labels or the complete Secret
annotation scope.

## Implemented rule

Recognise credential assignments at a line boundary or after `{`, `}`, `,` or
`;`, with optional horizontal indentation and `export`, `const`, `let` or `var`.
Allow `=` or `:` and balanced single/double quotes around the key. Key names are
case-insensitive, start with an ASCII letter and have at most 64 ASCII identifier
characters. Recognise API/access/
secret/private keys, access/auth/bearer/refresh tokens, client/API secrets,
password/passwd/pwd, token and secret. Permit underscore/hyphen-delimited prefixes
such as `SERVICE_API_KEY`, but not arbitrary substrings such as `monkey` or
suffixes such as `API_KEY_NAME`. A `public` key-name component is excluded.

Read the complete quoted or unquoted value on the same line. JavaScript-style
`const`/`let`/`var` assignments require a quoted literal, avoiding variable and
function expressions. Never mask a prefix of a value containing an unsupported
character. Quotes are not part of the entity; unquoted syntax delimiters and
whitespace end the value. Preserve permitted punctuation within the value.
Equality/compound operators and quoted-literal concatenation are excluded.
Quoted values must end at the assignment boundary, not at one piece of a
computed expression. The finding is an uncertain Secret shape; best-effort
dmz masks it, while strict core processing blocks it.

Require at least 20 bytes, both an ASCII letter and digit, an alphabet limited to
ASCII letters/digits and `_ . / + = ~ -` (without spaces), and empirical Shannon
entropy of at least 3.5 bits per byte. These are fixed DMZ heuristics, not issuer
rules or evidence of randomness. There is no maximum value length in the rule.
Known provider-specific rules continue to operate independently.

[Gitleaks' generic credential rule](https://github.com/gitleaks/gitleaks/blob/master/cmd/generate/config/rules/generic.go)
uses credential-related keywords and a 3.5 entropy threshold. Its
[configuration reference](https://github.com/gitleaks/gitleaks)
describes entropy as a filter on a matched group. This is prior art for the
approach; DMZ's assignment grammar, field-name policy and length/alphabet checks
are separate choices and do not inherit a quality guarantee from Gitleaks.

## Measurement and limitations

Create the new fabricated corpus before changing the detector. Keep explicit
credential values labelled Secret even when they fail the proposed length,
entropy or alphabet guard. Such values are expected misses, not negatives.
Measure exact spans, complete identifier masking, restoration and benign
alteration in both packaged paths, then replay every earlier corpus unchanged.
No issuer requests or real credentials are used.

This heuristic cannot discover arbitrary secrets. Short, low-entropy,
letters-only, non-ASCII, multiline and escaped values may be missed. Unfamiliar
field names, free-form prose, expression syntax and other serialization formats
are outside this grammar. Conversely, a noncredential literal satisfying these
context and shape checks can be a false positive. Independent source review and
representative labels remain necessary for the 99% quality target.

The frozen corpus contains 42 excerpts: 25 labelled credential values and 17
negatives. The native result is TP 19, FP 0, FN 6 (76% recall). The six misses are
low entropy, short length, letters-only content, unsupported punctuation,
non-ASCII content and unsupported prose context. They remain labelled positives.
Strict exposure checks block 19 positive prompts and leave six exposed, while
preserving all 17 negatives. `scripts/check-credential-context.sh` pins these
counts rather than presenting the supported subset as complete coverage.

Native tests also cover multiple fields, UTF-8 offsets after emoji, known entropy
distributions, mismatched key quotes, expressions and unsupported suffixes.
Session tests require complete masking and byte-exact restoration of supported
values. WASM and packaged-browser checks measure the same behavior separately.

The packaged [before](results/quality-99-v1/credential-context-before.json) and
[after](results/quality-99-v1/credential-context-after.json) reports improve both
paths from 17/42 to 36/42 exact excerpts, and from 0/25 to 19/25 completely masked
credentials. All 42 outputs restore exactly. The six known misses remain, all
17 negatives remain negative, and every earlier corpus retains its typed quality,
masking and restoration results. Both runs report zero external requests. The
after report identifies the dirty development artifact by its complete file hash
map; it is not a release archive.

The first WASM build panicked during runtime compilation of the counted key-name
regex, despite passing native tests. Replacing that repetition with a whole-key
length check removed the observed startup failure without changing the native
snapshot. Tests cover the 64-character boundary and longer rejected keys, and
the rebuilt WASM passes the full smoke suite and packaged replay.

The later [expression probe](credential-expression-v3.md) exposes false positives
on variable references and adds explicit programming-language fence context.
Quoted literals and configuration values remain covered; four unfenced reference
errors remain visible. The original corpus and six known misses are unchanged.

The later [quoted-literal extension](quoted-credential-protocol.md) broadens only
complete quoted values to printable non-whitespace punctuation and Unicode,
retaining the length, ASCII letter/digit and entropy guards and excluding escape
and interpolation syntax. Its native result on this unchanged corpus is now
TP 21, FP 0, FN 4: punctuation and non-ASCII suffixes are covered. Short,
low-entropy, letters-only and unsupported prose cases remain misses. The
current native snapshots and exposure gate reflect those four retained misses;
earlier packaged reports above remain historical evidence.
