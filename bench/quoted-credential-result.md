# Quoted credential alphabet results

The extension recognizes complete quoted credential values containing printable punctuation or Unicode. Unquoted syntax and the existing length, ASCII letter/digit and byte-entropy filters remain unchanged. Escapes, interpolation syntax, whitespace and nonprinting characters remain unsupported. [The fixed protocol](quoted-credential-protocol.md) and 46-case corpus were committed at `7419a64` before baseline scoring and detector edits.

| Probe | Basic exact excerpts before → after | Trained exact excerpts before → after | Fully masked credentials, both paths | Exact restoration, both paths |
| --- | --- | --- | --- | --- |
| Quoted alphabet | 13/46 → 38/46 | 13/46 → 38/46 | 0/33 → 25/33 | 46/46 |
| Existing assignment context | 36/42 → 38/42 | 36/42 → 38/42 | 19/25 → 21/25 | 42/42 |

Both probes have zero false positives after the change. The new probe retains eight misses: dollar, backtick, escaped, whitespace-containing, short, low-entropy, letters-only and template-shaped credentials. The earlier corpus retains four misses: short, low-entropy, letters-only and unsupported prose context. These remain labelled positives. Detection is incomplete; this is a development result, not independent acceptance.

The native detector, session tests and rebuilt WASM validate complete original UTF-8 spans, decomposed Unicode restoration, strict blocking of uncertain credentials and stable marker resubmission. All native suites, the seven earlier snapshots and the other pinned corpus gates pass. A fresh single-worker round-trip fuzz run passes 32,333 executions in 91.186 seconds; this count includes skipped input and is not an accuracy estimate. The two-corpus Chromium replay records zero external requests; one trained context scan uses the existing unsupported-input fallback. Its package hashes identify a dirty development build, not a release artifact.

- [Packaged baseline](results/quality-99-v1/quoted-before.json).
- [Native baseline](results/quality-99-v1/quoted-native-before.json) and [current native snapshot](results/quoted-credential-v3.json).
- [Targeted packaged result](results/quality-99-v1/quoted-targeted-after.json).
- [Source-pinned fuzz result](results/quality-99-v1/quoted-roundtrip-fuzz.json).

The [clean twenty-corpus replay and private-preview verification](private-preview-20261005.md) are complete at `a8a6704`. All eighteen unaffected earlier corpora retain every metric, and both changed probes retain their targeted results. The exact recorded package passes all browser workflows and archive-member checks. The frozen name model and auxiliary-training protocol are unchanged.
