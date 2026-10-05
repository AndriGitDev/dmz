# Bearer credential regression cases

The previous Bearer rule recognized only letters, digits, underscores and hyphens,
with a minimum length of 16. Opaque values containing other allowed token
characters could be missed or only partly masked. It also recognized `Bearer`
inside a longer word and could mistake response challenge parameters for a token.

The revised rule captures dots, tildes, pluses, slashes and padding as part of the
value. An explicit `Authorization:` or `Proxy-Authorization:` line establishes
credential context: short values and trailing periods remain inside the protected
span. A `WWW-Authenticate:` or `Proxy-Authenticate:` response challenge does not
supply a credential to this rule. Header and scheme names are case-insensitive;
the original token bytes remain unchanged for restoration.

This follows the token alphabet in [RFC 6750 section 2.1](https://www.rfc-editor.org/rfc/rfc6750#section-2.1).
The detector does not validate a token against an issuer. It also protects a
contiguous token-shaped value with misplaced padding rather than exposing a
suffix because the value is malformed.

Free-form prose retains the earlier 16-character minimum and removes trailing
sentence periods. This is a heuristic, not a protocol parser: a short token or
one ending in a period outside an explicit header line may remain unsupported or
partially covered. Serialized/quoted header names are not treated as explicit
header lines by the short-token rule. General credential discovery, validity
checking and arbitrary header serialization are outside this change.

The 25 fabricated cases in `corpus/bearer-v3.jsonl` contain 15 credential spans and
10 negatives. They cover the token alphabet, short header values, line endings,
case, prose punctuation, response challenges and embedded scheme words. The
values are deliberately synthetic and were never used to authenticate. Separate
native tests include long prefixes followed by slash/dot suffixes, guarding
against partial masking. Native session and WASM tests require complete spans,
exact restoration and strict-policy rejection.

The existing public corpora and their labels are unchanged. These are selected
regression cases, not an independent secret-detection sample. Run
`scripts/check-bearer.sh` for the pinned core and exposure snapshots. Packaged
browser before/after results are recorded separately with artifact identities.

The packaged Chromium replay improves both basic and trained paths from 7/25 to
25/25 exact excerpts, TP 1 to 15, FP 4 to 0 and FN 14 to 0. Complete masking
improves from 1/15 to 15/15; all 25 outputs restore exactly before and after.
All seven earlier corpora retain identical typed-quality, masking and restoration
results. Both runs report zero external requests. The aggregate
[before](results/quality-99-v1/bearer-before.json) and
[after](results/quality-99-v1/bearer-after.json) records preserve corpus and package
hashes; no raw prompts or individual predictions are stored in those reports.
